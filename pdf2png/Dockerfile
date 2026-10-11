# syntax=docker/dockerfile:1

# deps and runtime must share one interpreter: the venv is bound to it.
ARG PYTHON_IMAGE=python:3.14.8-slim-trixie@sha256:a2b82f3c48559aa0a8446d9af49826b6e2b2016f4cd2afabfe6013ec53729170

FROM ${PYTHON_IMAGE} AS deps

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

FROM ${PYTHON_IMAGE} AS runtime

# upgrade pulls Debian security fixes published after the base image was built.
RUN apt-get update \
  && apt-get upgrade -y \
  && apt-get install -y --no-install-recommends poppler-utils fonts-dejavu-core \
  && rm -rf /var/lib/apt/lists/*

RUN useradd --system --uid 10001 --no-create-home --shell /usr/sbin/nologin pdf2png

WORKDIR /app

COPY --from=deps /opt/venv /opt/venv

# Packages are already installed; pip and its vendored libraries are only attack surface here.
RUN /opt/venv/bin/python -m pip uninstall --yes pip \
  && python -m pip uninstall --yes pip \
  && rm -rf /usr/local/lib/python3.*/ensurepip

COPY app.py gunicorn.conf.py ./

ENV PATH="/opt/venv/bin:$PATH" \
  PYTHONDONTWRITEBYTECODE=1 \
  PYTHONUNBUFFERED=1

HEALTHCHECK --interval=10s --timeout=5s --start-period=5s --retries=3 \
  CMD ["python", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:5000/health/live', timeout=3)"]

USER 10001:10001
EXPOSE 5000

CMD ["gunicorn", "-c", "gunicorn.conf.py", "app:app"] 
