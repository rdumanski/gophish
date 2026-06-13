# Custom Domains — Plan

Status: **planning** (2026-06-13).

## Context

We buy look-alike domains (e.g. `pse.org.pl` to impersonate `pse.pl`) and want to
use them in campaigns for **both** roles: the **email sending identity** (the
`From:`) and the **landing/links host** (the URL recipients click, the landing
page, tracking, and the awareness learner portal). Today a domain is free-typed
per campaign (the URL field) and per sending profile (the From address); nothing
tracks which domains we own or whether they're configured/healthy. The goal is a
first-class **Domains registry**: register a bought domain once, with its role and
DNS/TLS/email-auth health, and pick it from dropdowns when building campaigns and
sending profiles.

## A domain plays two independent roles

| Role | What it controls | Where it's applied today |
|---|---|---|
| **Sending identity** | the `From:` / envelope of the email | Sending Profile `FromAddress` (`models/smtp.go:41`) → SMTP MAIL FROM (`maillog.go:160` `GetSmtpFrom`) and fallback `From:` header; `Template.EnvelopeSender` (`models/template.go:19`) overrides the visible `From:` (`maillog.go:188-195`). Set in `templates/sending_profiles.html` + the template editor. |
| **Landing host** | links, landing page, tracking pixel, learner portal | `Campaign.URL` (`models/campaign.go:36`) → expanded to `BaseURL`/`URL`/`TrackingURL`/`Tracker` in `NewPhishingTemplateContext` (`models/template_context.go:47-68`). Set in the New-Campaign modal URL field (`templates/campaigns.html`, `campaigns.ts` line 47). |

Because the phishing server routes by `rid`/`token` (not `Host`), the **same
landing domain automatically covers** the landing page, `/track`, teachable
moments, and the learner portal (`/learn/{token}`) — they all ride whatever host
points at the phishing server.

## Infrastructure realities (these constrain the feature)

1. **HTTPS for many domains needs a proxy or autocert.** The phishing server's
   `Start()` (`controllers/phish.go:84-99`) calls `ListenAndServeTLS` with a single
   cert/key and `util.CheckAndCreateSSL` generates one self-signed cert with no
   SANs/SNI (`util/util.go:65-133`). So one instance serves any number of landing
   domains over **HTTP**, but only **one** over HTTPS. Options: (a) terminate TLS at
   a reverse proxy (Caddy auto-issues Let's Encrypt per host — recommended, zero
   gophish change); (b) add `acme/autocert` SNI to the phishing server, with the
   host allowlist fed by this registry (turnkey, but new code); (c) one instance
   per domain (heavy — avoid).
2. **DKIM signing is relay-side.** gophish relays through the SMTP `Host`; it does
   not DKIM-sign. The registry can generate/store a DKIM keypair and tell you the
   exact SPF/DKIM/DMARC records to publish, but actual signing happens at your mail
   relay (or would require adding a signer to the send path — out of scope for the
   registry).
3. **No DNS/cert/ACME code exists** to reuse (confirmed by grep). Health checks are
   net-new (`net.LookupHost`, `tls.Dial`+`x509`, TXT lookups). Outbound calls should
   note the restricted `dialer` (`dialer/dialer.go`) used elsewhere.

## Proposed feature

### Model `Domain` (`models/domain.go`)
`id, user_id, name, role ("sending"|"landing"|"both"), landing_target (the IP/host
DNS should point at), dkim_selector, dkim_private_key, dkim_public_key,
last_checked, health (JSON or columns: dns_ok, tls_ok, spf_ok, dkim_ok, dmarc_ok),
status, notes, modified_date`. CRUD mirrors the SMTP/IMAP model pattern.

### Health-check endpoint — model on IMAP/webhook "validate"
`POST /api/domains/{id}/check` (like `IMAPServerValidate` `controllers/api/imap.go:14`
and webhook `ValidateWebhook`), returning `{success, message, health}`:
- **landing role:** `net.LookupHost(name)` resolves to our `landing_target`? `tls.Dial`
  `name:443` → cert valid and covers the name?
- **sending role:** TXT lookups for SPF (`v=spf1…`), DKIM (`<selector>._domainkey`),
  DMARC (`_dmarc`). Report present/aligned.
A "Check" button on the Domains row drives it (mirrors the webhook **Ping** button).

### Admin UI
A **Domains** page (CRUD + Check + health badges), built on the established
model→CRUD-API→admin-page pattern (as used for training modules, quizzes, etc.).
For each domain show the **exact DNS records to publish** (A → landing_target;
SPF/DKIM/DMARC for sending) so the operator can hand them to whoever runs DNS.

### Integration points (the payoff)
- **New-Campaign modal:** a **Domain** dropdown (landing/both) that pre-fills the
  `URL` field (`campaigns.ts` builds `url:` — point it at the selected domain).
  Keep free-text URL as a fallback.
- **Sending-Profile modal:** a **Domain** dropdown (sending/both) that sets the
  `from_address` domain and surfaces the DKIM record to publish.
- (Optional) **Template editor:** offer registered sending domains for
  `EnvelopeSender`.

## Phases

- **15a — Registry + health checks (core).** `Domain` model + migration + CRUD API +
  admin Domains page + `POST /domains/{id}/check` doing DNS/TLS/SPF/DKIM/DMARC
  lookups + a DKIM keypair generator. Self-contained; the registry is usable on its
  own (it tells you exactly what to configure).
- **15b — Campaign + sending-profile wiring.** Domain dropdowns that pre-fill the
  campaign URL and the sending-profile From; "records to publish" panels.
- **15c — Turnkey HTTPS (optional, infra).** Either document the Caddy reverse-proxy
  setup (no code), or add `acme/autocert` SNI to the phishing server with its host
  allowlist sourced from landing-role domains in this registry — so a freshly bought
  domain serves HTTPS automatically once DNS points at us.

## Decisions to confirm before building

- **TLS strategy:** reverse proxy (recommended first; no code) vs. built-in
  autocert (15c). The registry is the same either way.
- **DKIM:** registry generates/tracks keys and you configure your relay to sign,
  vs. (much larger) adding a DKIM signer to gophish's send path.
- **Scope of health checks** for v1 (DNS + TLS + the three email-auth records is a
  good first cut).

## Verification

- Unit tests for the model/CRUD and the record-format generators (pure functions:
  given a domain, produce the SPF/DKIM/DMARC strings; parse a TXT lookup result).
- The live DNS/TLS check is exercised against a real domain in a manual pass (like
  the SMTP/MailHog and IMAP verifications) — point a test domain's A record at a
  throwaway host and confirm the check reports correctly; the lookup helpers are
  unit-tested with synthetic inputs.
- A page-render integration test for the Domains admin page (as for the other
  admin pages).
