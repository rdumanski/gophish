package models

import (
	"errors"
	"math"
	"time"

	log "github.com/rdumanski/gophish/logger"
)

// Quiz is an assessment attached to a TrainingModule. It has ordered
// single-correct multiple-choice questions; pass_threshold is the percentage
// of correct answers required to pass (enforced by the learner portal in 12b).
type Quiz struct {
	Id            int64          `json:"id" gorm:"primaryKey;column:id"`
	UserID        int64          `json:"-" gorm:"column:user_id"`
	ModuleID      int64          `json:"module_id" gorm:"column:module_id"`
	Title         string         `json:"title"`
	PassThreshold int            `json:"pass_threshold" gorm:"column:pass_threshold"`
	ModifiedDate  time.Time      `json:"modified_date"`
	Questions     []QuizQuestion `json:"questions" gorm:"-"`
	// QuestionCount is populated for list views (GetQuizzes) so the UI can
	// show a count without loading every nested question.
	QuestionCount int64 `json:"question_count" gorm:"-"`
}

// QuizQuestion is one prompt with a set of options.
type QuizQuestion struct {
	Id         int64        `json:"id" gorm:"primaryKey;column:id"`
	QuizID     int64        `json:"-" gorm:"column:quiz_id"`
	Prompt     string       `json:"prompt"`
	OrderIndex int          `json:"order_index" gorm:"column:order_index"`
	Options    []QuizOption `json:"options" gorm:"-"`
}

// TableName pins the questions table name (avoids GORM pluralization surprises).
func (QuizQuestion) TableName() string { return "quiz_questions" }

// QuizOption is one selectable answer; IsCorrect marks the right answer(s).
type QuizOption struct {
	Id         int64  `json:"id" gorm:"primaryKey;column:id"`
	QuestionID int64  `json:"-" gorm:"column:question_id"`
	Text       string `json:"text"`
	IsCorrect  bool   `json:"is_correct" gorm:"column:is_correct"`
	OrderIndex int    `json:"order_index" gorm:"column:order_index"`
}

// TableName pins the options table name.
func (QuizOption) TableName() string { return "quiz_options" }

// Quiz validation errors.
var (
	ErrQuizTitleNotSpecified = errors.New("quiz title not specified")
	ErrQuizNoQuestions       = errors.New("quiz must have at least one question")
	ErrQuizQuestionNoPrompt  = errors.New("every question needs a prompt")
	ErrQuizQuestionOptions   = errors.New("every question needs at least two options")
	ErrQuizQuestionNoCorrect = errors.New("every question needs at least one correct option")
	ErrQuizBadThreshold      = errors.New("pass_threshold must be between 0 and 100")
)

// Validate checks the quiz and its nested questions/options.
func (q *Quiz) Validate() error {
	if q.Title == "" {
		return ErrQuizTitleNotSpecified
	}
	if q.PassThreshold < 0 || q.PassThreshold > 100 {
		return ErrQuizBadThreshold
	}
	if len(q.Questions) == 0 {
		return ErrQuizNoQuestions
	}
	for _, question := range q.Questions {
		if question.Prompt == "" {
			return ErrQuizQuestionNoPrompt
		}
		if len(question.Options) < 2 {
			return ErrQuizQuestionOptions
		}
		hasCorrect := false
		for _, o := range question.Options {
			if o.IsCorrect {
				hasCorrect = true
				break
			}
		}
		if !hasCorrect {
			return ErrQuizQuestionNoCorrect
		}
	}
	return nil
}

// ScoreQuiz grades a set of answers (question id -> selected option id) against
// a quiz and returns the percent score and whether it meets the pass threshold.
// Pure function (no DB/network) so it can be unit-tested. Unanswered or
// wrongly-answered questions simply don't count toward the correct tally.
func ScoreQuiz(q Quiz, answers map[int64]int64) (scorePercent int, passed bool) {
	if len(q.Questions) == 0 {
		return 0, false
	}
	correct := 0
	for _, question := range q.Questions {
		selected, ok := answers[question.Id]
		if !ok {
			continue
		}
		for _, opt := range question.Options {
			if opt.Id == selected && opt.IsCorrect {
				correct++
				break
			}
		}
	}
	scorePercent = int(math.Round(100 * float64(correct) / float64(len(q.Questions))))
	passed = scorePercent >= q.PassThreshold
	return scorePercent, passed
}

// loadQuizChildren populates a quiz's questions (ordered) and their options.
func loadQuizChildren(q *Quiz) error {
	if err := db.Where("quiz_id=?", q.Id).Order("order_index asc").Find(&q.Questions).Error; err != nil {
		return err
	}
	for i := range q.Questions {
		if err := db.Where("question_id=?", q.Questions[i].Id).Order("order_index asc").Find(&q.Questions[i].Options).Error; err != nil {
			return err
		}
	}
	return nil
}

// GetQuizzes returns the operator's quizzes (without nested questions).
func GetQuizzes(uid int64) ([]Quiz, error) {
	qs := []Quiz{}
	err := db.Where("user_id=?", uid).Find(&qs).Error
	if err != nil {
		log.Error(err)
		return qs, err
	}
	for i := range qs {
		db.Model(&QuizQuestion{}).Where("quiz_id=?", qs[i].Id).Count(&qs[i].QuestionCount)
	}
	return qs, nil
}

// GetQuiz returns a quiz with its questions and options.
func GetQuiz(id int64, uid int64) (Quiz, error) {
	q := Quiz{}
	if err := db.Where("user_id=? and id=?", uid, id).First(&q).Error; err != nil {
		log.Error(err)
		return q, err
	}
	if err := loadQuizChildren(&q); err != nil {
		log.Error(err)
		return q, err
	}
	return q, nil
}

// GetQuizzesByModule returns quizzes (with questions) attached to a module.
// Used by the learner portal (Phase 12b).
func GetQuizzesByModule(moduleID int64, uid int64) ([]Quiz, error) {
	qs := []Quiz{}
	if err := db.Where("user_id=? and module_id=?", uid, moduleID).Find(&qs).Error; err != nil {
		return qs, err
	}
	for i := range qs {
		if err := loadQuizChildren(&qs[i]); err != nil {
			return qs, err
		}
	}
	return qs, nil
}

// saveQuizChildren writes a quiz's questions and options on the given tx,
// stamping the foreign keys and order indices.
func saveQuizChildren(q *Quiz) error {
	tx := db.Begin()
	for qi := range q.Questions {
		question := &q.Questions[qi]
		question.Id = 0
		question.QuizID = q.Id
		question.OrderIndex = qi
		if err := tx.Save(question).Error; err != nil {
			tx.Rollback()
			return err
		}
		for oi := range question.Options {
			opt := &question.Options[oi]
			opt.Id = 0
			opt.QuestionID = question.Id
			opt.OrderIndex = oi
			if err := tx.Save(opt).Error; err != nil {
				tx.Rollback()
				return err
			}
		}
	}
	return tx.Commit().Error
}

// PostQuiz creates a quiz and its questions/options after validation.
func PostQuiz(q *Quiz, uid int64) error {
	if err := q.Validate(); err != nil {
		return err
	}
	if _, err := GetTrainingModule(q.ModuleID, uid); err != nil {
		return ErrEnrollmentModuleNotFound
	}
	q.UserID = uid
	q.ModifiedDate = time.Now().UTC()
	if err := db.Save(q).Error; err != nil {
		log.Error(err)
		return err
	}
	return saveQuizChildren(q)
}

// PutQuiz replaces a quiz's questions/options (PUT semantics) after validation.
func PutQuiz(q *Quiz, uid int64) error {
	if err := q.Validate(); err != nil {
		return err
	}
	existing, err := GetQuiz(q.Id, uid)
	if err != nil {
		return err
	}
	if err := deleteQuizChildren(existing.Id); err != nil {
		return err
	}
	q.UserID = uid
	q.ModifiedDate = time.Now().UTC()
	if err := db.Where("id=?", q.Id).Save(q).Error; err != nil {
		log.Error(err)
		return err
	}
	return saveQuizChildren(q)
}

// deleteQuizChildren removes a quiz's questions and their options.
func deleteQuizChildren(quizID int64) error {
	questions := []QuizQuestion{}
	if err := db.Where("quiz_id=?", quizID).Find(&questions).Error; err != nil {
		return err
	}
	for _, question := range questions {
		if err := db.Where("question_id=?", question.Id).Delete(&QuizOption{}).Error; err != nil {
			return err
		}
	}
	return db.Where("quiz_id=?", quizID).Delete(&QuizQuestion{}).Error
}

// DeleteQuiz deletes a quiz and all its questions/options.
func DeleteQuiz(id int64, uid int64) error {
	if _, err := GetQuiz(id, uid); err != nil {
		return err
	}
	if err := deleteQuizChildren(id); err != nil {
		log.Error(err)
		return err
	}
	if err := db.Where("user_id=?", uid).Delete(&Quiz{Id: id}).Error; err != nil {
		log.Error(err)
		return err
	}
	return nil
}
