package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rdumanski/gophish/config"
	"github.com/rdumanski/gophish/models"
)

type testContext struct {
	apiKey    string
	config    *config.Config
	apiServer *Server
	admin     models.User
}

func setupTest(t *testing.T) *testContext {
	conf := &config.Config{
		DBName:         "sqlite3",
		DBPath:         ":memory:",
		MigrationsPath: "../../db/db_sqlite3/migrations/",
	}
	err := models.Setup(conf)
	if err != nil {
		t.Fatalf("Failed creating database: %v", err)
	}
	ctx := &testContext{}
	ctx.config = conf
	// Get the API key to use for these tests
	u, err := models.GetUser(1)
	if err != nil {
		t.Fatalf("error getting admin user: %v", err)
	}
	ctx.apiKey = u.APIKey
	ctx.admin = u
	ctx.apiServer = NewServer()
	return ctx
}

func TestSiteImportBaseHref(t *testing.T) {
	ctx := setupTest(t)
	h := "<html><head></head><body><img src=\"/test.png\"/></body></html>"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, h)
	}))
	expected := fmt.Sprintf("<html><head><base href=\"%s\"/></head><body><img src=\"/test.png\"/>\n</body></html>", ts.URL)
	defer ts.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/import/site",
		bytes.NewBuffer([]byte(fmt.Sprintf(`
			{
				"url" : "%s",
				"include_resources" : false
			}
		`, ts.URL))))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	ctx.apiServer.ImportSite(response, req)
	cs := cloneResponse{}
	err := json.NewDecoder(response.Body).Decode(&cs)
	if err != nil {
		t.Fatalf("error decoding response: %v", err)
	}
	if cs.HTML != expected {
		t.Fatalf("unexpected response received. expected %s got %s", expected, cs.HTML)
	}
}
