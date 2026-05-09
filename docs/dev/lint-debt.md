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

### Phase 7c sandbox click filtering (2026-05-08)

Adds suppression of sandbox-driven opens/clicks (Microsoft Defender
Safe Links, Proofpoint URL Defense, etc.) so campaign metrics aren't
polluted by vendor pre-scans. Two filters, both off by default:

- **Time threshold** — events firing within
  `phish_server.sandbox_filter.min_click_seconds` of `Result.SendDate`
  are recorded as audit-only events without bumping `Result.Status`.
- **Source-IP allowlist (negative)** — `phish_server.sandbox_filter.sandbox_ips`
  takes a list of CIDR ranges or bare IPs; matches are filtered the
  same way.

Audit events use new `EventOpenedSandboxFiltered` /
`EventClickedSandboxFiltered` constants and write a `sandbox_reason`
field into `EventDetails.Browser` so admins can see WHY each filtered
event was suppressed in the per-result timeline. Campaign summary
aggregations (`models/campaign.go:278-288`) already filter by
`Result.Status` so filtered events naturally don't count toward the
"Opened" / "Clicked" totals.

- **`config/config.go`** — new `PhishFilterConfig` struct on
  `config.PhishServer` with `min_click_seconds int` + `sandbox_ips
  []string`. Zero values disable filtering (backward-compatible).
- **`controllers/phish.go`** — `sandboxMatcher` parses the operator's
  CIDRs at startup (bare IPs promoted to /32 or /128); invalid entries
  fail the process via `log.Fatal` so misconfigured allowlists surface
  immediately. `TrackHandler` and the GET branch of `PhishHandler`
  call `matcher.classify(rs.SendDate, ip)` and route to the filtered
  variant when matched. The response body is identical to the
  non-filtered path so sandboxes don't see different behavior.
- **`models/result.go`** — two new `Handle*Filtered` methods that
  reuse `createEvent` to write the audit row but skip the Status
  update.
- **Tests** — 6 new sandboxMatcher unit tests (empty config, invalid
  IP, time threshold, IP CIDR including IPv6, nil-receiver safety,
  time-vs-IP precedence) + 2 new model tests (filtered handlers
  preserve Status, write audit event) + 1 new config_test fixture
  loading a `sandbox_filter` block.
- **Lint** — repo-wide finding count holds at 115. New code in
  `phish.go` is lint-clean.

Out of scope (deferred):
- UI surface in `/settings` (operator-edited config.json fits the
  rarely-changing security-policy nature of the setting).
- Per-campaign overrides; filtered-vs-counted dashboard summary;
  auto-curated sandbox IP feed; POST/form-submit filtering.

### Phase 8a SMS data model groundwork (2026-05-09)

First step in landing smishing (SMS phishing) campaigns. Pure data-model
change — no dispatcher, no UI, no observable behavior change for existing
email campaigns. Phase 8b wires the new fields into the worker and adds
the Twilio sender; Phase 8c hardens with delivery callbacks, char counter,
and opt-out.

Migration `20260509000000_0.13.0_sms_data_model`:

- `targets`, `results`, `email_requests` gain a `phone varchar(255) NOT
  NULL DEFAULT ''` column. (`email_requests` also embeds `BaseRecipient`
  via GORM, so the column is required there too even though SMS doesn't
  use the test-email path.)
- `campaigns` gains `channel varchar(16) NOT NULL DEFAULT 'email'` and
  `sms_profile_id integer` (nullable, no FK constraint to keep the down
  migration simple — same convention as `smtp_id`).
- `templates` gains `channel varchar(16) NOT NULL DEFAULT 'email'`.
- New tables `sms_profiles` (provider, account_sid, auth_token,
  from_number, messaging_service_sid, ...) and `sms_logs` (mirror of
  `mail_logs`).

Model changes:

- **`models/group.go::BaseRecipient`** — adds `Phone string`. New
  `ValidatePhone` exported helper using a strict E.164 regex.
  `insertTargetIntoGroup` now accepts targets with email-only OR
  phone-only OR both; rejects targets with neither
  (`ErrNoContactSpecified`). `UpdateTarget` includes phone in the
  update set; `GetTargets` SELECT includes the phone column.
- **`models/template.go::Template`** — adds `Channel string`
  (default "email"). `Validate()` switches on Channel: "email"/"" keeps
  the original contract; "sms" requires `Text`, rejects
  Subject/HTML/EnvelopeSender/Attachments
  (`ErrSMSTemplateMissingText`, `ErrSMSTemplateHasEmailFields`); other
  values return `ErrUnknownTemplateChannel`.
- **`models/campaign.go::Campaign`** — adds `Channel string`,
  `SMSProfileID int64`, and a non-DB `SMSProfile SMSProfile` for the
  same eager-load convention SMTP uses. `Validate()` is restructured
  so Channel determines whether `SMTP.Name` or `SMSProfile.Name` is
  required (`ErrSMSProfileNotSpecified`,
  `ErrUnknownCampaignChannel`); also enforces that the campaign and
  template agree on Channel (`ErrChannelMismatch`).
- **`models/sms_profile.go`** (NEW) — `SMSProfile` type with
  `Validate()` (provider whitelist via `SupportedSMSProviders`,
  E.164 check on `FromNumber`, exclusivity rule between
  `FromNumber` and `MessagingServiceSID`) and standard CRUD
  helpers (`Get*`, `Post`, `Put`, `Delete`). Provider whitelist
  starts with just `"twilio"` — adding a provider in 8b/8c is a
  single-map-entry change.
- **`models/sms_log.go`** (NEW) — `SMSLog` row + `GenerateSMSLog`.
  Backoff/Lock/Unlock/Error/Success/Generate methods deferred to
  Phase 8b alongside the worker integration.

Tests:

- `models/sms_profile_test.go` — `ValidatePhone` accept/reject
  matrix, `SMSProfile.Validate` matrix, DB round-trip via the
  gocheck suite.
- `models/sms_log_test.go` — `GenerateSMSLog` round-trip.
- `models/sms_channel_test.go` — phone-only / email+phone / no-contact
  group acceptance, Template.Validate cases for each channel,
  Campaign.Validate cases including the channel-mismatch check.
- `models/models_test.go::TearDownTest` — adds `SMSProfile{}` and
  `SMSLog{}` to the per-test cleanup list so cross-test pollution
  doesn't bleed.

Deferred to 8b: `mailer.Mail`-style methods on `SMSLog`
(Backoff/Lock/Unlock/Error/Success/Generate); `sms.Sender` interface
+ Twilio implementation; worker dispatcher branch on row type;
`models/result.go::HandleSMSDelivered` + `HandleSMSFailed`; Settings
UI for SMS profiles; Template editor channel selector + char counter;
Campaign creation modal channel selector.

Known pre-existing flake: `internal/gomail::TestAttachments` continues
to fail on Windows (multipart MIME boundary randomization); not
introduced here, CI on Linux unaffected.

### Phase 7c.2 sandbox filter Settings UI + query-time evaluation (2026-05-08)

Reframes 7c around two pieces of feedback:

1. *"I expected to change these from the panel."* — config.json + restart
   is the wrong UX for an org-wide security policy.
2. *"The report should always consider the current parameters. If I
   change the time, the report should recalculate."* — record-time
   filtering can't reclassify history; the filter has to be a **view**
   over raw data.

7c was the first step toward this; 7c.2 lands the right shape:

- **`phish_filter` table** — single-row policy storage (`id=1`),
  migrations in `db/db_{sqlite3,mysql}/migrations/20260508000000_*`.
- **`models/phish_filter.go`** — new `PhishFilter` model,
  `Get/PutPhishFilter`, `SeedPhishFilterFromConfig`, plus the
  `sandboxMatcher` relocated from `controllers/phish.go`. An
  `atomic.Pointer[sandboxMatcher]` cache is refreshed on every
  Get/Put so query-time evaluation reads the live policy without a
  per-event DB round-trip.
- **`Filtered(event, sendDate) (bool, reason)`** helper applied
  by `models/campaign.go::countFilteredClicksAndOpens` — campaign
  stats now walk the events table and apply the current policy in
  Go (CIDR membership doesn't translate cleanly to portable SQL
  across SQLite + MySQL; the summary is not a hot loop). The
  `Result.Status` column continues to track "highest event ever
  recorded" but aggregations bypass it for clicks/opens, so policy
  changes propagate retroactively.
- **`controllers/api/phish_filter.go`** — `GET`/`PUT /api/phish_filter/`,
  gated by `mid.RequirePermission(PermissionModifySystem)`. Validates
  CIDRs and returns 400 with the offending entry on bad input.
- **Settings UI** — new "Sandbox Filter" tab in `templates/settings.html`
  (gated on `{{if .ModifySystem}}`); `static/js/src/app/settings.ts`
  wires `loadPhishFilter` / `savePhishFilter` against
  `api.phish_filter.get/put`.
- **Seed-from-config bridge** — `gophish.go` calls
  `models.SeedPhishFilterFromConfig(conf.PhishConf.SandboxFilter)`
  after `models.Setup`. Idempotent: a populated DB row blocks the
  seed so existing edits aren't overwritten by stale config.json.

7c rollback (kept as a clean delta in this PR):

- Removed `EventOpenedSandboxFiltered` / `EventClickedSandboxFiltered`
  constants and the `Handle*Filtered` `Result` methods — record-time
  filtering is the wrong shape and shouldn't be carried as legacy.
- `controllers/phish.go::TrackHandler`/`PhishHandler` revert to
  unconditional `HandleEmailOpened` / `HandleClickedLink`.
- 6 `TestSandboxMatcher*` tests moved out of `controllers/phish_test.go`
  into `models/phish_filter_test.go` to follow the type to its new home.

Tests:

- `models/phish_filter_test.go` — Get/Put round-trip, seed
  idempotency, matcher unit tests (relocated), `Filtered` helper
  edge cases, retroactive-reclassification spot check.
- `controllers/api/phish_filter_test.go` — handler tests covering
  200/400/405 paths.
- `models/campaign_test.go::TestCampaignStatsAppliesPhishFilterRetroactively` —
  end-to-end proof that flipping `min_click_seconds` mid-campaign
  reclassifies historical clicks in the summary. Pins the defining
  property of 7c.2.

Out of scope (deferred):

- Per-campaign overrides (org-wide single policy per user
  clarification: "the IPs are same for the entire company").
- Audit history of who edited what when.
- `Result.Status` column rewrite to reflect the **filtered** view
  on the dashboard (the column drives several other surfaces;
  rewriting needs a broader pass and is a separate UX call).
- Filtered-events count on the dashboard summary (separate UX
  call from "exclude from totals").

Known pre-existing flake: `internal/gomail::TestAttachments`
fails locally on Windows due to multipart MIME boundary
randomization; not introduced here. CI on Linux unaffected.

### Phase 7b AI-scored template difficulty (2026-05-07)

Adds a complementary AI call to Phase 7a: admins can ask the model to
rate a candidate template 1..5 for how convincing it is. Same provider
abstraction; same auth/permission gates; no DB schema change (scoring
is informational only, not persisted).

- **`ai/scorer.go`** — new `Scorer` interface + `Subject`/`Score`
  types and a sentinel `ErrInvalidSubject` matching the existing
  `ErrInvalidBrief`/`ErrRefused` pattern. Compile-time assertion
  `var _ Scorer = (*AnthropicGenerator)(nil)` documents the contract.
- **`ai/anthropic.go`** — adds `ScoreTemplate(ctx, sub) (Score, error)`
  on the existing `*AnthropicGenerator`. Reuses `client`, `model`,
  `maxTokens`, `mapAnthropicError`, `firstText`, `stripJSONFence`. New
  `scoreSystemPrompt` constant defines the 1..5 rubric (1=obvious,
  5=AitM-tier) and is cached via `cache_control: ephemeral` like the
  drafting prompt. Out-of-range model output is rejected with a clear
  error rather than passed through.
- **`POST /api/templates/score`** (`controllers/api/template.go`).
  Auth + `PermissionModifyObjects` permission gate, same as
  `/generate`. Status mapping: 200 (Score), 400 (invalid body), 422
  (refusal), 502 (provider error), 503 (AI off OR provider doesn't
  implement `Scorer`). The handler uses a type assertion
  `as.aiGenerator.(ai.Scorer)` so future providers that ship Generator
  but not Scorer degrade gracefully.
- **Frontend**: "Score with AI" button next to Subject (gated on
  `{{if .AIEnabled}}`). New `#aiScoreModal` shows a big colored
  number, rationale, strengths/weaknesses, and "make it harder"
  bullets. Result is informational; the modal has no save button.
- **Tests** — 6 new ai/ unit tests (happy path, refusal, 401, empty
  subject, out-of-range score, non-JSON output) + 8 new
  controllers/api/ handler tests (happy, 503-AI-off,
  503-no-scorer-impl, 400-bad-json, 400-invalid-subject, 422, 502, 405).
- **Lint** — repo-wide finding count holds at 115 (no change).
  `npm run typecheck` clean; `npm run build` produces a 12.7 KB
  `templates.min.js` (was 12.1 KB).

Out of scope (deferred):
- Persisting scores on the `templates` table; revisit when there's a
  list-view surface that benefits.
- Auto-scoring on every generate (cost concern; admin opts in).
- Batch scoring multiple drafts in one request.
- Score history / trend chart.

### Phase 7a.1 AI-assisted template generation backend (2026-05-07)

First feature beyond pure modernization. Adds an `ai/` package with a
provider-agnostic `Generator` interface and an Anthropic-backed
implementation, plus a new `POST /api/templates/generate` endpoint that
admins can hit to draft phishing-simulation email templates.

- **`ai/`** — new package. `Generator` interface + `Brief` / `Draft`
  types in `generator.go`; Anthropic implementation in `anthropic.go`
  using the official `github.com/anthropics/anthropic-sdk-go`. Sane
  defaults: `claude-sonnet-4-6` model, 4096 max tokens. The system
  prompt is package-level constant + `cache_control: ephemeral` so
  every draft pays the cached input rate, not full price. Output is
  parsed from a JSON object the model is instructed to emit; defense
  in depth strips ```json ``` fences, validates `{{.URL}}` / `{{.Tracker}}`
  presence, and surfaces missing variables as a soft warning in
  `Draft.Notes` rather than failing.
- **`controllers/api/template.go`** — adds `GenerateTemplate` handler.
  Status mapping: 200 (Draft JSON), 400 (invalid brief), 422
  (`ai.ErrRefused`), 502 (upstream provider error or auth-config bug),
  503 (AI disabled in config — `ai.enabled=false` or no `ai` block).
- **`controllers/api/server.go`** — `Server.aiGenerator` + the
  `WithAIGenerator` option following the existing
  `WithWorker`/`WithLimiter` pattern. Route registered with
  `mid.RequirePermission(models.PermissionModifyObjects)` so view-only
  users can't burn tokens.
- **`controllers/route.go`** — new `WithAIGenerator` `AdminServerOption`
  plumbed through to `api.WithAIGenerator`. The actual generator is
  built in `gophish.go` so `controllers/` doesn't depend on
  `config.Config` (only on `config.AdminServer`, as today).
- **`gophish.go`** — `buildAIGenerator(config.AIConfig)` constructs
  the appropriate provider (currently only Anthropic). Failures here
  are logged but don't block startup; `/generate` simply returns 503
  until the config is fixed.
- **`config/config.go`** — `AIConfig` + `AnthropicAIConfig` added.
  Disabled by default. `config.json` example documented in
  `docs/dev/`.
- **Migration** —
  `db/db_{sqlite3,mysql}/migrations/20260506000000_0.13.0_template_generated_by.{up,down}.sql`
  adds a `generated_by varchar(255)` column on `templates`. Default
  NULL; `models.Template.GeneratedBy` is the corresponding Go field
  with a `gorm:"column:generated_by"` tag and `json:"generated_by,omitempty"`.
  The `/generate` endpoint does **not** persist; the admin saves via
  the existing `POST /api/templates/`. The `GeneratedBy` field is set
  by the upcoming Phase 7a.2 UI when the admin saves a generated
  template; for 7a.1 the column ships unused so the migration
  is in the deployment pipeline ahead of the UI work.
- **Lint impact** — repo-wide finding count holds at **115** (no
  change from Phase 6a). The new `ai` package starts lint-clean. The
  Anthropic SDK pulls in a handful of indirect deps (gjson, sjson,
  go-ordered-map); all are MIT/BSD-licensed and lint-clean for our
  purposes.

### Phase 6a naming cleanup + dead code (2026-05-06)

Burns down `revive`'s `var-naming` rule (suppressed since Phase 2) and
retires the last two `unused` findings from the original lint baseline:

- **Field renames** (Go identifiers only — JSON tags, GORM column tags,
  and route URLs are unchanged):
  - `UserId` → `UserID` across 28 files / 122 sites.
  - `RId` → `RID` across 15 files / 81 sites for the model-level fields
    (`Result.RID`, `MailLog.RID`, `EmailRequest.RID`, etc.). The
    template-visible `PhishingTemplateContext` keeps **both** spellings
    — see "Backward compat" below.
  - `ApiKey` → `APIKey` across 10 files / 20 sites.
  - `CampaignId`, `GroupId`, `TemplateId`, `PageId`, `Id` etc.
    → `…ID` across the model layer and downstream callers.
- **Dead code removed**:
  - `dialer.restrictedDialer` (private struct, dialer/dialer.go:115-118
    in pre-cleanup numbering — the exported `RestrictedDialer` is the
    type that's actually in use).
  - `controllers/api.createTestData` (test helper that was defined in
    `controllers/api/api_test.go` but never referenced; the survey for
    this phase initially mis-identified it as live, lint correctly
    flagged it as dead).
- **Lint rule re-enabled**: `var-naming: disabled: true` removed from
  `.golangci.yml`. `golangci-lint run ./...` reports 0 `var-naming`
  findings post-burn-down.

### Backward compat: PhishingTemplateContext.RId

`PhishingTemplateContext.RID` (the new spelling) is the canonical
field, but `PhishingTemplateContext.RId` is preserved as a
struct-field alias populated to the same value at construction.
Reason: user-authored email and landing-page templates stored in the
database before this phase reference `{{.RId}}`. Renaming the field
without an alias would silently break every existing Gophish
installation's templates on upgrade (the Go template engine would
return an empty string for the unknown field). The alias has a
`//nolint:revive` annotation pointing at the explanation. New code
should use `RID`.

`models.Result.RID`, `models.MailLog.RID`, and
`models.EmailRequest.RID` were renamed without aliases because they
are internal model fields, never exposed directly to user templates.

### Numbers

- Repo-wide finding floor: **117 → 115** (−2 from the two retired
  `unused` items).
- `var-naming`: previously suppressed; now enabled and at 0 findings.
- `unused`: 2 → 0 (the post-Phase 5b count was 2 and both retire here).

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
