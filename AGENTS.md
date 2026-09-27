# SheetAble containerization project

## Project context

SheetAble is a legacy self-hosted application composed of:

- a React frontend;
- a Go HTTP API that also serves the compiled frontend through `go.rice`;
- a Python/Flask PDF-to-PNG service backed by Poppler;
- SQLite by default, with PostgreSQL and MySQL support.

We are treating this repository as a real company project being prepared for reliable development, CI, and production operation. The learning format is senior DevOps mentor plus junior DevOps engineer: explain important decisions and trade-offs instead of applying opaque templates.

The detailed delivery plan and current status live in `docs/containerization-roadmap.md`. Read that file before starting planned infrastructure work and update its status when a milestone changes.

## Target architecture

- Build the React frontend and Go backend into one production `app` image. React is a static build artifact, not a separate production process.
- Run `pdf2png` as a separate internal service.
- Use the official PostgreSQL image; do not create a custom database image without a concrete requirement.
- Use Docker Compose for local integration, development, and the initial single-host production baseline.
- Publish only the application entrypoint. Keep PostgreSQL and `pdf2png` on internal networks.
- Persist PostgreSQL data and user-generated application files outside container writable layers.
- Keep the images portable so a later orchestrator migration does not require rebuilding the application architecture.

## Engineering principles

- Prefer small, reviewable, reversible changes. Do not perform a big-bang rewrite.
- Establish a working baseline before changing build or runtime behavior.
- Separate application modernization from containerization unless a code change is required for correct container operation.
- Optimize for reproducibility, security, operability, and developer feedback speed, in that order.
- Pin application dependencies and base-image versions. Use immutable digests in release automation when maintainable update automation is in place.
- Use multi-stage builds and BuildKit cache mounts. Keep build contexts small with `.dockerignore` files.
- Production images must not contain compilers, development servers, source trees, package-manager caches, or debugging tools unless explicitly justified.
- Run production processes as non-root and apply least privilege. Prefer read-only root filesystems and explicitly declared writable mounts or `tmpfs` locations.
- Never commit real secrets or place secrets in Docker build arguments, image layers, or generated documentation.
- Treat healthchecks as contracts: distinguish process liveness from dependency-aware readiness.
- Application processes must handle `SIGTERM` and shut down gracefully.
- Log to stdout/stderr. Do not rely on mutable log files inside containers.
- Test data persistence, backup/restore, clean startup, upgrade, and rollback behavior rather than assuming they work.
- Keep raw Docker and Compose commands documented even if convenience wrappers are added.
- Do not add Kubernetes, Helm, Terraform, a service mesh, or an observability stack until the roadmap reaches that decision point and there is a demonstrated requirement.

## Known issues that affect the containerization work

- The retired legacy backend Docker build used only `./backend` as its context and did not rebuild the frontend. Its material findings are recorded in the roadmap and its pre-removal state remains in Git history.
- `backend/api/controllers/rice-box.go` is a generated embedded frontend artifact and can become stale.
- `frontend/package-lock.json` is missing, so frontend installation is not reproducible.
- The retired legacy backend Dockerfile exposed port 8000 while the application defaults to 8080.
- The retired legacy images ran production processes as root and used unpinned or obsolete base images.
- The backend hard-codes the public PDF service URL and disables TLS verification in related HTTP code.
- The backend exits immediately on database connection failure and does not implement graceful shutdown.
- Default admin credentials and API secrets are unsafe for production.
- User PDFs, thumbnails, portraits, and SQLite data are written beneath `CONFIG_PATH`; these paths require an explicit persistence strategy.
- The retired legacy GitHub Actions Docker workflow used old action versions and built only the backend context. New container automation must be designed from the verified build graph rather than copied from it.
- Several application dependencies are legacy. Upgrade them in isolated, tested work items rather than silently while authoring the replacement Dockerfiles.

## Working agreement

For each roadmap block:

1. State the problem and acceptance criteria.
2. Inspect the relevant source and current behavior.
3. Explain the proposed design and alternatives.
4. Implement the smallest coherent change.
5. Run proportionate tests, builds, and security/operability checks.
6. Record material decisions and update the roadmap status.
7. Report remaining risks and the next small block.

Do not claim a containerization milestone complete if it has only been written but not built and exercised. If Docker is unavailable to the current process, record that limitation and provide the exact verification command rather than treating an untested artifact as verified.
