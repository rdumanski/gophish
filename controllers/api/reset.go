package api

import (
	"net/http"

	"github.com/rdumanski/gophish/auth"
	ctx "github.com/rdumanski/gophish/context"
	"github.com/rdumanski/gophish/models"
)

// Reset (/api/reset) resets the currently authenticated user's API key
func (as *Server) Reset(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "POST":
		u := ctx.Get(r, "user").(models.User)
		key, err := auth.GenerateSecureKey(auth.APIKeyLength)
		if err != nil {
			http.Error(w, "Error generating API Key", http.StatusInternalServerError)
			return
		}
		u.ApiKey = key
		err = models.PutUser(&u)
		if err != nil {
			http.Error(w, "Error setting API Key", http.StatusInternalServerError)
		} else {
			JSONResponse(w, models.Response{Success: true, Message: "API Key successfully reset!", Data: u.ApiKey}, http.StatusOK)
		}
	}
}
