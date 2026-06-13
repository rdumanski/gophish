package controllers

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/rdumanski/gophish/models"
)

func attemptLogin(t *testing.T, ctx *testContext, client *http.Client, username, password, optionalPath string) *http.Response {
	resp, err := http.Get(fmt.Sprintf("%s/login", ctx.adminServer.URL))
	if err != nil {
		t.Fatalf("error requesting the /login endpoint: %v", err)
	}
	got := resp.StatusCode
	expected := http.StatusOK
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		t.Fatalf("error parsing /login response body")
	}
	elem := doc.Find("input[name='csrf_token']").First()
	token, ok := elem.Attr("value")
	if !ok {
		t.Fatal("unable to find csrf_token value in login response")
	}
	if client == nil {
		client = &http.Client{}
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/login%s", ctx.adminServer.URL, optionalPath), strings.NewReader(url.Values{
		"username":   {username},
		"password":   {password},
		"csrf_token": {token},
	}.Encode()))
	if err != nil {
		t.Fatalf("error creating new /login request: %v", err)
	}

	req.Header.Set("Cookie", resp.Header.Get("Set-Cookie"))
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("error requesting the /login endpoint: %v", err)
	}
	return resp
}

func TestLoginCSRF(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp, err := http.PostForm(fmt.Sprintf("%s/login", ctx.adminServer.URL),
		url.Values{
			"username": {"admin"},
			"password": {"gophish"},
		})

	if err != nil {
		t.Fatalf("error requesting the /login endpoint: %v", err)
	}

	got := resp.StatusCode
	expected := http.StatusForbidden
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}

func TestInvalidCredentials(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp := attemptLogin(t, ctx, nil, "admin", "bogus", "")
	got := resp.StatusCode
	expected := http.StatusUnauthorized
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}

func TestSuccessfulLogin(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp := attemptLogin(t, ctx, nil, "admin", "gophish", "")
	got := resp.StatusCode
	expected := http.StatusOK
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}

func TestSuccessfulRedirect(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	next := "/campaigns"
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}}
	resp := attemptLogin(t, ctx, client, "admin", "gophish", fmt.Sprintf("?next=%s", next))
	got := resp.StatusCode
	expected := http.StatusFound
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
	url, err := resp.Location()
	if err != nil {
		t.Fatalf("error parsing response Location header: %v", err)
	}
	if url.Path != next {
		t.Fatalf("unexpected Location header received. expected %s got %s", next, url.Path)
	}
}

// TestTrainingModulesPageRenders exercises the full admin-page render path for
// the Phase 10b training-modules library: route -> getTemplate -> base.html +
// nav.html + training_modules.html. A passing go build only proves the route
// compiles; this proves the templates parse and execute together and the page
// ships its expected wiring (the content-type select, the table, the bundle).
func TestTrainingModulesPageRenders(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)

	// The seeded admin is flagged password-change-required, which would
	// bounce every inner page to /reset_password. Clear it so we reach the
	// real training-modules page like a normal logged-in admin.
	u, err := models.GetUser(1)
	if err != nil {
		t.Fatalf("error getting admin user: %v", err)
	}
	u.PasswordChangeRequired = false
	if err := models.PutUser(&u); err != nil {
		t.Fatalf("error clearing password-change flag: %v", err)
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp := attemptLogin(t, ctx, client, "admin", "gophish", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: status %d", resp.StatusCode)
	}

	resp, err = client.Get(fmt.Sprintf("%s/training_modules", ctx.adminServer.URL))
	if err != nil {
		t.Fatalf("error requesting /training_modules: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /training_modules, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading /training_modules body: %v", err)
	}
	for _, marker := range []string{
		"Training Modules",
		`id="content_type"`,
		`id="moduleTable"`,
		"/js/dist/app/training_modules.min.js",
		`href="/training_modules"`, // nav link rendered
	} {
		if !strings.Contains(string(body), marker) {
			t.Fatalf("rendered training-modules page missing marker %q", marker)
		}
	}
}

// TestTrainingCampaignsPageRenders verifies the Phase 11a training-campaigns
// admin page renders through the full template chain.
func TestTrainingCampaignsPageRenders(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)

	u, err := models.GetUser(1)
	if err != nil {
		t.Fatalf("error getting admin user: %v", err)
	}
	u.PasswordChangeRequired = false
	if err := models.PutUser(&u); err != nil {
		t.Fatalf("error clearing password-change flag: %v", err)
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp := attemptLogin(t, ctx, client, "admin", "gophish", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: status %d", resp.StatusCode)
	}

	resp, err = client.Get(fmt.Sprintf("%s/training_campaigns", ctx.adminServer.URL))
	if err != nil {
		t.Fatalf("error requesting /training_campaigns: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /training_campaigns, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading body: %v", err)
	}
	for _, marker := range []string{
		"Training Campaigns",
		`id="module"`,
		`id="groups"`,
		`id="campaignTable"`,
		"/js/dist/app/training_campaigns.min.js",
		`href="/training_campaigns"`,
	} {
		if !strings.Contains(string(body), marker) {
			t.Fatalf("rendered training-campaigns page missing marker %q", marker)
		}
	}
}

// TestQuizzesPageRenders verifies the Phase 12a quiz-authoring page renders.
func TestQuizzesPageRenders(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)

	u, err := models.GetUser(1)
	if err != nil {
		t.Fatalf("error getting admin user: %v", err)
	}
	u.PasswordChangeRequired = false
	if err := models.PutUser(&u); err != nil {
		t.Fatalf("error clearing password-change flag: %v", err)
	}

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}
	resp := attemptLogin(t, ctx, client, "admin", "gophish", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed: status %d", resp.StatusCode)
	}

	resp, err = client.Get(fmt.Sprintf("%s/quizzes", ctx.adminServer.URL))
	if err != nil {
		t.Fatalf("error requesting /quizzes: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for /quizzes, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("error reading body: %v", err)
	}
	for _, marker := range []string{
		"Quizzes",
		`id="quizTable"`,
		`id="add_question"`,
		"/js/dist/app/quizzes.min.js",
		`href="/quizzes"`,
	} {
		if !strings.Contains(string(body), marker) {
			t.Fatalf("rendered quizzes page missing marker %q", marker)
		}
	}
}

func TestAccountLocked(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	resp := attemptLogin(t, ctx, nil, "houdini", "gophish", "")
	got := resp.StatusCode
	expected := http.StatusUnauthorized
	if got != expected {
		t.Fatalf("invalid status code received. expected %d got %d", expected, got)
	}
}
