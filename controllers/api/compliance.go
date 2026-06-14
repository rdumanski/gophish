package api

import (
	"net/http"

	ctx "github.com/rdumanski/gophish/context"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// Compliance handles GET /api/compliance/ — the NIS2 compliance report as JSON,
// for the on-screen summary. The PDF download is served by the admin server
// (session-authed) so the api_key never lands in the download URL.
func (as *Server) Compliance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	start, end, err := models.ParseReportPeriod(r.URL.Query().Get("start"), r.URL.Query().Get("end"))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	uid := ctx.Get(r, "user_id").(int64)
	report, err := models.GetComplianceReport(uid, start, end)
	if err != nil {
		log.Error(err)
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusInternalServerError)
		return
	}
	report.Operator = ctx.Get(r, "user").(models.User).Username
	JSONResponse(w, report, http.StatusOK)
}
