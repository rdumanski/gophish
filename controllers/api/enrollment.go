package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	ctx "github.com/rdumanski/gophish/context"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// createEnrollmentRequest is the POST body for /api/enrollments. The operator
// enrolls a person (by email; the recipient is upserted) into a module. Phase
// 11 will add group-level assignment.
type createEnrollmentRequest struct {
	RecipientEmail string `json:"recipient_email"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Position       string `json:"position"`
	ModuleID       int64  `json:"module_id"`
}

// enrollmentResponse augments the stored Enrollment with the relative learner
// portal path. The absolute URL (scheme+host) is composed by the caller /
// Phase 11 delivery, the same way campaign URLs are operator-supplied.
type enrollmentResponse struct {
	models.Enrollment
	PortalPath string `json:"portal_path"`
}

func toEnrollmentResponse(e models.Enrollment) enrollmentResponse {
	return enrollmentResponse{Enrollment: e, PortalPath: "/learn/" + e.Token}
}

// Enrollments handles the /api/enrollments/ endpoint (list + create).
func (as *Server) Enrollments(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	switch {
	case r.Method == "GET":
		es, err := models.GetEnrollments(uid)
		if err != nil {
			log.Error(err)
		}
		out := make([]enrollmentResponse, 0, len(es))
		for _, e := range es {
			out = append(out, toEnrollmentResponse(e))
		}
		JSONResponse(w, out, http.StatusOK)
	case r.Method == "POST":
		req := createEnrollmentRequest{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		if req.RecipientEmail == "" {
			JSONResponse(w, models.Response{Success: false, Message: "recipient_email is required"}, http.StatusBadRequest)
			return
		}
		br := models.BaseRecipient{
			Email:     req.RecipientEmail,
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Position:  req.Position,
		}
		e, err := models.CreateEnrollmentByEmail(uid, br, req.ModuleID)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, toEnrollmentResponse(e), http.StatusCreated)
	}
}

// Enrollment handles the /api/enrollments/:id endpoint (get/delete).
func (as *Server) Enrollment(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	e, err := models.GetEnrollment(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Enrollment not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, toEnrollmentResponse(e), http.StatusOK)
	case r.Method == "DELETE":
		if err := models.DeleteEnrollment(id, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting enrollment"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Enrollment deleted successfully!"}, http.StatusOK)
	}
}
