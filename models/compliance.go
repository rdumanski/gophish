package models

import (
	"errors"
	"math"
	"strings"
	"time"

	log "github.com/rdumanski/gophish/logger"
)

// reportPeriodLayout is the wire format for compliance-report period params.
const reportPeriodLayout = "2006-01-02"

// ErrInvalidReportPeriod is returned when the requested period is not start<end.
var ErrInvalidReportPeriod = errors.New("start must be before end")

// ParseReportPeriod resolves a [start, end] reporting window from raw query
// params (YYYY-MM-DD). Both are optional: end defaults to now (UTC), start to 12
// months earlier; start="all" means the full history (zero time). end is
// normalized to the end of its day so BETWEEN is inclusive. Shared by the JSON
// API (api package) and the session-authed PDF download (controllers package).
func ParseReportPeriod(startParam, endParam string) (start, end time.Time, err error) {
	now := time.Now().UTC()
	end = now
	start = now.AddDate(-1, 0, 0)

	if endParam != "" {
		end, err = time.Parse(reportPeriodLayout, endParam)
		if err != nil {
			return
		}
		end = end.Add(24*time.Hour - time.Nanosecond)
	}
	if startParam != "" {
		if startParam == "all" {
			start = time.Time{}
		} else {
			start, err = time.Parse(reportPeriodLayout, startParam)
			if err != nil {
				return
			}
		}
	}
	if !start.Before(end) {
		err = ErrInvalidReportPeriod
	}
	return
}

// ComplianceReport is a period-scoped, board-ready summary of the awareness
// program, built to evidence NIS2 obligations (cyber-hygiene/training under
// Art. 21(2)(g) and management-body oversight under Art. 20). It is computed on
// demand from existing Results, Enrollments, Recipients, Groups and risk
// scores — no stored state.
//
// Period semantics: phishing campaigns and the training cohort are filtered to
// [PeriodStart, PeriodEnd]; per-recipient risk is a CURRENT snapshot (reuses
// GetRiskScores), so the risk posture reflects standing as of GeneratedAt.
type ComplianceReport struct {
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	GeneratedAt time.Time `json:"generated_at"`
	Operator    string    `json:"operator"`

	Population int64 `json:"population"` // distinct recipients (current)

	Training TrainingCoverage     `json:"training"`
	Phishing PhishingProgram      `json:"phishing"`
	Risk     RiskPosture          `json:"risk"`
	Groups   []GroupComplianceRow `json:"groups"`
}

// TrainingCoverage rolls up the training enrollments ASSIGNED within the period
// (the cohort), so the completion rate reflects that same cohort.
type TrainingCoverage struct {
	Assigned       int64   `json:"assigned"`
	Started        int64   `json:"started"`
	Completed      int64   `json:"completed"`
	Outstanding    int64   `json:"outstanding"`
	CompletionRate float64 `json:"completion_rate"` // percent, 0–100
}

// PhishingProgram rolls up simulations launched within the period. Counts reuse
// getCampaignStats so the phish_filter sandbox exclusion stays consistent with
// the rest of the app.
type PhishingProgram struct {
	Campaigns int64 `json:"campaigns"`
	Targeted  int64 `json:"targeted"` // recipients in scope (sum of campaign totals)
	Sent      int64 `json:"sent"`
	Opened    int64 `json:"opened"`
	Clicked   int64 `json:"clicked"`
	Submitted int64 `json:"submitted"`
	Reported  int64 `json:"reported"`
	// Rates are a percentage of recipients TARGETED, not of emails sent: a
	// per-recipient count can't exceed the targeted population, so the rate
	// stays in 0–100 even when reports are reconciled onto unsent results.
	ClickRate  float64 `json:"click_rate"`
	SubmitRate float64 `json:"submit_rate"`
	ReportRate float64 `json:"report_rate"`
}

// RiskPosture is the current distribution of per-recipient risk scores, using
// the same thresholds as the Risk report UI (<40 low, <70 medium, else high).
type RiskPosture struct {
	Low     int64       `json:"low"`
	Medium  int64       `json:"medium"`
	High    int64       `json:"high"`
	Average float64     `json:"average"`
	Top     []RiskScore `json:"top"`
}

// GroupComplianceRow is a per-group/department breakdown. Membership is by
// Target email (current, not period-bound); trained% is current coverage (any
// completed enrollment ever); click%/report% are over the period's campaigns;
// avg risk is the current snapshot averaged over members that are recipients.
type GroupComplianceRow struct {
	GroupID    int64   `json:"group_id"`
	Name       string  `json:"name"`
	Members    int64   `json:"members"`
	TrainedPct float64 `json:"trained_pct"`
	ClickPct   float64 `json:"click_pct"`
	ReportPct  float64 `json:"report_pct"`
	AvgRisk    float64 `json:"avg_risk"`
}

// pct returns part/whole as a percent (0–100) rounded to one decimal, 0 when
// whole is 0.
func pct(part, whole int64) float64 {
	if whole <= 0 {
		return 0
	}
	return math.Round(float64(part)/float64(whole)*1000) / 10
}

func normEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

// bucketRisk turns per-recipient scores into a distribution + average + top-N.
// Pure (no DB), so it's unit-tested directly. Expects scores sorted high-first.
func bucketRisk(scores []RiskScore) RiskPosture {
	rp := RiskPosture{Top: []RiskScore{}}
	var sum int
	for _, s := range scores {
		switch {
		case s.Score < 40:
			rp.Low++
		case s.Score < 70:
			rp.Medium++
		default:
			rp.High++
		}
		sum += s.Score
	}
	if len(scores) > 0 {
		rp.Average = math.Round(float64(sum)/float64(len(scores))*10) / 10
	}
	const topN = 10
	for i := 0; i < len(scores) && i < topN; i++ {
		rp.Top = append(rp.Top, scores[i])
	}
	return rp
}

// inPeriodCampaignIDs returns the operator's campaign ids launched within the
// period.
func inPeriodCampaignIDs(uid int64, start, end time.Time) ([]int64, error) {
	ids := []int64{}
	err := db.Model(&Campaign{}).
		Where("user_id = ? AND launch_date BETWEEN ? AND ?", uid, start, end).
		Pluck("id", &ids).Error
	if err != nil {
		log.Error(err)
	}
	return ids, err
}

// GetComplianceReport builds the full report for the operator over the period.
// It runs a fixed set of queries up front and rolls everything up in memory so
// org totals and the per-group table derive from the same data and can't
// disagree. Each query starts from a fresh *gorm.DB (GORM v2 accumulates WHERE
// conditions on a reused handle — see getCampaignStats).
func GetComplianceReport(uid int64, start, end time.Time) (ComplianceReport, error) {
	rep := ComplianceReport{
		PeriodStart: start,
		PeriodEnd:   end,
		GeneratedAt: time.Now().UTC(),
		Groups:      []GroupComplianceRow{},
	}

	// Population: distinct recipients (current).
	if err := db.Model(&Recipient{}).Where("user_id = ?", uid).Count(&rep.Population).Error; err != nil {
		log.Error(err)
		return rep, err
	}

	// Phishing: in-period campaigns, summed via getCampaignStats.
	ids, err := inPeriodCampaignIDs(uid, start, end)
	if err != nil {
		return rep, err
	}
	rep.Phishing.Campaigns = int64(len(ids))
	for _, id := range ids {
		s, err := getCampaignStats(id)
		if err != nil {
			log.Error(err)
			return rep, err
		}
		rep.Phishing.Targeted += s.Total
		rep.Phishing.Sent += s.EmailsSent
		rep.Phishing.Opened += s.OpenedEmail
		rep.Phishing.Clicked += s.ClickedLink
		rep.Phishing.Submitted += s.SubmittedData
		rep.Phishing.Reported += s.EmailReported
	}
	rep.Phishing.ClickRate = pct(rep.Phishing.Clicked, rep.Phishing.Targeted)
	rep.Phishing.SubmitRate = pct(rep.Phishing.Submitted, rep.Phishing.Targeted)
	rep.Phishing.ReportRate = pct(rep.Phishing.Reported, rep.Phishing.Targeted)

	// Training: cohort assigned within the period, tallied as a funnel.
	cohort := []Enrollment{}
	if err := db.Where("user_id = ? AND assigned_date BETWEEN ? AND ?", uid, start, end).
		Find(&cohort).Error; err != nil {
		log.Error(err)
		return rep, err
	}
	for _, e := range cohort {
		rep.Training.Assigned++
		if e.Status == EnrollmentStarted || e.Status == EnrollmentCompleted {
			rep.Training.Started++
		}
		if e.Status == EnrollmentCompleted {
			rep.Training.Completed++
		}
	}
	rep.Training.Outstanding = rep.Training.Assigned - rep.Training.Completed
	rep.Training.CompletionRate = pct(rep.Training.Completed, rep.Training.Assigned)

	// Risk: current snapshot.
	scores, err := GetRiskScores(uid)
	if err != nil {
		return rep, err
	}
	rep.Risk = bucketRisk(scores)
	riskByEmail := make(map[string]int, len(scores))
	for _, s := range scores {
		riskByEmail[normEmail(s.Email)] = s.Score
	}

	// Email-keyed membership maps for the group breakdown.
	clickedEmails := resultEmailSet(uid, ids, "clicked")
	reportedEmails := resultEmailSet(uid, ids, "reported")
	completedEmails, err := completedEverEmails(uid)
	if err != nil {
		return rep, err
	}

	// Groups: pure map lookups, no per-group SQL.
	groups, err := GetGroups(uid)
	if err != nil {
		return rep, err
	}
	for _, g := range groups {
		row := GroupComplianceRow{GroupID: g.Id, Name: g.Name}
		var clicked, reported, trained, riskCount, riskSum int64
		for _, t := range g.Targets {
			e := normEmail(t.Email)
			if e == "" {
				continue
			}
			row.Members++
			if clickedEmails[e] {
				clicked++
			}
			if reportedEmails[e] {
				reported++
			}
			if completedEmails[e] {
				trained++
			}
			if sc, ok := riskByEmail[e]; ok {
				riskCount++
				riskSum += int64(sc)
			}
		}
		row.TrainedPct = pct(trained, row.Members)
		row.ClickPct = pct(clicked, row.Members)
		row.ReportPct = pct(reported, row.Members)
		if riskCount > 0 {
			row.AvgRisk = math.Round(float64(riskSum)/float64(riskCount)*10) / 10
		}
		rep.Groups = append(rep.Groups, row)
	}

	return rep, nil
}

// emailSet returns the normalized set of recipient emails that clicked (status
// Clicked or Submitted) or reported within the given campaigns. An empty
// campaign id list yields an empty set (no query).
func resultEmailSet(uid int64, campaignIDs []int64, kind string) map[string]bool {
	set := map[string]bool{}
	if len(campaignIDs) == 0 {
		return set
	}
	emails := []string{}
	q := db.Model(&Result{}).Where("user_id = ? AND campaign_id IN ?", uid, campaignIDs)
	switch kind {
	case "clicked":
		q = q.Where("status IN ?", []string{EventClicked, EventDataSubmit})
	case "reported":
		q = q.Where("reported = ?", true)
	}
	if err := q.Pluck("email", &emails).Error; err != nil {
		log.Error(err)
		return set
	}
	for _, e := range emails {
		set[normEmail(e)] = true
	}
	return set
}

// completedEverEmails returns the normalized set of recipient emails that have
// ever completed an assigned training module (current coverage).
func completedEverEmails(uid int64) (map[string]bool, error) {
	set := map[string]bool{}
	recipientIDs := []int64{}
	if err := db.Model(&Enrollment{}).
		Where("user_id = ? AND status = ?", uid, EnrollmentCompleted).
		Distinct().Pluck("recipient_id", &recipientIDs).Error; err != nil {
		log.Error(err)
		return set, err
	}
	if len(recipientIDs) == 0 {
		return set, nil
	}
	emails := []string{}
	if err := db.Model(&Recipient{}).
		Where("user_id = ? AND id IN ?", uid, recipientIDs).
		Pluck("email", &emails).Error; err != nil {
		log.Error(err)
		return set, err
	}
	for _, e := range emails {
		set[normEmail(e)] = true
	}
	return set, nil
}
