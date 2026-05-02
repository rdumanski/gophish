# Lint Debt Log

Tracks accepted lint-rule suppressions and any findings deferred until a later phase.

## Status

`.golangci.yml` is in place with the safe linter set (`errcheck`, `govet`, `ineffassign`, `staticcheck`, `unused`, `gosec`, `revive`). CI uses **golangci-lint v2.12.0** (must be built with Go 1.25+ to typecheck our `go 1.25.0` go.mod).

## Phase 2 baseline (commit fa57a0b on PR #3)

The first authoritative lint run produced **164 findings** carried over from upstream Gophish:

| Linter | Count | Dominant pattern |
|---|---|---|
| errcheck | 75 | unchecked `log.*`, `Close()`, and DB cleanup calls |
| staticcheck | 74 | mostly **ST1005** (capitalized error strings in `models/*.go`); a few **S1007** (un-raw regex), **SA4003** (`uint16 > 65535` is always false) |
| gosec | 8 | needs case-by-case review |
| ineffassign | 4 | dead writes |
| unused | 3 | `dialer.restrictedDialer`, `models.generateSecureKey`, `controllers/api.createTestData` |

The plan called this out as the expected "lint avalanche": rules will be burned down **incrementally as each package is touched**, not in a single sweep. The CI lint job runs but currently has `continue-on-error: true` so it does not gate merges. Once findings are below ~20, switch lint to blocking.

### Burn-down strategy

- **Phase 3** (GORM v2 + golang-migrate): touches every model file → fix ST1005 + errcheck on DB calls there.
- **Phase 5** (auth + IMAP backoff): re-enables csrf v1.7.x and addresses errcheck around login/IMAP code.
- **Phase 6** (plugin architecture + API v2): cleans up unused functions and reorganizes API package.

### Phase 3 progress (commit on phase-3b-gorm-v2)

- **ST1005**: 31 of 32 capitalized error literals in `models/*.go` lowercased (campaign, group, page, smtp, template, imap, webhook, user). `webhook.ErrURLNotSpecified` retained capitalized "URL" (acronym, ST1005 allows). Remaining ST1005 hits live in packages not touched by this phase (auth, controllers/api JSON response strings are not Go errors — exempt).
- **errcheck on DB calls**: deferred. Wrapping every unchecked `db.*` write/delete in `models/` with explicit error handling would expand this PR significantly past the scope of "GORM v1 → v2". Fold into Phase 5 (auth/IMAP errcheck pass) per existing plan.
- **Other linters**: untouched in 3b — all findings remain owned by their listed phase.

**Phase 3c update (2026-05-02)**: local lint now works without CGO. The
sqlite driver was swapped to `modernc.org/sqlite` (pure Go) and
`gorm.io/driver/sqlite` was reconfigured via `Config.DriverName: "sqlite"`
to consume it. Running `CGO_ENABLED=0 golangci-lint run --timeout 5m` on
Windows now completes successfully. The first post-3c local run reports
**137 findings** (down from the Phase 2 baseline of 164, mostly via the
ST1005 burn-down that landed with 3b).

Historical note: before Phase 3c, local lint runs on Windows without a C
toolchain failed with:

```
typechecking error: could not import bitbucket.org/liamstask/goose/lib/goose
(-: dialect.go:119:15: undefined: sqlite3.Error)
```

Phase 3a replaced goose with golang-migrate and Phase 3c replaced the
sqlite driver, finally removing the CGO requirement.

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
