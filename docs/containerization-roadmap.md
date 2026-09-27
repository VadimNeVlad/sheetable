# SheetAble: containerization and operational readiness roadmap

## Document purpose

This is the working plan for containerizing SheetAble and operating it on one Linux server. It also supports learning Docker through small changes that solve concrete application problems.

The plan deliberately separates discovery, application prerequisites, image construction, local orchestration, CI/CD, and production hardening. Work should be delivered in small reviewable changes, with every completed milestone backed by executable verification.

## Status

| Field | Value |
|---|---|
| Overall status | In progress |
| Current milestone | M7 in progress — local Trivy and native-advisory checks pass; three native CVEs have a scoped PDF-path assessment expiring 2026-10-27; GitHub verification and remaining M4/M5 checks stay open |
| Initial delivery target | Docker Compose on a single Linux host |
| Development platform | Windows/WSL 2 with Docker Desktop, plus Linux compatibility |
| Runtime platforms | `linux/amd64`; add `linux/arm64` only for a deployment requirement |
| Primary database | PostgreSQL |
| Last updated | 2026-09-27 |

Status values used in this document: `Planned`, `In progress`, `Blocked`, `Done`.

## Practical route

Use these four steps as the main route. The numbered milestones below provide acceptance criteria; the decision and progress logs preserve the work already verified.

| Step | Milestones | Working result |
|---|---|---|
| 1. Build the images | M0-M4 | One Go/React image and one PDF service image, with locked dependencies, non-root processes, and verified startup/shutdown |
| 2. Run the stack and preserve data | M5 | Compose runs app, PDF service, and official PostgreSQL; uploads and database data survive container replacement |
| 3. Develop and release safely | M6-M7 | A practical local workflow, PR checks, and tested images published to a registry with traceable digests |
| 4. Operate on one server | M8 | HTTPS, runtime secrets, controlled updates, working backup/restore, and documented host maintenance |

**Current position:** M0-M3 are complete. M4 application checks passed, its Dockerfile is written, and the learner has verified image builds, container health, real conversion, missing-input handling, and exit code 0 after stop in WSL. Remaining runtime checks are listed in M4 and will accompany Compose integration in M5.

Steps 1-2 establish the containerized integration baseline. Steps 3-4 prepare it for routine team use and real production data. M9 is an optional backlog and is not required to complete this route.

### How to use the plan

- Take one small block at a time: explain the problem, implement the change, run its checks, and record the result.
- Keep non-root execution, locked dependencies, health, graceful shutdown, runtime secrets, and persistent data in the baseline.
- Add performance tuning and additional infrastructure when a measured problem requires them.
- Keep operating instructions short and executable. One maintainer can own registry, host, secrets, and backups; separate teams or approval systems are not required.

Here, reproducible builds mean controlled source, locked application dependencies, pinned base images, and a documented build process. They do not yet mean byte-identical rebuilds: packages installed from Debian repositories can change. Deploy the exact tested image digest, maintain base-image updates, and defer repository snapshots or stronger reproducibility controls until required.

## Scope and assumptions

### In scope

- Container build definitions for the Go/React application and `pdf2png`.
- PostgreSQL integration using an official upstream image.
- Development, integration, and single-host production Compose configurations.
- Required application changes for container configuration, service discovery, health, graceful shutdown, and persistence.
- Dependency locking and build-cache optimization.
- CI validation, image publishing, vulnerability scanning, and automatically available build metadata.
- Runtime hardening, data lifecycle, backup/restore, upgrade, rollback, and operational documentation.
- Deployment of the tested image digest without rebuilding it for production.
- Docker-host security, lifecycle, capacity, logging, and recovery requirements for the initial single-host deployment.
- Open-source and third-party license compliance for source and container-image distribution.

### Initial assumptions

- The first production deployment target is one Linux Docker host.
- TLS termination will be added after the internal application stack is stable.
- PostgreSQL runs in Compose for development and the initial self-hosted production baseline. A managed database may replace it later without changing the application image.
- User-uploaded PDFs and generated images use a persistent filesystem initially. Object storage is a future scalability decision.
- The React production bundle continues to be served by the Go process.

### Decisions before release and deployment

Record these choices when reaching M7 or M8. A short note naming the choice and maintainer is sufficient for the initial single-host service.

| Decision/input | Needed by | Current planning assumption |
|---|---|---|
| Registry and image retention | M7 | GHCR: vadimnevlad/sheetable-app and vadimnevlad/sheetable-pdf2png; preserve current and rollback releases |
| Source and license notices | M7 | Keep the released source revision and required notices discoverable |
| Host, storage, access, and updates | M8 | One supported Linux server with a named maintainer |
| Domain and HTTPS | M8 | Reverse proxy with automatic certificate renewal |
| Acceptable downtime | M8 | One server; document maintenance and outage expectations |
| Backups and recovery | M8 | Set acceptable data loss (RPO), recovery time (RTO), retention, encryption, and an off-host destination |
| Runtime secrets | M8 | Choose restricted storage, generation, and rotation procedures |

### Explicit non-goals for the initial delivery

- Kubernetes, Helm, Terraform, or a service mesh.
- A separate production frontend container without a measured need.
- A custom PostgreSQL image without required extensions or configuration.
- Horizontal application scaling while user files remain on a single local filesystem.
- A full observability platform before stable health, logging, and runtime contracts exist.
- Unrelated application rewrites hidden inside infrastructure changes.

## Starting baseline and findings

This section records the state discovered in M0. Resolved findings remain here as context; current implementation status is recorded in the milestones and progress log.

### Application topology

```text
Browser
   |
   v
Go server :8080
   |-- /api/*
   |-- embedded React assets via go.rice
   |-- SQLite by default; PostgreSQL/MySQL supported
   |-- filesystem data below CONFIG_PATH
   `-- hard-coded external request to pdf2png.sheetable.net

Repository also contains a local Python/Flask + Poppler pdf2png service.
```

### Material findings

1. The retired GitHub Actions Docker workflow built with `context: ./backend`; it could not construct the frontend bundle.
2. `rice-box.go` contains generated frontend content, creating a stale-artifact risk.
3. The repository initially lacked `frontend/package-lock.json`; the frontend dependency graph is now locked and validated with Node 24.21.0 and npm 11.19.0.
4. The retired backend Dockerfile copied the source redundantly, invalidated useful cache, ran as root, used mutable base tags, and documented the wrong port.
5. The retired PDF Dockerfile used an obsolete Python/Debian base, unpinned packages, the Flask development server, and root execution.
6. The backend hard-codes a public PDF endpoint and uses insecure TLS behavior.
7. Database startup failure terminates the process immediately. Compose ordering alone cannot make the application resilient.
8. The server does not perform graceful shutdown on `SIGTERM`.
9. Migrations and administrator seeding execute during ordinary application startup.
10. Default API secret and administrator credentials are not acceptable for production.
11. PostgreSQL data and application files need separate explicit persistence and backup policies.
12. CI action versions and image publication behavior require modernization.
13. Major application dependencies are legacy and require a separate tested modernization track.
14. Runtime behavior depends on external services including SMTP, Open Opus, GitHub Releases, remote images, and Google Fonts; their failure modes and privacy/availability implications are not documented.
15. The initial single-host target has no documented daemon-access policy, host patching policy, log rotation, disk-capacity monitoring, or disaster-recovery objectives.
16. The repository is AGPL-3.0 licensed, but the release process does not yet document source-availability and third-party license obligations for a modified network service and distributed images.

## Target architecture

```text
                            public network
                                  |
                                  v
                     reverse proxy / TLS (later)
                                  |
                                  v
                         +-----------------+
                         | app             |
                         | Go API + React  |
                         +---+---------+---+
                             |         |
               internal net |         | internal net
                             v         v
                    +----------+   +-----------+
                    | postgres |   | pdf2png   |
                    +----+-----+   +-----------+
                         |
                         v
                   postgres-data

app also mounts application-data for PDFs, thumbnails, and portraits.
Only the public entrypoint is published on the host.
```

### Image boundaries

| Image/service | Responsibility | Build strategy |
|---|---|---|
| `app` | Go API and compiled React static assets | Multi-stage Node + Go + minimal runtime |
| `pdf2png` | PDF first-page rendering | Pinned Python dependencies, Poppler, production WSGI server |
| `db` | PostgreSQL | Official image, no local Dockerfile by default |

The unsupported legacy Dockerfiles and image workflow were retired after baseline capture. Their content remains in Git history and their findings are recorded above. The replacement images use the verified build and runtime requirements.

## Delivery strategy

Changes should normally map to one milestone or a coherent subset of a milestone. Avoid a single pull request that combines dependency upgrades, application behavior changes, Docker builds, Compose, and CI publication.

Each milestone is complete only when its acceptance criteria have been executed and evidence is available in the pull request or delivery notes.

Checks and documentation accompany each change. M6 automates the local verification already established in earlier milestones:

- M0 establishes the first repeatable validation commands and records baseline evidence.
- Until M6, milestone verification is executed with documented local commands and recorded in delivery notes.
- M6 automates the stable language, image-build, and Compose smoke checks used for pull requests.
- M7 adds release publication, vulnerability policy, registry caching, and immutable release metadata; it does not introduce testing for the first time.
- Documentation and the progress/decision logs are updated in the same change that alters supported behavior.
- Legacy dependency upgrades are delivered as isolated application changes, but production release remains blocked by unresolved policy-level vulnerabilities or unsupported production runtimes.

### Program success measures

M0 records the non-image baseline and unavailable-tool limitations. Image-specific measurements begin when the replacement images become executable in M3 and M4; Compose and host measurements follow in M5 and M8. Track the small set of signals that directly affect release and recovery:

- final image sizes;
- container startup-to-ready time;
- vulnerability counts by policy severity and exception expiry;
- release and post-deployment smoke-test results;
- backup freshness and successful restore duration;
- rollback success and duration.

Build timings, detailed layer analysis, edit-to-feedback latency, and CPU/memory/PID benchmarks are collected only when performance, cost, or capacity becomes a concrete concern.

## Milestones

### M0 — Baseline and discovery

**Status:** Done

**Goal:** Establish a known-good behavioral and build baseline before changing the runtime model.

**Evidence:** [`docs/m0-baseline.md`](m0-baseline.md)

**Deliverables:**

- Record Docker Engine, Compose, Buildx, host, and architecture information.
- Run the existing Go tests and record failures.
- Determine a compatible Node version and reproduce the current frontend build.
- Run or characterize the current Python service.
- Record historical Docker build and workflow findings from Git history.
- Document current environment variables and persistent paths.
- Inventory external runtime dependencies, DNS destinations, credentials, timeouts, and expected degraded behavior, including SMTP, Open Opus, GitHub Releases, remote image hosts, and remote fonts.
- Record the current Docker-host assumptions: Engine installation, daemon access, firewall exposure, logging driver, data root, available disk, and startup behavior.
- Record applicable project and dependency licenses and identify where legal review is required rather than making an undocumented compliance assumption.
- Add a minimal smoke-test plan for health, login, upload, thumbnail generation, and data persistence.
- Capture architecture decisions that materially constrain subsequent work.

**Acceptance criteria:**

- Another engineer can reproduce the recorded baseline.
- Existing failures are distinguished from regressions introduced later.
- The current frontend-to-backend artifact flow is understood.
- Persistent and temporary data paths are enumerated.
- External-service and host-level dependencies have named owners or an explicit follow-up item.
- Language-level build/test results, host facts, historical container findings, and unavailable-tool limitations are recorded. Image build, startup, resource, size, and security baselines are explicitly deferred to the first executable replacement images in M3 and M4.

### M1 — Reproducible dependencies and repository hygiene

**Status:** Done

**Goal:** Make dependency restoration deterministic and prepare intentional build contexts for the new image definitions without introducing a production Dockerfile yet.

**Deliverables:**

- Create and validate `frontend/package-lock.json`.
- Pin the PDF service dependency set using an auditable lock strategy.
- Confirm `go.mod` and `go.sum` consistency.
- Add root and service-specific `.dockerignore` rules where appropriate.
- Define supported builder versions for Node, Go, and Python.
- Ensure generated artifacts are either produced by the build or clearly validated as current.
- Create a dependency-modernization backlog with owner, severity, runtime-support status, and release-blocking criteria rather than upgrading unrelated packages silently.
- Define automated update coverage and review policy for language dependencies, GitHub Actions, and Docker base images.

**Acceptance criteria:**

- Repeated clean dependency installs resolve identical versions.
- Docker build contexts exclude VCS data, local dependencies, secrets, build output, and irrelevant documentation.
- A frontend source change cannot accidentally reuse a stale embedded bundle.
- Production builders and runtimes are within their supported lifecycle or have a documented, time-bounded exception.

### M2 — Application container-readiness prerequisites

**Status:** Done

**Progress:** `PDF2PNG_URL` now accepts local or service-DNS endpoints. The PDF
client uses normal TLS verification, a bounded timeout, checked HTTP/content-type
responses, error propagation, and atomic thumbnail publication. Configuration,
success, failure, invalid-URL, and TLS-verification paths have automated tests.
Database startup now has bounded exponential retry; liveness and database-aware
readiness are separate; `SIGTERM`/`SIGINT` trigger a ten-second graceful shutdown
and database close. Production configuration rejects unsafe defaults, sensitive
values support `_FILE`, and the application data root is created explicitly.
Runtime, migration, upgrade, rollback, and single-replica contracts are recorded
in [`docs/runtime-contracts.md`](runtime-contracts.md).

**Goal:** Remove application behaviors that prevent reliable container operation before any production image definition is introduced.

**Deliverables:**

- Add a configurable `PDF2PNG_URL` that accepts service-DNS URLs; actual Compose DNS wiring is introduced and exercised in M5 rather than hard-coded in application code.
- Remove insecure TLS bypass from the PDF request path.
- Add bounded HTTP timeouts and actionable error handling.
- Define retry, backoff, and degraded-mode behavior for required and optional outbound dependencies; retries must be bounded and must not amplify an outage.
- Add database connection retry with bounded backoff.
- Implement graceful HTTP and database shutdown on `SIGTERM`/`SIGINT`.
- Separate liveness from readiness; readiness must verify required dependencies.
- Validate production configuration and reject unsafe missing secrets.
- Define an explicit application data root and create required directories safely.
- Decide how migrations and initial administrator creation are executed without unsafe replica races.
- Define backward-compatible migration and rollback expectations before automated deployment is introduced.
- Add support for file-backed secrets if Compose secrets are adopted.

**Acceptance criteria:**

- Configuration tests demonstrate that the application accepts service-DNS URLs without requiring a public hard-coded endpoint.
- Automated process/integration tests demonstrate that temporary database unavailability uses bounded retry rather than an uncontrolled restart loop.
- Process-level signal tests demonstrate graceful `SIGTERM`/`SIGINT` handling; `docker stop` verification follows when the `app` image exists in M3.
- Health endpoints have documented semantics and automated tests.
- Production startup cannot silently use repository default credentials.
- Optional external-service failures do not make unrelated read-only application paths unavailable, and required dependency failures are observable.

### M3 — Production application image

**Status:** Done

**Evidence:** The new root multi-stage Dockerfile restores locked Node and Go
dependencies with named BuildKit caches, builds React, regenerates `go.rice`
assets, exposes an explicit test target, compiles the CGO backend, and copies
only the resulting binary into a pinned Debian Bookworm slim runtime. The
runtime is configured for UID/GID `10001`, a root-owned read-only executable,
an explicit writable data root, OCI metadata, port 8080, and direct `SIGTERM`
delivery. On `linux/amd64`, the image served liveness, readiness, and the React
entrypoint with HTTP 200 and exited 0 after `docker stop` without an OOM kill.

**Goal:** Create from scratch one minimal, repeatable application image containing the current React bundle and Go server.

**Deliverables:**

- Create the root multi-stage Dockerfile with named dependency, build, test, and runtime targets.
- Build the frontend with `npm ci`.
- Generate or replace `go.rice` assets during the image build.
- Restore Go modules before copying frequently changing source files.
- Use BuildKit cache mounts for npm and Go caches.
- Run tests in an explicit build target.
- Copy only runtime artifacts into the final stage.
- Use an explicit non-root UID/GID.
- Add OCI image metadata and document port 8080.
- Preserve CA certificates and other runtime data explicitly required for outbound HTTPS; do not add general-purpose debugging tools to satisfy healthchecks.
- Select a small runtime compatible with the actual CGO requirements; do not choose Alpine, scratch, or distroless without compatibility evidence.

**Acceptance criteria:**

- A clean build creates an image without pre-generated local frontend output.
- Rebuilding after a small source change reuses dependency layers and caches.
- The runtime image contains no Node runtime, Go toolchain, source tree, or package-manager cache.
- The process runs as non-root and can write only to declared paths.
- The image serves both React and the API and passes the smoke test.
- `docker stop` results in graceful application and database shutdown within the configured timeout.
- The initially supported `linux/amd64` image is exercised on its target architecture. Any additional platform must be built and exercised on that architecture or an approved equivalent before it is claimed as supported or released.

### M4 — Production PDF service image

**Status:** In progress

**Progress:** The service now uses request-scoped temporary storage, validates
multipart input and request size, bounds Poppler conversion time, returns
controlled API errors, exposes liveness, and runs through a configurable
Gunicorn contract. Gunicorn 26.2.0 is hash-locked. On Python 3.14.7, a clean
hash-verified install, dependency check, eight unit tests, Gunicorn config
validation, real PDF-to-PNG smoke test, temporary-file cleanup, and graceful
`SIGTERM` shutdown all passed. `pdf2png/Dockerfile` is now written. Build and
runtime smoke checks were then performed by the learner in WSL: the `test` and
`runtime` targets built successfully (runtime used cache), health responded,
a real PDF returned a 380 x 535 PNG, missing input returned the expected error
without breaking health, and `docker stop --timeout 15` was followed by exit
code 0. These are learner-reported results, not checks rerun by the mentor.
The learner subsequently reported another successful UI PDF upload after
app/PDF read-only, tmpfs, capability, and privilege restrictions were applied
through Compose. Effective UID/GID, container temporary-file cleanup, and
cancellation behavior have not been explicitly checked in the running stack.
M4 remains in progress.

**Goal:** Build and verify the internal PDF conversion image.

**Deliverables:**

- Use a supported pinned Python base and restore locked dependencies separately from application source.
- Install Poppler with minimal OS packages; keep package-manager caches out of image layers. APT cache mounts are optional.
- Run Gunicorn as non-root, with health, stdout/stderr logging, and explicit conversion/worker/shutdown timeouts.
- Keep request-size validation and request-scoped temporary files with cleanup on success and error; exercise cancellation during container verification.

**Acceptance criteria:**

- The service image exposes only its documented application port and does not require public internet discovery; Compose network isolation and service-DNS integration are verified in M5.
- Malformed input returns a controlled client error rather than terminating a worker.
- A real PDF request returns the expected PNG from Poppler inside the container.
- Temporary files do not accumulate across requests.
- The container stops gracefully and passes its healthcheck as non-root.

### M5 — Compose integration and persistence

**Status:** In progress

**Progress:** The learner authored `compose.yaml` for app, pdf2png, and pinned
official PostgreSQL 17.11 Bookworm. It includes local `.env` interpolation,
service-DNS configuration, app-only localhost port publication, named data
volumes, and health-based dependency ordering. Source review confirmed the
configuration names and paths against the application. After the initial
commands below, the learner reported successful stack startup, the frontend
at localhost, administrator login with configured credentials, and working
UI navigation. These are learner-reported results; Docker is unavailable and
access to WSL is denied in the mentor's process. The initial commands, from
the repository root in WSL, are:

```bash
docker compose config --quiet
docker compose up --build --detach
```

The learner subsequently reported uploading the test PDF through the UI,
seeing it on the page, recreating the Compose containers, and finding the
saved upload still present after refreshing the page. This supports the
database/files persistence scenario; separate original-file download and
thumbnail verification after recreation have not been reported explicitly.
The current file now places db and pdf2png on an internal backend network and
app on backend plus an egress network. App and pdf2png have read-only root
filesystems, writable /tmp tmpfs mounts, all capabilities dropped, and
no-new-privileges. The learner reported recreating services and successfully
saving a second PDF afterward. This verifies the upload scenario under these
settings; network isolation itself and remaining M4 runtime checks have not
been explicitly exercised. It is not a production-ready deployment or a
completed milestone.

**Goal:** Introduce the first supported Compose model and run the complete production-like stack with one documented command.

**Deliverables:**

- Add a base `compose.yaml` for `app`, `pdf2png`, and `db`.
- Use the official pinned PostgreSQL image.
- Add PostgreSQL and application-data named volumes.
- Use internal networks and publish only the application entrypoint.
- Add healthchecks and dependency conditions.
- Separate non-secret configuration from secrets.
- Add compatible least-privilege runtime controls: `read_only`, `tmpfs`, `cap_drop`, and `no-new-privileges`.
- Validate the fully merged model with `docker compose config`; avoid `privileged`, host networking, Docker-socket mounts, and fixed `container_name` values unless a reviewed requirement exists.
- Verify clean startup, service restart, and container recreation.

**Acceptance criteria:**

- `docker compose up --build` reaches a healthy state on a clean machine.
- Recreating `app` and `db` containers preserves their respective data.
- PostgreSQL and `pdf2png` are not reachable through published host ports.
- The application reaches PostgreSQL and `pdf2png` through Compose service names on internal networks; no public PDF endpoint is used.
- Removing an application container does not remove user uploads.
- An intentional volume removal is clearly documented as destructive.
- A configuration validation check catches unresolved variables, invalid Compose structure, and unsafe production defaults before containers start.

### M6 — Development workflow and basic CI

**Status:** Done

**Progress:** README now documents the learner-exercised local integration
workflow, environment template, volumes, configuration validation, lifecycle,
and troubleshooting commands. Commands not yet exercised, including clean
checkout and destructive reset, are not claimed verified. The frontend's dev
API URL and backend CORS configuration were inspected for the hybrid workflow.
The learner authored `compose.dev.yaml`, which was source-reviewed: expose
database/PDF ports only on localhost, disable internal isolation for the
development backend network, and use a separate PostgreSQL development volume
so host-side files do not conflict with the existing integration database.
The next commands, from the repository root in WSL, validate the merged model
without printing secrets and start only the containerized dependencies:

```bash
docker compose -f compose.yaml -f compose.dev.yaml config --quiet
docker compose -f compose.yaml -f compose.dev.yaml up --detach --wait db pdf2png
```

Stop/remove the existing integration stack with `docker compose down` before
switching network configuration; retain its named volumes. The learner reported
that the dev dependency containers started and reached healthy status. Merged
configuration validation was not separately reported. At the learner's request,
Go and Compose now share one root `.env`; no backend copy is needed. Host Go
selects it with `ENV_FILE=../.env go run .` from backend/. Localhost dependency
addresses, `DEV=true` for CORS, and `../.data/dev` for files pair with the
separate development database. The loader preserves process-environment and
file-secret precedence and fails on an explicitly selected missing/invalid
file without exposing dotenv contents in errors. Full Go tests and vet passed
on Go 1.27.1 in WSL. Existing root credentials were preserved while adding
missing non-secret host defaults. The learner subsequently reported successful
host Go startup/readiness, React startup, and a development UI PDF upload with
thumbnail generation. The learner authored `.github/workflows/ci.yaml` for pull
requests and manual runs: explicit Docker test targets, both runtime image
builds, Compose startup/readiness, authenticated PDF upload and PNG retrieval,
failure logs, and unconditional temporary-volume cleanup. YAML parsing and
source review passed. The learner supplied a GitHub screenshot showing successful
`Container CI #1` for pull request #1 on branch `docker-v`, lasting 4m 22s.
This is learner-supplied execution evidence; individual run logs were not
independently retrieved. Together with the exercised local development workflow,
the successful CI run completes M6. M4/M5 are not marked complete.

**Goal:** Provide a practical local workflow and automate the stable checks already used during development.

**Deliverables:**

- Add an explicit `compose.dev.yaml`.
- Make the hybrid workflow the default: run PostgreSQL and `pdf2png` in Compose while Go and React run from the host or IDE.
- Preserve container-local dependency caches with named volumes or BuildKit caches where containers perform builds.
- Add full-container frontend/Go hot reload or Compose Watch only if the team needs that workflow and it remains reliable on Windows/WSL.
- Document debugging, test execution, database reset, logs, and common failure recovery.
- Add pull-request CI for the frontend build, Go tests/vet, Python tests, application image builds, and one Compose smoke test.
- Invoke both Dockerfile `test` targets explicitly before release builds. The final runtime stages do not depend on them, so a normal image build skips them.

**Acceptance criteria:**

- The documented hybrid workflow supports normal frontend and backend development without rebuilding infrastructure services.
- If full-container hot reload is implemented, a source edit updates the relevant service without rebuilding the entire stack.
- Development credentials are clearly non-production and cannot be confused with production secrets.
- Raw Compose commands remain documented even if convenience wrappers are added.
- Pull-request CI runs the established language checks, builds both application images, and exercises the Compose stack without publishing a release.

The existing test targets can be invoked from the repository root with:

```bash
docker build --target test .
docker build --target test ./pdf2png
```

These commands build the test stages; they do not replace runtime image builds or the Compose smoke test. They were not executed during this documentation revision because Docker is unavailable to the current process.

### M7 — Tested image releases

**Status:** In progress

**Progress:** Selected GHCR for the two project images:
`ghcr.io/vadimnevlad/sheetable-app` and
`ghcr.io/vadimnevlad/sheetable-pdf2png`. The learner authored
`.github/workflows/release-images.yaml` in explained blocks; the mentor copied
the five existing Compose smoke/log/cleanup steps at the learner's request.
The pushed-tag trigger matches `v*`; a Bash check then accepts only stable
`vMAJOR.MINOR.PATCH` versions without leading zeroes. Release tags should point
to reviewed commits on main and must not be moved or reused.
The workflow explicitly runs both test targets, builds and loads linux/amd64
runtime images with pinned Docker Actions, tests them through Compose, and
blocks publication on Trivy HIGH/CRITICAL vulnerabilities, including unfixed
findings. After successful checks it pushes those same loaded images with
version and full commit-SHA tags using `GITHUB_TOKEN`, then records repository
digests in the run summary. Both images have source/revision/version labels.
Registry build caches are exported separately under mutable `buildcache` tags
before release checks. They contain intermediate build layers, are not release
images, and do not preserve all cache-mount contents between runners.
Provenance generation is disabled for this local Docker export path; SBOM and
provenance delivery remain follow-up work. Initial retention keeps all numbered
release images, including rollback versions; no automated deletion is configured.
The four pushes are sequential, not an atomic two-image publication. Only a
successful complete run identifies a usable release pair.
YAML parsing, action-pin/order review, and Bash syntax checks passed. The learner
added the same Trivy gates to PR CI and supplied a failed application-image scan:
56 Debian findings (52 HIGH, 4 CRITICAL) and 32 Go-module findings (all HIGH).
This is learner-supplied scan evidence, not a verified exploitability assessment.
Available dependency fixes and Debian advisory applicability require separate,
tested remediation; the PDF-image scan is not evidenced by this report.
Subsequent local remediation upgraded the six affected Go modules, replaced
archived jwt-go with golang-jwt/jwt/v5, patched a new transitive QUIC advisory,
and moved app/PDF runtimes to digest-pinned Debian 13 bases. Python runtime pip
and ensurepip were removed after identifying two vendored installation-tool
findings. Host Go tests/vet/module verification, both Docker test/runtime builds,
and an isolated Compose login/PDF/PNG/invalid-token smoke check passed. Trivy
0.70.0 reports zero HIGH/CRITICAL Go or Python findings.
The app uses verified digest-pinned Distroless Debian 13; PDF now uses official
Python 3.14.7 on Alpine 3.24 with DejaVu fallback fonts. Both local Trivy scans
report zero HIGH/CRITICAL without ignore files. Test/runtime builds, integration
and shutdown checks passed; a font regression was found and fixed, with a
rendered-text check added to both workflows. Alpine's database omits three
known TIFF/X11 advisory IDs; a temporary gate checks system and Pillow-bundled
libraries. Exact Poppler 25.12.0 source and runtime dependency review found
these three CVEs inapplicable to the explicitly fixed PDF-to-PPM-to-PNG path.
The assessment expires 2026-10-27 and fails closed on changed application source,
dependency versions or X11 linkage. Local positive/negative gate checks passed.
This is not a claim that the affected libraries were patched.
Severity gates remain unchanged and no residual-risk waiver was added. Details are in
[`docs/security-remediation-2026-09-27.md`](security-remediation-2026-09-27.md).
No release workflow execution, registry publication, or pull/run of a published
image has been verified yet. M7 remains in progress.

**Goal:** Publish tested, traceable, scanned images without rebuilding a different production artifact.

**Deliverables:**

- Update Docker GitHub Actions and pin third-party actions according to repository policy.
- Reuse the M6 language, image-build, and smoke checks as release gates.
- Add registry-backed or GitHub Actions build cache.
- Add vulnerability scanning with an explicit severity/failure policy.
- Keep secret scanning and automated dependency updates enabled with documented exception ownership and expiry.
- Publish version and commit-SHA tags; channel tags are optional convenience aliases.
- Publish `linux/amd64`; add `linux/arm64` only when a real deployment target requires it and the image is exercised on that architecture or an approved equivalent.
- Retain the image digest as the release identity and attach SBOM/provenance when the selected Buildx/registry workflow provides them automatically.
- Ensure build secrets use secret mounts rather than build arguments.
- Automate base-image, language-dependency, and GitHub Action update proposals and schedule rebuilds even when application source has not changed.
- Define registry naming, immutable release tags, and a basic retention policy.
- Add a release checklist that keeps the AGPL source revision and required third-party notices discoverable.

**Acceptance criteria:**

- Pull requests cannot publish production images.
- A failed test, smoke test, or policy-level vulnerability prevents release publication.
- A published digest can be traced to source revision, workflow, dependencies, and build metadata.
- Production deployment refers to an immutable version or digest, not only `latest`.
- Automatically generated SBOM/provenance, when enabled, are attached to the same published digest.
- The released source revision and required notices are discoverable from the release record.

### M8 — Single-server production and recovery

**Status:** Planned

**Goal:** Promote a tested immutable release into a supportable, secure, and recoverable single-host deployment.

**Deliverables:**

- Add a production Compose overlay with reverse proxy/TLS and runtime secrets; document secret generation, restricted storage, and rotation.
- Deploy the tested digest, validate configuration, and run a post-deployment smoke test. Document and rehearse update, rollback, and failed-migration recovery commands.
- Set RPO/RTO and backup retention. Store encrypted backups off-host and test integrity and a consistent PostgreSQL-plus-files restore into a clean environment.
- Document host patching, firewall/SSH access, and daemon-socket protection; name the maintainer.
- Configure restart policy and log rotation; check disk capacity and document host reboot, daemon restart, and certificate renewal behavior.

A short runbook with tested commands is sufficient. Separate staging approval workflows, a dedicated secret-management platform, and an observability stack are optional M9 decisions.

**Acceptance criteria:**

- The same immutable digest that passed release checks is deployed and can be rolled back without rebuilding it.
- Production deployment uses restricted credentials and runtime secrets that are absent from Git and image layers.
- Database and application files can be restored into a clean stack within the agreed RPO/RTO.
- Operators can identify unhealthy services using documented commands and health output.
- No production secret or persistent data depends solely on a container writable layer.
- Host reboot, Docker daemon restart, disk-pressure behavior, and certificate renewal have documented outcomes appropriate to the single-host service.

### M9 — Post-baseline architecture decisions

**Status:** Planned

**Goal:** Decide future platform work using measured requirements rather than trend-driven additions.

Potential decisions:

- Managed PostgreSQL versus self-hosted PostgreSQL.
- Object storage for uploaded PDFs and generated images.
- Horizontal scaling and asynchronous PDF-processing jobs.
- Kubernetes or another orchestrator.
- Central metrics, tracing, and log aggregation.
- Migration from `go.rice` to Go-native embedding.
- Separate migration jobs and release orchestration.
- High availability or automated host failover beyond the accepted single-host risk.
- Full-container hot reload when the hybrid development workflow is insufficient.
- Additional release architectures such as `linux/arm64` without a current deployment consumer.
- Image signing and custom attestation verification beyond automatically generated SBOM/provenance.
- Formal registry deletion/recovery procedures beyond basic retention and immutable release tags.
- Staging approval workflows and a wider deployment audit system.
- Formal data-classification, retention, privacy, and deletion programs that require organizational or legal ownership.
- Full SLI/alerting coverage and exhaustive incident runbooks.
- Docker live-restore, rootless mode, or user-namespace remapping when the threat model or availability target requires them.
- Detailed CPU, memory, PID, build-time, and deployment-time benchmarking when capacity or cost requires it.

These items require a concrete capacity, availability, compliance, or team need. They are not acceptance requirements for M0-M8.

## Definition of Done for the initial program

The integration baseline is complete at M5: a clean checkout builds both non-root images, serves the current frontend/API, converts a PDF, and preserves database data and uploads when containers are replaced. Health, dependency failure, and graceful shutdown are exercised.

The initial production program is complete when M0-M8 are `Done`, including:

- A documented development workflow and CI that tests, scans, and publishes traceable `linux/amd64` releases; available SBOM/provenance metadata is retained when enabled.
- Deployment and rollback using the tested digests, with HTTPS and runtime secrets.
- An exercised, application-consistent restore of database and files from encrypted off-host backups within the chosen RPO/RTO.
- A short operating runbook covering deploy, diagnose, update, recover, host access/patching, restart, log rotation, disk capacity, and external-service failures.
- Discoverable released source and required license notices.

Written artifacts alone do not complete a milestone. Record executed checks and tool limitations; optional M9 work does not block this definition of done.

## Risk register

| Risk | Impact | Mitigation |
|---|---|---|
| Legacy frontend cannot build on a supported Node release | Blocks secure reproducible build | Establish baseline first; isolate dependency modernization and preserve behavior with tests |
| Go dependencies fail on a current builder | Build or behavior regression | Test supported Go versions; upgrade in narrow changes; retain rollback point |
| CGO/SQLite prevents minimal runtime selection | Larger or incompatible image | Prefer compatible slim runtime initially; measure before removing SQLite/building static |
| `go.rice` embeds stale assets | Backend serves wrong frontend | Generate embedded assets within the same build; later evaluate `go:embed` |
| Application exits before PostgreSQL is ready | Restart loop/outage | Compose health dependency plus bounded application-level retry |
| Startup migrations race across replicas | Schema corruption or failed rollout | Keep single replica initially; design explicit migration step before scaling |
| Local filesystem prevents horizontal scaling | Inconsistent files between replicas | Keep one replica; later adopt shared/object storage with an explicit migration |
| Host bind mounts are slow on Windows | Poor development feedback time | Measure Compose Watch, WSL filesystem, and bind mounts; document the supported fast path |
| Default credentials reach production | Account compromise | Fail production startup on defaults; use secrets and rotation procedures |
| Scanning reveals extensive legacy vulnerabilities | Release blocked or risk accepted informally | Define severity policy, ownership, exception expiry, and staged modernization |
| Unneeded additional-platform builds slow CI | Slow feedback and high CI cost | Build the native platform on pull requests and add another release platform only for a real deployment consumer |
| Data volume exists but cannot be restored | False sense of recoverability | Make restore exercise a release criterion |
| Database and filesystem backups are taken at inconsistent points | Restored metadata references missing or mismatched files | Define an application-consistent backup sequence and test full-system restore |
| External APIs, SMTP, fonts, or remote images are unavailable | Partial feature failure, latency, or broken UI | Inventory dependencies; apply bounded timeouts; define degraded behavior and observability |
| Docker socket or remote daemon access is compromised | Root-equivalent host compromise | Restrict trusted users; avoid socket mounts; use protected SSH/TLS access only when required; evaluate rootless/user namespaces |
| Container logs, images, cache, or volumes fill the host disk | Full service outage or database corruption | Configure log rotation; monitor data root and volume growth; define safe cleanup and capacity procedures |
| Production is rebuilt rather than promoted | Untested artifact reaches users | Deploy the exact CI-produced digest and retain the previous release for rollback |
| AGPL or third-party license obligations are missed | Legal/compliance exposure and delayed release | Assign compliance ownership; review corresponding-source delivery and license notices before release |
| CI controls arrive only after most implementation changes | Regressions enter before gates exist | Add executable checks incrementally in every milestone; reserve M7 for final publication and enforcement |

## Decision log

| Date | Decision | Rationale |
|---|---|---|
| 2026-09-23 | Use one production image for Go and compiled React | React is a static artifact and the existing Go service already owns HTTP delivery |
| 2026-09-23 | Keep `pdf2png` as a separate service | It has an independent runtime, system packages, failure domain, and scaling profile |
| 2026-09-23 | Use the official PostgreSQL image | No current requirement justifies maintaining a custom database image |
| 2026-09-23 | Use Compose as the initial runtime baseline | It meets development and single-host deployment needs without premature orchestration complexity |
| 2026-09-23 | Deliver changes incrementally | Separates regressions, simplifies review and rollback, and supports learning objectives |
| 2026-09-23 | Treat CI, security, documentation, and dependency maintenance as cross-cutting work | Delaying all controls until M7 would leave earlier milestones unprotected and increase integration risk |
| 2026-09-23 | Promote immutable image digests between environments | Prevents production from running an artifact different from the one tested and approved |
| 2026-09-23 | Commit dependency lock files and normalize repository text to LF | Lock files are required for reproducibility, while LF prevents Windows/WSL line endings from breaking Linux entrypoints and scripts |
| 2026-09-23 | Retire the unsupported legacy Docker implementation before replacement | Its material findings are preserved in this roadmap and its exact content remains in Git history; removing it prevents accidental reuse while the new build and delivery path is designed from verified requirements |
| 2026-09-23 | Create replacement container artifacts from scratch | M1 and M2 establish deterministic inputs and runtime contracts; M3 creates the new app Dockerfile, M4 creates the new PDF Dockerfile, and M5 introduces the first supported Compose model. Retired files are evidence, not templates |
| 2026-09-23 | Standardize frontend builds on Node 24.21.0 and npm 11.19.0 | The production image will need a supported, reproducible builder. Upgrading only `react-scripts` from 2.1.8 to 5.0.1 removes the obsolete `http_parser` dependency path while preserving React 17 and application behavior; broader framework modernization remains a separate work item |
| 2026-09-24 | Standardize `pdf2png` builds on Python 3.14.7 with hash-locked dependencies | Python 3.10 reaches end of life in October 2026. Direct dependencies live in `requirements.in`; pip-tools 7.6.1 generates `requirements.txt` with exact transitive versions and hashes, and installation uses `--require-hashes` |
| 2026-09-24 | Keep dependency modernization separate from container definition work | M1 records owner, severity, runtime-support status, and release gates. Weekly update PRs cover npm, Go, Python, GitHub Actions, and future base images, but major upgrades and bot-generated lock changes require isolated review and tests |
| 2026-09-24 | Exclude local generated frontend artifacts from the root image context | The M3 application image must build React and regenerate `rice-box.go` from the same source revision; excluding both local `frontend/build` and the tracked generated embed prevents a stale workstation artifact from entering an image |
| 2026-09-26 | Support `linux/amd64` as the initial verified runtime platform and defer `linux/arm64` | The current development and initial deployment baseline is x86-64, while no ARM consumer or production host requirement exists. Multi-platform publication and ARM CGO validation will be added in M7 only when justified by a deployment requirement |
| 2026-09-26 | Keep M4-M8 focused on a practical single-host delivery baseline | The earlier plan mixed required containerization with mature enterprise platform controls. Basic CI moves to M6, while optional architectures, signing, formal governance, advanced observability, and detailed benchmarking move to the M9 backlog until a concrete requirement exists |
| 2026-09-26 | Use a four-step learning route with detailed milestone checks as reference | Separate images, Compose/data, development/releases, and production/recovery. Keep practical controls and tested recovery; use short maintainer notes and runbooks instead of mandatory approval systems. Locked inputs do not claim byte-identical OS package rebuilds |
| 2026-09-26 | Share one root dotenv file for local Compose and host Go | Avoid duplicated credentials. ENV_FILE explicitly selects the file for host Go, while Compose supplies container-specific addresses through environment. Preserve default loader behavior, process overrides, and file-backed secrets; reject explicitly selected files that cannot be loaded |

## Progress log

| Date | Milestone | Update |
|---|---|---|
| 2026-09-23 | Planning | Created the initial containerization and operational-readiness roadmap |
| 2026-09-23 | Planning | Reworked the repository README to document the current architecture, modernization status, safety notice, and roadmap entry point without publishing unverified setup commands |
| 2026-09-23 | Planning review | Expanded cross-cutting CI, external-dependency inventory, delivery promotion, host hardening, log/disk controls, RPO/RTO, backup consistency, and license-compliance coverage after a formal roadmap review |
| 2026-09-23 | Pre-M0 guardrails | Added repository-wide secret/local-artifact ignore rules and explicit cross-platform line-ending policy without performing a noisy bulk renormalization |
| 2026-09-23 | M0 | Started baseline capture; recorded the initial host/tool limits, frontend restore and build evidence, configuration and data paths, external dependencies, licenses, and the initial smoke-test plan. Replacement image measurements begin in M3/M4 |
| 2026-09-23 | M0 cleanup | Removed the unsupported legacy backend/PDF Dockerfiles, Docker image workflow, and its BuildKit configuration after preserving their material findings in this roadmap and their exact content in Git history |
| 2026-09-23 | Roadmap clarification | Made the fresh-start boundary explicit: M0–M2 contain discovery and prerequisites, M3/M4 create new Dockerfiles from scratch, and M5 introduces the first supported Compose model |
| 2026-09-23 | M0 / M1 frontend | Replaced the Node-24-incompatible Create React App 2 build chain with `react-scripts` 5.0.1 while keeping React 17 and application source unchanged. A clean Node 24.21.0/npm 11.19.0 `npm ci`, test-runner invocation, production build, dev-server compile, and HTTP 200 smoke test passed. The build retains existing ESLint warnings; `npm audit` reports 33 known findings (19 high, 5 moderate, 9 low, 0 critical), which remain in the dependency-modernization backlog |
| 2026-09-24 | M0 backend baseline | Restored and verified Go modules and exercised tests, build, startup, health, version, login, SQLite persistence, and restart on Go 1.27.1 with CGO. Existing findings are a malformed JSON struct tag reported by `go vet`, a compiler warning in legacy `go-sqlite3`, sparse test coverage, and exit code 143 without graceful `SIGTERM` handling |
| 2026-09-24 | M0 pdf2png baseline | Characterized the service on Python 3.10.12 with Poppler, Flask 3.1.3, and pdf2image 1.17.0. Updated `send_file` compatibility and path/MIME handling; a real multipart PDF request returned HTTP 200 and a 380×535 PNG, and request files were removed after the response. A production WSGI server, safe request-scoped files, input validation, and concurrency remain open work |
| 2026-09-24 | M1 pdf2png dependencies | Selected Python 3.14.7, verified the service and Poppler conversion on that runtime, and added pip-tools input plus a complete hash-locked dependency graph. A clean `pip install --require-hashes`, `pip check`, syntax check, and HTTP PDF-to-PNG smoke test passed using the lock file |
| 2026-09-24 | M1 backend and build inputs | Standardized the backend builder on Go 1.27.1, removed four stale checksum entries with `go mod tidy`, and re-ran module verification and tests successfully. The existing `go vet` struct-tag defect remains an explicitly owned application backlog item rather than being hidden in dependency work |
| 2026-09-24 | M1 build contexts | Added root and `pdf2png` `.dockerignore` contracts and exercised them through BuildKit context exports. Required source and lock files were present; VCS data, local dependencies, runtime data, secrets, build output, the separate PDF service, and stale generated frontend/embed artifacts were absent |
| 2026-09-24 | M1 complete | Added the dependency modernization/release policy and weekly Dependabot coverage for npm, Go modules, Python, GitHub Actions, and future Docker bases. Reproducible restores, supported builder versions, clean module metadata, intentional build contexts, and stale-embed prevention now satisfy the M1 acceptance criteria |
| 2026-09-24 | M0 complete | Consolidated the verified host/tool versions, language baselines, environment variables, persistent and temporary paths, external dependencies, credentials, smoke-test contract, host assumptions, license boundary, owners, and milestone follow-ups in `docs/m0-baseline.md`. Image-specific evidence remains intentionally assigned to M3-M5 |
| 2026-09-24 | M2 started | Began the application prerequisite phase. The first bounded change replaces the hard-coded public PDF conversion endpoint and unsafe HTTP client behavior with a tested runtime configuration contract |
| 2026-09-24 | M2 PDF client contract | Added `PDF2PNG_URL` with a local default and verified a Compose-style `http://pdf2png:5000/createthumbnail` value. Replaced the test-server/insecure-TLS production client with a 30-second standard client, validated status and PNG media type, returned actionable errors instead of panicking, and atomically published thumbnails. Added automated success/failure/URL/TLS tests; all Go tests, vet, module verification, and tidy checks pass |
| 2026-09-24 | M2 lifecycle and health | Added bounded exponential database startup retry, process-only liveness, two-second database-aware readiness, and graceful signal handling with a ten-second drain plus database close. Unit tests cover retry limits and health semantics; a compiled process returned live/ready 200, persisted SQLite data, and exited 0 after `SIGTERM` |
| 2026-09-24 | M2 production configuration and data | Added `APP_ENV` validation, rejected unsafe production secrets/default administrator credentials, validated URLs/database/SMTP/ports before startup, added file-backed secret inputs, and recursively prepared the explicit application data directories. Tests cover production rejection/acceptance, service URL validation, file-secret precedence, and data paths |
| 2026-09-24 | M2 external dependencies and database lifecycle | Bounded Open Opus and SMTP calls, restored normal SMTP TLS verification, converted outbound failures from panic paths into errors/fallback, made schema migration errors actionable, and made initial administrator creation idempotent when the users table is empty. Recorded the initial one-replica migration and backward-compatible rollback contract in `docs/runtime-contracts.md` |
| 2026-09-24 | M2 complete | Verified the full Go suite, `go vet`, module integrity, clean tidy state, and race detector. Process-level production checks rejected repository default credentials, loaded file-backed secrets, created the complete data root, returned live/ready 200, persisted SQLite data, and exited 0 after `SIGTERM`. M3 is the first Docker-authoring milestone and is intentionally handed to the learner |
| 2026-09-26 | M3 complete | Created and exercised the replacement Go/React production image from scratch. Named dependency, frontend, test, application-build, and runtime stages use pinned multi-platform bases and BuildKit caches; the build regenerates embedded React assets and produces a 26 MB CGO binary. The `linux/amd64` runtime image reported 41.3 MB of content, ran as UID/GID 10001, served live/ready/frontend endpoints with HTTP 200, and exited 0 on `SIGTERM`. ARM64 is deferred until a concrete deployment requirement or M7 release validation |
| 2026-09-26 | Roadmap audit | Simplified M4-M8 around the work normally required for a small real-world single-host service: hardened images, Compose integration, a hybrid development workflow, basic PR CI, immutable releases, and tested recovery. Deferred optional enterprise governance, extra architectures, signing, advanced observability, and detailed performance measurement to M9 |
| 2026-09-26 | M4 application preparation | Replaced user-controlled working filenames with request-scoped temporary storage; added input/size validation, bounded Poppler conversion, controlled API errors, liveness, and Gunicorn timeouts/logging. Added Gunicorn 26.2.0 to the hash lock. A clean Python 3.14.7 restore, dependency check, eight unit tests, Gunicorn configuration check, real Poppler conversion to a 380×535 PNG, temporary-file cleanup, and graceful `SIGTERM` shutdown passed. Dockerfile authoring was the next step at this point |
| 2026-09-26 | Roadmap learning review | Added the four-step route, removed repeated legacy-build restrictions, simplified release/deployment decisions and M4/M8 checklists, and separated the M5 integration baseline from M8 production readiness. Recorded explicit Docker test targets and the limits of rebuild reproducibility. M4 Dockerfile is written but container verification remains pending; Docker was unavailable to this review process. No milestone was marked complete |
| 2026-09-26 | M4 learner container smoke checks | Learner reported successful test/runtime target builds in WSL, working container health, real conversion to a 380 x 535 PNG, expected missing-file error with continuing health, and exit code 0 after docker stop with a 15-second timeout. Runtime build used cache. Remaining runtime checks are tracked in M4 and will accompany M5 integration; mentoring now groups actions into one task/result review rather than pausing after each command |
| 2026-09-26 | M5 learner Compose preparation | Learner wrote the three-service Compose file with local .env settings, named volumes, service-DNS wiring, app-only published port, and health-based startup dependencies. Source reviewed; Docker/Compose execution remains for the learner's WSL terminal. Compose is the normal integration path; manual docker run is reserved for useful isolated diagnostics. M5 is in progress |
| 2026-09-26 | M5 learner first stack startup | Learner reported that the Compose stack started, the app opened on localhost, configured administrator credentials worked, and UI navigation worked. Upload/PDF-service integration, container replacement persistence, explicit network isolation, and runtime restrictions remain unverified. M5 remains in progress |
| 2026-09-26 | M5 learner upload and persistence | Learner reported a successful UI PDF upload and that the saved upload remained visible after Compose container recreation and page refresh. Separate post-recreation original-file download and thumbnail checks were not explicitly reported. Next block is explicit networks and app/PDF runtime restrictions in the learner-authored Compose file; M5 remains in progress |
| 2026-09-26 | M5 learner networks and runtime restrictions | Reviewed learner-authored backend internal network, app egress attachment, and app/PDF read-only/tmpfs/cap-drop/no-new-privileges settings. Learner reported recreating services and saving a second PDF successfully. Network connectivity restrictions themselves and remaining M4 runtime checks are not claimed verified; M5 remains in progress |
| 2026-09-26 | M5 documentation / M6 started | Learner added .env.example with placeholders. Updated README from obsolete M0 status to the current local Compose workflow, persistence semantics, destructive-reset distinction, and troubleshooting. Inspected frontend development API URL and backend CORS handling; designed a learner-authored hybrid development overlay with localhost dependency ports and separate development database data. Runtime verification remains in WSL; M4/M5 acceptance checks remain open |
| 2026-09-26 | M6 shared local configuration | Learner reported healthy dev dependency containers and requested one root .env. Added ENV_FILE selection and loader tests, expanded the template/README, and appended only missing non-secret host settings to the existing ignored root file. Backend .env did not exist and no copy was created. Go test ./... and go vet ./... passed in WSL on Go 1.27.1 after compatibility fixes for legacy feeder errors. Live host Go/React startup remains unverified |
| 2026-09-26 | M6 learner host backend startup | Learner reported successful host Go startup using the shared root .env and a successful readiness check through curl. React startup and a complete development UI upload remain unverified; M6 stays in progress |
| 2026-09-26 | M6 learner development workflow | Learner reported React startup, login, and successful PDF upload with thumbnail generation using host Go/React and Compose PostgreSQL/pdf2png. Documented the frontend commands. The local hybrid workflow has functional learner evidence; PR CI remains the next block and M6 stays in progress |
| 2026-09-26 | M6 learner CI authored | Reviewed learner-authored ci.yaml: pinned checkout, read-only repository permissions, explicit test/runtime targets, disposable CI configuration, full Compose readiness and authenticated PDF/PNG smoke check, failure logs, and cleanup. YAML parsing passed and the PDF fixture is tracked by Git. GitHub execution is pending; M6 remains in progress |
| 2026-09-26 | M6 complete — first GitHub CI run | Learner supplied a screenshot showing successful Container CI #1 for pull request #1 on docker-v, duration 4m 22s. Individual job logs were not independently retrieved. Local hybrid workflow and PR CI now have execution evidence; M6 is Done. M7 release preparation is next; remaining M4/M5 checks stay open |
| 2026-09-26 | M7 release workflow prepared | Learner authored the tag-triggered GHCR workflow with stable-version validation, pinned actions, explicit test targets, linux/amd64 runtime builds and registry caches, Compose PDF smoke checks, blocking HIGH/CRITICAL vulnerability scans, publication of the tested images, and digest summary. Mentor copied the existing smoke steps at request. YAML and Bash syntax checks passed. First GitHub release run, scanner findings, registry publication and published-image execution remain pending |
| 2026-09-27 | M7 PR vulnerability triage | Learner added Trivy scans to PR CI and supplied a failing app-image report: 56 Debian and 32 Go-module HIGH/CRITICAL findings. Initial source/advisory review groups Go remediation into x/crypto, x/net, x/text, YAML, Gin, and replacement of archived jwt-go; Debian findings include repeated CVEs and missing Bookworm fixes. Debian explicitly states the vulnerable MiniZip code for CVE-2023-45853 is not built into the affected Bookworm zlib binary packages. No dependencies, base images, or scan exceptions were changed; fix compatibility, image re-scan, PDF-image findings, and release execution remain pending |
| 2026-09-27 | M7 dependency and runtime remediation | Upgraded Go x/*, YAML and Gin; migrated JWT to v5 with HS256/expiration checks and negative tests; patched Gin's QUIC dependency. Raised module minimum to Go 1.26 and fixed a legacy dynamic Printf call. Moved app/PDF runtimes to digest-pinned Debian 13 and removed runtime pip/ensurepip. Host tests/vet/verify, govulncheck (zero affected calls), both Docker test targets/runtime builds, and isolated Compose readiness/login/PDF/PNG/401 checks passed. Trivy 0.70.0 reports zero HIGH/CRITICAL language findings, app OS 43 HIGH and PDF OS 64 HIGH, with zero CRITICAL in both. Gates remain unchanged and release remains blocked; see the remediation record for evidence and residual assessment work |
| 2026-09-27 | M7 scoped OS applicability review | Replaced the full GnuPG dependency with minimal gpg in the PDF runtime and verified conversion after rebuilding. Added exact-version non-applicability records for absent systemd-homed, tiffcrop and tpm2daemon, expiring 2026-10-27, with pre-scan absence checks in both workflows. Positive checks and a negative image containing tiffcrop behaved correctly. Trivy reports app 41 HIGH and PDF 54 HIGH after records (raw 43/59), zero CRITICAL and zero HIGH/CRITICAL language findings. Isolated Compose smoke and YAML/Bash validation passed. GitHub execution remains pending; M7 and the release remain blocked by residual OS findings |
| 2026-09-27 | M7 curl/Expat/TIFF review | Reviewed nine library advisories and upstream Poppler 25.03.0 call paths. Fresh Trixie package metadata offers no newer candidate for the installed curl/Expat/TIFF libraries. Added one exact-version expiring record for CVE-2026-12064, which upstream says affects only absent curl CLI; real-image checks passed and a negative curl image was rejected. Re-scan leaves PDF 53 HIGH (17 IDs), app 41 HIGH, zero CRITICAL. Remaining library findings stay enabled; no custom library builds, unstable packages or severity reductions were introduced. M7 remains in progress and release remains blocked |
| 2026-09-27 | M7 shared-base applicability and runtime reduction | Reviewed seven application IDs; four util-linux CVEs account for 36 repeated package pairs. Removed unused nsenter and infocmp executables from both runtimes; confirmed Archive::Tar is absent. Added three exact-PURL records expiring 2026-10-27 and extended pre-scan guards. Both runtime builds and isolated Compose smoke passed; three negative images were rejected. Trivy raw counts remain app 43/PDF 59 HIGH; after all records app 28 HIGH (4 IDs), PDF 39 HIGH (14 IDs), zero CRITICAL and no HIGH/CRITICAL language findings. Mount/libmount and libacl findings stay enabled; M7 remains in progress and release remains blocked |
| 2026-09-27 | M7 app runtime reduced to Distroless | Tested a separate official Distroless Debian 13 candidate, then replaced the root runtime with its immutable digest while keeping UID/GID 10001 and data paths. Candidate/final Compose PostgreSQL PDF/PNG smoke passed; final SQLite persistence across replacement and SIGTERM exit 0 passed. Strict Trivy scan recognizes OS and Go packages and returns zero HIGH/CRITICAL without app ignores. Removed the app ignore file and its shell-based prerequisite step from both workflows; documented shell-free diagnostics. PDF remains at 39 HIGH (14 IDs), zero CRITICAL; release/GitHub execution remain pending and M7 stays in progress |
| 2026-09-27 | M7 PDF Alpine runtime and scanner coverage | Verified official Python 3.14.7/Alpine 3.24 with the unchanged hash lock and eight tests. Added DejaVu after detecting missing fixture text; PNG now matches the old runtime pixel-for-pixel, and both workflows check rendered text. Final builds, Compose PDF/PNG/401 smoke, health and SIGTERM exit 0 passed. Both Trivy scans return zero HIGH/CRITICAL without ignore files, but Alpine data omits three known TIFF/X11 CVEs; a temporary inventory/version gate rejects the real PDF image and passes a synthetic fixed-version fixture. Release remains blocked on these three assessments, not silently approved by the green scanner; M7 stays in progress |
| 2026-09-27 | M7 native PDF path assessment | Confirmed stable Alpine offers no fixes for the three native IDs. Verified the checksum-matched Poppler 25.12.0 source, upstream TIFF patch, actual pdfinfo/pdftoppm linkage, pdf2image PPM decoder and Pillow bundled TIFF 4.7.1. Explicitly selected PPM without Cairo. Added exact-source/dependency, amd64-only non-applicability assessment expiring 2026-10-27 to the existing gate, including bundled TIFF. Eight tests, runtime build, Compose PDF/PNG/text/401 smoke and real-image gate passed; five negative scope checks were rejected. GitHub CI/release execution remains pending and M7 is not complete |
