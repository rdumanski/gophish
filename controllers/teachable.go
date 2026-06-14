package controllers

import (
	"bytes"
	"html/template"
	"net/http"

	"github.com/rdumanski/gophish/i18n"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// teachableData is the render context for the first-party teachable-moment page.
// It carries only the recipient's first name (auto-escaped) and the resolved UI
// language; all prose comes from the i18n catalog via T.
type teachableData struct {
	FirstName string
	Lang      string
}

// T localizes a key for the teachable page's language.
func (d teachableData) T(key string, args ...interface{}) string {
	return i18n.T(d.Lang, key, args...)
}

// teachableMomentTmpl is the first-party "you've been phished" education page
// rendered when a campaign has TeachableMoment enabled (Phase 9). It is
// html/template (recipient FirstName is auto-escaped) and fully self-contained
// (inline CSS, no external resources) because the phishing server is frequently
// deployed in network-isolated environments where the browser can't reach a CDN.
var teachableMomentTmpl = template.Must(template.New("teachable").Parse(teachableMomentHTML))

const teachableMomentHTML = `<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>{{ .T "teachable.title" }}</title>
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
        <h1>{{ .T "teachable.heading" }}</h1>
      </div>
      <div class="body">
        <p class="lead">{{if .FirstName}}{{ .T "teachable.lead_hi" .FirstName }}{{else}}{{ .T "teachable.lead_nohi" }}{{end}} {{ .T "teachable.lead_body" }}</p>

        <div class="reassure">
          <strong>{{ .T "teachable.reassure_strong" }}</strong> {{ .T "teachable.reassure_body" }}
        </div>

        <h2>{{ .T "teachable.watch_heading" }}</h2>
        <ul>
          <li><strong>{{ .T "teachable.watch_urgency_strong" }}</strong> {{ .T "teachable.watch_urgency_body" }}</li>
          <li><strong>{{ .T "teachable.watch_links_strong" }}</strong> {{ .T "teachable.watch_links_body" }}</li>
          <li><strong>{{ .T "teachable.watch_creds_strong" }}</strong> {{ .T "teachable.watch_creds_body" }}</li>
          <li><strong>{{ .T "teachable.watch_details_strong" }}</strong> {{ .T "teachable.watch_details_body" }}</li>
        </ul>

        <h2>{{ .T "teachable.do_heading" }}</h2>
        <ul>
          <li>{{ .T "teachable.do_report" }}</li>
          <li>{{ .T "teachable.do_password" }}</li>
        </ul>
      </div>
      <div class="footer">{{ .T "teachable.footer" }}</div>
    </div>
  </div>
</body>
</html>`

// renderTeachableMoment writes the first-party security-awareness page,
// localized to the recipient's Accept-Language and personalised with their
// first name. Called from PhishHandler in place of renderPhishResponse when the
// campaign has TeachableMoment enabled.
func renderTeachableMoment(w http.ResponseWriter, r *http.Request, ptx models.PhishingTemplateContext) {
	data := teachableData{
		FirstName: ptx.FirstName,
		Lang:      i18n.FromAcceptLanguage(r.Header.Get("Accept-Language")),
	}
	var buf bytes.Buffer
	if err := teachableMomentTmpl.Execute(&buf, data); err != nil {
		// The template is a compile-time constant; an error here is unexpected.
		// Fall back to a plain generic page rather than leaking an error.
		log.Error(err)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(genericTeachableMomentHTML))
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(buf.Bytes())
}

// genericTeachableMomentHTML is a name-free, English fallback used only if
// rendering the personalised page fails.
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
