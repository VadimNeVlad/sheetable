# SheetAble: containerization and operational readiness roadmap

## Document purpose

This is the living delivery plan for turning SheetAble into a reproducible, secure, and operable containerized application. It is intended to be readable by engineering leadership, application developers, DevOps engineers, security reviewers, and operators.

The plan deliberately separates discovery, application prerequisites, image construction, local orchestration, CI/CD, and production hardening. Work should be delivered in small reviewable changes, with every completed milestone backed by executable verification.

## Status

| Field | Value |
|---|---|
| Overall status | In progress |
| Current milestone | M4 ready — PDF service hardening and image construction are next |
| Initial delivery target | Docker Compose on a single Linux host |
| Development platform | Windows/WSL 2 with Docker Desktop, plus Linux compatibility |
| Runtime platforms | `linux/amd64` initially; `linux/arm64` deferred until a deployment requirement or M7 release validation |
| Primary database | PostgreSQL |
| Last updated | 2026-09-26 |

Status values used in this document: `Planned`, `In progress`, `Blocked`, `Done`.

## Executive summary

SheetAble entered this program with partial, outdated containerization. After its M0 findings were recorded, the unsupported legacy Dockerfiles and Docker image workflow were retired so they could not be mistaken for a supported path. Their pre-removal state remains available in Git history, while the material findings are retained in this roadmap. The replacement Dockerfiles, Compose model, and image workflow will be designed from verified requirements rather than copied from the retired implementation. The application still cannot be built and operated reproducibly as one system: the frontend artifact flow is manual, the backend depends on a hard-coded external PDF service, persistence is implicit, and the default configuration is unsafe for production.

The target outcome is:

- a reproducible production image containing the Go API and compiled React assets;
- a separate hardened PDF-to-PNG service image;
- PostgreSQL based on the official image;
- a fast, documented development workflow;
- a production-like Compose environment with healthchecks, internal networking, persistent storage, and least privilege;
- CI that tests, scans, attests, and publishes immutable multi-platform images;
- documented backup, restore, upgrade, rollback, and troubleshooting procedures.

Success is not measured by the number of containers. It is measured by deployment repeatability, change safety, developer feedback time, security posture, and recoverability.

## Scope and assumptions

### In scope

- Container build definitions for the Go/React application and `pdf2png`.
- PostgreSQL integration using an official upstream image.
- Development, integration, and single-host production Compose configurations.
- Required application changes for container configuration, service discovery, health, graceful shutdown, and persistence.
- Dependency locking and build-cache optimization.
- CI validation, image publishing, vulnerability scanning, SBOM, and provenance.
- Runtime hardening, data lifecycle, backup/restore, upgrade, rollback, and operational documentation.
- Release promotion through protected environments without rebuilding an already tested image.
- Docker-host security, lifecycle, capacity, logging, and recovery requirements for the initial single-host deployment.
- Open-source and third-party license compliance for source and container-image distribution.

### Initial assumptions

- The first production deployment target is one Linux Docker host.
- TLS termination will be added after the internal application stack is stable.
- PostgreSQL runs in Compose for development and the initial self-hosted production baseline. A managed database may replace it later without changing the application image.
- User-uploaded PDFs and generated images use a persistent filesystem initially. Object storage is a future scalability decision.
- The React production bundle continues to be served by the Go process.

### Open decisions and required organizational inputs

These do not block M0 unless stated otherwise. They must receive an accountable owner and be resolved before the indicated milestone is accepted.

| Decision/input | Needed by | Current planning assumption |
|---|---|---|
| Container registry, repository ownership, and retention policy | M7 | GHCR or Docker Hub; immutable release digests |
| Production host provider, Linux distribution, sizing, storage, and patch owner | M8 | One supported Linux host with replaceable infrastructure and monitored persistent storage |
| Domain, DNS owner, TLS issuer, and certificate-renewal owner | M8 | Reverse proxy terminates automatically renewed public TLS |
| Availability objective, maintenance window, and accepted single-host downtime | M8 | Single-host risk is explicitly accepted for the initial release |
| RPO, RTO, backup retention, encryption, and off-host destination | M8 | Values are set from business impact after M0 data-volume measurements |
| Production secret store and rotation owner | M8 | Secrets are delivered at runtime and never stored in Git or image layers |
| Data classification, retention, deletion, and privacy requirements for uploaded documents and account data | M8 | Persistent user data is access-controlled and retained only according to an approved policy |
| AGPL and third-party license compliance approver | M7 | Legal/compliance owner approves corresponding-source delivery and notices before release |
| Self-hosted versus managed PostgreSQL for the long-term platform | M9 | Self-hosted PostgreSQL is used for the initial Compose baseline |

### Explicit non-goals for the initial delivery

- Kubernetes, Helm, Terraform, or a service mesh.
- A separate production frontend container without a measured need.
- A custom PostgreSQL image without required extensions or configuration.
- Horizontal application scaling while user files remain on a single local filesystem.
- A full observability platform before stable health, logging, and runtime contracts exist.
- Unrelated application rewrites hidden inside infrastructure changes.

## Current-state assessment

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

### Containerization implementation boundaries

The retirement of the legacy Docker implementation creates an intentional gap: the repository has no supported application image definitions until M3 and M4. This is expected and must not be filled by restoring or lightly editing the retired files.

| Milestone | Containerization work introduced | Primary repository artifacts |
|---|---|---|
| M0 | Record application, host, dependency, and historical container findings; do not restore or build the retired images as an acceptance requirement | Baseline evidence and roadmap updates only |
| M1 | Make future build inputs deterministic and build contexts intentional | Frontend/Python lock strategy, verified Go modules, `.dockerignore` files, supported builder versions |
| M2 | Establish application runtime contracts required by containers | Application configuration, service discovery, health/readiness, shutdown, retry, secrets, and data-root changes |
| M3 | Create the Go/React production image from scratch | New root `Dockerfile` for the `app` image and executable image verification |
| M4 | Create the PDF conversion production image from scratch | New `pdf2png/Dockerfile`, pinned Python restore, production WSGI runtime, and executable image verification |
| M5 | Introduce the first supported multi-container runtime model | New `compose.yaml` for `app`, `pdf2png`, and official PostgreSQL |
| M6 | Add containerized development behavior | `compose.dev.yaml`, watch/hot-reload configuration, and development cache/mount rules |
| M7 | Add image validation and publication automation | New CI image workflows, Buildx configuration if required, scanning, SBOM, provenance, and registry publication |
| M8 | Add the production deployment and host-operation layer | Production Compose overlay, reverse proxy/TLS, promotion, rollback, backup/restore, and host runbooks |

Fresh-start rule: historical Dockerfiles, image workflows, and BuildKit configuration may be inspected in Git history only to understand previous failures. They are not templates, supported commands, or acceptance artifacts. Any useful behavior must be re-justified against the current target architecture and implemented in the milestone that owns it.

## Delivery strategy

Changes should normally map to one milestone or a coherent subset of a milestone. Avoid a single pull request that combines dependency upgrades, application behavior changes, Docker builds, Compose, and CI publication.

Each milestone is complete only when its acceptance criteria have been executed and evidence is available in the pull request or delivery notes.

CI, security, documentation, and dependency maintenance are cross-cutting workstreams rather than activities deferred until their final milestone. No supported application Dockerfile exists before M3/M4, and no supported Compose topology exists before M5:

- M0 establishes the first repeatable validation commands and records baseline evidence.
- Every subsequent milestone adds its new checks to CI as soon as they become executable.
- M7 completes publication, attestations, multi-platform assembly, and enforcement policy; it does not introduce testing for the first time.
- Documentation and the progress/decision logs are updated in the same change that alters supported behavior.
- Legacy dependency upgrades are delivered as isolated application changes, but production release remains blocked by unresolved policy-level vulnerabilities or unsupported production runtimes.

### Program success measures

M0 records the non-image baseline, owners, measurement method, and unavailable-tool limitations. Image-specific measurements begin when the replacement images become executable in M3 and M4; Compose and host measurements follow in M5 and M8. Target values are approved before the related implementation milestone is considered complete. At minimum, track:

- clean and cached image build duration;
- final image sizes and layer composition;
- container startup-to-ready time;
- development edit-to-feedback time;
- idle and representative CPU/memory/PID usage;
- vulnerability counts by policy severity and exception expiry;
- backup age, restore duration, recovery-point objective (RPO), and recovery-time objective (RTO);
- deployment duration, smoke-test result, and rollback duration;
- host disk usage for images, volumes, build cache, and container logs.

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
- Record the material findings from the retired Dockerfiles and image workflow using Git history. Do not restore or build them as an M0 acceptance requirement; the first supported image measurements belong to M3 and M4.
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

- Create a new root multi-stage Dockerfile with named dependency, build, test, and runtime targets; do not copy or restore the retired backend Dockerfile.
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
- The new image definition has no runtime or build dependency on files from the retired Docker implementation.

### M4 — Production PDF service image

**Status:** Planned

**Goal:** Create from scratch an internal, hardened, observable PDF conversion service image.

**Deliverables:**

- Create a new `pdf2png/Dockerfile`; do not copy or restore the retired PDF Dockerfile.
- Upgrade to a supported Python base image.
- Install Poppler with minimal OS packages and remove package-manager metadata.
- Restore pinned Python dependencies separately from application source.
- Replace the Flask development server with a production WSGI server.
- Run under a non-root UID/GID.
- Use safe temporary files and deterministic cleanup on success, error, and cancellation.
- Validate request content and enforce an upload-size limit.
- Add a lightweight health endpoint and structured stdout/stderr logging.
- Define request, worker, and shutdown timeouts.

**Acceptance criteria:**

- The service image exposes only its documented application port and does not require public internet discovery; Compose network isolation and service-DNS integration are verified in M5.
- Malformed input returns a controlled client error rather than terminating a worker.
- Temporary files do not accumulate across requests.
- The container stops gracefully and passes its healthcheck as non-root.

### M5 — Compose integration and persistence

**Status:** Planned

**Goal:** Introduce the first supported Compose model and run the complete production-like stack with one documented command.

**Deliverables:**

- Add a base `compose.yaml` for `app`, `pdf2png`, and `db`.
- Use the official pinned PostgreSQL image.
- Add PostgreSQL and application-data named volumes.
- Use internal networks and publish only the application entrypoint.
- Add healthchecks and dependency conditions.
- Add restart behavior appropriate for single-host operation.
- Separate non-secret configuration from secrets.
- Add least-privilege runtime controls: `read_only`, `tmpfs`, `cap_drop`, `no-new-privileges`, and resource/PID limits where compatible.
- Validate the fully merged model with `docker compose config`; avoid `privileged`, host networking, Docker-socket mounts, and fixed `container_name` values unless a reviewed requirement exists.
- Define log-driver and rotation settings that prevent an unbounded container log from filling the host disk.
- Verify clean startup, restart, container recreation, and host reboot behavior.

**Acceptance criteria:**

- `docker compose up --build` reaches a healthy state on a clean machine.
- Recreating `app` and `db` containers preserves their respective data.
- PostgreSQL and `pdf2png` are not reachable through published host ports.
- The application reaches PostgreSQL and `pdf2png` through Compose service names on internal networks; no public PDF endpoint is used.
- Removing an application container does not remove user uploads.
- An intentional volume removal is clearly documented as destructive.
- A configuration validation check catches unresolved variables, invalid Compose structure, and unsafe production defaults before containers start.

### M6 — Development experience

**Status:** Planned

**Goal:** Provide a fast full-container workflow and a supported hybrid workflow.

**Deliverables:**

- Add an explicit `compose.dev.yaml`.
- Add frontend hot reload.
- Add Go hot reload or an equivalently fast rebuild loop.
- Use bind mounts or Compose Watch based on measured Windows/WSL behavior.
- Preserve container-local dependency caches with named volumes or BuildKit caches.
- Support an infrastructure-only mode for developers running Go and React from the host/IDE.
- Document debugging, test execution, database reset, logs, and common failure recovery.
- Measure clean start and edit-to-feedback latency for both full-container and hybrid workflows and document the supported fast path for Windows/WSL 2.

**Acceptance criteria:**

- A source edit produces a visible development update without rebuilding every service.
- Switching between full-container and hybrid development does not require changing committed configuration.
- Development credentials are clearly non-production and cannot be confused with production secrets.
- Raw Compose commands remain documented even if convenience wrappers are added.
- The agreed edit-to-feedback target is met without sharing host dependency directories such as `node_modules` into incompatible container platforms.

### M7 — CI, supply-chain security, and image publication

**Status:** Planned

**Goal:** Ensure every published image is tested, traceable, scanned, and reproducible.

**Deliverables:**

- Update Docker GitHub Actions and pin third-party actions according to repository policy.
- Add frontend, Go, and Python checks.
- Add Docker build checks and native-platform PR builds.
- Run container smoke tests in CI.
- Add registry-backed or GitHub Actions build cache.
- Add vulnerability scanning with an explicit severity/failure policy.
- Add secret scanning and dependency/license review with documented exception ownership and expiry.
- Publish semantic-version, commit-SHA, and controlled channel tags.
- Build `linux/amd64` and `linux/arm64` release images.
- Attach SBOM and build provenance to registry releases.
- Verify generated SBOM/provenance and retain the image digest as the release identity.
- Ensure build secrets use secret mounts rather than build arguments.
- Automate base-image, language-dependency, and GitHub Action update proposals and schedule rebuilds even when application source has not changed.
- Define registry naming, immutability, retention, and deletion/recovery policy.
- Review AGPL-3.0 source-availability requirements and third-party notices with the appropriate owner; make the corresponding source for released modifications discoverable as required by the approved compliance plan.
- Add keyless image signing after registry publication is stable, or record an approved time-bounded exception.

**Acceptance criteria:**

- Pull requests cannot publish production images.
- A failed test, smoke test, or policy-level vulnerability prevents release publication.
- A published digest can be traced to source revision, workflow, dependencies, and build metadata.
- Production deployment refers to an immutable version or digest, not only `latest`.
- The release process can verify attestations and signatures according to the adopted policy before promotion.
- License and source-availability checks have an accountable owner and do not rely on an undocumented assumption.

### M8 — Release promotion, production operations, and host hardening

**Status:** Planned

**Goal:** Promote a tested immutable release into a supportable, secure, and recoverable single-host deployment.

**Deliverables:**

- Add an explicit production Compose overlay.
- Add reverse proxy and TLS termination when the internal stack is stable.
- Define staging and production environments, protected deployment credentials, approval rules, and an audit trail.
- Promote the exact tested image digest between environments; never rebuild source separately for production.
- Add pre-deployment validation, post-deployment smoke tests, automatic failure detection, and a rehearsed rollback command.
- Define secret generation, storage, rotation, and incident replacement procedures.
- Define data classification, retention, deletion, access, and privacy controls for uploaded documents, generated assets, account data, backups, and logs.
- Define RPO, RTO, backup retention, encryption, integrity verification, and off-host/off-machine storage requirements.
- Implement and test application-consistent PostgreSQL and application-data backup and restore procedures.
- Document upgrade, rollback, and failed-migration recovery.
- Define Docker-host patching, firewall, SSH access, daemon socket protection, rootless/user-namespace decision, Docker data-root capacity, and Engine upgrade policy.
- Ensure the Docker API is not exposed insecurely and deployment identities receive only the access required by the chosen deployment method.
- Define restart policy, host boot behavior, and whether Docker live-restore is appropriate; do not combine conflicting restart supervisors.
- Configure log retention/rotation and integration points for external observability.
- Define basic service-level indicators: availability, latency, error rate, storage, and backup freshness.
- Alert on disk exhaustion risk, unhealthy/restarting services, TLS expiry, backup age/failure, and sustained resource saturation.
- Define registry outage, external-service outage, host loss, credential compromise, and disk-full runbooks.
- Run a recovery exercise from backups on a clean environment.

**Acceptance criteria:**

- The same immutable digest that passed release checks is deployed and can be rolled back without rebuilding it.
- Production deployment is protected by the approved environment and credential controls.
- Database and application files can be restored into a clean stack within the agreed RPO/RTO.
- Operators can identify unhealthy services using documented commands and health output.
- No production secret or persistent data depends solely on a container writable layer.
- Host reboot, Docker daemon restart/upgrade, disk-pressure behavior, and certificate renewal have documented and tested outcomes appropriate to the service objective.

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

These items require explicit capacity, availability, compliance, or organizational justification.

## Definition of Done for the initial program

The initial containerization program is complete when M0 through M8 are `Done` and all of the following are demonstrated:

- A clean checkout can build and run the complete stack from documented commands.
- Application images are reproducible, non-root, minimal, and pass policy-level vulnerability checks.
- The frontend embedded in an image is built from the same source revision as the backend.
- PostgreSQL and application files survive container replacement.
- Backup restoration has been exercised, not merely documented.
- Shutdown, restart, unhealthy dependency, and rollback behavior have been tested.
- CI publishes immutable, traceable multi-platform images with SBOM and provenance.
- The exact tested digest is promoted through protected environments and can be verified before deployment.
- Development supports both a fast full-container workflow and an infrastructure-only hybrid workflow.
- Operations documentation is sufficient for an engineer unfamiliar with the implementation to deploy, diagnose, update, and restore it.
- The production host has explicit access, patching, firewall, logging, disk-capacity, daemon, and reboot policies.
- RPO/RTO, encrypted off-host backup retention, and application-consistent restore behavior are approved and exercised.
- External runtime dependencies and their degraded behavior are documented and observable.
- AGPL source-availability and third-party license obligations have an accountable, approved compliance path.

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
| Large multi-platform builds slow every PR | Slow feedback and high CI cost | Native build on PR; multi-platform build on merge/release |
| Data volume exists but cannot be restored | False sense of recoverability | Make restore exercise a release criterion |
| Database and filesystem backups are taken at inconsistent points | Restored metadata references missing or mismatched files | Define an application-consistent backup sequence and test full-system restore |
| External APIs, SMTP, fonts, or remote images are unavailable | Partial feature failure, latency, or broken UI | Inventory dependencies; apply bounded timeouts; define degraded behavior and observability |
| Docker socket or remote daemon access is compromised | Root-equivalent host compromise | Restrict trusted users; avoid socket mounts; use protected SSH/TLS access only when required; evaluate rootless/user namespaces |
| Container logs, images, cache, or volumes fill the host disk | Full service outage or database corruption | Configure log rotation; monitor data root and volume growth; define safe cleanup and capacity procedures |
| Production is rebuilt rather than promoted | Untested artifact reaches users | Promote the exact CI-produced digest through protected environments |
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
