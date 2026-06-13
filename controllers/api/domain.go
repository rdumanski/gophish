package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	ctx "github.com/rdumanski/gophish/context"
	"github.com/rdumanski/gophish/domain"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
	"gorm.io/gorm"
)

// Domains handles /api/domains/ (list + create).
func (as *Server) Domains(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	switch {
	case r.Method == "GET":
		ds, err := models.GetDomains(uid)
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ds, http.StatusOK)
	case r.Method == "POST":
		d := models.Domain{}
		if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		_, err := models.GetDomainByName(d.Name, uid)
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			JSONResponse(w, models.Response{Success: false, Message: "Domain already registered"}, http.StatusConflict)
			return
		}
		if err := models.PostDomain(&d, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		d.Records = d.SuggestedRecords()
		JSONResponse(w, d, http.StatusCreated)
	}
}

// Domain handles /api/domains/:id (get/update/delete).
func (as *Server) Domain(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	d, err := models.GetDomain(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Domain not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, d, http.StatusOK)
	case r.Method == "DELETE":
		if err := models.DeleteDomain(id, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting domain"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Domain deleted successfully!"}, http.StatusOK)
	case r.Method == "PUT":
		body := models.Domain{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Error(err)
		}
		if body.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "Error: /:id and domain id mismatch"}, http.StatusBadRequest)
			return
		}
		if err := models.PutDomain(&body, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		body.Records = body.SuggestedRecords()
		JSONResponse(w, body, http.StatusOK)
	}
}

// CheckDomain runs the live health check for a domain and returns the result.
func (as *Server) CheckDomain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	uid := ctx.Get(r, "user_id").(int64)
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	d, err := models.GetDomain(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Domain not found"}, http.StatusNotFound)
		return
	}
	domain.Check(&d)
	JSONResponse(w, d, http.StatusOK)
}
