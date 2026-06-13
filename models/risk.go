package models

import (
	"math"
	"sort"
	"strings"

	log "github.com/rdumanski/gophish/logger"
)

// RiskScore is a per-recipient risk summary blending phishing-simulation
// behaviour with training engagement. It is computed on demand from existing
// Results and Enrollments (no stored state in this phase) and keyed on the
// Recipient master (Phase 10a). Higher Score = higher risk.
type RiskScore struct {
	RecipientID       int64  `json:"recipient_id"`
	Email             string `json:"email"`
	Name              string `json:"name"`
	Sims              int64  `json:"sims"`
	Clicked           int64  `json:"clicked"`
	Submitted         int64  `json:"submitted"`
	Reported          int64  `json:"reported"`
	TrainingAssigned  int64  `json:"training_assigned"`
	TrainingCompleted int64  `json:"training_completed"`
	Score             int    `json:"score"`
}

// computeRiskScore turns the component counts into a 0–100 score. It is a
// transparent heuristic (and a pure function, so it's unit-tested):
//   - baseline 30
//   - submitting credentials to a sim is the strongest risk signal (+40 * rate)
//   - clicking without submitting is a weaker signal (+20 * rate)
//   - reporting the sim is protective (-15 * rate)
//   - completing assigned training is protective (-25 * completion rate)
//
// The raw counts travel with the score so the UI can explain it.
func computeRiskScore(r RiskScore) int {
	score := 30.0
	if r.Sims > 0 {
		f := float64(r.Sims)
		clickedOnly := r.Clicked - r.Submitted
		if clickedOnly < 0 {
			clickedOnly = 0
		}
		score += 40 * float64(r.Submitted) / f
		score += 20 * float64(clickedOnly) / f
		score -= 15 * float64(r.Reported) / f
	}
	if r.TrainingAssigned > 0 {
		score -= 25 * float64(r.TrainingCompleted) / float64(r.TrainingAssigned)
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return int(math.Round(score))
}

// GetRiskScores returns a risk summary for every recipient owned by the
// operator, sorted highest-risk first.
func GetRiskScores(uid int64) ([]RiskScore, error) {
	recipients := []Recipient{}
	if err := db.Where("user_id=?", uid).Find(&recipients).Error; err != nil {
		log.Error(err)
		return nil, err
	}
	out := make([]RiskScore, 0, len(recipients))
	for _, rec := range recipients {
		rs := RiskScore{
			RecipientID: rec.Id,
			Email:       rec.Email,
			Name:        strings.TrimSpace(rec.FirstName + " " + rec.LastName),
		}
		db.Model(&Result{}).Where("recipient_id=?", rec.Id).Count(&rs.Sims)
		db.Model(&Result{}).Where("recipient_id=? AND status IN ?", rec.Id, []string{EventClicked, EventDataSubmit}).Count(&rs.Clicked)
		db.Model(&Result{}).Where("recipient_id=? AND status=?", rec.Id, EventDataSubmit).Count(&rs.Submitted)
		db.Model(&Result{}).Where("recipient_id=? AND reported=?", rec.Id, true).Count(&rs.Reported)
		db.Model(&Enrollment{}).Where("recipient_id=?", rec.Id).Count(&rs.TrainingAssigned)
		db.Model(&Enrollment{}).Where("recipient_id=? AND status=?", rec.Id, EnrollmentCompleted).Count(&rs.TrainingCompleted)
		rs.Score = computeRiskScore(rs)
		out = append(out, rs)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out, nil
}
