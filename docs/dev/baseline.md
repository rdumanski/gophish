# Phase 1 Baseline Metrics

Captured **2026-05-01** against commit `ab76477` (Phase 0 merge to master) on:

| Field | Value |
|---|---|
| Host OS | Windows 11 Pro 10.0.26200 |
| Build environment | Docker Desktop 29.2.1 |
| Host CPU | (record on first run) |
| Host RAM | (record on first run) |
| Network | (record bandwidth notes if relevant) |

These numbers are **guardrails** for Phases 2 (Go modernization), 3 (GORM v2), and 4 (Vite frontend). If any future phase regresses a metric by more than ~2×, investigate before merging.

## Build metrics

### Cold Docker build (no cache)

| Stage | Time | Notes |
|---|---|---|
| `build-js` `npm install --only=dev` | **82.2 s** | 745 packages, **47 vulnerabilities** (2 low, 15 moderate, 21 high, 9 critical) — Phase 4 will replace this stack |
| `build-js` `gulp` | (folded into total) | Webpack 4 + Gulp 4, ES5 transpile |
| `build-golang` `go get -v && go build -v` | **34.6 s** | Go 1.15.2 in container, CGO enabled (sqlite3) |
| Image export + unpack | 4.5 s | |
| **Total** | **2 min 35.6 s** | |

### Image size

- **Compressed**: 103 MB
- **On-disk uncompressed**: 326 MB
- Largest layer: 103 MB (debian:stable-slim base + apt installs: jq, libcap2-bin, ca-certificates)

### Startup time (Docker)

- `docker run` to "Starting admin server" log line: **< 1 second** after migrations complete
- Goose migrations on fresh DB: ~0.3 s for 27 migration files
- Total cold start (run → ready to serve): **~1.5 s**

### Admin server reachability

- `https://localhost:3333` → HTTP 307 (redirect to login) — admin server up and reachable
- TLS handshake: self-signed cert generated on first run (logged: "Creating new self-signed certificates")

## Native Windows build

**Phase 3c update (2026-05-02)**: native Windows build now works without
gcc. Phase 3c swapped `gorm.io/driver/sqlite`'s default `mattn/go-sqlite3`
backend to `modernc.org/sqlite` (pure Go) by pinning the gorm dialect's
`Config.DriverName` to `"sqlite"`. The Dockerfile's build stage now sets
`CGO_ENABLED=0` and the resulting binary is statically linked.

Local validation (Windows 11, Go 1.25, no gcc):
- `CGO_ENABLED=0 go build ./...` — green
- `CGO_ENABLED=0 go test ./...` — every package passes
- `CGO_ENABLED=0 golangci-lint run` — runs to completion (was previously
  blocked by mattn's CGO type-check failure, see lint-debt.md)

## Frontend metrics

Captured indirectly through the Docker `build-js` stage. Standalone `npm install && gulp` on the host has not been run — Phase 4's Vite migration will provide a clean comparison point.

| Metric | Value |
|---|---|
| `package.json` total deps (transitive) | 745 |
| `npm audit` vulnerabilities | 47 (9 critical) |
| Tooling versions | gulp 4.x, webpack 4.32, babel 7.4, node:latest |

## Smoke test status

- ✅ Docker build succeeds
- ✅ Container starts without error
- ✅ Admin server responds on `https://localhost:3333`
- ⏭️ End-to-end campaign smoke test (MailHog → send → open → click) — not yet run; deferred to first manual verification by the developer per [smoke-test.md](./smoke-test.md)

## Phase-1 fixes applied during baseline capture

The first attempt to run the container failed with `exec ./docker/run.sh: no such file or directory`. Root cause: Windows Git's `core.autocrlf=true` produced **CRLF line endings** on `docker/run.sh`, so the `#!/bin/bash` shebang was interpreted as `/bin/bash\r` inside the Linux container. Fix:

- Added `*.sh text eol=lf` and `Dockerfile text eol=lf` rules to `.gitattributes`.
- Re-converted `docker/run.sh` and `Dockerfile` to LF locally.
- Documented in [build-windows.md](./build-windows.md) under "Known issues on Windows".

This bug had been latent since the original 2013 Gophish codebase but only surfaces when the repo is cloned on Windows AND built via Docker — apparently not part of upstream's CI matrix.

## Sources of truth for these numbers

- Docker build log: build was run with `time docker build -t gophish:0.13.0-dev .` — the `real` time line at the end is authoritative.
- Image size: `docker images gophish:0.13.0-dev --format "{{.Size}}"`
- Startup logs: `docker run --rm -d ...` then `docker logs <container>` — timestamps are container-local UTC.

To re-baseline after a phase, repeat the same commands on a clean Docker cache (`docker system prune -af` first).
