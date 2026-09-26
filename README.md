<p align="center">
  <a href="https://github.com/SheetAble">
    <img src="docs/LogoT.png" alt="SheetAble logo" width="110" height="110">
  </a>
</p>

<h1 align="center">SheetAble</h1>

<p align="center">
  Self-hosted software for uploading, organizing, and sharing music sheets.
</p>

<p align="center">
  <a href="https://github.com/SheetAble/SheetAble"><img src="https://img.shields.io/github/stars/SheetAble/SheetAble?color=d08770&labelColor=3b4252&style=for-the-badge" alt="GitHub stars"></a>
  <a href="https://github.com/SheetAble/SheetAble/issues"><img src="https://img.shields.io/github/issues-raw/SheetAble/SheetAble?color=a3be8c&labelColor=3b4252&style=for-the-badge" alt="Open issues"></a>
  <a href="LICENSE"><img src="https://img.shields.io/static/v1?label=license&message=AGPL-3.0&color=81a1c1&labelColor=3b4252&style=for-the-badge" alt="AGPL-3.0 license"></a>
</p>

> [!IMPORTANT]
> The local Compose stack has been exercised in WSL: startup, administrator login, PDF upload, and persistence after container recreation. Development workflow and production operations are still in progress. The current configuration is for local integration, not public production deployment. Follow the [containerization roadmap](docs/containerization-roadmap.md) for evidence and remaining checks.

## About the project

SheetAble is a web application for managing a personal or shared music-sheet library. Users can upload PDF sheets, organize them by composer and tags, search the library, generate thumbnails, and manage access for additional users.

<p align="center">
  <img src="docs/SheetAbleShowcase.gif" alt="SheetAble application demonstration">
</p>

The original project and community resources are available through the [SheetAble organization](https://github.com/SheetAble) and the [upstream documentation site](https://sheetable.net/).

## Current architecture

The repository contains four runtime concerns:

```text
Browser
   |
   v
Go HTTP server :8080
   |-- REST API under /api
   |-- compiled React frontend embedded with go.rice
   |-- SQLite by default; PostgreSQL and MySQL are supported
   |-- user PDFs, thumbnails, and portraits below CONFIG_PATH
   `-- HTTP request to the PDF-to-PNG service

Python/Flask + Poppler
   `-- converts the first page of an uploaded PDF into a thumbnail
```

### Technology stack

| Area | Technology | Current responsibility |
|---|---|---|
| Frontend | React | Browser UI and API client |
| Backend | Go, Gin, GORM | API, authentication, persistence, and static frontend delivery |
| PDF processing | Python, Flask, pdf2image, Poppler | PDF thumbnail generation |
| Database | SQLite, PostgreSQL, or MySQL | Users, sheets, composers, and related metadata |
| Build and delivery | Docker, BuildKit, Docker Compose, GitHub Actions | Under active modernization |

## Repository layout

```text
.
|-- backend/       Go server, API, and models
|-- frontend/      React application
|-- pdf2png/       Python PDF conversion service
|-- docs/          Images and project/operations documentation
|-- Dockerfile     Multi-stage Go/React application build
|-- compose.yaml   Local three-service integration stack
|-- .env.example   Local configuration template; actual .env stays untracked
|-- .github/       Existing CI workflows and repository configuration
|-- AGENTS.md      Working context and engineering rules for Codex sessions
`-- README.md      Project entry point
```

## Containerization program

The target runtime consists of:

- one production `app` image containing the Go server and the React production bundle;
- one internal `pdf2png` service image;
- the official PostgreSQL image;
- internal container networking;
- explicit persistent storage for PostgreSQL and application files;
- separate development and production Compose configurations;
- non-root, least-privilege runtime settings;
- tested health, shutdown, backup, restore, upgrade, and rollback behavior;
- CI-produced `linux/amd64` images with vulnerability scanning and available SBOM/provenance; additional architectures require a deployment need.

The work is deliberately incremental. Application modernization, container construction, Compose integration, CI publication, and production hardening are separate milestones so that failures remain attributable and changes remain reviewable.

See [SheetAble: containerization and operational readiness roadmap](docs/containerization-roadmap.md) for milestones, acceptance criteria, risks, and the decision log.

## Getting started

### Local Compose stack

Requirements: Docker Engine with BuildKit and Docker Compose v2, running Linux
containers. The exercised environment is WSL. Run the commands below from the
repository root; host Node, Go, Python, and Poppler installations are not needed
for this container workflow.

For a new local checkout, copy the configuration template:

```bash
cp .env.example .env
```

Replace every `your-...` placeholder in `.env` with local values:

| Variable | Purpose |
|---|---|
| `DB_PASSWORD` | Password shared by PostgreSQL initialization and the app connection |
| `ADMIN_EMAIL` | Valid email address for the initial administrator |
| `ADMIN_PASSWORD` | Initial administrator password; use at least 12 characters |
| `API_SECRET` | Random token-signing secret; use at least 32 characters |

Existing users should keep their current `.env`. It is ignored by Git. Keep
actual credentials out of the committed template and image builds.

The same root `.env` also contains host-side Go development settings. Compose
only passes the variables declared in each service's `environment`; its app
service explicitly uses `DB_HOST=db`, the internal PDF URL, and the container
data path. Host-side localhost settings are not automatically injected into
the container.

Validate configuration without printing substituted secrets, then build and
start the stack:

```bash
docker compose config --quiet
docker compose up --build --detach
```

Open [http://localhost:8080](http://localhost:8080) and sign in with the
administrator configured in `.env`. Upload a PDF to exercise app-to-pdf2png
communication and thumbnail generation.

| Service | Role | Host access |
|---|---|---|
| `app` | Go API and embedded React bundle | `127.0.0.1:8080` |
| `db` | Official PostgreSQL 17 image | No published host port |
| `pdf2png` | Gunicorn, Flask, and Poppler | No published host port |

App startup waits for database and PDF-service healthchecks. The app has HTTP
liveness at `/health/live` and database-aware readiness at `/health/ready`;
the current app image does not yet have a Docker healthcheck. The database and
PDF service share an internal `backend` network. App also joins `egress` for
external API calls. App and PDF containers run as UID/GID 10001 with read-only
root filesystems and a writable, size-limited `/tmp` tmpfs.

The commands and functional results above were reported by the learner in
the current working checkout. A clean-checkout run has not yet been recorded;
remaining milestone checks are tracked in the roadmap.

### Data and container lifecycle

`postgres-data` stores database state; `application-data` stores uploaded PDFs,
thumbnails, and portraits under `/var/lib/sheetable`. Compose prefixes volume
names with the project name. Keep the same project name when reusing data.
These volumes provide persistence, not backups.

To apply Compose changes without rebuilding unchanged images:

```bash
docker compose up --detach
```

To replace containers while retaining their named volumes:

```bash
docker compose up --detach --force-recreate
```

The learner exercised recreation and confirmed that the saved upload remained
visible. After applying internal networks and runtime restrictions, another
PDF upload succeeded.

To stop and remove the stack's containers and networks while retaining named
volumes:

```bash
docker compose down
```

**Destructive reset:** `docker compose down --volumes` also removes the stack's
named volumes, including PostgreSQL data and uploaded files. Use it only when
you deliberately want to discard that local data. This reset is not required
for ordinary updates.

PostgreSQL initialization variables apply to an empty database data directory.
The app creates an administrator only when its users table is empty. Changing
passwords in `.env` does not rotate existing database or administrator
credentials.

### Troubleshooting

Inspect status and the last service logs when startup or a request fails:

```bash
docker compose ps
docker compose logs --tail 50 app db pdf2png
```

If a required variable is missing, fill it in `.env`. If port 8080 is occupied,
stop the process using it or change the published app port and `SERVER_URL`
together. For image-build failures, inspect the failed build step before
changing dependencies or discarding cache.

### Development workflow

M6 introduces a hybrid workflow: PostgreSQL and pdf2png run in Compose, while
Go and React run on the host or in the IDE. `compose.dev.yaml` exposes
dependency ports on localhost and uses the separate `dev-postgres-data` volume.
The learner reported healthy development dependency containers and successful
host Go startup with a readiness check through curl. React startup and the
complete development upload workflow remain to be exercised.

After stopping the integration stack with `docker compose down`, start the
development dependencies from the repository root:

```bash
docker compose -f compose.yaml -f compose.dev.yaml up --detach --wait db pdf2png
```

Keep one `.env` at the repository root; no `backend/.env` copy is needed. Add
the non-secret host settings from `.env.example` to an existing `.env`, keeping
the current credentials. Host Go uses `127.0.0.1:5432` for PostgreSQL,
`http://127.0.0.1:5000/createthumbnail` for PDF conversion, `DEV=true` for CORS,
and `CONFIG_PATH=../.data/dev` for files paired with the development database.
`SERVER_URL=http://localhost:3000` is the planned React dev-server URL.

With Go and a C compiler installed in WSL, run the host backend:

```bash
cd backend
ENV_FILE=../.env go run .
```

`ENV_FILE` selects the dotenv file. An explicitly selected missing or invalid
file stops startup rather than silently using defaults. Paths are relative
to the process working directory, including `CONFIG_PATH`. Process environment
variables override dotenv values; supported `_FILE` secrets override both.
Without `ENV_FILE`, the existing optional `.env` in the working directory
remains the default. The loader is covered by Go tests; host startup and readiness
were reported successful by the learner. Next is React host setup and a UI upload
smoke test.

### Upstream references

Historical upstream instructions remain available for reference:

- [Installation](https://sheetable.net/docs/Installation/installation/)
- [Development and contributions](https://sheetable.net/docs/development/)

These external instructions describe the upstream release and may not match the toolchain or containerization work in this repository.

## Configuration and security notice

The application currently includes development-oriented defaults such as the administrator password and JWT secret. Do not expose an unreviewed instance to an untrusted network and do not reuse those defaults in production.

Real secrets must not be committed to Git, added to Docker build arguments, or copied into image layers. The app already rejects unsafe production configuration and supports file-backed secrets. The current Compose file uses local `.env` interpolation with `APP_ENV=development`; production secret delivery, restricted database privileges, TLS, backups, and release procedures remain roadmap work.

## Contributing

Before making infrastructure or container changes:

1. Read [AGENTS.md](AGENTS.md) for the repository working agreement.
2. Read the [containerization roadmap](docs/containerization-roadmap.md).
3. Keep changes small, reviewable, and limited to one coherent milestone or subtask.
4. Include the build, test, or runtime evidence used to satisfy the relevant acceptance criteria.
5. Update the roadmap status or decision log when a material milestone or architectural decision changes.

General upstream contribution guidance is available in [CONTRIBUTING.md](CONTRIBUTING.md). Bugs and feature requests for the original project can be reported through the [upstream issue tracker](https://github.com/SheetAble/SheetAble/issues).

## License

SheetAble is distributed under the GNU Affero General Public License v3.0. See [LICENSE](LICENSE) for the complete license text.

## Acknowledgements

- [SheetAble maintainers and contributors](https://github.com/SheetAble/SheetAble/graphs/contributors)
- [Open Opus API](https://openopus.org/) for classical-music metadata
