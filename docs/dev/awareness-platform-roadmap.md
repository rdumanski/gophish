# Awareness Platform Roadmap

Status: **planning** (2026-06-12). Supersedes the shelved SMS direction (Phase 8b
is committed but unmerged on `phase-8b-sms-sending`).

Goal: grow gophish from a phishing-simulation tool into a security-awareness
platform comparable to commercial offerings (KnowBe4, Proofpoint Security
Awareness, Hoxhunt) — phishing simulation **plus** teachable moments, training
content, assessments, learner tracking, and longitudinal risk scoring.

This document is the architecture + phase plan. Each phase maps to one
`phase-N-*` branch / PR, consistent with the existing workflow.

---

## 1. The central design problem

gophish runs **two servers** (`gophish.go`):

- **Admin server** — authenticated (session + API key + RBAC). Manages
  campaigns, templates, pages, groups.
- **Phishing server** — *unauthenticated*, public-facing. Identifies a recipient
  solely by a 7-char `rid` query param (`models.RecipientParameter`) that resolves
  `rid -> Result -> Campaign -> Page`.

There is **no end-user (learner) identity or portal**. A "recipient" is not an
account; it is a `BaseRecipient` value (email, name, position) embedded *by value*
into per-campaign `Result` rows (`models/result.go`) and into `Target` rows
(`models/group.go:49`, group-scoped via the `GroupTarget` M:M join). Nothing ties
the same person across two campaigns except a matching email string.

A commercial awareness platform needs the opposite: a **stable person record**
that accumulates history (sims failed, training completed, risk over time) and a
**learner-facing surface** where that person consumes training and takes quizzes
without an admin login. Those two needs drive the whole roadmap. Three decisions
fall out of this and are made up front (Section 2) so later phases don't require
rework.

---

## 2. Foundational decisions (made before phasing)

### Decision A — Introduce a `Recipient` (person) master entity in Phase 10

**Problem:** "per-user risk over time" and "auto-enroll the user who failed a sim"
both require a person entity that outlives any single campaign. Matching by raw
email across deleted campaigns will not hold up.

**Decision:** Phase 10 introduces a `recipients` table — one row per
(owner `user_id`, email) — the canonical person. New tables (`Enrollment`,
quiz attempts, risk snapshots) FK to `recipients.id`, **not** to an email string.

- `Result` and `Target` keep their embedded `BaseRecipient` for backward
  compatibility (the phishing path is untouched), but gain a nullable
  `recipient_id` FK, backfilled by email on write.
- A small reconciliation step (on group save / campaign create / event ingest)
  upserts the `Recipient` and links it. This is the only way Phases 11–13 get
  longitudinal identity without each re-deriving it from email.

**Why early:** this is the classic roadmap-killer — a Phase 13 feature needing a
Phase 10 table. Build it in the foundation, not at the end.

### Decision B — Training delivery reuses the low-level `mailer`, not the phishing `MailLog`

**Problem:** the mail **worker** (`worker/worker.go`) and `MailLog.Generate()`
(`models/maillog.go`) are welded to `Campaign`/`Result` and build their body via
`NewPhishingTemplateContext`. "Reuse the mail worker" is not free.

The clean seam is *lower*: the `mailer` package (the queue + SMTP send loop) is
already decoupled from phishing — the worker merely hands it `gomail.Message`
values. Training delivery should build its **own** `gomail.Message` from a
`TrainingTemplateContext` (enrollment link, due date, learner name) and queue to
the **same `mailer`**.

**Decision:** training gets a **parallel delivery pipeline**
(`TrainingCampaign` + `Enrollment` + an enrollment send/reminder queue) that
shares the low-level `mailer` and SMTP profiles, but **does not** route through
`Campaign`/`Result`/phishing `MailLog`. Rejected alternative: modelling a training
campaign as a `Campaign` variant — it would entangle the phishing send path with
training semantics (no landing page, no tracking pixel, different lifecycle).

### Decision C — Learner-portal tokens are signed, not `rid`-style

The phish `rid` is deliberately weak/enumerable because the phish server exposes
nothing sensitive. The learner portal exposes quiz scores, completion status, and
(later) certificates — mildly sensitive and persistent.

**Decision:** enrollment access tokens are **long, random, and signed**
(HMAC over `enrollment_id`, validated server-side), single-purpose, and
revocable. Do **not** inherit the phish server's "no secret needed" property.

---

## 3. Phase plan

Each phase is independently shippable and demoable.

### Phase 9 — Teachable moments (the wedge)

Lowest effort, highest immediate value; builds on the existing landing flow.

- **Per-campaign training redirect.** Today `Page.RedirectURL` redirects only on
  POST (form submit), shared across all campaigns using the page
  (`controllers/phish.go:renderPhishResponse`). Add a campaign-level override and
  extend redirect to the **click (GET)** path so a click *or* a submit can land on
  a teachable-moment page.
- **Built-in "you've been phished" page.** A first-party education template
  (red-flags explainer, what to do next), tokenized so it shows which cues the
  just-clicked email contained.
- No new person entity yet; pure extension of the phishing path.

Scope: `Campaign` field + migration (both DB dirs), `phish.go` redirect logic, one
admin toggle, one built-in template. Closes the loop "click -> immediate lesson."

### Phase 10 — Recipient master + content library + learner-portal foundation

The foundation. Introduces person identity (Decision A) and the learner surface.

- **`Recipient` master entity** + `recipient_id` FK on `Result`/`Target`, backfill
  by email (Decision A).
- **`TrainingModule` model** — content library. Types: inline HTML, hosted video
  URL, external/SCORM reference (SCORM = stretch). Admin CRUD modelled on the
  `Template`/`ai` feature pattern (`ai/generator.go` interface style for any
  content-rendering abstraction).
- **`Enrollment` model** — the learner-side analog of `Result`: FK to
  `Recipient` + `TrainingModule`, status (`assigned/started/completed`),
  signed access token (Decision C), timestamps, score (nullable until Phase 12).
- **Learner portal** — a public surface that resolves a signed token to an
  `Enrollment`, renders the module, and records completion. Decision pending in
  this phase: extend the phishing server with a `/learn/{token}` route vs. a third
  listener; default to extending the phishing server (one less port, same deploy).

### Phase 11 — Training campaigns & enrollment delivery

Turns the library into assignable, tracked programs.

- **`TrainingCampaign`** — assign modules to groups/recipients with a due date.
- **Delivery** via the parallel pipeline sharing the `mailer` (Decision B):
  enrollment invitation email with the signed portal link.
- **Reminders** — scheduled nudges for incomplete enrollments before/after due
  date (reuse the worker's polling cadence with a separate queue).
- **Completion dashboard** — per-campaign and per-group completion %, modelled on
  the existing campaign-results UI.

### Phase 12 — Quizzes / assessments

- **`Quiz` / `Question` / `QuestionOption`** models; a quiz attaches to a module or
  stands alone.
- **Quiz-taking** in the learner portal; **`QuizAttempt` / `QuizResponse`** capture
  answers against the `Recipient`.
- **Scoring + pass thresholds**; completion gated on pass. Optional **certificate**
  (PDF/HTML) issued on pass — the first artifact justifying Decision C's signed
  tokens.

### Phase 13 — Risk scoring, reporting & auto-enrollment (closing the loop)

- **Per-recipient & per-group risk score** blending phishing behaviour (clicked,
  submitted, reported, repeat offenses) with training (assigned vs. completed,
  quiz scores) — all keyed on the `Recipient` from Phase 10.
- **Trend snapshots** — periodic `risk_snapshot` rows so the score is graphable
  over time (a score with no history is not "on par with commercial").
- **Auto-enrollment policy** — the closed loop: a sim failure
  (`HandleFormSubmit` / `HandleClickedLink`, which already fire webhooks/events)
  auto-creates a remedial `Enrollment`. The event hook exists today; this phase
  wires it to enrollment.
- **Reporting** — board-level awareness reports, exportable.

---

## 4. Cross-cutting concerns

- **Migrations:** every schema change ships paired `.up.sql`/`.down.sql` in
  **both** `db/db_sqlite3/migrations/` and `db/db_mysql/migrations/`, named
  `YYYYMMDDHHMMSS_<version>_<feature>.sql`, kept in lockstep (verified convention).
- **RBAC:** likely a new permission (e.g. `manage_training`) and possibly a
  "training manager" role distinct from phishing operators (`models/rbac.go`).
- **Feature pattern to follow:** the Phase 7 AI subsystem (`ai/` interface +
  provider + config block + startup wiring + API endpoint + frontend gate) is the
  established template for a self-contained feature; mirror it.
- **Frontend:** new admin pages follow the `static/js/src/app/*.ts` + `templates/*.html`
  + `build.mjs` entry + `nav.html` link recipe; the learner portal is a separate,
  minimal, unauthenticated bundle (no admin chrome).
- **Privacy/retention:** the `Recipient` master concentrates personal data and
  longitudinal behaviour — needs a retention/delete policy (GDPR), especially as
  campaigns are currently deletable.
- **i18n:** commercial platforms ship localized training; treat module content as
  translatable from the start (don't hard-code copy in handlers).

---

## 5. Open questions to resolve per-phase (not now)

- Learner portal: extend phishing server vs. dedicated listener (leaning extend).
- SCORM/xAPI support depth (stretch — most value is first-party HTML/video).
- Certificate format and whether it needs verifiable signing.
- Whether `Result` should eventually FK-hard to `Recipient` (migration risk) or
  keep the soft email-backfill link indefinitely.
