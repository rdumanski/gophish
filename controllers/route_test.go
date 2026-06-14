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
	client := loggedInClient(t, ctx)
	assertPageRenders(t, client, ctx.adminServer.URL+"/training_modules",
		[]string{"Training Modules", `id="content_type"`, `id="moduleTable"`, "/js/dist/app/training_modules.min.js", `href="/training_modules"`})
}

// TestTrainingCampaignsPageRenders verifies the Phase 11a training-campaigns page renders.
func TestTrainingCampaignsPageRenders(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	client := loggedInClient(t, ctx)
	assertPageRenders(t, client, ctx.adminServer.URL+"/training_campaigns",
		[]string{"Training Campaigns", `id="module"`, `id="groups"`, `id="campaignTable"`, "/js/dist/app/training_campaigns.min.js", `href="/training_campaigns"`})
}

// TestQuizzesPageRenders verifies the Phase 12a quiz-authoring page renders.
func TestQuizzesPageRenders(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	client := loggedInClient(t, ctx)
	assertPageRenders(t, client, ctx.adminServer.URL+"/quizzes",
		[]string{"Quizzes", `id="quizTable"`, `id="add_question"`, "/js/dist/app/quizzes.min.js", `href="/quizzes"`})
}

// TestRiskPageRenders verifies the Phase 13 risk-report page renders.
func TestRiskPageRenders(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	client := loggedInClient(t, ctx)
	assertPageRenders(t, client, ctx.adminServer.URL+"/risk",
		[]string{"Risk Report", `id="riskTable"`, "/js/dist/app/risk.min.js", `href="/risk"`})
}

func TestCompliancePageRenders(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	client := loggedInClient(t, ctx)
	assertPageRenders(t, client, ctx.adminServer.URL+"/compliance",
		[]string{"NIS2 Compliance Report", `id="groupTable"`, "/js/dist/app/compliance.min.js", `href="/compliance"`})
}

// TestComplianceReportPDF pins the session-authed PDF download: a logged-in
// client gets a real application/pdf body (so the api_key never rides the URL).
func TestComplianceReportPDF(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	client := loggedInClient(t, ctx)
	resp, err := client.Get(ctx.adminServer.URL + "/compliance/report.pdf?start=all")
	if err != nil {
		t.Fatalf("GET /compliance/report.pdf: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("expected application/pdf, got %q", ct)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) < 4 || string(body[:4]) != "%PDF" {
		t.Fatalf("expected %%PDF magic prefix, got %q (len %d)", string(body[:min(4, len(body))]), len(body))
	}
}

// loggedInClient returns an http.Client whose cookie jar holds an authenticated
// admin session. Unlike attemptLogin, it drives the WHOLE flow (GET /login →
// POST → page fetch) through the jar so the CSRF and session cookies round-trip
// consistently — the manual single-cookie forwarding in attemptLogin breaks the
// session on some platforms (Linux CI), bouncing authenticated GETs to /login.
// It also clears the admin's password-change flag so inner pages aren't
// redirected to /reset_password.
func loggedInClient(t *testing.T, ctx *testContext) *http.Client {
	t.Helper()
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

	resp, err := client.Get(ctx.adminServer.URL + "/login")
	if err != nil {
		t.Fatalf("GET /login: %v", err)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("parse /login: %v", err)
	}
	token, ok := doc.Find("input[name='csrf_token']").First().Attr("value")
	if !ok {
		t.Fatal("csrf_token not found on /login")
	}
	resp, err = client.PostForm(ctx.adminServer.URL+"/login", url.Values{
		"username":   {"admin"},
		"password":   {"gophish"},
		"csrf_token": {token},
	})
	if err != nil {
		t.Fatalf("POST /login: %v", err)
	}
	resp.Body.Close()
	return client
}

// assertPageRenders fetches an authenticated admin page and asserts the markers.
func assertPageRenders(t *testing.T, client *http.Client, url string, markers []string) {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: expected 200, got %d", url, resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read %s: %v", url, err)
	}
	for _, m := range markers {
		if !strings.Contains(string(body), m) {
			t.Fatalf("page %s missing marker %q", url, m)
		}
	}
}

// TestDomainsPageRenders verifies the Phase 15a domains registry page renders.
func TestDomainsPageRenders(t *testing.T) {
	ctx := setupTest(t)
	defer tearDown(t, ctx)
	client := loggedInClient(t, ctx)
	assertPageRenders(t, client, ctx.adminServer.URL+"/domains",
		[]string{"Domains", `id="domainTable"`, `id="role"`, "/js/dist/app/domains.min.js", `href="/domains"`})
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
