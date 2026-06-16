import { api, errorFlash, escapeHtml, modalError, successFlash, T } from './common'

let domains = []

const badge = (ok, label) => {
    const cls = ok ? "label-success" : "label-default"
    return `<span class="label ${cls}" style="margin-right:4px;">${label}${ok ? " &#10003;" : ""}</span>`
}

const healthCell = (d) => {
    const parts: string[] = []
    if (d.role === "landing" || d.role === "both") {
        parts.push(badge(d.landing_ok, "HTTPS"))
    }
    if (d.role === "sending" || d.role === "both") {
        parts.push(badge(d.spf_ok, "SPF"))
        parts.push(badge(d.dkim_ok, "DKIM"))
        parts.push(badge(d.dmarc_ok, "DMARC"))
    }
    let html = parts.join(" ")
    if (d.status) {
        html += `<br><small class="text-muted">${escapeHtml(d.status)}</small>`
    }
    return html || '<span class="text-muted">' + T("domains.not_checked") + '</span>'
}

const dismiss = () => {
    $("#name").val("")
    $("#role").val("both")
    $("#dkim_selector").val("")
    $("#flashes").empty()
}

const save = (id) => {
    const d: any = {
        name: $("#name").val(),
        role: $("#role").val(),
        dkim_selector: $("#dkim_selector").val(),
    }
    const done = (msg) => {
        dismiss()
        $("#modal").modal("hide")
        load()
        successFlash(msg)
    }
    if (id != -1) {
        d.id = parseInt(id)
        api.domainId.put(d).success(() => done(T("domains.updated", escapeHtml(d.name)))).error((x) => modalError(x.responseJSON.message))
    } else {
        api.domains.post(d).success(() => done(T("domains.registered", escapeHtml(d.name)))).error((x) => modalError(x.responseJSON.message))
    }
}

const editDomain = (id) => {
    $("#modalSubmit").unbind("click").click(() => save(id))
    if (id === -1) {
        $("#domainModalLabel").text(T("domains.new_domain"))
        dismiss()
        return
    }
    $("#domainModalLabel").text(T("domains.edit_domain"))
    const d = domains.find(x => x.id == id)
    if (d) {
        $("#name").val(d.name)
        $("#role").val(d.role)
        $("#dkim_selector").val(d.dkim_selector)
    }
}

const checkDomain = (id, btn) => {
    const original = $(btn).html()
    $(btn).prop("disabled", true).html('<i class="fa fa-spinner fa-spin"></i>')
    api.domainId.check(id)
        .success(() => { load() })
        .error((x) => { $(btn).prop("disabled", false).html(original); errorFlash(x.responseJSON.message) })
}

const showRecords = (id) => {
    const d = domains.find(x => x.id == id)
    if (!d) {
        return
    }
    const rows = (d.records || []).map(r =>
        `<tr><td>${escapeHtml(r.type)}</td><td><code>${escapeHtml(r.host)}</code></td>
         <td><code>${escapeHtml(r.value)}</code><br><small class="text-muted">${escapeHtml(r.note)}</small></td></tr>`
    ).join("")
    $("#recordsBody").html(rows)
    $("#recordsModal").modal("show")
}

const deleteDomain = (id) => {
    const d = domains.find(x => x.id == id)
    if (!d) {
        return
    }
    Swal.fire({
        title: T("domains.confirm_delete_title"),
        text: T("domains.confirm_delete_text", escapeHtml(d.name)),
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: T("common.delete"),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: () => new Promise<void>((resolve, reject) => {
            api.domainId.delete(id).success(() => resolve()).error((x) => reject(x.responseJSON.message))
        }).catch(e => Swal.showValidationMessage(e))
    }).then((result) => {
        if (result.value) {
            Swal.fire(T("domains.deleted_title"), T("domains.deleted_text"), "success")
        }
        $("button:contains('OK')").on("click", () => location.reload())
    })
}

const load = () => {
    $("#domainTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.domains.get()
        .success((ds) => {
            domains = ds
            $("#loading").hide()
            if (!domains || domains.length === 0) {
                $("#emptyMessage").show()
                return
            }
            $("#domainTable").show()
            const table = $("#domainTable").DataTable({ destroy: true, columnDefs: [{ orderable: false, targets: "no-sort" }] })
            table.clear()
            $.each(domains, (i, d) => {
                const last = d.last_checked && !moment(d.last_checked).year().toString().startsWith("0001")
                    ? moment(d.last_checked).fromNow() : "&mdash;"
                table.row.add([
                    escapeHtml(d.name),
                    escapeHtml(d.role),
                    healthCell(d),
                    last,
                    `<div class="pull-right" style="white-space:nowrap">
                        <button class="btn btn-primary check_button" data-domain-id="${d.id}" title="${T("domains.run_health_check")}"><i class="fa fa-heartbeat"></i></button>
                        <button class="btn btn-default records_button" data-domain-id="${d.id}" title="${T("domains.dns_records")}"><i class="fa fa-list"></i></button>
                        <button class="btn btn-primary edit_button" data-toggle="modal" data-backdrop="static" data-target="#modal" data-domain-id="${d.id}"><i class="fa fa-pencil"></i></button>
                        <button class="btn btn-danger delete_button" data-domain-id="${d.id}"><i class="fa fa-trash-o"></i></button>
                     </div>`
                ]).draw(false)
            })
        })
        .error(() => { $("#loading").hide(); errorFlash(T("domains.fetch_error")) })
}

$(document).ready(function () {
    load()
    $("#modal").on("hide.bs.modal", dismiss)
    $("#new_button").on("click", () => editDomain(-1))
    $("#domainTable").on("click", ".edit_button", function () { editDomain($(this).attr("data-domain-id")) })
    $("#domainTable").on("click", ".delete_button", function () { deleteDomain($(this).attr("data-domain-id")) })
    $("#domainTable").on("click", ".check_button", function () { checkDomain($(this).attr("data-domain-id"), this) })
    $("#domainTable").on("click", ".records_button", function () { showRecords($(this).attr("data-domain-id")) })
})
