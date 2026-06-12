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

// TrainingCampaigns handles /api/training_campaigns/ (list + create).
func (as *Server) TrainingCampaigns(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	switch {
	case r.Method == "GET":
		tcs, err := models.GetTrainingCampaigns(uid)
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, tcs, http.StatusOK)
	case r.Method == "POST":
		tc := models.TrainingCampaign{}
		if err := json.NewDecoder(r.Body).Decode(&tc); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		if err := models.PostTrainingCampaign(&tc, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		// Return with freshly computed stats.
		out, err := models.GetTrainingCampaign(tc.Id, uid)
		if err != nil {
			JSONResponse(w, tc, http.StatusCreated)
			return
		}
		JSONResponse(w, out, http.StatusCreated)
	}
}

// TrainingCampaign handles /api/training_campaigns/:id (get/delete).
func (as *Server) TrainingCampaign(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	tc, err := models.GetTrainingCampaign(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Training campaign not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, tc, http.StatusOK)
	case r.Method == "DELETE":
		if err := models.DeleteTrainingCampaign(id, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting training campaign"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Training campaign deleted successfully!"}, http.StatusOK)
	}
}
