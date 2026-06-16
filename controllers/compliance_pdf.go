package controllers

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/http"

	"github.com/go-pdf/fpdf"

	ctx "github.com/rdumanski/gophish/context"
	log "github.com/rdumanski/gophish/logger"
	"github.com/rdumanski/gophish/models"
)

// DejaVu Sans Condensed is embedded so the report renders Polish diacritics
// (ł ą ś ż ń ę ó ć ź …) — fpdf's built-in core fonts are Latin-1 only. See
// controllers/fonts/README.md for license/attribution.
//
//go:embed fonts/DejaVuSansCondensed.ttf
var dejaVuRegular []byte

//go:embed fonts/DejaVuSansCondensed-Bold.ttf
var dejaVuBold []byte

// pdfFont is the family name registered with fpdf for both weights.
const pdfFont = "DejaVu"

// ComplianceReportPDF serves the NIS2 compliance report as a downloadable PDF.
// It lives on the admin server (session-authed via RequireLogin) rather than the
// API so the api_key never appears in the download URL / access logs.
func (as *AdminServer) ComplianceReportPDF(w http.ResponseWriter, r *http.Request) {
	user := ctx.Get(r, "user").(models.User)
	start, end, err := models.ParseReportPeriod(r.URL.Query().Get("start"), r.URL.Query().Get("end"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	report, err := models.GetComplianceReport(user.Id, start, end)
	if err != nil {
		log.Error(err)
		http.Error(w, "failed to build report", http.StatusInternalServerError)
		return
	}
	report.Operator = user.Username

	// Render to a buffer first so a render error doesn't leave a half-written
	// response with headers already committed.
	buf, err := renderCompliancePDF(report)
	if err != nil {
		log.Error(err)
		http.Error(w, "failed to render PDF", http.StatusInternalServerError)
		return
	}
	startStamp := "all"
	if !report.PeriodStart.IsZero() {
		startStamp = report.PeriodStart.Format("2006-01-02")
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="nis2-compliance-%s_%s.pdf"`,
			startStamp, report.PeriodEnd.Format("2006-01-02")))
	w.Write(buf.Bytes())
}

const (
	pdfGray   = 245
	pdfMargin = 15.0
)

// renderCompliancePDF lays out the report onto an A4 page. An embedded UTF-8
// font (DejaVu) is registered so full Unicode — including Polish diacritics —
// renders correctly.
func renderCompliancePDF(rep models.ComplianceReport) (*bytes.Buffer, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes(pdfFont, "", dejaVuRegular)
	pdf.AddUTF8FontFromBytes(pdfFont, "B", dejaVuBold)
	pdf.SetMargins(pdfMargin, pdfMargin, pdfMargin)
	pdf.SetAutoPageBreak(true, pdfMargin)
	pdf.AddPage()

	periodStart := "All time"
	if !rep.PeriodStart.IsZero() {
		periodStart = rep.PeriodStart.Format("2006-01-02")
	}
	periodEnd := rep.PeriodEnd.Format("2006-01-02")

	// Title
	pdf.SetFont(pdfFont, "B", 18)
	pdf.SetTextColor(20, 30, 40)
	pdf.CellFormat(0, 10, "NIS2 Compliance Report", "", 1, "L", false, 0, "")
	pdf.SetFont(pdfFont, "", 10)
	pdf.SetTextColor(110, 110, 110)
	pdf.CellFormat(0, 6, fmt.Sprintf("Reporting period: %s to %s", periodStart, periodEnd), "", 1, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Prepared by %s  |  Generated %s",
		rep.Operator, rep.GeneratedAt.Format("2006-01-02 15:04 UTC")), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// Executive summary KPIs
	sectionHeader(pdf, "Executive summary")
	kpiRow(pdf, []kpi{
		{"People", fmt.Sprintf("%d", rep.Population)},
		{"Training completion", pctStr(rep.Training.CompletionRate)},
		{"Click rate", pctStr(rep.Phishing.ClickRate)},
		{"Report rate", pctStr(rep.Phishing.ReportRate)},
	})
	pdf.Ln(2)
	pdf.SetFont(pdfFont, "", 10)
	pdf.SetTextColor(60, 60, 60)
	pdf.CellFormat(0, 6, fmt.Sprintf("Human risk: %d low / %d medium / %d high  (average score %.1f)",
		rep.Risk.Low, rep.Risk.Medium, rep.Risk.High, rep.Risk.Average), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// Training coverage — Art. 21(2)(g)
	sectionHeader(pdf, "Training coverage  (NIS2 Art. 21(2)(g))")
	kvTable(pdf, [][2]string{
		{"Assigned (in period)", fmt.Sprintf("%d", rep.Training.Assigned)},
		{"Started", fmt.Sprintf("%d", rep.Training.Started)},
		{"Completed", fmt.Sprintf("%d", rep.Training.Completed)},
		{"Outstanding", fmt.Sprintf("%d", rep.Training.Outstanding)},
		{"Completion rate", pctStr(rep.Training.CompletionRate)},
	})
	pdf.Ln(3)

	// Phishing program
	sectionHeader(pdf, "Phishing simulation program")
	kvTable(pdf, [][2]string{
		{"Campaigns (launched in period)", fmt.Sprintf("%d", rep.Phishing.Campaigns)},
		{"Recipients targeted", fmt.Sprintf("%d", rep.Phishing.Targeted)},
		{"Emails sent", fmt.Sprintf("%d", rep.Phishing.Sent)},
		{"Clicked link", fmt.Sprintf("%d (%s of targeted)", rep.Phishing.Clicked, pctStr(rep.Phishing.ClickRate))},
		{"Submitted data", fmt.Sprintf("%d (%s of targeted)", rep.Phishing.Submitted, pctStr(rep.Phishing.SubmitRate))},
		{"Reported", fmt.Sprintf("%d (%s of targeted)", rep.Phishing.Reported, pctStr(rep.Phishing.ReportRate))},
	})
	pdf.Ln(3)

	// Management body (NIS2 Art. 20)
	sectionHeader(pdf, "Management body  (NIS2 Art. 20)")
	mb := rep.ManagementBody
	kvTable(pdf, [][2]string{
		{"Members (Prezes / Wiceprezes)", fmt.Sprintf("%d", mb.Members)},
		{"Training completed", pctStr(mb.TrainedPct)},
		{"Phishing click rate", pctStr(mb.ClickPct)},
		{"Phishing report rate", pctStr(mb.ReportPct)},
		{"Average risk", fmt.Sprintf("%.1f", mb.AvgRisk)},
	})
	pdf.Ln(3)

	// By org unit (Department > Sub-Department > Wydzial)
	sectionHeader(pdf, "By organizational unit")
	orgUnitTable(pdf, rep.OrgUnits)
	pdf.Ln(3)

	// By position level
	sectionHeader(pdf, "By position level")
	positionTable(pdf, rep.Positions)
	pdf.Ln(3)

	// Per-group breakdown
	sectionHeader(pdf, "By group")
	groupTable(pdf, rep.Groups)
	pdf.Ln(3)

	// Footer / framing
	pdf.SetFont(pdfFont, "", 8)
	pdf.SetTextColor(130, 130, 130)
	pdf.MultiCell(0, 4, "This report supports demonstrating security-awareness governance under NIS2 "+
		"Art. 21(2)(g) (cyber-hygiene and training) and Art. 20 (management-body oversight). "+
		"Per-recipient risk is a current snapshot; phishing and training figures cover the selected "+
		"period. Group members are matched by email; phone-only contacts are excluded. This is an "+
		"operational summary, not a legal certification of NIS2 conformance.", "", "L", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return &buf, nil
}

type kpi struct{ label, value string }

func sectionHeader(pdf *fpdf.Fpdf, title string) {
	pdf.SetFont(pdfFont, "B", 12)
	pdf.SetTextColor(20, 30, 40)
	pdf.CellFormat(0, 8, title, "", 1, "L", false, 0, "")
	pdf.SetDrawColor(200, 200, 200)
	y := pdf.GetY()
	pdf.Line(pdfMargin, y, 210-pdfMargin, y)
	pdf.Ln(2)
}

func kpiRow(pdf *fpdf.Fpdf, kpis []kpi) {
	const gap = 4.0
	avail := 210 - 2*pdfMargin
	w := (avail - gap*float64(len(kpis)-1)) / float64(len(kpis))
	x := pdf.GetX()
	y := pdf.GetY()
	for i, k := range kpis {
		cx := x + float64(i)*(w+gap)
		pdf.SetFillColor(pdfGray, pdfGray, pdfGray)
		pdf.Rect(cx, y, w, 18, "F")
		pdf.SetXY(cx, y+3)
		pdf.SetFont(pdfFont, "B", 16)
		pdf.SetTextColor(20, 30, 40)
		pdf.CellFormat(w, 8, k.value, "", 2, "C", false, 0, "")
		pdf.SetX(cx)
		pdf.SetFont(pdfFont, "", 8)
		pdf.SetTextColor(110, 110, 110)
		pdf.CellFormat(w, 5, k.label, "", 0, "C", false, 0, "")
	}
	pdf.SetXY(x, y+18)
}

func kvTable(pdf *fpdf.Fpdf, rows [][2]string) {
	pdf.SetFont(pdfFont, "", 10)
	pdf.SetTextColor(60, 60, 60)
	for i, row := range rows {
		fill := i%2 == 0
		if fill {
			pdf.SetFillColor(248, 248, 248)
		}
		pdf.CellFormat(110, 7, " "+row[0], "", 0, "L", fill, 0, "")
		pdf.CellFormat(70, 7, row[1]+" ", "", 1, "R", fill, 0, "")
	}
}

func groupTable(pdf *fpdf.Fpdf, groups []models.GroupComplianceRow) {
	headers := []string{"Group", "Members", "Trained %", "Click %", "Report %", "Avg risk"}
	widths := []float64{60, 24, 24, 24, 24, 24}
	pdf.SetFont(pdfFont, "B", 9)
	pdf.SetFillColor(225, 230, 235)
	pdf.SetTextColor(20, 30, 40)
	for i, h := range headers {
		align := "R"
		if i == 0 {
			align = "L"
		}
		pdf.CellFormat(widths[i], 7, h, "", 0, align, true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont(pdfFont, "", 9)
	pdf.SetTextColor(60, 60, 60)
	if len(groups) == 0 {
		pdf.CellFormat(0, 7, "  No groups defined.", "", 1, "L", false, 0, "")
		return
	}
	for i, g := range groups {
		fill := i%2 == 0
		if fill {
			pdf.SetFillColor(248, 248, 248)
		}
		cells := []string{
			g.Name,
			fmt.Sprintf("%d", g.Members),
			pctStr(g.TrainedPct),
			pctStr(g.ClickPct),
			pctStr(g.ReportPct),
			fmt.Sprintf("%.1f", g.AvgRisk),
		}
		for j, c := range cells {
			align := "R"
			if j == 0 {
				align = "L"
			}
			pdf.CellFormat(widths[j], 7, c, "", 0, align, fill, 0, "")
		}
		pdf.Ln(-1)
	}
}

// orgMetricCells appends the shared metric columns (members + 4 rates/score).
func orgMetricCells(pdf *fpdf.Fpdf, m models.OrgMetrics, widths []float64, fill bool) {
	cells := []string{
		fmt.Sprintf("%d", m.Members), pctStr(m.TrainedPct), pctStr(m.ClickPct),
		pctStr(m.ReportPct), fmt.Sprintf("%.1f", m.AvgRisk),
	}
	for i, c := range cells {
		pdf.CellFormat(widths[i+1], 7, c, "", 0, "R", fill, 0, "")
	}
	pdf.Ln(-1)
}

func orgMetricHeader(pdf *fpdf.Fpdf, first string, widths []float64) {
	headers := []string{first, "Members", "Trained %", "Click %", "Report %", "Avg risk"}
	pdf.SetFont(pdfFont, "B", 9)
	pdf.SetFillColor(225, 230, 235)
	pdf.SetTextColor(20, 30, 40)
	for i, h := range headers {
		align := "R"
		if i == 0 {
			align = "L"
		}
		pdf.CellFormat(widths[i], 7, h, "", 0, align, true, 0, "")
	}
	pdf.Ln(-1)
	pdf.SetFont(pdfFont, "", 9)
	pdf.SetTextColor(60, 60, 60)
}

func orgUnitTable(pdf *fpdf.Fpdf, units []models.OrgUnitRow) {
	widths := []float64{74, 18, 22, 22, 22, 22}
	orgMetricHeader(pdf, "Unit", widths)
	if len(units) == 0 {
		pdf.CellFormat(0, 7, "  No org-structure data (import Department/Wydzial columns).", "", 1, "L", false, 0, "")
		return
	}
	for i, u := range units {
		fill := i%2 == 0
		if fill {
			pdf.SetFillColor(248, 248, 248)
		}
		indent := map[string]string{"department": "", "sub_department": "    ", "wydzial": "        "}[u.Level]
		if u.Level == "department" {
			pdf.SetFont(pdfFont, "B", 9)
		}
		pdf.CellFormat(widths[0], 7, indent+u.Name, "", 0, "L", fill, 0, "")
		orgMetricCells(pdf, u.OrgMetrics, widths, fill)
		if u.Level == "department" {
			pdf.SetFont(pdfFont, "", 9)
		}
	}
}

func positionTable(pdf *fpdf.Fpdf, rows []models.PositionLevelRow) {
	widths := []float64{74, 18, 22, 22, 22, 22}
	orgMetricHeader(pdf, "Position level", widths)
	if len(rows) == 0 {
		pdf.CellFormat(0, 7, "  No position-level data.", "", 1, "L", false, 0, "")
		return
	}
	for i, r := range rows {
		fill := i%2 == 0
		if fill {
			pdf.SetFillColor(248, 248, 248)
		}
		pdf.CellFormat(widths[0], 7, r.Level, "", 0, "L", fill, 0, "")
		orgMetricCells(pdf, r.OrgMetrics, widths, fill)
	}
}

func pctStr(v float64) string { return fmt.Sprintf("%.1f%%", v) }
