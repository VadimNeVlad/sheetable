# SheetAble PDF-to-PNG service

This internal service renders the first page of an uploaded PDF as a 380×535
PNG thumbnail. It does not require persistent storage: every request uses an
isolated temporary directory that is removed before the response completes.

The supported container uses digest-pinned official Python 3.14.7 on Alpine
3.24, Poppler and DejaVu fonts. Python dependencies install from the existing
hash lock as binary wheels. DejaVu provides fallback rendering for PDFs that
do not embed fonts; removing it can produce successful but text-free thumbnails.
The runtime runs as UID/GID 10001 and needs writable temporary storage only.

## HTTP contract

- `GET /health/live` returns HTTP 200 while the process can serve requests.
- `POST /createthumbnail` accepts multipart fields `file` and `name` and
  returns `image/png` on success.
- Invalid or oversized input returns a controlled JSON error. Conversion
  timeout returns HTTP 504, and unavailable Poppler returns HTTP 503.

The `name` field is retained for compatibility with the Go client but is never
used as a filesystem path.

## Runtime configuration

| Variable | Default | Purpose |
|---|---:|---|
| `PDF2PNG_MAX_REQUEST_BYTES` | `20971520` | Maximum complete HTTP request size |
| `PDF2PNG_CONVERSION_TIMEOUT_SECONDS` | `25` | Poppler conversion timeout |
| `WEB_CONCURRENCY` | `2` | Gunicorn worker processes |
| `GUNICORN_THREADS` | `2` | Threads per Gunicorn worker |
| `GUNICORN_TIMEOUT_SECONDS` | `30` | Hard worker request timeout |
| `GUNICORN_GRACEFUL_TIMEOUT_SECONDS` | `10` | Worker shutdown grace period |

`GUNICORN_TIMEOUT_SECONDS` must remain greater than
`PDF2PNG_CONVERSION_TIMEOUT_SECONDS` so the application can return its
controlled timeout response before Gunicorn terminates the worker.

## Local verification

Install the hash-locked dependencies, then run:

```bash
python -m pip install --require-hashes -r requirements.txt
python -m pip check
python -m unittest discover -s tests -v
gunicorn --check-config -c gunicorn.conf.py app:app
```

The production process is:

```bash
gunicorn -c gunicorn.conf.py app:app
```
