package controllers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/rdumanski/gophish/models"
)

// TestLearnerPortalFlow exercises the Phase 10c learner portal end to end
// through the real phishing server: render + mark started, complete, a revisit
// that must stay completed (the downgrade-guard case), and a bad token 404.
func TestLearnerPortalFlow(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)

	module := models.TrainingModule{
		Name:        "Spot the Phish",
		Description: "A short lesson",
		ContentType: models.TrainingContentHTML,
		HTML:        "<p id='lesson'>Lesson body here</p>",
		UserID:      1,
	}
	if err := models.PostTrainingModule(&module); err != nil {
		t.Fatalf("error creating module: %v", err)
	}
	e, err := models.CreateEnrollmentByEmail(1, models.BaseRecipient{Email: "learner@example.com", FirstName: "Lee"}, module.Id)
	if err != nil {
		t.Fatalf("error creating enrollment: %v", err)
	}

	// 1. GET the portal: renders the module content and marks started.
	body := getBody(t, fmt.Sprintf("%s/learn/%s", ctx.phishServer.URL, e.Token), http.StatusOK)
	for _, marker := range []string{"Spot the Phish", "Lesson body here", "Hi Lee", "Mark as complete"} {
		if !strings.Contains(body, marker) {
			t.Fatalf("portal page missing %q; got:\n%s", marker, body)
		}
	}
	if reloaded, _ := models.GetEnrollmentByToken(e.Token); reloaded.Status != models.EnrollmentStarted {
		t.Fatalf("expected status %q after GET, got %q", models.EnrollmentStarted, reloaded.Status)
	}

	// 2. POST complete: marks completed and confirms.
	resp, err := (&http.Client{}).PostForm(fmt.Sprintf("%s/learn/%s/complete", ctx.phishServer.URL, e.Token), url.Values{})
	if err != nil {
		t.Fatalf("error posting complete: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on complete, got %d", resp.StatusCode)
	}
	completed, _ := models.GetEnrollmentByToken(e.Token)
	if completed.Status != models.EnrollmentCompleted {
		t.Fatalf("expected status %q after complete, got %q", models.EnrollmentCompleted, completed.Status)
	}
	completedAt := completed.CompletedDate

	// 3. Revisit (GET) must NOT downgrade a completed enrollment.
	getBody(t, fmt.Sprintf("%s/learn/%s", ctx.phishServer.URL, e.Token), http.StatusOK)
	revisited, _ := models.GetEnrollmentByToken(e.Token)
	if revisited.Status != models.EnrollmentCompleted {
		t.Fatalf("revisit downgraded status to %q", revisited.Status)
	}
	if !revisited.CompletedDate.Equal(completedAt) {
		t.Fatalf("revisit changed CompletedDate")
	}

	// 4. Unknown token -> 404.
	resp, err = http.Get(fmt.Sprintf("%s/learn/deadbeefdeadbeef", ctx.phishServer.URL))
	if err != nil {
		t.Fatalf("error requesting bad token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown token, got %d", resp.StatusCode)
	}
}

// TestLearnerQuizFlow covers the Phase 12b quiz path: a module with a quiz
// shows the quiz (not a self-attest button); a wrong answer fails (no
// completion); a correct answer passes and completes.
func TestLearnerQuizFlow(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)

	module := models.TrainingModule{Name: "Quiz Module", ContentType: models.TrainingContentHTML, HTML: "<p>read me</p>", UserID: 1}
	if err := models.PostTrainingModule(&module); err != nil {
		t.Fatalf("module: %v", err)
	}
	quiz := models.Quiz{
		ModuleID: module.Id, Title: "Check", PassThreshold: 100,
		Questions: []models.QuizQuestion{{
			Prompt:  "Pick the right one",
			Options: []models.QuizOption{{Text: "right", IsCorrect: true}, {Text: "wrong", IsCorrect: false}},
		}},
	}
	if err := models.PostQuiz(&quiz, 1); err != nil {
		t.Fatalf("quiz: %v", err)
	}
	e, err := models.CreateEnrollmentByEmail(1, models.BaseRecipient{Email: "lq@example.com"}, module.Id)
	if err != nil {
		t.Fatalf("enroll: %v", err)
	}

	// Portal shows the quiz form, not the self-attest complete button.
	body := getBody(t, fmt.Sprintf("%s/learn/%s", ctx.phishServer.URL, e.Token), http.StatusOK)
	if !strings.Contains(body, "Submit answers") || !strings.Contains(body, "Pick the right one") {
		t.Fatalf("quiz form not rendered; got:\n%s", body)
	}

	loaded, _ := models.GetQuiz(quiz.Id, 1)
	rightID := loaded.Questions[0].Options[0].Id
	wrongID := loaded.Questions[0].Options[1].Id
	qfield := fmt.Sprintf("q_%d", loaded.Questions[0].Id)
	quizURL := fmt.Sprintf("%s/learn/%s/quiz", ctx.phishServer.URL, e.Token)

	// Wrong answer -> not completed.
	resp, err := (&http.Client{}).PostForm(quizURL, url.Values{qfield: {fmt.Sprintf("%d", wrongID)}})
	if err != nil {
		t.Fatalf("post wrong: %v", err)
	}
	resp.Body.Close()
	if afterFail, _ := models.GetEnrollmentByToken(e.Token); afterFail.Status == models.EnrollmentCompleted {
		t.Fatalf("wrong answer should not complete the enrollment")
	}

	// Correct answer -> completed with score 100.
	resp, err = (&http.Client{}).PostForm(quizURL, url.Values{qfield: {fmt.Sprintf("%d", rightID)}})
	if err != nil {
		t.Fatalf("post right: %v", err)
	}
	resp.Body.Close()
	afterPass, _ := models.GetEnrollmentByToken(e.Token)
	if afterPass.Status != models.EnrollmentCompleted {
		t.Fatalf("correct answer should complete; got %q", afterPass.Status)
	}
	if afterPass.QuizScore != 100 {
		t.Fatalf("expected score 100, got %d", afterPass.QuizScore)
	}
}

func getBody(t *testing.T, url string, wantStatus int) string {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("error requesting %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s: expected status %d, got %d", url, wantStatus, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading body: %v", err)
	}
	return string(b)
}
