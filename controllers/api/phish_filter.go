package api

import (
	"encoding/json"
	"errors"
	"net/http"

	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// PhishFilter handles GET / PUT for the org-wide sandbox-filter policy
// stored in the phish_filter table. Used by the Settings UI.
//
// Status mapping:
//
//	200 — JSON PhishFilter row
//	400 — invalid body or unparseable IP/CIDR (returns the offending entry)
//	401 — handled upstream by RequireAPIKey
//	403 — handled upstream by RequirePermission(PermissionModifySystem)
//	500 — DB error
func (as *Server) PhishFilter(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pf, err := models.GetPhishFilter()
		if err != nil {
			log.Errorf("phish_filter: GET failed: %s", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, pf, http.StatusOK)

	case http.MethodPut:
		var pf models.PhishFilter
		if err := json.NewDecoder(r.Body).Decode(&pf); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "invalid request body: " + err.Error()}, http.StatusBadRequest)
			return
		}
		if err := models.PutPhishFilter(&pf); err != nil {
			// Distinguish validation errors (offending CIDR) from DB
			// failures so the UI can show the line back to the admin.
			if errors.Is(err, models.ErrInvalidSandboxIP) || err.Error() == "min_click_seconds must be >= 0" {
				JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
				return
			}
			log.Errorf("phish_filter: PUT failed: %s", err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, pf, http.StatusOK)

	default:
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
	}
}
