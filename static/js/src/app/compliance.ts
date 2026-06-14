import { api, errorFlash, escapeHtml, T } from './common'

// Format a Date as YYYY-MM-DD (the wire format the API expects).
const ymd = (d) => d.toISOString().slice(0, 10)

// Current period selection. start is 'all' or YYYY-MM-DD; end is YYYY-MM-DD.
let period = { start: '', end: '' }

const riskBadge = (score) => {
    let cls = 'label-success'
    if (score >= 70) {
        cls = 'label-danger'
    } else if (score >= 40) {
        cls = 'label-warning'
    }
    return `<span class="label ${cls}" style="font-size:13px;">${score}</span>`
}

const fmtPct = (v) => `${v}%`

const render = (rep) => {
    $('#kpiPopulation').text(rep.population)
    $('#kpiCompletion').text(fmtPct(rep.training.completion_rate))
    $('#kpiClickRate').text(fmtPct(rep.phishing.click_rate))
    $('#kpiReportRate').text(fmtPct(rep.phishing.report_rate))
    $('#kpiRiskLow').text(rep.risk.low)
    $('#kpiRiskMed').text(rep.risk.medium)
    $('#kpiRiskHigh').text(rep.risk.high)
    $('#kpiRiskAvg').text(rep.risk.average)

    $('#trAssigned').text(rep.training.assigned)
    $('#trStarted').text(rep.training.started)
    $('#trCompleted').text(rep.training.completed)
    $('#trOutstanding').text(rep.training.outstanding)

    $('#phCampaigns').text(rep.phishing.campaigns)
    $('#phTargeted').text(rep.phishing.targeted)
    $('#phSent').text(rep.phishing.sent)
    $('#phClicked').text(rep.phishing.clicked)
    $('#phSubmitted').text(rep.phishing.submitted)
    $('#phReported').text(rep.phishing.reported)

    const table = $('#groupTable').DataTable({
        destroy: true,
        order: [[5, 'desc']],
        columnDefs: [{ type: 'num', targets: [1, 2, 3, 4, 5] }]
    })
    table.clear()
    $.each(rep.groups || [], (i, g) => {
        table.row.add([
            escapeHtml(g.name),
            g.members,
            fmtPct(g.trained_pct),
            fmtPct(g.click_pct),
            fmtPct(g.report_pct),
            `<span data-order="${g.avg_risk}">${riskBadge(g.avg_risk)}</span>`
        ]).draw(false)
    })

    // Management body (NIS2 Art. 20).
    const mb = rep.management_body || {}
    $('#mbMembers').text(mb.members || 0)
    $('#mbTrained').text(fmtPct(mb.trained_pct || 0))
    $('#mbClick').text(fmtPct(mb.click_pct || 0))
    $('#mbReport').text(fmtPct(mb.report_pct || 0))
    $('#mbRisk').text(mb.avg_risk || 0)

    // By organizational unit (indent by level).
    const indent = { department: '', sub_department: '    ', wydzial: '       ' }
    const orgTable = $('#orgUnitTable').DataTable({
        destroy: true, paging: false, searching: false, info: false, ordering: false
    })
    orgTable.clear()
    $.each(rep.org_units || [], (i, u) => {
        const name = (indent[u.level] || '') + escapeHtml(u.name)
        orgTable.row.add([
            u.level === 'department' ? `<strong>${name}</strong>` : name,
            u.members, fmtPct(u.trained_pct), fmtPct(u.click_pct), fmtPct(u.report_pct),
            `<span data-order="${u.avg_risk}">${riskBadge(u.avg_risk)}</span>`
        ]).draw(false)
    })

    // By position level (already ranked low->high by the server).
    const posTable = $('#positionTable').DataTable({
        destroy: true, paging: false, searching: false, info: false, ordering: false
    })
    posTable.clear()
    $.each(rep.positions || [], (i, p) => {
        posTable.row.add([
            escapeHtml(p.level), p.members, fmtPct(p.trained_pct), fmtPct(p.click_pct),
            fmtPct(p.report_pct), `<span data-order="${p.avg_risk}">${riskBadge(p.avg_risk)}</span>`
        ]).draw(false)
    })

    const label = period.start === 'all'
        ? T("compliance.all_time_through", period.end)
        : T("compliance.reporting_period", period.start, period.end)
    $('#periodLabel').text(T("compliance.prepared_by", label, escapeHtml(rep.operator)))
    $('#downloadPdf').attr('href', `/compliance/report.pdf?start=${period.start}&end=${period.end}`)
}

const load = () => {
    $('#report').hide()
    $('#loading').show()
    api.compliance.get(period.start, period.end)
        .success((rep) => {
            $('#loading').hide()
            $('#report').show()
            render(rep)
        })
        .error(() => {
            $('#loading').hide()
            errorFlash(T("compliance.fetch_error"))
        })
}

// Set the period from a preset (days as number, or 'all') and reload.
const applyPreset = (days) => {
    const end = new Date()
    if (days === 'all') {
        period = { start: 'all', end: ymd(end) }
    } else {
        const start = new Date()
        start.setDate(start.getDate() - Number(days))
        period = { start: ymd(start), end: ymd(end) }
        $('#startDate').val(period.start)
    }
    $('#endDate').val(period.end)
    load()
}

$(document).ready(function () {
    $('.period-preset').on('click', function () {
        $('.period-preset').removeClass('active')
        $(this).addClass('active')
        applyPreset($(this).data('days'))
    })
    $('#applyPeriod').on('click', function () {
        const start = $('#startDate').val()
        const end = $('#endDate').val()
        if (!start || !end) {
            errorFlash(T("compliance.choose_dates"))
            return
        }
        $('.period-preset').removeClass('active')
        period = { start: start, end: end }
        load()
    })
    // Default: last 12 months.
    applyPreset(365)
})
