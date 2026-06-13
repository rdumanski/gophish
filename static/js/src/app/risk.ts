import { api, errorFlash, escapeHtml } from './common'

// Colour the score badge by band: low (green) < 40, medium (amber) < 70, high (red).
const scoreBadge = (score) => {
    let cls = "label-success"
    if (score >= 70) {
        cls = "label-danger"
    } else if (score >= 40) {
        cls = "label-warning"
    }
    return `<span class="label ${cls}" style="font-size:13px;">${score}</span>`
}

const load = () => {
    $("#riskTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.risk.get()
        .success((rows) => {
            $("#loading").hide()
            if (!rows || rows.length === 0) {
                $("#emptyMessage").show()
                return
            }
            $("#riskTable").show()
            const table = $("#riskTable").DataTable({
                destroy: true,
                order: [[0, "desc"]],
                columnDefs: [{ type: "num", targets: 0 }]
            })
            table.clear()
            $.each(rows, (i, r) => {
                const who = r.name
                    ? `${escapeHtml(r.name)} <br><small class="text-muted">${escapeHtml(r.email)}</small>`
                    : escapeHtml(r.email)
                const training = r.training_assigned > 0
                    ? `${r.training_completed}/${r.training_assigned} completed`
                    : '<span class="text-muted">none</span>'
                table.row.add([
                    `<span data-order="${r.score}">${scoreBadge(r.score)}</span>`,
                    who,
                    r.sims,
                    r.clicked,
                    r.submitted,
                    r.reported,
                    training
                ]).draw(false)
            })
        })
        .error(() => {
            $("#loading").hide()
            errorFlash("Error fetching risk report")
        })
}

$(document).ready(function () {
    load()
})
