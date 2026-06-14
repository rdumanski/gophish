package controllers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/rdumanski/gophish/i18n"
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
	Content template.HTML
	URL     string
	Token   string

	// Lang is the recipient's UI language (from Accept-Language); T localizes.
	Lang string

	// Engagement (Phase 19.2): the recipient's own gamified standing, shown as
	// a panel. HasEngagement guards rendering when the recipient resolved.
	HasEngagement bool
	Score         int
	Streak        int
	Badges        []string

	Completed bool

	// Quiz fields (Phase 12b). HasQuiz is true when the module has a quiz; the
	// learner must pass it to complete. ShowResult/LastScore/Passed render the
	// feedback after a submission.
	HasQuiz       bool
	Quiz          models.Quiz
	PassThreshold int
	ShowResult    bool
	LastScore     int
	Passed        bool
}

// T localizes a key for the learner page's language.
func (d learnerPageData) T(key string, args ...interface{}) string {
	return i18n.T(d.Lang, key, args...)
}

// learnerPageTmpl is the self-contained portal page. It is html/template (not
// the text/template used for phishing pages) so recipient- and quiz-supplied
// fields are auto-escaped; only Content is explicitly trusted. Inline CSS, no
// external resources — the phishing server is frequently network-isolated.
var learnerPageTmpl = template.Must(template.New("learner").Parse(learnerPageHTML))

const learnerPageHTML = `<!DOCTYPE html>
<html lang="{{.Lang}}">
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
  .fail { background:#fdecea; border-left:4px solid #c0392b; padding:14px 18px; border-radius:4px; margin:0 0 20px; }
  .content { margin:0 0 24px; }
  .video { position:relative; padding-bottom:56.25%; height:0; margin:0 0 24px; }
  .video iframe { position:absolute; top:0; left:0; width:100%; height:100%; border:0; border-radius:6px; }
  .quiz { border-top:1px solid #ecf0f1; margin-top:8px; padding-top:16px; }
  .quiz h2 { font-size:17px; margin:0 0 16px; }
  .question { margin:0 0 18px; }
  .q-prompt { font-weight:600; margin:0 0 8px; }
  .opt { display:block; padding:6px 0; cursor:pointer; }
  .opt input { margin-right:8px; }
  .btn { display:inline-block; background:#2c7be5; color:#fff; text-decoration:none; padding:11px 20px; border-radius:6px; border:0; font-size:15px; cursor:pointer; }
  .btn-complete { background:#27ae60; }
  .complete-form { margin:8px 0 0; }
  .footer { padding:18px 32px 24px; font-size:13px; color:#7f8c8d; border-top:1px solid #ecf0f1; }
  .score-panel { background:#f4f8fd; border:1px solid #d6e4f5; border-radius:8px; padding:14px 18px; margin:0 0 22px; }
  .score-panel .head { font-weight:600; margin:0 0 8px; }
  .score-num { display:inline-block; min-width:34px; text-align:center; background:#2c7be5; color:#fff; font-weight:700; border-radius:6px; padding:2px 8px; margin-left:6px; }
  .badge-pill { display:inline-block; background:#eaf3ff; color:#2c7be5; border:1px solid #cfe0f5; border-radius:12px; padding:2px 10px; font-size:12px; margin:4px 4px 0 0; }
  .streak { margin:8px 0 0; color:#27ae60; font-weight:600; }
</style>
</head>
<body>
  <div class="wrap">
    <div class="card">
      <div class="banner"><h1>{{.ModuleName}}</h1></div>
      <div class="body">
        {{if .FirstName}}<p class="greet">{{ .T "learner.greeting" .FirstName }}</p>{{end}}
        {{if .HasEngagement}}
        <div class="score-panel">
          <div class="head">{{ .T "learner.score_heading" }}<span class="score-num">{{.Score}}</span></div>
          {{range .Badges}}<span class="badge-pill">{{ $.T (printf "badge.%s" .) }}</span>{{end}}
          {{if .Streak}}<div class="streak">&#128293; {{ .T "learner.streak" .Streak }}</div>{{end}}
        </div>
        {{end}}
        {{if .Description}}<p class="desc">{{.Description}}</p>{{end}}
        {{if .Completed}}<div class="done">&#10003; {{ .T "learner.completed" }}</div>{{end}}

        {{if eq .ContentType "html"}}
          <div class="content">{{.Content}}</div>
        {{else if eq .ContentType "video"}}
          <div class="video"><iframe src="{{.URL}}" allowfullscreen></iframe></div>
        {{else}}
          <p>{{ .T "learner.external_intro" }}</p>
          <p><a class="btn" href="{{.URL}}" target="_blank" rel="noopener noreferrer">{{ .T "learner.open_course" }} &rarr;</a></p>
        {{end}}

        {{if not .Completed}}
          {{if .HasQuiz}}
            {{if .ShowResult}}
              <div class="fail">{{ .T "learner.quiz_fail" .LastScore .PassThreshold }}</div>
            {{end}}
            <div class="quiz">
              <h2>{{ .T "learner.quiz_heading" .PassThreshold }}</h2>
              <form method="POST" action="/learn/{{.Token}}/quiz">
                {{range .Quiz.Questions}}
                  {{$qid := .Id}}
                  <div class="question">
                    <p class="q-prompt">{{.Prompt}}</p>
                    {{range .Options}}
                      <label class="opt"><input type="radio" name="q_{{$qid}}" value="{{.Id}}" required> {{.Text}}</label>
                    {{end}}
                  </div>
                {{end}}
                <button type="submit" class="btn btn-complete">{{ .T "learner.submit_answers" }}</button>
              </form>
            </div>
          {{else}}
            <form class="complete-form" method="POST" action="/learn/{{.Token}}/complete">
              <button type="submit" class="btn btn-complete">{{ .T "learner.mark_complete" }}</button>
            </form>
          {{end}}
        {{end}}
      </div>
      <div class="footer">{{ .T "learner.footer" }}</div>
    </div>
  </div>
</body>
</html>`

// LearnHandler renders the learner portal for a given enrollment token and
// records that the module has been started. Marking-started is downgrade-safe
// (see Enrollment.MarkStarted), so revisiting after completion is harmless.
func (ps *PhishingServer) LearnHandler(w http.ResponseWriter, r *http.Request) {
	e, module, ok := ps.resolveLearner(w, r)
	if !ok {
		return
	}
	if err := e.MarkStarted(); err != nil {
		log.Error(err)
	}
	quizzes, _ := models.GetQuizzesByModule(module.Id, e.UserID)
	recipient, _ := models.GetRecipientByID(e.RecipientID)
	lang := i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
	renderLearnerPage(w, lang, e, module, recipient, quizzes, false, 0, false)
}

// LearnCompleteHandler marks an enrollment completed for modules WITHOUT a
// quiz (self-attested). POST-only so a prefetch/GET can't complete a module.
func (ps *PhishingServer) LearnCompleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	e, module, ok := ps.resolveLearner(w, r)
	if !ok {
		return
	}
	if err := e.MarkCompleted(); err != nil {
		log.Error(err)
	}
	quizzes, _ := models.GetQuizzesByModule(module.Id, e.UserID)
	recipient, _ := models.GetRecipientByID(e.RecipientID)
	lang := i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
	renderLearnerPage(w, lang, e, module, recipient, quizzes, false, 0, false)
}

// LearnQuizHandler scores a submitted quiz and gates completion on passing.
func (ps *PhishingServer) LearnQuizHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	e, module, ok := ps.resolveLearner(w, r)
	if !ok {
		return
	}
	quizzes, _ := models.GetQuizzesByModule(module.Id, e.UserID)
	if len(quizzes) == 0 {
		http.NotFound(w, r)
		return
	}
	quiz := quizzes[0]
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	answers := make(map[int64]int64)
	for _, q := range quiz.Questions {
		if v := r.PostForm.Get(fmt.Sprintf("q_%d", q.Id)); v != "" {
			if oid, err := strconv.ParseInt(v, 10, 64); err == nil {
				answers[q.Id] = oid
			}
		}
	}
	score, passed := models.ScoreQuiz(quiz, answers)
	if err := e.RecordQuizResult(score, passed); err != nil {
		log.Error(err)
	}
	recipient, _ := models.GetRecipientByID(e.RecipientID)
	lang := i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
	// Show the fail banner only when they didn't pass.
	renderLearnerPage(w, lang, e, module, recipient, quizzes, !passed, score, passed)
}

// resolveLearner loads the enrollment (by token) and its module, writing a 404
// and returning ok=false on any failure (bad token or deleted module).
func (ps *PhishingServer) resolveLearner(w http.ResponseWriter, r *http.Request) (models.Enrollment, models.TrainingModule, bool) {
	token := mux.Vars(r)["token"]
	e, err := models.GetEnrollmentByToken(token)
	if err != nil {
		http.NotFound(w, r)
		return e, models.TrainingModule{}, false
	}
	module, err := models.GetTrainingModule(e.ModuleID, e.UserID)
	if err != nil {
		http.NotFound(w, r)
		return e, module, false
	}
	return e, module, true
}

func renderLearnerPage(w http.ResponseWriter, lang string, e models.Enrollment, module models.TrainingModule, recipient models.Recipient, quizzes []models.Quiz, showResult bool, lastScore int, passed bool) {
	data := learnerPageData{
		ModuleName:  module.Name,
		Description: module.Description,
		FirstName:   recipient.FirstName,
		ContentType: module.ContentType,
		Lang:        lang,
		// module.URL is validated as absolute http(s) at module-save time
		// (models.TrainingModule.Validate), which is what keeps the iframe
		// src / external href safe — don't loosen that validation.
		Content:    template.HTML(module.HTML), //nolint:gosec // trusted operator content, see learnerPageData.Content
		URL:        module.URL,
		Token:      e.Token,
		Completed:  e.Status == models.EnrollmentCompleted,
		ShowResult: showResult,
		LastScore:  lastScore,
		Passed:     passed,
	}
	if len(quizzes) > 0 {
		data.HasQuiz = true
		data.Quiz = quizzes[0]
		data.PassThreshold = quizzes[0].PassThreshold
	}
	// The recipient's own engagement standing (Phase 19.2).
	if recipient.Id != 0 {
		if eng, err := models.GetRecipientEngagement(recipient.Id); err == nil {
			data.HasEngagement = true
			data.Score = eng.Score
			data.Streak = eng.Streak
			data.Badges = eng.Badges
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := learnerPageTmpl.Execute(w, data); err != nil {
		log.Error(err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
