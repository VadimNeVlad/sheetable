# syntax=docker/dockerfile:1

FROM node:24.21.0-trixie-slim@sha256:173f125896c3b47ddf056734c7ea789d04595a6a08769a8f78e0df642781fb66 AS frontend-deps
WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund

FROM frontend-deps AS frontend-build
COPY frontend/ .
RUN npm run build

FROM golang:1.27.2-trixie@sha256:e58d6f83b3416618d8bcac2b3dde1b7f7e3c4a77d25e88637f8bbae81536c48d AS backend-deps
WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download && go mod verify

FROM backend-deps AS backend-build

COPY backend/ .
COPY --from=frontend-build /app/build /frontend/build

WORKDIR /app/api/controllers
RUN go run github.com/GeertJohan/go.rice/rice@v1.0.3 embed-go

WORKDIR /app
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=1 go build -o /sheetable

RUN install -d -m 0750 /data

FROM backend-build AS test
RUN --mount=type=cache,target=/root/.cache/go-build go test ./... && go vet ./...

FROM gcr.io/distroless/base-debian13:nonroot@sha256:a0d70d6a97cd697d9362bc2aae4a6560dd65817e365d0043b07325a97975dc91 AS runtime
WORKDIR /app

COPY --from=backend-build /sheetable /sheetable
COPY --from=backend-build --chown=nonroot:nonroot /data /var/lib/sheetable
ENV CONFIG_PATH=/var/lib/sheetable

EXPOSE 8080

# non root user in distroless
USER 65532:65532 

ENTRYPOINT ["/sheetable"]
