# SheetAble: containerization and operational readiness roadmap

## Document purpose

This is the living delivery plan for turning SheetAble into a reproducible, secure, and operable containerized application. It is intended to be readable by engineering leadership, application developers, DevOps engineers, security reviewers, and operators.

The plan deliberately separates discovery, application prerequisites, image construction, local orchestration, CI/CD, and production hardening. Work should be delivered in small reviewable changes, with every completed milestone backed by executable verification.

## Status

| Field | Value |
|---|---|
| Overall status | Planned |
| Current milestone | M0 — baseline and discovery |
| Initial delivery target | Docker Compose on a single Linux host |
| Development platform | Windows/WSL 2 with Docker Desktop, plus Linux compatibility |
| Runtime platforms | `linux/amd64`, `linux/arm64` |
| Primary database | PostgreSQL |
| Last updated | 2026-09-23 |

Status values used in this document: `Planned`, `In progress`, `Blocked`, `Done`.

## Executive summary

SheetAble has partial, outdated containerization. The Go backend and Python PDF service have Dockerfiles, but the full application cannot be built and operated reproducibly as one system. The frontend is not rebuilt by the current container pipeline, the backend depends on a hard-coded external PDF service, persistence is implicit, the default configuration is unsafe for production, and CI publishes an incompletely assembled image.

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

1. The current GitHub Actions workflow builds with `context: ./backend`; it cannot construct the frontend bundle.
2. `rice-box.go` contains generated frontend content, creating a stale-artifact risk.
3. `frontend/package-lock.json` is absent; `npm install` is not deterministic.
4. The backend Dockerfile copies the source redundantly, invalidates useful cache, runs as root, uses mutable base tags, and documents the wrong port.
5. The PDF Dockerfile uses an obsolete Python/Debian base, unpinned packages, the Flask development server, and root execution.
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

## Delivery strategy

Changes should normally map to one milestone or a coherent subset of a milestone. Avoid a single pull request that combines dependency upgrades, application behavior changes, Docker builds, Compose, and CI publication.

Each milestone is complete only when its acceptance criteria have been executed and evidence is available in the pull request or delivery notes.

CI, security, documentation, and dependency maintenance are cross-cutting workstreams rather than activities deferred until their final milestone:

- M0 establishes the first repeatable validation commands and records baseline evidence.
- Every subsequent milestone adds its new checks to CI as soon as they become executable.
- M7 completes publication, attestations, multi-platform assembly, and enforcement policy; it does not introduce testing for the first time.
- Documentation and the progress/decision logs are updated in the same change that alters supported behavior.
- Legacy dependency upgrades are delivered as isolated application changes, but production release remains blocked by unresolved policy-level vulnerabilities or unsupported production runtimes.

### Program success measures

M0 records baselines and owners; target values are approved before the related implementation milestone is considered complete. At minimum, track:

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

**Status:** Planned

**Goal:** Establish a known-good behavioral and build baseline before changing the runtime model.

**Deliverables:**

- Record Docker Engine, Compose, Buildx, host, and architecture information.
- Run the existing Go tests and record failures.
- Determine a compatible Node version and reproduce the current frontend build.
- Run or characterize the current Python service.
- Build the existing images without silently fixing them and record results, sizes, users, ports, and vulnerabilities.
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
- Baseline build, startup, resource, image-size, and security measurements are recorded where tooling permits.

### M1 — Reproducible dependencies and repository hygiene

**Status:** Planned

**Goal:** Make dependency restoration deterministic and build contexts intentional.

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

**Status:** Planned

**Goal:** Remove application behaviors that prevent reliable container operation.

**Deliverables:**

- Add a configurable `PDF2PNG_URL` and use Compose DNS rather than a public hard-coded endpoint.
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

- The application starts using service DNS names on a container network.
- Temporary database unavailability does not create an uncontrolled restart loop.
- `docker stop` results in a graceful exit within the configured timeout.
- Health endpoints have documented semantics and automated tests.
- Production startup cannot silently use repository default credentials.
- Optional external-service failures do not make unrelated read-only application paths unavailable, and required dependency failures are observable.

### M3 — Production application image

**Status:** Planned

**Goal:** Produce one minimal, repeatable application image containing the current React bundle and Go server.

**Deliverables:**

- Add a root multi-stage Dockerfile with named dependency, build, test, and runtime targets.
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
- Supported target-platform images are exercised on their target architecture or an approved equivalent before release.

### M4 — Production PDF service image

**Status:** Planned

**Goal:** Make PDF conversion an internal, hardened, observable service.

**Deliverables:**

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

- The backend reaches the service only through internal service discovery.
- Malformed input returns a controlled client error rather than terminating a worker.
- Temporary files do not accumulate across requests.
- The container stops gracefully and passes its healthcheck as non-root.

### M5 — Compose integration and persistence

**Status:** Planned

**Goal:** Run the complete production-like stack with one documented Compose command.

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

## Progress log

| Date | Milestone | Update |
|---|---|---|
| 2026-09-23 | Planning | Created the initial containerization and operational-readiness roadmap |
| 2026-09-23 | Planning | Reworked the repository README to document the current architecture, modernization status, safety notice, and roadmap entry point without publishing unverified setup commands |
| 2026-09-23 | Planning review | Expanded cross-cutting CI, external-dependency inventory, delivery promotion, host hardening, log/disk controls, RPO/RTO, backup consistency, and license-compliance coverage after a formal roadmap review |
| 2026-09-23 | Pre-M0 guardrails | Added repository-wide secret/local-artifact ignore rules and explicit cross-platform line-ending policy without performing a noisy bulk renormalization |
