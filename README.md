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
> The container build, development environment, and deployment workflow in this repository are currently being modernized. The existing Dockerfiles and historical external installation instructions must not yet be treated as a verified production deployment path. Follow the [containerization roadmap](docs/containerization-roadmap.md) for current scope and progress.

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
|-- backend/       Go server, API, models, and the existing backend Dockerfile
|-- frontend/      React application
|-- pdf2png/       Python PDF conversion service and its existing Dockerfile
|-- docs/          Images and project/operations documentation
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
- CI-produced multi-platform images with vulnerability scanning, SBOM, and provenance.

The work is deliberately incremental. Application modernization, container construction, Compose integration, CI publication, and production hardening are separate milestones so that failures remain attributable and changes remain reviewable.

See [SheetAble: containerization and operational readiness roadmap](docs/containerization-roadmap.md) for milestones, acceptance criteria, risks, and the decision log.

## Getting started

### Current status

A new supported quick-start workflow is not documented yet because the current application and existing images are still being baselined. Commands will be added here only after they have been executed successfully from a clean checkout.

The next delivery milestone is `M0 — Baseline and discovery`, which verifies:

- Docker Engine, Compose, and Buildx;
- the existing Go tests and build;
- the current frontend dependency installation and build;
- the Python service behavior;
- the existing container images;
- health, login, upload, thumbnail, and persistence smoke-test paths.

Historical upstream instructions remain available for reference:

- [Installation](https://sheetable.net/docs/Installation/installation/)
- [Development and contributions](https://sheetable.net/docs/development/)

These external instructions describe the upstream release and may not match the toolchain or containerization work in this repository.

## Configuration and security notice

The application currently includes development-oriented defaults such as the administrator password and JWT secret. Do not expose an unreviewed instance to an untrusted network and do not reuse those defaults in production.

Real secrets must not be committed to Git, added to Docker build arguments, or copied into image layers. The roadmap includes production configuration validation and a managed secret-delivery strategy before the deployment workflow is declared ready.

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
