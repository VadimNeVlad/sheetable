# syntax=docker/dockerfile:1

ARG PYTHON_IMAGE=python:3.14.7-alpine3.24@sha256:9e9fde4d32eedce0b661d9ab91e826b62dddf28e928c230ec55f1866cac66b01

FROM ${PYTHON_IMAGE} AS python-deps

ENV VIRTUAL_ENV=/opt/venv \
  PATH="/opt/venv/bin:${PATH}" \
  PIP_DISABLE_PIP_VERSION_CHECK=1

WORKDIR /build

RUN python -m venv "${VIRTUAL_ENV}"

COPY requirements.txt ./

RUN --mount=type=cache,id=sheetable-pip,target=/root/.cache/pip,sharing=locked \
  python -m pip install \
  --require-hashes \
  --only-binary=:all: \
  --requirement requirements.txt

FROM python-deps AS test

WORKDIR /src

COPY app.py gunicorn.conf.py ./
COPY tests ./tests

RUN python -m pip check && \
  python -m unittest discover -s tests -v && \
  gunicorn --check-config -c gunicorn.conf.py app:app

FROM ${PYTHON_IMAGE} AS runtime

# Non-embedded PDF text needs fallback fonts even when conversion returns 200.
RUN apk add --no-cache poppler-utils font-dejavu

RUN addgroup -S -g 10001 pdf2png && \
  adduser -S -D -H -u 10001 -G pdf2png -h /nonexistent -s /sbin/nologin pdf2png

ENV VIRTUAL_ENV=/opt/venv \
  PATH="/opt/venv/bin:${PATH}" \
  PYTHONDONTWRITEBYTECODE=1 \
  PYTHONUNBUFFERED=1

WORKDIR /app

COPY --from=python-deps --chown=0:0 /opt/venv /opt/venv

# Installation tools belong to the build stages, not the running PDF service.
RUN /opt/venv/bin/python -m pip uninstall --yes pip && \
  /usr/local/bin/python -m pip uninstall --yes pip && \
  rm -rf /usr/local/lib/python3.14/ensurepip

COPY --chown=0:0 --chmod=0444 \
  app.py \
  gunicorn.conf.py \
  ./

ARG APP_VERSION=dev
ARG VCS_REF=unknown

LABEL org.opencontainers.image.title="SheetAble PDF-to-PNG" \
  org.opencontainers.image.description="Internal PDF thumbnail conversion service" \
  org.opencontainers.image.source="https://github.com/VadimNeVlad/sheetable" \
  org.opencontainers.image.version="${APP_VERSION}" \
  org.opencontainers.image.revision="${VCS_REF}" \
  org.opencontainers.image.licenses="AGPL-3.0-only"

EXPOSE 5000

USER 10001:10001

STOPSIGNAL SIGTERM

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/opt/venv/bin/python", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:5000/health/live', timeout=2).read()"]

ENTRYPOINT ["/opt/venv/bin/gunicorn"]

CMD ["-c", "gunicorn.conf.py", "app:app"]
