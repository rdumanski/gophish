package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	ctx "github.com/rdumanski/gophish/context"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
	"github.com/rdumanski/gophish/roster"
)

// RosterSources handles /api/roster_sources/ (list + create).
func (as *Server) RosterSources(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	switch {
	case r.Method == "GET":
		rss, err := models.GetRosterSources(uid)
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, rss, http.StatusOK)
	case r.Method == "POST":
		rs := models.RosterSource{}
		if err := json.NewDecoder(r.Body).Decode(&rs); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		if err := models.PostRosterSource(&rs, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, rs, http.StatusCreated)
	}
}

// RosterSource handles /api/roster_sources/:id (get/update/delete).
func (as *Server) RosterSource(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	rs, err := models.GetRosterSource(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Roster source not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, rs, http.StatusOK)
	case r.Method == "DELETE":
		if err := models.DeleteRosterSource(id, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting roster source"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Roster source deleted successfully!"}, http.StatusOK)
	case r.Method == "PUT":
		body := models.RosterSource{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			log.Error(err)
		}
		if body.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "Error: /:id and roster source id mismatch"}, http.StatusBadRequest)
			return
		}
		if err := models.PutRosterSource(&body, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, body, http.StatusOK)
	}
}

// SyncRosterSource runs a roster sync on demand ("Sync now"). It connects to
// the mailbox synchronously and returns the outcome.
func (as *Server) SyncRosterSource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	uid := ctx.Get(r, "user_id").(int64)
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	rs, err := models.GetRosterSource(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Roster source not found"}, http.StatusNotFound)
		return
	}
	result := roster.Sync(&rs)
	JSONResponse(w, models.Response{Success: result.Applied, Message: result.Message, Data: result}, http.StatusOK)
}
