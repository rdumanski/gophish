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

// TrainingModules handles the /api/training_modules/ endpoint (list + create).
func (as *Server) TrainingModules(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		ms, err := models.GetTrainingModules(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ms, http.StatusOK)
	case r.Method == "POST":
		m := models.TrainingModule{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		_, err := models.GetTrainingModuleByName(m.Name, ctx.Get(r, "user_id").(int64))
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			JSONResponse(w, models.Response{Success: false, Message: "Training module name already in use"}, http.StatusConflict)
			return
		}
		m.ModifiedDate = time.Now().UTC()
		m.UserID = ctx.Get(r, "user_id").(int64)
		if err := models.PostTrainingModule(&m); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, m, http.StatusCreated)
	}
}

// TrainingModule handles the /api/training_modules/:id endpoint (get/update/delete).
func (as *Server) TrainingModule(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	m, err := models.GetTrainingModule(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Training module not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, m, http.StatusOK)
	case r.Method == "DELETE":
		if err := models.DeleteTrainingModule(id, ctx.Get(r, "user_id").(int64)); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting training module"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Training module deleted successfully!"}, http.StatusOK)
	case r.Method == "PUT":
		m = models.TrainingModule{}
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			log.Error(err)
		}
		if m.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "Error: /:id and module id mismatch"}, http.StatusBadRequest)
			return
		}
		m.ModifiedDate = time.Now().UTC()
		m.UserID = ctx.Get(r, "user_id").(int64)
		if err := models.PutTrainingModule(&m); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, m, http.StatusOK)
	}
}
