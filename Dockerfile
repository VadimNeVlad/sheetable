# syntax=docker/dockerfile:1

FROM node:24.21.0-bookworm-slim@sha256:0e0ff40c39bc087845bfb27465a0df4ea419520094bc35842ff83dd8cbe6f9b6 AS frontend-deps

WORKDIR /src/frontend

COPY frontend/package.json frontend/package-lock.json ./

RUN --mount=type=cache,id=sheetable-npm,target=/root/.npm,sharing=locked \
  npm ci

FROM frontend-deps AS frontend-build

COPY frontend/ ./

RUN --mount=type=cache,id=sheetable-frontend-build,target=/src/frontend/node_modules/.cache,sharing=locked \
  npm run build

FROM golang:1.27.1-bookworm@sha256:69a7b9788769bec032d238959b61854e9ae87f57be9029ec04e9885fabf99195 AS go-deps

WORKDIR /src/backend

COPY backend/go.mod backend/go.sum ./

RUN --mount=type=cache,id=sheetable-go-mod,target=/go/pkg/mod,sharing=locked \
  go mod download && go mod verify

FROM go-deps AS app-build-base

COPY backend/ ./

COPY --from=frontend-build /src/frontend/build /src/frontend/build

RUN --mount=type=cache,id=sheetable-go-mod,target=/go/pkg/mod,sharing=locked \
  --mount=type=cache,id=sheetable-go-build,target=/root/.cache/go-build,sharing=locked \
  cd api/controllers && \
  go run github.com/GeertJohan/go.rice/rice@v1.0.3 embed-go && \
  test -s rice-box.go

FROM app-build-base AS test

RUN --mount=type=cache,id=sheetable-go-mod,target=/go/pkg/mod,sharing=locked \
  --mount=type=cache,id=sheetable-go-build,target=/root/.cache/go-build,sharing=locked \
  go test -count=1 ./... && \
  go vet ./...

FROM app-build-base AS app-build

RUN --mount=type=cache,id=sheetable-go-mod,target=/go/pkg/mod,sharing=locked \
  --mount=type=cache,id=sheetable-go-build,target=/root/.cache/go-build,sharing=locked \
  mkdir -p /out && \
  CGO_ENABLED=1 go build \
  -trimpath \
  -ldflags="-s -w" \
  -o /out/sheetable \
  .

FROM debian:bookworm-slim@sha256:3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251 AS runtime

RUN apt-get update && \
  apt-get install --yes --no-install-recommends ca-certificates && \
  rm -rf /var/lib/apt/lists/* && \
  groupadd --gid 10001 sheetable && \
  useradd \
  --uid 10001 \
  --gid 10001 \
  --no-create-home \
  --home-dir /nonexistent \
  --shell /usr/sbin/nologin \
  sheetable && \
  install -d -o sheetable -g sheetable -m 0750 /var/lib/sheetable

WORKDIR /app

COPY --from=app-build --chown=0:0 --chmod=0555 \
  /out/sheetable \
  /app/sheetable

ARG APP_VERSION=dev
ARG VCS_REF=unknown

LABEL org.opencontainers.image.title="SheetAble" \
  org.opencontainers.image.description="SheetAble Go API with embedded React frontend" \
  org.opencontainers.image.source="https://github.com/VadimNeVlad/sheetable" \
  org.opencontainers.image.version="${APP_VERSION}" \
  org.opencontainers.image.revision="${VCS_REF}" \
  org.opencontainers.image.licenses="AGPL-3.0-only"

ENV CONFIG_PATH=/var/lib/sheetable \
  SERVER_PORT=8080

EXPOSE 8080

USER 10001:10001

STOPSIGNAL SIGTERM

ENTRYPOINT ["/app/sheetable"]
