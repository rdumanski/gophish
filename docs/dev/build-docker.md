# Build via Docker (recommended for Phase 1)

The repository ships with a multi-stage `Dockerfile` that builds:
1. **build-js** — Node-based stage: `npm install --only=dev` + `gulp` to produce minified JS/CSS bundles.
2. **build-golang** — Go 1.15.2 stage: `go get -v && go build -v` to produce the `gophish` binary.
3. **runtime** — `debian:stable-slim` with the binary, minified assets, and `cap_net_bind_service` so it can bind port 80 as a non-root user.

> **Note:** The Dockerfile still uses Go 1.15.2. This will be bumped to Go 1.22+ in Phase 2 of the modernization roadmap. For Phase 1 baseline measurement, we deliberately build with the original toolchain to capture an authentic "before modernization" baseline.

## Prerequisites

- **Docker Desktop** (or any Docker engine) running.
- **No Go or Node toolchain on the host required** — everything happens inside the build stages.

## Build

```bash
docker build -t gophish:0.13.0-dev .
```

Or via Taskfile:

```bash
task docker-build
```

A cold build (no cache) takes roughly **5–15 minutes** depending on bandwidth (most of that time is `npm install` pulling Webpack/Gulp/Babel).

## Run

```bash
docker run --rm -p 3333:3333 -p 8080:8080 -p 8443:8443 gophish:0.13.0-dev
```

Or:

```bash
task docker-run
```

The container's `config.json` is patched at build time to listen on `0.0.0.0` (instead of `127.0.0.1`), so the admin UI is reachable from the host on `https://localhost:3333`.

The first-run admin password is printed to the container's stdout.

## Limitations

- The Dockerfile builds against the **fork's master**, so any changes you want to test must be committed (or copied into the build context) before `docker build`.
- The container runs as a non-root `app` user; if you mount volumes for persistence (`/opt/gophish/gophish.db`), make sure the host directory is writable by the container's `app` UID (1000 by default).
- This image is **not** the upstream Gophish image at `gophish/gophish` on Docker Hub — that image is built from upstream and not affected by this fork.

## Phase 2 Dockerfile changes (preview)

When Phase 2 lands, the Dockerfile will:
- Bump `golang:1.15.2` → `golang:1.22-bookworm`.
- Replace `go get -v && go build -v` with `go build -v ./...` (Go 1.16+ deprecates `go get` for installing).
- Optionally drop CGO if we migrate to `modernc.org/sqlite`.

When Phase 4 lands, the `build-js` stage will switch from Gulp + Webpack to Vite.
