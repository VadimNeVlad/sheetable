# syntax=docker/dockerfile:1

FROM node:24.21.0-trixie-slim@sha256:173f125896c3b47ddf056734c7ea789d04595a6a08769a8f78e0df642781fb66 AS frontend-deps
WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund

FROM frontend-deps AS frontend-build
COPY frontend/ .
RUN npm run build

FROM golang:1.27.1-trixie@sha256:8f58fd67ea075142d947a60e0caa4317746a55118d312f027793d382c7741734 AS backend-deps
WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download && go mod verify

FROM backend-deps AS backend-build

COPY backend/ .
COPY --from=frontend-build /app/build /frontend/build

RUN cd api/controllers && go run github.com/GeertJohan/go.rice/rice@v1.0.3 embed-go
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
USER nonroot:nonroot

ENTRYPOINT ["/sheetable"]
