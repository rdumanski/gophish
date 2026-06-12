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
