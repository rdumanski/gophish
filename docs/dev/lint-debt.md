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
- **Phase 5** (auth + IMAP backoff): addresses errcheck around login/IMAP code (csrf v1.7.x re-bumped in Phase 5a — see below).
- **Phase 6** (plugin architecture + API v2): cleans up unused functions and reorganizes API package.

### Phase 3 progress (commit on phase-3b-gorm-v2)

- **ST1005**: 31 of 32 capitalized error literals in `models/*.go` lowercased (campaign, group, page, smtp, template, imap, webhook, user). `webhook.ErrURLNotSpecified` retained capitalized "URL" (acronym, ST1005 allows). Remaining ST1005 hits live in packages not touched by this phase (auth, controllers/api JSON response strings are not Go errors — exempt).
- **errcheck on DB calls**: deferred. Wrapping every unchecked `db.*` write/delete in `models/` with explicit error handling would expand this PR significantly past the scope of "GORM v1 → v2". Fold into Phase 5 (auth/IMAP errcheck pass) per existing plan.
- **Other linters**: untouched in 3b — all findings remain owned by their listed phase.

### Phase 4d TypeScript debt (2026-05-03)

All 13 app files now typecheck cleanly under the existing
`tsconfig.json` (no `// @ts-nocheck` pragmas remain). The previous
307 errors were resolved by:

- Annotating object literals that get fields added later as
  `: any` (e.g. `var page: any = {}` in landing_pages, similar in
  campaigns / templates / sending_profiles / webhooks / users).
- Adding loose `JQuery` plugin type augmentations in
  `static/js/types/global.d.ts` for `ckeditor`, `datetimepicker`,
  `highcharts`, plus widening `select2`/`fileupload`/`dataTable` to
  varargs/any returns to cover their many call shapes.
- Declaring previously-implicit-global vars: `let userRows`,
  `let lastlogin`, `let pagesTable`, `let pageRows`, `let groupRows`,
  etc. (~15 sites). Several were latent bugs that ES5 implicit
  globals had been masking.
- Replacing `.attr("disabled", boolean)` (no-op in modern jQuery)
  with `.prop("disabled", boolean)` in users.ts and settings.ts —
  actual upstream bug fixed in passing.
- Removed duplicate `deleteTemplate` function in templates.ts (one
  Swal-based, one basic confirm; the latter was dead code per
  upstream hoisting).
- `new Promise(...)` → `new Promise<void>(...)` in 7 SweetAlert2
  preConfirm callbacks.
- Various per-call-site casts (`(navigator as any).msSaveBlob`,
  `(this as HTMLInputElement).value`, `(reader.result as string)`)
  for IE-only / DOM-narrowing APIs.

Phase 4d also kept the document.ready setup and inline-handler
window exports unchanged. The build still produces the same per-page
IIFE bundles at the same URLs.

### Phase 4c TypeScript debt (2026-05-03)

After the JS → TS rename, `tsc --noEmit` initially reported 498 errors
across 13 files (mostly upstream loose-typed jQuery + DataTables plugin
calls, deprecated `.success/.error` Deferred shims, top-level vars
with no declarations, mixed types from `$().val()`). Phase 4c added
ambient declarations for window globals + `JQuery.jqXHR.success/error`
fluent shims, which dropped the error count to **307 across 10 files**.
Those 10 files now ship a `// @ts-nocheck` pragma so the typecheck
passes; the four files that already typecheck cleanly are
`autocomplete.ts`, `common.ts`, `gophish.ts`, `passwords.ts`.

Per-file remaining error counts (drop these by removing
`@ts-nocheck` and fixing what surfaces):

| file | errors |
|---|---|
| campaign_results.ts | 122 |
| settings.ts | 58 |
| templates.ts | 56 |
| sending_profiles.ts | 42 |
| campaigns.ts | 34 |
| landing_pages.ts | 26 |
| groups.ts | 18 |
| users.ts | 10 |
| dashboard.ts | 9 |
| webhooks.ts | 2 |

Recurring patterns to address as we touch each file:
- Undeclared top-level vars (`var x = ...` missing in places — actual
  bugs masked by ES5 implicit globals)
- DataTables row-buttons constructed as HTML strings reference fields
  on `{}` literals before they're populated
- `$().val(true)` / `$().val(false)` for checkboxes — should use
  `$().prop('checked', bool)`
- `new Promise()` without an executor argument inside SweetAlert2
  `preConfirm` blocks

### Phase 5d gophish/gomail vendored to internal/gomail/ (2026-05-06)

`github.com/gophish/gomail` was a 2020-vintage fork of the long-dormant
`go-gomail/gomail` (last upstream commit 2017). With both repos effectively
dead and only two Gophish-specific extensions worth keeping (`NewWithDialer`
for the SSRF-safe outbound dialer, `SendCustomFrom` for envelope-from
phishing tracking), the cleanest fix was to **vendor** the fork into
`internal/gomail/` and drop the external dependency. See
`internal/gomail/VENDORED.md` for full provenance and edits.

Local edits during vendoring:

- Removed `mime_go14.go` (Go 1.4 quoted-printable shim, gated by
  `// +build !go1.5`). Dead code at our Go 1.25 floor; this also retires
  the only remaining transitive dependency
  (`gopkg.in/alexcesaro/quotedprintable.v3`, last touched 2015).
- Dropped the `// +build go1.5` tag from `mime.go` (no longer conditional).
- Skipped `example_test.go` (`package gomail_test`, doc-only examples).
- Updated `doc.go` to record vendoring provenance.

`.golangci.yml` was extended to exclude `internal/gomail/` from `errcheck`,
`staticcheck`, `revive`, and `gosec`. The vendored package carries 15
upstream-style findings (mostly unchecked `Close` calls); patching them
would defeat the point of vendoring (preserve upstream byte-identical so
any future diffing against an upstream PR is mechanical). If those findings
ever become a problem the path forward is **replacement** with a maintained
library (e.g. `wneessen/go-mail`), not piecemeal patching.

Repository-wide finding count holds at 117 thanks to the exclusion.

### Phase 5c errcheck + miscellaneous burn-down on models/ (2026-05-06)

Cleared every lint finding in `models/`:

- **errcheck (9 sites)**:
  - `attachment.go` × 6 — `b.ReadFrom` now propagates the read error;
    `defer ff.Close()` (read-only) wrapped in an explicit `_ =` discard;
    the four `zipWriter.Close()` calls in error paths are explicit
    discards (Close errors are dominated by the original error being
    returned), and the final success-path `Close` now propagates so a
    truncated archive (Close failure flushing the central directory)
    surfaces correctly.
  - `maillog.go` — `r.HandleEmailError(...)` in `Backoff` logs at error
    level if the result-event update fails. The original send error
    still wins as the function's return value.
  - `result.go` × 2 — `AddEvent(...)` now propagates from `createEvent`
    so callers don't silently lose campaign-event writes; `mmdb.Close()`
    (read-only maxminddb handle) wrapped in `_ =`.
- **ineffassign (1 site)**: `email_request_test.go` — added the missing
  `ch.Assert(err, ...)` between two `err :=` lines.
- **staticcheck (3 sites)**:
  - `attachment.go` — `if a.vanillaFile == true` → `if a.vanillaFile`
    (S1002).
  - `imap.go` — dropped the dead `im.Port > 65535` check; `Port` is a
    `uint16` so the upper bound is statically guaranteed (SA4003).
  - `smtp.go` — `validateFromAddress` regex switched to a raw string
    literal (S1007).
- **gosec (G402)**: `models.SMTP.GetDialer` sets
  `tls.Config.InsecureSkipVerify` from the user-controlled
  `IgnoreCertErrors` SMTP profile flag. This is intentional (admin opt-in
  for self-signed dev/staging relays). Annotated inline with
  `//nolint:gosec` and a comment explaining the design rather than
  blanket-excluding G402 in `.golangci.yml` — keeps the linter useful for
  any future TLS misconfiguration elsewhere in the codebase.

After this PR `golangci-lint run ./models/...` reports **0 issues**.
Repository-wide floor moves from **137 → 117** findings (Phase 3c
baseline → post-5c).

### Phase 5b errcheck pass on auth/ + imap/ (2026-05-06)

Burns down all errcheck findings in `auth/` and `imap/`:

- **`auth.GenerateSecureKey`**: signature changed from `func(int) string` to
  `func(int) (string, error)` to match `auth.GeneratePasswordHash` style and
  surface `crypto/rand` failures instead of silently emitting all-zero keys
  (the previous `io.ReadFull(rand.Reader, k)` discarded the error). Five
  callers updated: `controllers/route.go` (log.Fatalf at startup),
  `controllers/api/reset.go` + `controllers/api/user.go` (HTTP 500), and
  `models/models.go` x2 (return wrapped error from `createTemporaryPassword`
  / admin-bootstrap).
- **`models.generateSecureKey`**: dead code (was the duplicate carried for a
  no-longer-needed cyclic-import workaround). Removed along with the now-unused
  `crypto/rand` + `io` imports. This also retires one of the three `unused`
  findings from the Phase 2 baseline.
- **`imap.imap.go` Logout deferrals**: introduced a `logoutClient` helper that
  logs cleanup errors at error level. Replaces 4 unchecked `imapClient.Logout()`
  call sites in `Validate`, `MarkAsUnread`, `DeleteEmails`, `GetUnread`.
- **`imap.imap.go` body Read**: `value.Read(buf)` (silently dropping error and
  short-read) replaced with `io.ReadFull(value, buf)` + error check.
- **`imap.monitor.go` `SuccessfulLogin`**: explicit `_ = ...` discard with a
  comment noting that the model layer already logs DB errors internally —
  acknowledges errcheck without double-logging.

Errors now surface where they matter (fatal at startup, HTTP 500 for API
clients, logged at error level for IMAP cleanup) and silent failures around
secure-key generation are gone.

### Phase 5a csrf re-bump (2026-05-06)

`github.com/gorilla/csrf` was un-pinned and bumped from **v1.6.2 →
v1.7.3** (latest, picks up CVE-2025-24358). The v1.7 line introduces
context-driven scheme detection: the middleware now defaults to
"assume HTTPS" and enforces a strict Referer check on every
state-changing POST unless the request context carries
`csrf.PlaintextHTTPContextKey=true`. The previous v1.6.2 only ran the
strict check when `r.URL.Scheme == "https"`, which is always empty for
server-side requests — so the check was effectively dead code on the
server, masking the issue.

The fix lives in `controllers/route.go`: when `as.config.UseTLS` is
false, the admin handler is wrapped to call `csrf.PlaintextHTTPRequest(r)`
before the csrf middleware runs. This restores the previous behavior
for plain-HTTP deployments (e.g. behind a TLS-terminating reverse
proxy) and keeps strict Referer enforcement intact when TLS is enabled
in-process. The four controllers tests that used to fail with 403
(`TestInvalidCredentials`, `TestSuccessfulLogin`,
`TestSuccessfulRedirect`, `TestAccountLocked`) now pass without
modification because the test config defaults to `UseTLS=false`.

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
