package models

import (
	"math"
	"sort"
	"strings"
	"time"
)

// EngagementScore is the positive, gamified counterpart to RiskScore: a
// per-recipient 0–100 engagement score (higher = more security-positive
// behaviour) plus earned badges. Derived on demand from the same behavioural
// data as the risk report — no stored state.
type EngagementScore struct {
	RecipientID       int64    `json:"recipient_id"`
	Email             string   `json:"email"`
	Name              string   `json:"name"`
	Sims              int64    `json:"sims"`
	Clicked           int64    `json:"clicked"`
	Submitted         int64    `json:"submitted"`
	Reported          int64    `json:"reported"`
	TrainingAssigned  int64    `json:"training_assigned"`
	TrainingCompleted int64    `json:"training_completed"`
	Score             int      `json:"score"`
	Streak            int      `json:"streak"` // consecutive most-recent sims reported
	Badges            []string `json:"badges"`
}

// DeptLeader is a department's standing on the leaderboard.
type DeptLeader struct {
	Department string  `json:"department"`
	Members    int64   `json:"members"`
	AvgScore   float64 `json:"avg_score"`
	ReportRate float64 `json:"report_rate"` // percent of members who reported >=1 sim
}

// EngagementReport bundles the individuals leaderboard and the department
// leaderboard.
type EngagementReport struct {
	Individuals []EngagementScore `json:"individuals"`
	Departments []DeptLeader      `json:"departments"`
}

// computeEngagement turns a recipient's behavioural counts into a 0–100
// engagement score plus earned badges. Pure (no DB) so it's unit-tested.
// Reporting a simulation is the hero behaviour; completing training helps;
// clicking and (worse) submitting credentials subtract. Mirrors the risk
// heuristic but framed positively.
func computeEngagement(r RiskScore) (int, []string) {
	score := 50.0
	if r.Sims > 0 {
		f := float64(r.Sims)
		clickedOnly := r.Clicked - r.Submitted
		if clickedOnly < 0 {
			clickedOnly = 0
		}
		score += 25 * float64(r.Reported) / f
		score -= 15 * float64(clickedOnly) / f
		score -= 25 * float64(r.Submitted) / f
	}
	if r.TrainingAssigned > 0 {
		score += 25 * float64(r.TrainingCompleted) / float64(r.TrainingAssigned)
	}
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	badges := []string{}
	if r.Reported >= 1 {
		badges = append(badges, "reporter")
	}
	if r.Reported >= 3 {
		badges = append(badges, "vigilant")
	}
	if r.Sims > 0 && r.Reported == r.Sims {
		badges = append(badges, "perfect_reporter")
	}
	if r.Sims > 0 && r.Clicked == 0 {
		badges = append(badges, "sharp_eye")
	}
	if r.TrainingAssigned > 0 && r.TrainingCompleted == r.TrainingAssigned {
		badges = append(badges, "scholar")
	}
	return int(math.Round(score)), badges
}

// reportStreak counts the recipient's leading run of most-recent simulations
// that were reported. Only delivered sims (send_date <= now) count, so a
// future-scheduled result can't reset the streak.
func reportStreak(recipientID int64) int {
	results := []Result{}
	db.Model(&Result{}).
		Where("recipient_id = ? AND send_date <= ?", recipientID, time.Now().UTC()).
		Order("send_date desc").Find(&results)
	streak := 0
	for _, r := range results {
		if !r.Reported {
			break
		}
		streak++
	}
	return streak
}

// GetRecipientEngagement returns one recipient's engagement score, badges, and
// report streak — for the recipient-facing learner portal.
func GetRecipientEngagement(recipientID int64) (EngagementScore, error) {
	rec, err := GetRecipientByID(recipientID)
	if err != nil {
		return EngagementScore{}, err
	}
	rs := recipientRiskScore(rec)
	sc, badges := computeEngagement(rs)
	return EngagementScore{
		RecipientID: rs.RecipientID, Email: rs.Email, Name: rs.Name,
		Sims: rs.Sims, Clicked: rs.Clicked, Submitted: rs.Submitted, Reported: rs.Reported,
		TrainingAssigned: rs.TrainingAssigned, TrainingCompleted: rs.TrainingCompleted,
		Score: sc, Streak: reportStreak(recipientID), Badges: badges,
	}, nil
}

// GetEngagementReport builds the individuals leaderboard (engagement score
// desc) and the department leaderboard (avg engagement desc), reusing
// GetRiskScores for the per-recipient behavioural data and the canonical
// Recipient placement for the department rollup.
func GetEngagementReport(uid int64) (EngagementReport, error) {
	rep := EngagementReport{Individuals: []EngagementScore{}, Departments: []DeptLeader{}}
	scores, err := GetRiskScores(uid)
	if err != nil {
		return rep, err
	}
	byEmail := make(map[string]EngagementScore, len(scores))
	for _, rs := range scores {
		sc, badges := computeEngagement(rs)
		es := EngagementScore{
			RecipientID: rs.RecipientID, Email: rs.Email, Name: rs.Name,
			Sims: rs.Sims, Clicked: rs.Clicked, Submitted: rs.Submitted, Reported: rs.Reported,
			TrainingAssigned: rs.TrainingAssigned, TrainingCompleted: rs.TrainingCompleted,
			Score: sc, Streak: reportStreak(rs.RecipientID), Badges: badges,
		}
		rep.Individuals = append(rep.Individuals, es)
		byEmail[normEmail(rs.Email)] = es
	}
	sort.SliceStable(rep.Individuals, func(i, j int) bool {
		if rep.Individuals[i].Score != rep.Individuals[j].Score {
			return rep.Individuals[i].Score > rep.Individuals[j].Score
		}
		return rep.Individuals[i].Name < rep.Individuals[j].Name
	})

	// Department leaderboard from canonical Recipient placement.
	recipients := []Recipient{}
	if err := db.Where("user_id = ?", uid).Find(&recipients).Error; err != nil {
		return rep, err
	}
	type deptAgg struct {
		members, reporters int64
		sum                int64
	}
	aggs := map[string]*deptAgg{}
	for _, rc := range recipients {
		dept := strings.TrimSpace(rc.Department)
		if dept == "" {
			continue
		}
		es, ok := byEmail[normEmail(rc.Email)]
		if !ok {
			continue
		}
		a := aggs[dept]
		if a == nil {
			a = &deptAgg{}
			aggs[dept] = a
		}
		a.members++
		a.sum += int64(es.Score)
		if es.Reported > 0 {
			a.reporters++
		}
	}
	for dept, a := range aggs {
		avg := 0.0
		if a.members > 0 {
			avg = math.Round(float64(a.sum)/float64(a.members)*10) / 10
		}
		rep.Departments = append(rep.Departments, DeptLeader{
			Department: dept, Members: a.members, AvgScore: avg, ReportRate: pct(a.reporters, a.members),
		})
	}
	sort.SliceStable(rep.Departments, func(i, j int) bool {
		if rep.Departments[i].AvgScore != rep.Departments[j].AvgScore {
			return rep.Departments[i].AvgScore > rep.Departments[j].AvgScore
		}
		return rep.Departments[i].Department < rep.Departments[j].Department
	})
	return rep, nil
}
