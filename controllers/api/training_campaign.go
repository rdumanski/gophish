package api

import (
	"encoding/json"
	"fmt"
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

// sendInvitationsRequest is the POST body for the /send sub-resource.
type sendInvitationsRequest struct {
	SMTP struct {
		Name string `json:"name"`
	} `json:"smtp"`
	URL string `json:"url"`
}

// SendTrainingInvitations emails every enrollee in a campaign their learner
// portal link via the chosen SMTP profile.
func (as *Server) SendTrainingInvitations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	uid := ctx.Get(r, "user_id").(int64)
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	req := sendInvitationsRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
		return
	}
	sent, err := models.SendTrainingInvitations(uid, id, req.SMTP.Name, req.URL)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		return
	}
	JSONResponse(w, models.Response{Success: true, Message: fmt.Sprintf("Sent %d invitation(s)", sent)}, http.StatusOK)
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
