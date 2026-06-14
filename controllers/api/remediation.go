package api

import (
	"encoding/json"
	"net/http"

	ctx "github.com/rdumanski/gophish/context"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// Remediation handles GET/PUT /api/remediation/ — the operator's remediation
// auto-enrollment settings.
func (as *Server) Remediation(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	switch r.Method {
	case http.MethodGet:
		JSONResponse(w, models.GetRemediationSettings(uid), http.StatusOK)
	case http.MethodPut:
		s := models.RemediationSettings{}
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		s.UserID = uid
		if err := models.PutRemediationSettings(&s); err != nil {
			log.Error(err)
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Successfully saved remediation settings."}, http.StatusCreated)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
	}
}
