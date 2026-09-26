# syntax=docker/dockerfile:1

ARG PYTHON_IMAGE=python:3.14.7-slim-bookworm@sha256:82bc3c539b8813ada9d68c63b40158fa002f7f33de9bf3312a3dfdc0620dff56

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

RUN --mount=type=cache,id=sheetable-pdf2png-apt-cache,target=/var/cache/apt,sharing=locked \
  --mount=type=cache,id=sheetable-pdf2png-apt-lists,target=/var/lib/apt/lists,sharing=locked \
  apt-get update && \
  apt-get install --yes --no-install-recommends \
  poppler-utils

RUN groupadd --gid 10001 pdf2png && \
  useradd \
  --uid 10001 \
  --gid 10001 \
  --no-create-home \
  --home-dir /nonexistent \
  --shell /usr/sbin/nologin \
  pdf2png

ENV VIRTUAL_ENV=/opt/venv \
  PATH="/opt/venv/bin:${PATH}" \
  PYTHONDONTWRITEBYTECODE=1 \
  PYTHONUNBUFFERED=1

WORKDIR /app

COPY --from=python-deps --chown=0:0 /opt/venv /opt/venv

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