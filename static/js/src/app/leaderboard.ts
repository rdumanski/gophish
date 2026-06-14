import { api, errorFlash, escapeHtml, T } from './common'

// Engagement score badge: green high (>=70), amber mid (>=40), red low.
const scoreBadge = (score) => {
    let cls = 'label-danger'
    if (score >= 70) {
        cls = 'label-success'
    } else if (score >= 40) {
        cls = 'label-warning'
    }
    return `<span class="label ${cls}" style="font-size:13px;">${score}</span>`
}

// Render earned badges as small labels, localized via badge.<key>.
const renderBadges = (badges) => {
    if (!badges || !badges.length) {
        return '<span class="text-muted">—</span>'
    }
    return badges.map((b) =>
        `<span class="label label-info" style="margin-right:4px;">${escapeHtml(T('badge.' + b))}</span>`
    ).join('')
}

const load = () => {
    $('#report').hide()
    $('#emptyMessage').hide()
    $('#loading').show()
    api.engagement.get()
        .success((rep) => {
            $('#loading').hide()
            const individuals = (rep && rep.individuals) || []
            if (!individuals.length) {
                $('#emptyMessage').show()
                return
            }
            $('#report').show()

            const deptTable = $('#deptTable').DataTable({
                destroy: true, paging: false, searching: false, info: false, ordering: false
            })
            deptTable.clear()
            $.each(rep.departments || [], (i, d) => {
                deptTable.row.add([
                    i + 1,
                    escapeHtml(d.department),
                    d.members,
                    `<span data-order="${d.avg_score}">${scoreBadge(d.avg_score)}</span>`,
                    `${d.report_rate}%`
                ]).draw(false)
            })

            const table = $('#leaderTable').DataTable({
                destroy: true, order: [], ordering: false, paging: true, pageLength: 25
            })
            table.clear()
            $.each(individuals, (i, e) => {
                const who = e.name
                    ? `${escapeHtml(e.name)} <br><small class="text-muted">${escapeHtml(e.email)}</small>`
                    : escapeHtml(e.email)
                const streak = e.streak > 0
                    ? `<span class="label label-success">${e.streak}</span>`
                    : '<span class="text-muted">—</span>'
                table.row.add([
                    i + 1,
                    `<span data-order="${e.score}">${scoreBadge(e.score)}</span>`,
                    who,
                    renderBadges(e.badges),
                    streak,
                    e.reported,
                    e.sims
                ]).draw(false)
            })
        })
        .error(() => {
            $('#loading').hide()
            errorFlash(T('leaderboard.fetch_error'))
        })
}

$(document).ready(function () {
    load()
})
