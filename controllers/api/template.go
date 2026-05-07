package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rdumanski/gophish/ai"
	ctx "github.com/rdumanski/gophish/context"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
	"gorm.io/gorm"
)

// Templates handles the functionality for the /api/templates endpoint
func (as *Server) Templates(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET":
		ts, err := models.GetTemplates(ctx.Get(r, "user_id").(int64))
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, ts, http.StatusOK)
	//POST: Create a new template and return it as JSON
	case r.Method == "POST":
		t := models.Template{}
		// Put the request into a template
		err := json.NewDecoder(r.Body).Decode(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		_, err = models.GetTemplateByName(t.Name, ctx.Get(r, "user_id").(int64))
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			JSONResponse(w, models.Response{Success: false, Message: "Template name already in use"}, http.StatusConflict)
			return
		}
		t.ModifiedDate = time.Now().UTC()
		t.UserID = ctx.Get(r, "user_id").(int64)
		err = models.PostTemplate(&t)
		if err == models.ErrTemplateNameNotSpecified {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		if err == models.ErrTemplateMissingParameter {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error inserting template into database"}, http.StatusInternalServerError)
			log.Error(err)
			return
		}
		JSONResponse(w, t, http.StatusCreated)
	}
}

// Template handles the functions for the /api/templates/:id endpoint
func (as *Server) Template(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	t, err := models.GetTemplate(id, ctx.Get(r, "user_id").(int64))
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Template not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, t, http.StatusOK)
	case r.Method == "DELETE":
		err = models.DeleteTemplate(id, ctx.Get(r, "user_id").(int64))
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting template"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Template deleted successfully!"}, http.StatusOK)
	case r.Method == "PUT":
		t = models.Template{}
		err = json.NewDecoder(r.Body).Decode(&t)
		if err != nil {
			log.Error(err)
		}
		if t.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "Error: /:id and template_id mismatch"}, http.StatusBadRequest)
			return
		}
		t.ModifiedDate = time.Now().UTC()
		t.UserID = ctx.Get(r, "user_id").(int64)
		err = models.PutTemplate(&t)
		if err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, t, http.StatusOK)
	}
}

// scoreTemplateRequest is the JSON body shape accepted by
// POST /api/templates/score.
type scoreTemplateRequest struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
	From    string `json:"from"`
	Hint    string `json:"hint"`
}

// scoreTemplateResponse is the JSON body shape returned by
// POST /api/templates/score.
type scoreTemplateResponse struct {
	Score           int      `json:"score"`
	Rationale       string   `json:"rationale"`
	Strengths       []string `json:"strengths"`
	Weaknesses      []string `json:"weaknesses"`
	WouldMakeHarder []string `json:"would_make_harder"`
	Model           string   `json:"model"`
}

// ScoreTemplate evaluates the difficulty of a candidate phishing-
// simulation template via the configured AI provider. Purely
// informational — the result is NOT persisted.
//
// Status codes:
//
//	200 — JSON Score
//	400 — invalid subject (missing subject + body)
//	422 — model declined the request
//	502 — upstream provider error or auth/config bug
//	503 — AI is disabled, OR the configured provider doesn't implement scoring
func (as *Server) ScoreTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	if as.aiGenerator == nil {
		JSONResponse(w, models.Response{Success: false, Message: "AI features are disabled — see docs/dev for the ai config block"}, http.StatusServiceUnavailable)
		return
	}
	scorer, ok := as.aiGenerator.(ai.Scorer)
	if !ok {
		JSONResponse(w, models.Response{Success: false, Message: "the configured AI provider does not implement template scoring"}, http.StatusServiceUnavailable)
		return
	}

	var req scoreTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "invalid request body: " + err.Error()}, http.StatusBadRequest)
		return
	}
	sub := ai.Subject{
		Subject: req.Subject,
		Text:    req.Text,
		HTML:    req.HTML,
		From:    req.From,
		Hint:    req.Hint,
	}

	score, err := scorer.ScoreTemplate(r.Context(), sub)
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrInvalidSubject):
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		case errors.Is(err, ai.ErrRefused):
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusUnprocessableEntity)
		default:
			log.Errorf("ai: template scoring failed: %s", err)
			JSONResponse(w, models.Response{Success: false, Message: "AI provider error — see server logs"}, http.StatusBadGateway)
		}
		return
	}

	JSONResponse(w, scoreTemplateResponse{
		Score:           score.Score,
		Rationale:       score.Rationale,
		Strengths:       score.Strengths,
		Weaknesses:      score.Weaknesses,
		WouldMakeHarder: score.WouldMakeHarder,
		Model:           score.Model,
	}, http.StatusOK)
}

// generateTemplateRequest is the JSON body shape accepted by
// POST /api/templates/generate.
type generateTemplateRequest struct {
	Audience string `json:"audience"`
	Theme    string `json:"theme"`
	Urgency  string `json:"urgency"`
	Length   string `json:"length"`
	Language string `json:"language"`
	Brand    string `json:"brand"`
}

// generateTemplateResponse is the JSON body shape returned by
// POST /api/templates/generate.
type generateTemplateResponse struct {
	Subject string `json:"subject"`
	Text    string `json:"text"`
	HTML    string `json:"html"`
	Notes   string `json:"notes"`
	Model   string `json:"model"`
}

// GenerateTemplate drafts a phishing-simulation email template via the
// configured AI provider and returns it as JSON. It is purely
// generative — the result is NOT persisted; the admin reviews + saves
// it via the existing POST /api/templates/ flow.
//
// Status codes:
//
//	200 — JSON Draft
//	400 — invalid brief (missing audience/theme)
//	422 — model declined the request
//	502 — upstream provider error or auth/config bug
//	503 — AI is disabled in config (no ai block, or ai.enabled=false)
func (as *Server) GenerateTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		JSONResponse(w, models.Response{Success: false, Message: "method not allowed"}, http.StatusMethodNotAllowed)
		return
	}
	if as.aiGenerator == nil {
		JSONResponse(w, models.Response{Success: false, Message: "AI features are disabled — see docs/dev for the ai config block"}, http.StatusServiceUnavailable)
		return
	}

	var req generateTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "invalid request body: " + err.Error()}, http.StatusBadRequest)
		return
	}
	brief := ai.Brief{
		Audience: req.Audience,
		Theme:    req.Theme,
		Urgency:  ai.Urgency(req.Urgency),
		Length:   ai.Length(req.Length),
		Language: req.Language,
		Brand:    req.Brand,
	}

	draft, err := as.aiGenerator.Generate(r.Context(), brief)
	if err != nil {
		switch {
		case errors.Is(err, ai.ErrInvalidBrief):
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
		case errors.Is(err, ai.ErrRefused):
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusUnprocessableEntity)
		default:
			log.Errorf("ai: template generation failed: %s", err)
			JSONResponse(w, models.Response{Success: false, Message: "AI provider error — see server logs"}, http.StatusBadGateway)
		}
		return
	}

	JSONResponse(w, generateTemplateResponse{
		Subject: draft.Subject,
		Text:    draft.Text,
		HTML:    draft.HTML,
		Notes:   draft.Notes,
		Model:   draft.Model,
	}, http.StatusOK)
}
