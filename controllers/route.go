package controllers

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/csrf"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/jordan-wright/unindexed"
	"github.com/rdumanski/gophish/ai"
	"github.com/rdumanski/gophish/auth"
	"github.com/rdumanski/gophish/config"
	ctx "github.com/rdumanski/gophish/context"
	"github.com/rdumanski/gophish/controllers/api"
	"github.com/rdumanski/gophish/i18n"
	log "github.com/rdumanski/gophish/logger"
	mid "github.com/rdumanski/gophish/middleware"
	"github.com/rdumanski/gophish/middleware/ratelimit"
	"github.com/rdumanski/gophish/models"
	"github.com/rdumanski/gophish/util"
	"github.com/rdumanski/gophish/worker"
)

// AdminServerOption is a functional option that is used to configure the
// admin server
type AdminServerOption func(*AdminServer)

// AdminServer is an HTTP server that implements the administrative Gophish
// handlers, including the dashboard and REST API.
type AdminServer struct {
	server      *http.Server
	worker      worker.Worker
	config      config.AdminServer
	limiter     *ratelimit.PostLimiter
	aiGenerator ai.Generator
}

var defaultTLSConfig = &tls.Config{
	PreferServerCipherSuites: true,
	CurvePreferences: []tls.CurveID{
		tls.X25519,
		tls.CurveP256,
	},
	MinVersion: tls.VersionTLS12,
	CipherSuites: []uint16{
		tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,

		// Kept for backwards compatibility with some clients
		tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	},
}

// WithWorker is an option that sets the background worker.
func WithWorker(w worker.Worker) AdminServerOption {
	return func(as *AdminServer) {
		as.worker = w
	}
}

// WithAIGenerator wires an ai.Generator into the admin server. When
// nil, POST /api/templates/generate returns 503.
func WithAIGenerator(g ai.Generator) AdminServerOption {
	return func(as *AdminServer) {
		as.aiGenerator = g
	}
}

// NewAdminServer returns a new instance of the AdminServer with the
// provided config and options applied.
func NewAdminServer(config config.AdminServer, options ...AdminServerOption) *AdminServer {
	defaultWorker, _ := worker.New()
	defaultServer := &http.Server{
		ReadTimeout: 10 * time.Second,
		Addr:        config.ListenURL,
	}
	defaultLimiter := ratelimit.NewPostLimiter()
	as := &AdminServer{
		worker:  defaultWorker,
		server:  defaultServer,
		limiter: defaultLimiter,
		config:  config,
	}
	for _, opt := range options {
		opt(as)
	}
	as.registerRoutes()
	return as
}

// Start launches the admin server, listening on the configured address.
func (as *AdminServer) Start() {
	if as.worker != nil {
		go as.worker.Start()
	}
	if as.config.UseTLS {
		// Only support TLS 1.2 and above - ref #1691, #1689
		as.server.TLSConfig = defaultTLSConfig
		err := util.CheckAndCreateSSL(as.config.CertPath, as.config.KeyPath)
		if err != nil {
			log.Fatal(err)
		}
		log.Infof("Starting admin server at https://%s", as.config.ListenURL)
		log.Fatal(as.server.ListenAndServeTLS(as.config.CertPath, as.config.KeyPath))
	}
	// If TLS isn't configured, just listen on HTTP
	log.Infof("Starting admin server at http://%s", as.config.ListenURL)
	log.Fatal(as.server.ListenAndServe())
}

// Shutdown attempts to gracefully shutdown the server.
func (as *AdminServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	return as.server.Shutdown(ctx)
}

// SetupAdminRoutes creates the routes for handling requests to the web interface.
// This function returns an http.Handler to be used in http.ListenAndServe().
func (as *AdminServer) registerRoutes() {
	router := mux.NewRouter()
	// Base Front-end routes
	router.HandleFunc("/", mid.Use(as.Base, mid.RequireLogin))
	router.HandleFunc("/login", mid.Use(as.Login, as.limiter.Limit))
	router.HandleFunc("/logout", mid.Use(as.Logout, mid.RequireLogin))
	router.HandleFunc("/reset_password", mid.Use(as.ResetPassword, mid.RequireLogin))
	router.HandleFunc("/campaigns", mid.Use(as.Campaigns, mid.RequireLogin))
	router.HandleFunc("/campaigns/{id:[0-9]+}", mid.Use(as.CampaignID, mid.RequireLogin))
	router.HandleFunc("/templates", mid.Use(as.Templates, mid.RequireLogin))
	router.HandleFunc("/training_modules", mid.Use(as.TrainingModules, mid.RequireLogin))
	router.HandleFunc("/training_campaigns", mid.Use(as.TrainingCampaignsPage, mid.RequireLogin))
	router.HandleFunc("/quizzes", mid.Use(as.Quizzes, mid.RequireLogin))
	router.HandleFunc("/risk", mid.Use(as.Risk, mid.RequireLogin))
	router.HandleFunc("/leaderboard", mid.Use(as.Leaderboard, mid.RequireLogin))
	router.HandleFunc("/compliance", mid.Use(as.Compliance, mid.RequireLogin))
	router.HandleFunc("/language", mid.Use(as.SetLanguage, mid.RequireLogin))
	router.HandleFunc("/compliance/report.pdf", mid.Use(as.ComplianceReportPDF, mid.RequireLogin))
	router.HandleFunc("/groups", mid.Use(as.Groups, mid.RequireLogin))
	router.HandleFunc("/landing_pages", mid.Use(as.LandingPages, mid.RequireLogin))
	router.HandleFunc("/sending_profiles", mid.Use(as.SendingProfiles, mid.RequireLogin))
	router.HandleFunc("/settings", mid.Use(as.Settings, mid.RequireLogin))
	router.HandleFunc("/users", mid.Use(as.UserManagement, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	router.HandleFunc("/webhooks", mid.Use(as.Webhooks, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	router.HandleFunc("/domains", mid.Use(as.Domains, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	router.HandleFunc("/impersonate", mid.Use(as.Impersonate, mid.RequirePermission(models.PermissionModifySystem), mid.RequireLogin))
	// Create the API routes
	apiOpts := []api.ServerOption{
		api.WithWorker(as.worker),
		api.WithLimiter(as.limiter),
	}
	if as.aiGenerator != nil {
		apiOpts = append(apiOpts, api.WithAIGenerator(as.aiGenerator))
	}
	apiSrv := api.NewServer(apiOpts...)
	router.PathPrefix("/api/").Handler(apiSrv)

	// Setup static file serving
	router.PathPrefix("/").Handler(http.FileServer(unindexed.Dir("./static/")))

	// Setup CSRF Protection
	csrfKey := []byte(as.config.CSRFKey)
	if len(csrfKey) == 0 {
		key, err := auth.GenerateSecureKey(auth.APIKeyLength)
		if err != nil {
			log.Fatalf("could not generate CSRF key: %s", err)
		}
		csrfKey = []byte(key)
	}
	// Mark the session cookie Secure only when actually serving TLS. gorilla/
	// sessions v1.3.0 defaults Options.Secure=true, which makes the cookie
	// unusable over plain HTTP (browsers and Go 1.25+'s cookiejar drop Secure
	// cookies on non-HTTPS) — breaking login behind a non-TLS listener or a
	// TLS-terminating reverse proxy. Mirrors csrf.Secure(UseTLS) below.
	mid.Store.Options.Secure = as.config.UseTLS
	csrfHandler := csrf.Protect(csrfKey,
		csrf.FieldName("csrf_token"),
		csrf.Secure(as.config.UseTLS),
		csrf.TrustedOrigins(as.config.TrustedOrigins))
	adminHandler := csrfHandler(router)
	// gorilla/csrf v1.7+ defaults to assuming HTTPS and enforces a strict
	// Referer check. Opt out via PlaintextHTTPRequest when the admin server
	// is configured to serve plain HTTP, otherwise every state-changing POST
	// without a Referer header is rejected with 403.
	if !as.config.UseTLS {
		inner := adminHandler
		adminHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			inner.ServeHTTP(w, csrf.PlaintextHTTPRequest(r))
		})
	}
	adminHandler = mid.Use(adminHandler.ServeHTTP, mid.CSRFExceptions, mid.GetContext, mid.ApplySecurityHeaders)

	// Setup GZIP compression
	gzipWrapper, _ := gziphandler.NewGzipLevelHandler(gzip.BestCompression)
	adminHandler = gzipWrapper(adminHandler)

	// Respect X-Forwarded-For and X-Real-IP headers in case we're behind a
	// reverse proxy.
	adminHandler = handlers.ProxyHeaders(adminHandler)

	// Setup logging
	adminHandler = handlers.CombinedLoggingHandler(log.Writer(), adminHandler)
	as.server.Handler = adminHandler
}

type templateParams struct {
	Title        string
	Flashes      []interface{}
	User         models.User
	Token        string
	Version      string
	ModifySystem bool
	// AIEnabled is true when the admin server was constructed with an
	// AI generator (i.e. config.AI.Enabled == true and a provider key was
	// supplied). Page templates use it to hide the "Draft with AI"
	// affordance when the feature is off.
	AIEnabled bool
	// Language is the resolved UI language ("en"/"pl"). Templates localize via
	// the T method below; the active catalog is injected for the JS side via
	// I18nJSON.
	Language string
}

// T localizes a message key for the page's language. Templates call it as
// {{ .T "nav.dashboard" }}; optional args feed fmt.Sprintf placeholders.
func (p templateParams) T(key string, args ...interface{}) string {
	return i18n.T(p.Language, key, args...)
}

// I18nJSON is the active catalog as JSON, injected into the page as window.i18n
// so the browser bundles share the same single source of truth.
func (p templateParams) I18nJSON() template.JS {
	return i18n.CatalogJSON(p.Language)
}

// SupportedLanguages lists the available UI languages for the switcher.
func (p templateParams) SupportedLanguages() []string {
	return i18n.Supported()
}

// loginParams is the render context for the pre-auth login page (which defines
// its own base layout). Language is resolved from Accept-Language since there is
// no logged-in user yet.
type loginParams struct {
	User     models.User
	Title    string
	Flashes  []interface{}
	Token    string
	Language string
}

// T localizes a key for the login page's language.
func (p loginParams) T(key string, args ...interface{}) string {
	return i18n.T(p.Language, key, args...)
}

// newTemplateParams returns the default template parameters for a user and
// the CSRF token.
func (as *AdminServer) newTemplateParams(r *http.Request) templateParams {
	user := ctx.Get(r, "user").(models.User)
	session := ctx.Get(r, "session").(*sessions.Session)
	modifySystem, _ := user.HasPermission(models.PermissionModifySystem)
	return templateParams{
		Token:        csrf.Token(r),
		User:         user,
		ModifySystem: modifySystem,
		Version:      config.Version,
		Flashes:      session.Flashes(),
		AIEnabled:    as.aiGenerator != nil,
		Language:     i18n.Normalize(user.Language),
	}
}

// Base handles the default path and template execution
func (as *AdminServer) Base(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Dashboard"
	getTemplate(w, "dashboard").ExecuteTemplate(w, "base", params)
}

// Campaigns handles the default path and template execution
func (as *AdminServer) Campaigns(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Campaigns"
	getTemplate(w, "campaigns").ExecuteTemplate(w, "base", params)
}

// CampaignID handles the default path and template execution
func (as *AdminServer) CampaignID(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Campaign Results"
	getTemplate(w, "campaign_results").ExecuteTemplate(w, "base", params)
}

// Templates handles the default path and template execution
func (as *AdminServer) Templates(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Email Templates"
	getTemplate(w, "templates").ExecuteTemplate(w, "base", params)
}

// TrainingModules handles the training-modules library page.
func (as *AdminServer) TrainingModules(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Training Modules"
	getTemplate(w, "training_modules").ExecuteTemplate(w, "base", params)
}

// TrainingCampaignsPage handles the training-campaigns page.
func (as *AdminServer) TrainingCampaignsPage(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Training Campaigns"
	getTemplate(w, "training_campaigns").ExecuteTemplate(w, "base", params)
}

// Quizzes handles the quizzes authoring page.
func (as *AdminServer) Quizzes(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Quizzes"
	getTemplate(w, "quizzes").ExecuteTemplate(w, "base", params)
}

// Risk handles the per-recipient risk-score report page.
func (as *AdminServer) Risk(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Risk Report"
	getTemplate(w, "risk").ExecuteTemplate(w, "base", params)
}

// Leaderboard renders the gamified engagement / leaderboard page.
func (as *AdminServer) Leaderboard(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Leaderboard"
	getTemplate(w, "leaderboard").ExecuteTemplate(w, "base", params)
}

// Compliance renders the NIS2 compliance report page.
func (as *AdminServer) Compliance(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "NIS2 Compliance Report"
	getTemplate(w, "compliance").ExecuteTemplate(w, "base", params)
}

// SetLanguage persists the current user's preferred UI language and redirects
// back. Language is a personal preference, so it's session-authed only (no
// RBAC) and a simple GET so it can be a plain link. e.g. GET /language?lang=pl
func (as *AdminServer) SetLanguage(w http.ResponseWriter, r *http.Request) {
	user := ctx.Get(r, "user").(models.User)
	user.Language = i18n.Normalize(r.URL.Query().Get("lang"))
	if err := models.PutUser(&user); err != nil {
		log.Error(err)
	}
	dest := r.Header.Get("Referer")
	if dest == "" {
		dest = "/"
	}
	http.Redirect(w, r, dest, http.StatusFound)
}

// Groups handles the default path and template execution
func (as *AdminServer) Groups(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Users & Groups"
	getTemplate(w, "groups").ExecuteTemplate(w, "base", params)
}

// LandingPages handles the default path and template execution
func (as *AdminServer) LandingPages(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Landing Pages"
	getTemplate(w, "landing_pages").ExecuteTemplate(w, "base", params)
}

// SendingProfiles handles the default path and template execution
func (as *AdminServer) SendingProfiles(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Sending Profiles"
	getTemplate(w, "sending_profiles").ExecuteTemplate(w, "base", params)
}

// Settings handles the changing of settings
func (as *AdminServer) Settings(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		params := as.newTemplateParams(r)
		params.Title = "Settings"
		session := ctx.Get(r, "session").(*sessions.Session)
		session.Save(r, w)
		getTemplate(w, "settings").ExecuteTemplate(w, "base", params)
	case r.Method == "POST":
		u := ctx.Get(r, "user").(models.User)
		currentPw := r.FormValue("current_password")
		newPassword := r.FormValue("new_password")
		confirmPassword := r.FormValue("confirm_new_password")
		// Check the current password
		err := auth.ValidatePassword(currentPw, u.Hash)
		msg := models.Response{Success: true, Message: "Settings Updated Successfully"}
		if err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusBadRequest)
			return
		}
		newHash, err := auth.ValidatePasswordChange(u.Hash, newPassword, confirmPassword)
		if err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusBadRequest)
			return
		}
		u.Hash = string(newHash)
		if err = models.PutUser(&u); err != nil {
			msg.Message = err.Error()
			msg.Success = false
			api.JSONResponse(w, msg, http.StatusInternalServerError)
			return
		}
		api.JSONResponse(w, msg, http.StatusOK)
	}
}

// UserManagement is an admin-only handler that allows for the registration
// and management of user accounts within Gophish.
func (as *AdminServer) UserManagement(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "User Management"
	getTemplate(w, "users").ExecuteTemplate(w, "base", params)
}

func (as *AdminServer) nextOrIndex(w http.ResponseWriter, r *http.Request) {
	next := "/"
	url, err := url.Parse(r.FormValue("next"))
	if err == nil {
		path := url.EscapedPath()
		if path != "" {
			next = "/" + strings.TrimLeft(path, "/")
		}
	}
	http.Redirect(w, r, next, http.StatusFound)
}

func (as *AdminServer) handleInvalidLogin(w http.ResponseWriter, r *http.Request, messageKey string) {
	session := ctx.Get(r, "session").(*sessions.Session)
	lang := i18n.FromAcceptLanguage(r.Header.Get("Accept-Language"))
	Flash(w, r, "danger", i18n.T(lang, messageKey))
	params := loginParams{Title: "Login", Token: csrf.Token(r), Language: lang}
	params.Flashes = session.Flashes()
	session.Save(r, w)
	templates := template.New("template")
	_, err := templates.ParseFiles("templates/login.html", "templates/flashes.html")
	if err != nil {
		log.Error(err)
	}
	// w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	template.Must(templates, err).ExecuteTemplate(w, "base", params)
}

// Webhooks is an admin-only handler that handles webhooks
func (as *AdminServer) Webhooks(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Webhooks"
	getTemplate(w, "webhooks").ExecuteTemplate(w, "base", params)
}

// Domains handles the custom-domains registry page.
func (as *AdminServer) Domains(w http.ResponseWriter, r *http.Request) {
	params := as.newTemplateParams(r)
	params.Title = "Domains"
	getTemplate(w, "domains").ExecuteTemplate(w, "base", params)
}

// Impersonate allows an admin to login to a user account without needing the password
func (as *AdminServer) Impersonate(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" {
		username := r.FormValue("username")
		u, err := models.GetUserByUsername(username)
		if err != nil {
			log.Error(err)
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		session := ctx.Get(r, "session").(*sessions.Session)
		session.Values["id"] = u.Id
		session.Save(r, w)
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

// Login handles the authentication flow for a user. If credentials are valid,
// a session is created
func (as *AdminServer) Login(w http.ResponseWriter, r *http.Request) {
	params := loginParams{
		Title:    "Login",
		Token:    csrf.Token(r),
		Language: i18n.FromAcceptLanguage(r.Header.Get("Accept-Language")),
	}
	session := ctx.Get(r, "session").(*sessions.Session)
	switch {
	case r.Method == "GET":
		params.Flashes = session.Flashes()
		session.Save(r, w)
		templates := template.New("template")
		_, err := templates.ParseFiles("templates/login.html", "templates/flashes.html")
		if err != nil {
			log.Error(err)
		}
		template.Must(templates, err).ExecuteTemplate(w, "base", params)
	case r.Method == "POST":
		// Find the user with the provided username
		username, password := r.FormValue("username"), r.FormValue("password")
		u, err := models.GetUserByUsername(username)
		if err != nil {
			log.Error(err)
			as.handleInvalidLogin(w, r, "login.invalid")
			return
		}
		// Validate the user's password
		err = auth.ValidatePassword(password, u.Hash)
		if err != nil {
			log.Error(err)
			as.handleInvalidLogin(w, r, "login.invalid")
			return
		}
		if u.AccountLocked {
			as.handleInvalidLogin(w, r, "login.locked")
			return
		}
		u.LastLogin = time.Now().UTC()
		err = models.PutUser(&u)
		if err != nil {
			log.Error(err)
		}
		// If we've logged in, save the session and redirect to the dashboard
		session.Values["id"] = u.Id
		session.Save(r, w)
		as.nextOrIndex(w, r)
	}
}

// Logout destroys the current user session
func (as *AdminServer) Logout(w http.ResponseWriter, r *http.Request) {
	session := ctx.Get(r, "session").(*sessions.Session)
	delete(session.Values, "id")
	Flash(w, r, "success", "You have successfully logged out")
	session.Save(r, w)
	http.Redirect(w, r, "/login", http.StatusFound)
}

// ResetPassword handles the password reset flow when a password change is
// required either by the Gophish system or an administrator.
//
// This handler is meant to be used when a user is required to reset their
// password, not just when they want to.
//
// This is an important distinction since in this handler we don't require
// the user to re-enter their current password, as opposed to the flow
// through the settings handler.
//
// To that end, if the user doesn't require a password change, we will
// redirect them to the settings page.
func (as *AdminServer) ResetPassword(w http.ResponseWriter, r *http.Request) {
	u := ctx.Get(r, "user").(models.User)
	session := ctx.Get(r, "session").(*sessions.Session)
	if !u.PasswordChangeRequired {
		Flash(w, r, "info", "Please reset your password through the settings page")
		session.Save(r, w)
		http.Redirect(w, r, "/settings", http.StatusTemporaryRedirect)
		return
	}
	params := as.newTemplateParams(r)
	params.Title = "Reset Password"
	switch {
	case r.Method == http.MethodGet:
		params.Flashes = session.Flashes()
		session.Save(r, w)
		getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
		return
	case r.Method == http.MethodPost:
		newPassword := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		newHash, err := auth.ValidatePasswordChange(u.Hash, newPassword, confirmPassword)
		if err != nil {
			Flash(w, r, "danger", err.Error())
			params.Flashes = session.Flashes()
			session.Save(r, w)
			w.WriteHeader(http.StatusBadRequest)
			getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
			return
		}
		u.PasswordChangeRequired = false
		u.Hash = newHash
		if err = models.PutUser(&u); err != nil {
			Flash(w, r, "danger", err.Error())
			params.Flashes = session.Flashes()
			session.Save(r, w)
			w.WriteHeader(http.StatusInternalServerError)
			getTemplate(w, "reset_password").ExecuteTemplate(w, "base", params)
			return
		}
		// TODO: We probably want to flash a message here that the password was
		// changed successfully. The problem is that when the user resets their
		// password on first use, they will see two flashes on the dashboard-
		// one for their password reset, and one for the "no campaigns created".
		//
		// The solution to this is to revamp the empty page to be more useful,
		// like a wizard or something.
		as.nextOrIndex(w, r)
	}
}

// TODO: Make this execute the template, too
func getTemplate(w http.ResponseWriter, tmpl string) *template.Template {
	templates := template.New("template")
	_, err := templates.ParseFiles("templates/base.html", "templates/nav.html", "templates/"+tmpl+".html", "templates/flashes.html")
	if err != nil {
		log.Error(err)
	}
	return template.Must(templates, err)
}

// Flash handles the rendering flash messages
func Flash(w http.ResponseWriter, r *http.Request, t string, m string) {
	session := ctx.Get(r, "session").(*sessions.Session)
	session.AddFlash(models.Flash{
		Type:    t,
		Message: m,
	})
}
