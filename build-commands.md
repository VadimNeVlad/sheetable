# Verified local build sequence

This is the non-container baseline. M3 will make the same artifact graph
reproducible inside the new multi-stage application Dockerfile.

Prerequisites:

- Node 24.21.0 and npm 11.19.0 (`frontend/.nvmrc` and `packageManager`);
- Go 1.27.1 with CGO and a C compiler (`backend/.go-version`).

From the repository root:

```bash
cd frontend
npm ci
npm run build

cd ../backend/api/controllers
go run github.com/GeertJohan/go.rice/rice@v1.0.3 embed-go

cd ../..
mkdir -p build
go mod verify
go mod tidy -diff
go test ./...
go vet ./...
go build -o build/sheetable .
```

`frontend/build` and `backend/api/controllers/rice-box.go` are generated artifacts.
The future image build must generate both from the current checkout. They are
excluded from the root Docker build context so a stale local copy cannot be reused.

`go vet ./...` is a required local gate. The malformed legacy `composers` JSON tag
was corrected during M2 so the command now completes successfully.
