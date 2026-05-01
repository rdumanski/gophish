# Smoke Test — End-to-End Verification

This is the **contract** that all subsequent phases must preserve. Run this after every meaningful change to confirm no regressions in the core campaign flow.

## What it tests

1. The binary builds and starts.
2. The admin web UI loads at `https://localhost:3333`.
3. A user can log in with the auto-generated admin password.
4. A sending profile (SMTP server settings) can be created.
5. A campaign can be launched with a single recipient.
6. The recipient email arrives at the configured SMTP server.
7. Open tracking fires when the email's tracking pixel is loaded.
8. Click tracking fires when a link in the email is clicked.

## Prerequisites

- The gophish binary built (see [build-windows.md](./build-windows.md) or [build-docker.md](./build-docker.md)).
- **MailHog** for receiving test emails. Easiest install via Docker:

  ```bash
  docker run --rm -d --name mailhog \
    -p 1025:1025 -p 8025:8025 \
    mailhog/mailhog
  ```

  MailHog SMTP listens on port 1025; its inbox UI is at `http://localhost:8025`.

## Procedure

### 1. Start gophish

**Native:**
```bash
./gophish.exe
```

**Docker (with MailHog on the same Docker network):**
```bash
docker network create phishtest
docker run --rm -d --name mailhog --network phishtest \
  -p 1025:1025 -p 8025:8025 mailhog/mailhog
docker run --rm --name gophish --network phishtest \
  -p 3333:3333 -p 8080:8080 -p 8443:8443 \
  gophish:0.13.0-dev
```

Capture the admin password from the log line:
```
time="..." level=info msg="Please login with the username admin and the password 4304d5255378177d"
```

### 2. Log in

Open `https://localhost:3333` (accept the self-signed cert warning). Log in as `admin` with the password from the log. You will be prompted to set a new password — choose one and continue.

### 3. Create a sending profile

Navigate to **Sending Profiles** → **New Profile**:

| Field | Value |
|---|---|
| Name | `MailHog (test)` |
| Interface Type | SMTP |
| From | `phisher@test.local` |
| Host | `localhost:1025` (native) or `mailhog:1025` (Docker network) |
| Username | (blank) |
| Password | (blank) |
| Ignore Certificate Errors | ✅ |

Click **Send Test Email**, target `target@test.local`. Open `http://localhost:8025` (MailHog UI) and confirm the email arrived.

### 4. Create a landing page

**Landing Pages** → **New Page**:

| Field | Value |
|---|---|
| Name | `Test Landing` |
| Import Site | (skip) |
| HTML | `<html><body><h1>You've been phished!</h1><p>Tracking: {{.URL}}</p></body></html>` |
| Capture Submitted Data | ☐ (off for smoke test) |

### 5. Create an email template

**Email Templates** → **New Template**:

| Field | Value |
|---|---|
| Name | `Test Email` |
| Subject | `[smoke test] Click me` |
| Text | (skip) |
| HTML | `<html><body><p>Hi {{.FirstName}},</p><p><a href="{{.URL}}">Click here to verify</a></p><img src="{{.Tracker}}"/></body></html>` |

### 6. Create a target group

**Users & Groups** → **New Group**:

| Field | Value |
|---|---|
| Name | `Smoke Test Targets` |
| Members | `target@test.local` (First: `Smoke`, Last: `Tester`) |

### 7. Launch a campaign

**Campaigns** → **New Campaign**:

| Field | Value |
|---|---|
| Name | `Smoke Test Campaign` |
| Email Template | `Test Email` |
| Landing Page | `Test Landing` |
| URL | `http://localhost:8080` (the gophish phishing server) |
| Sending Profile | `MailHog (test)` |
| Groups | `Smoke Test Targets` |

Launch immediately.

### 8. Verify the events

- **MailHog UI** (`http://localhost:8025`): the test email should appear within a few seconds.
- **Open the email in MailHog** — the tracking pixel `{{.Tracker}}` will hit gophish; campaign Results table should show **Email Opened**.
- **Click the link** in the email's HTML view — gophish serves the landing page; campaign Results table should show **Clicked Link**.

If all three states (`Email Sent`, `Email Opened`, `Clicked Link`) appear in the campaign **Results** tab, the smoke test passes.

## Smoke test as part of CI

Phase 1 only documents this; Phase 6 will automate it as an integration test. Until then, run manually after every phase merge.

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `dial tcp 127.0.0.1:1025: connect: connection refused` | MailHog not running | `docker ps`; restart MailHog |
| Admin login fails immediately | Wrong password (case-sensitive) | Re-read the log line |
| Email arrives but Open event doesn't fire | Tracking pixel blocked by mail client | Use MailHog's "HTML" preview, not "Source" |
| Click event doesn't fire | URL field points at wrong host | Should be `http://<gophish-host>:8080`, not `:3333` |
| Self-signed cert blocks browser | Browser strict mode | Click "Advanced" → "Proceed to localhost (unsafe)" |
