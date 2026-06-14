package api

import (
	"net/http"

	ctx "github.com/rdumanski/gophish/context"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// Engagement handles GET /api/engagement/ — the gamified engagement report
// (individuals leaderboard + department leaderboard).
func (as *Server) Engagement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	report, err := models.GetEngagementReport(ctx.Get(r, "user_id").(int64))
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	JSONResponse(w, report, http.StatusOK)
}
