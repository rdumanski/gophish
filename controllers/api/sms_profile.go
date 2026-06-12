package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	ctx "github.com/rdumanski/gophish/context"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
	"gorm.io/gorm"
)

// SMSProfiles handles GET (list) and POST (create) on /api/sms_profiles/.
// Mirrors SendingProfiles for the email side; the only conceptual
// difference is the resource being managed.
func (as *Server) SMSProfiles(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ps, err := models.GetSMSProfiles(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ps, http.StatusOK)
	case http.MethodPost:
		p := models.SMSProfile{}
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		// Names must be unique per user — same convention SMTP uses.
		_, err := models.GetSMSProfileByName(p.Name, ctx.Get(r, "user_id").(int64))
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			JSONResponse(w, models.Response{Success: false, Message: "SMS profile name already in use"}, http.StatusConflict)
			return
		}
		p.UserID = ctx.Get(r, "user_id").(int64)
		p.ModifiedDate = time.Now().UTC()
		if err := models.PostSMSProfile(&p); err != nil {
			// Validate errors (missing name / unsupported provider /
			// missing creds / bad from_number) get a 400 so the
			// admin sees the rule they violated.
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, p, http.StatusCreated)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
	}
}

// SMSProfile handles GET / PUT / DELETE on /api/sms_profiles/{id}.
func (as *Server) SMSProfile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	uid := ctx.Get(r, "user_id").(int64)
	p, err := models.GetSMSProfile(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "SMS profile not found"}, http.StatusNotFound)
		return
	}
	switch r.Method {
	case http.MethodGet:
		JSONResponse(w, p, http.StatusOK)
	case http.MethodDelete:
		if err := models.DeleteSMSProfile(id, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting SMS profile"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "SMS profile deleted successfully"}, http.StatusOK)
	case http.MethodPut:
		updated := models.SMSProfile{}
		if err := json.NewDecoder(r.Body).Decode(&updated); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid request"}, http.StatusBadRequest)
			return
		}
		if updated.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "/:id and body id mismatch"}, http.StatusBadRequest)
			return
		}
		updated.UserID = uid
		updated.ModifiedDate = time.Now().UTC()
		if err := models.PutSMSProfile(&updated); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, updated, http.StatusOK)
	default:
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
	}
}
