package controllers

import (
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// learnerPageData is the render context for the public learner portal.
type learnerPageData struct {
	ModuleName  string
	Description string
	FirstName   string
	ContentType string
	// Content is the module's inline HTML for html-type modules. It is
	// operator-authored and injected unescaped as template.HTML — the same
	// trust model gophish already applies to landing-page and email-template
	// HTML served from the phishing origin. Recipient-derived fields
	// (FirstName) are NOT trusted and stay auto-escaped by html/template.
	Content   template.HTML
	URL       string
	Token     string
	Completed bool
}

// learnerPageTmpl is the self-contained portal page. It is html/template (not
// the text/template used for phishing pages) so that recipient-supplied fields
// are auto-escaped; only Content is explicitly trusted. Inline CSS, no external
// resources — the phishing server is frequently network-isolated.
var learnerPageTmpl = template.Must(template.New("learner").Parse(learnerPageHTML))

const learnerPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex, nofollow">
<title>{{.ModuleName}}</title>
<style>
  * { box-sizing: border-box; }
  body { margin:0; font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;
         background:#f4f6f9; color:#2c3e50; line-height:1.55; }
  .wrap { max-width:760px; margin:0 auto; padding:32px 20px 64px; }
  .card { background:#fff; border-radius:10px; box-shadow:0 1px 3px rgba(0,0,0,.08),0 8px 24px rgba(0,0,0,.06); overflow:hidden; }
  .banner { background:#2c7be5; color:#fff; padding:24px 32px; }
  .banner h1 { margin:0; font-size:22px; font-weight:600; }
  .body { padding:28px 32px; }
  .greet { font-size:16px; margin:0 0 8px; }
  .desc { color:#5a6b7b; margin:0 0 20px; }
  .done { background:#eef7ee; border-left:4px solid #27ae60; padding:14px 18px; border-radius:4px; margin:0 0 20px; font-weight:600; }
  .content { margin:0 0 24px; }
  .video { position:relative; padding-bottom:56.25%; height:0; margin:0 0 24px; }
  .video iframe { position:absolute; top:0; left:0; width:100%; height:100%; border:0; border-radius:6px; }
  .btn { display:inline-block; background:#2c7be5; color:#fff; text-decoration:none; padding:11px 20px; border-radius:6px; border:0; font-size:15px; cursor:pointer; }
  .btn-complete { background:#27ae60; }
  .complete-form { margin:8px 0 0; }
  .footer { padding:18px 32px 24px; font-size:13px; color:#7f8c8d; border-top:1px solid #ecf0f1; }
</style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <div class="banner"><h1>{{.ModuleName}}</h1></div>
      <div class="body">
        {{if .FirstName}}<p class="greet">Hi {{.FirstName}},</p>{{end}}
        {{if .Description}}<p class="desc">{{.Description}}</p>{{end}}
        {{if .Completed}}<div class="done">&#10003; You&rsquo;ve completed this module. Thank you!</div>{{end}}

        {{if eq .ContentType "html"}}
          <div class="content">{{.Content}}</div>
        {{else if eq .ContentType "video"}}
          <div class="video"><iframe src="{{.URL}}" allowfullscreen></iframe></div>
        {{else}}
          <p>This training is hosted externally.</p>
          <p><a class="btn" href="{{.URL}}" target="_blank" rel="noopener noreferrer">Open the course &rarr;</a></p>
        {{end}}

        {{if not .Completed}}
        <form class="complete-form" method="POST" action="/learn/{{.Token}}/complete">
          <button type="submit" class="btn btn-complete">Mark as complete</button>
        </form>
        {{end}}
      </div>
      <div class="footer">Security awareness training</div>
    </div>
  </div>
</body>
</html>`

// LearnHandler renders the learner portal for a given enrollment token and
// records that the module has been started. Marking-started is downgrade-safe
// (see Enrollment.MarkStarted), so revisiting after completion is harmless.
func (ps *PhishingServer) LearnHandler(w http.ResponseWriter, r *http.Request) {
	token := mux.Vars(r)["token"]
	e, err := models.GetEnrollmentByToken(token)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	module, err := models.GetTrainingModule(e.ModuleID, e.UserID)
	if err != nil {
		// The module was deleted out from under the enrollment; there's
		// nothing to show. 404 rather than panic on the empty module.
		http.NotFound(w, r)
		return
	}
	if err := e.MarkStarted(); err != nil {
		log.Error(err)
	}
	recipient, _ := models.GetRecipientByID(e.RecipientID) // best-effort greeting
	renderLearnerPage(w, e, module, recipient)
}

// LearnCompleteHandler marks an enrollment completed. It is POST-only so a
// prefetch or accidental GET can't complete a module on the learner's behalf.
func (ps *PhishingServer) LearnCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	token := mux.Vars(r)["token"]
	e, err := models.GetEnrollmentByToken(token)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	module, err := models.GetTrainingModule(e.ModuleID, e.UserID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if err := e.MarkCompleted(); err != nil {
		log.Error(err)
	}
	recipient, _ := models.GetRecipientByID(e.RecipientID)
	renderLearnerPage(w, e, module, recipient)
}

func renderLearnerPage(w http.ResponseWriter, e models.Enrollment, module models.TrainingModule, recipient models.Recipient) {
	data := learnerPageData{
		ModuleName:  module.Name,
		Description: module.Description,
		FirstName:   recipient.FirstName,
		ContentType: module.ContentType,
		// module.URL is validated as absolute http(s) at module-save time
		// (models.TrainingModule.Validate), which is what keeps the iframe
		// src / external href safe — don't loosen that validation.
		Content:   template.HTML(module.HTML), //nolint:gosec // trusted operator content, see learnerPageData.Content
		URL:       module.URL,
		Token:     e.Token,
		Completed: e.Status == models.EnrollmentCompleted,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := learnerPageTmpl.Execute(w, data); err != nil {
		log.Error(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
