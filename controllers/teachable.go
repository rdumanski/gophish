package controllers

import (
	"net/http"

	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// teachableMomentHTML is the first-party "you've been phished" education page
// rendered when a campaign has TeachableMoment enabled (Phase 9). It is a Go
// text/template executed against a models.PhishingTemplateContext, so it can
// reference recipient fields such as {{.FirstName}}. The markup is fully
// self-contained (inline CSS, no external resources) because the phishing
// server is frequently deployed in network-isolated environments where the
// target's browser cannot reach a CDN.
const teachableMomentHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>Security Awareness</title>
<style>
  :root { color-scheme: light; }
  * { box-sizing: border-box; }
  body {
    margin: 0;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    background: #f4f6f9;
    color: #2c3e50;
    line-height: 1.55;
  }
  .wrap { max-width: 680px; margin: 0 auto; padding: 32px 20px 64px; }
  .card {
    background: #fff;
    border-radius: 10px;
    box-shadow: 0 1px 3px rgba(0,0,0,.08), 0 8px 24px rgba(0,0,0,.06);
    overflow: hidden;
  }
  .banner {
    background: #c0392b;
    color: #fff;
    padding: 28px 32px;
  }
  .banner .icon { font-size: 40px; line-height: 1; }
  .banner h1 { margin: 12px 0 0; font-size: 24px; font-weight: 600; }
  .body { padding: 28px 32px; }
  .body p { margin: 0 0 16px; }
  .lead { font-size: 17px; }
  h2 { font-size: 16px; margin: 28px 0 12px; color: #c0392b; text-transform: uppercase; letter-spacing: .04em; }
  ul { margin: 0 0 8px; padding-left: 20px; }
  li { margin: 0 0 10px; }
  .reassure {
    background: #eef7ee;
    border-left: 4px solid #27ae60;
    padding: 14px 18px;
    border-radius: 4px;
    margin: 0 0 20px;
  }
  .footer {
    padding: 20px 32px 28px;
    font-size: 13px;
    color: #7f8c8d;
    border-top: 1px solid #ecf0f1;
  }
</style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <div class="banner">
        <div class="icon">&#9888;</div>
        <h1>This was a simulated phishing test</h1>
      </div>
      <div class="body">
        <p class="lead">{{if .FirstName}}Hi {{.FirstName}}, this{{else}}This{{end}}
        message was part of an internal security-awareness exercise &mdash; not a
        real attack. No harm was done, but a genuine attacker could have used the
        same trick to steal your credentials.</p>

        <div class="reassure">
          <strong>You're not in trouble.</strong> These tests exist to help everyone
          practice spotting suspicious messages in a safe setting.
        </div>

        <h2>What to watch for next time</h2>
        <ul>
          <li><strong>Unexpected urgency.</strong> Messages that pressure you to act
          immediately ("your account will be locked") are a classic red flag.</li>
          <li><strong>Mismatched sender or links.</strong> Hover over links before
          clicking and check that the address really belongs to who it claims.</li>
          <li><strong>Requests for credentials.</strong> Legitimate services rarely
          ask you to confirm your password through an emailed link.</li>
          <li><strong>Small details that feel off.</strong> Odd phrasing, generic
          greetings, or slightly wrong logos often give a phish away.</li>
        </ul>

        <h2>What to do</h2>
        <ul>
          <li>When a message feels suspicious, don't click &mdash; report it using
          your organisation's phishing-report process.</li>
          <li>If you ever did enter a password into a page like this, change it and
          let your IT/security team know.</li>
        </ul>
      </div>
      <div class="footer">
        You're seeing this page because information was entered into a simulated
        phishing page. Thanks for taking a moment to learn &mdash; awareness is
        the best defence.
      </div>
    </div>
  </div>
</body>
</html>`

// renderTeachableMoment writes the first-party security-awareness page,
// templated with the recipient's context. It is called from PhishHandler in
// place of renderPhishResponse when the campaign has TeachableMoment enabled.
func renderTeachableMoment(w http.ResponseWriter, r *http.Request, ptx models.PhishingTemplateContext) {
	html, err := models.ExecuteTemplate(teachableMomentHTML, ptx)
	if err != nil {
		// The template is a compile-time constant, so an error here means the
		// recipient context held something unparseable; fall back to a plain
		// generic page rather than leaking an error to the target.
		log.Error(err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(genericTeachableMomentHTML))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

// genericTeachableMomentHTML is a name-free fallback used only if templating
// the personalised page fails.
const genericTeachableMomentHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="utf-8"><meta name="robots" content="noindex, nofollow">
<title>Security Awareness</title></head>
<body style="font-family:sans-serif;max-width:640px;margin:40px auto;padding:0 20px;color:#2c3e50">
<h1 style="color:#c0392b">This was a simulated phishing test</h1>
<p>This message was part of an internal security-awareness exercise &mdash; not a real
attack. A genuine attacker could have used the same trick to steal your credentials.</p>
<p>Next time, watch for unexpected urgency, mismatched links, and requests for your
password. When something feels suspicious, don't click &mdash; report it.</p>
</body></html>`
