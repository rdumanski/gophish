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

// Quizzes handles /api/quizzes/ (list + create).
func (as *Server) Quizzes(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	switch {
	case r.Method == "GET":
		qs, err := models.GetQuizzes(uid)
		if err != nil {
			log.Error(err)
		}
		JSONResponse(w, qs, http.StatusOK)
	case r.Method == "POST":
		q := models.Quiz{}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Invalid JSON structure"}, http.StatusBadRequest)
			return
		}
		if err := models.PostQuiz(&q, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, q, http.StatusCreated)
	}
}

// Quiz handles /api/quizzes/:id (get/update/delete).
func (as *Server) Quiz(w http.ResponseWriter, r *http.Request) {
	uid := ctx.Get(r, "user_id").(int64)
	vars := mux.Vars(r)
	id, _ := strconv.ParseInt(vars["id"], 0, 64)
	q, err := models.GetQuiz(id, uid)
	if err != nil {
		JSONResponse(w, models.Response{Success: false, Message: "Quiz not found"}, http.StatusNotFound)
		return
	}
	switch {
	case r.Method == "GET":
		JSONResponse(w, q, http.StatusOK)
	case r.Method == "DELETE":
		if err := models.DeleteQuiz(id, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: "Error deleting quiz"}, http.StatusInternalServerError)
			return
		}
		JSONResponse(w, models.Response{Success: true, Message: "Quiz deleted successfully!"}, http.StatusOK)
	case r.Method == "PUT":
		q = models.Quiz{}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			log.Error(err)
		}
		if q.Id != id {
			JSONResponse(w, models.Response{Success: false, Message: "Error: /:id and quiz id mismatch"}, http.StatusBadRequest)
			return
		}
		if err := models.PutQuiz(&q, uid); err != nil {
			JSONResponse(w, models.Response{Success: false, Message: err.Error()}, http.StatusBadRequest)
			return
		}
		JSONResponse(w, q, http.StatusOK)
	}
}
