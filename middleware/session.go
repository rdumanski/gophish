package middleware

import (
	"encoding/gob"
	"net/http"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/rdumanski/gophish/models"
)

// init registers the necessary models to be saved in the session later
func init() {
	gob.Register(&models.User{})
	gob.Register(&models.Flash{})
	Store.Options.HttpOnly = true
	// SameSite=Lax is the right default for a first-party admin session cookie:
	// it ships on top-level navigations (so login works) while withholding the
	// cookie on cross-site requests (CSRF mitigation). Without an explicit mode
	// the cookie carried no SameSite attribute, which Go 1.25's cookiejar (and
	// modern browsers) drop over plain HTTP — breaking login behind a non-TLS
	// listener / reverse proxy.
	Store.Options.SameSite = http.SameSiteLaxMode
	// This sets the maxAge to 5 days for all cookies
	Store.MaxAge(86400 * 5)
}

// Store contains the session information for the request
var Store = sessions.NewCookieStore(
	[]byte(securecookie.GenerateRandomKey(64)), //Signing key
	[]byte(securecookie.GenerateRandomKey(32)))
