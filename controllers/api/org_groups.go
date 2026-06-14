package api

import (
	"fmt"
	"net/http"

	ctx "github.com/rdumanski/gophish/context"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// RegenerateOrgGroups handles POST /api/org_groups/regenerate — it rebuilds the
// system-managed groups (one per Department / Sub-Department / Wydzial) from the
// operator's recipients, so campaigns can target an org unit. Idempotent.
func (as *Server) RegenerateOrgGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	uid := ctx.Get(r, "user_id").(int64)
	n, err := models.RegenerateOrgGroups(uid)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: fmt.Sprintf("Regenerated %d org-unit groups", n)}, http.StatusOK)
}
