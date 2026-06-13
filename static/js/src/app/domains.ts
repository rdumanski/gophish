import { api, errorFlash, escapeHtml, modalError, successFlash } from './common'

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
    return html || '<span class="text-muted">not checked</span>'
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
    const done = (verb) => {
        dismiss()
        $("#modal").modal("hide")
        load()
        successFlash(`Domain "${escapeHtml(d.name)}" ${verb}.`)
    }
    if (id != -1) {
        d.id = parseInt(id)
        api.domainId.put(d).success(() => done("updated")).error((x) => modalError(x.responseJSON.message))
    } else {
        api.domains.post(d).success(() => done("registered")).error((x) => modalError(x.responseJSON.message))
    }
}

const editDomain = (id) => {
    $("#modalSubmit").unbind("click").click(() => save(id))
    if (id === -1) {
        $("#domainModalLabel").text("New Domain")
        dismiss()
        return
    }
    $("#domainModalLabel").text("Edit Domain")
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
        title: "Are you sure?",
        text: `This will remove the domain '${escapeHtml(d.name)}' from the registry.`,
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: () => new Promise<void>((resolve, reject) => {
            api.domainId.delete(id).success(() => resolve()).error((x) => reject(x.responseJSON.message))
        }).catch(e => Swal.showValidationMessage(e))
    }).then((result) => {
        if (result.value) {
            Swal.fire("Deleted!", "The domain has been removed.", "success")
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
                    `<div class="pull-right">
                        <button class="btn btn-primary check_button" data-domain-id="${d.id}" title="Run health check"><i class="fa fa-heartbeat"></i></button>
                        <button class="btn btn-default records_button" data-domain-id="${d.id}" title="DNS records to publish"><i class="fa fa-list"></i></button>
                        <button class="btn btn-primary edit_button" data-toggle="modal" data-backdrop="static" data-target="#modal" data-domain-id="${d.id}"><i class="fa fa-pencil"></i></button>
                        <button class="btn btn-danger delete_button" data-domain-id="${d.id}"><i class="fa fa-trash-o"></i></button>
                     </div>`
                ]).draw(false)
            })
        })
        .error(() => { $("#loading").hide(); errorFlash("Error fetching domains") })
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
