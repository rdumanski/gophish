# Lint Debt Log

Tracks accepted lint-rule suppressions and any findings deferred until a later phase.

## Status

`.golangci.yml` is in place with the safe linter set (`errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gosec`, `revive`).

The first authoritative lint baseline is produced by **CI** (see `.github/workflows/ci.yml`), because `golangci-lint` requires CGO to type-check `models/models.go` (transitively through goose → mattn/go-sqlite3). Local lint runs on Windows without a C toolchain fail with:

```
typechecking error: could not import bitbucket.org/liamstask/goose/lib/goose
(-: dialect.go:119:15: undefined: sqlite3.Error)
```

Phase 3 replaces goose with golang-migrate and evaluates `modernc.org/sqlite`; once that lands, local lint runs will work without CGO.

## Currently suppressed rules (in `.golangci.yml`)

| Linter | Rule | Why |
|---|---|---|
| gosec | G104 | Overlaps with `errcheck`; redundant noise |
| gosec | G304 | False positives on template/fixture loaders that read paths from config |
| gosec | G404 | Codebase uses `crypto/rand` for security; `math/rand` only in non-security paths |
| revive | var-naming | Legacy `ID`/`URL`/`SMTP` field naming throughout — cleanup is its own phase |
| revive | package-comments | Many files lack package-level docstrings; cosmetic-only |
| revive | exported | Public types missing godoc on exported fields; would generate ~hundreds of findings |
| revive | unused-parameter | Common in interface-satisfying methods (mailer, plugin handlers) |

## Deferred-to-future-phase findings

| Phase target | Item |
|---|---|
| Phase 3 (GORM v2) | Replace `err == gorm.ErrRecordNotFound` with `errors.Is(err, gorm.ErrRecordNotFound)` — v2 wraps errors, so direct `==` will silently break |
| Phase 5 (gomail) | Audit `mailer/`, `models/email_request.go`, `models/maillog.go`, `models/smtp.go` for the gomail fork's local code — vendor-or-replace decision |
| Phase 6 (plugin API) | Naming cleanup of legacy stutters once package boundaries change |

## Re-running lint locally

When the CGO requirement is removed (Phase 3) or you have gcc on `PATH`:

```bash
golangci-lint run --timeout 5m
```

Or via the Taskfile:

```bash
task lint
```
