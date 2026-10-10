# syntax=docker/dockerfile:1

FROM python:3.14.7-slim-trixie@sha256:51dafde81dbdb6ebde285137a295cf18a47ca95234fe388a343719cb97305b3d AS deps

RUN python -m venv /opt/venv
ENV PATH="/opt/venv/bin:$PATH"

WORKDIR /app
COPY requirements.txt .
RUN --mount=type=cache,target=/root/.cache/pip python -m pip install --require-hashes -r requirements.txt && \
  python -m pip check

FROM deps AS test
COPY app.py gunicorn.conf.py ./
COPY tests ./tests

RUN gunicorn --check-config -c gunicorn.conf.py app:app \
  && python -m unittest discover -s tests

FROM python:3.14.7-slim-trixie@sha256:51dafde81dbdb6ebde285137a295cf18a47ca95234fe388a343719cb97305b3d AS runtime

RUN apt-get update \
  && apt-get install -y --no-install-recommends poppler-utils fonts-dejavu-core \
  && rm -rf /var/lib/apt/lists/*

RUN useradd --system --uid 10001 --no-create-home --shell /usr/sbin/nologin pdf2png

WORKDIR /app

COPY --from=deps /opt/venv /opt/venv
COPY app.py gunicorn.conf.py ./

ENV PATH="/opt/venv/bin:$PATH" \
  PYTHONDONTWRITEBYTECODE=1 \
  PYTHONUNBUFFERED=1

HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
  CMD ["python", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:5000/health/live', timeout=3)"]

USER 10001:10001
EXPOSE 5000

CMD ["gunicorn", "-c", "gunicorn.conf.py", "app:app"] 
