import { api, errorFlash, escapeHtml, modalError, successFlash } from './common'

let campaigns = []

const setupOptions = () => {
    api.training_modules.get()
        .success((modules) => {
            if (!modules || modules.length === 0) {
                modalError("No training modules found! Create one first.")
                return
            }
            const data = $.map(modules, (m) => ({ id: m.id, text: m.name }))
            $("#module").select2({ placeholder: "Select a module", data: data })
        })
    api.groups.summary()
        .success((summaries) => {
            const groups = summaries.groups || []
            if (groups.length === 0) {
                modalError("No groups found! Create one first.")
                return
            }
            const data = $.map(groups, (g) => ({ id: g.id, text: g.name }))
            $("#groups").select2({ placeholder: "Select groups", data: data })
        })
}

const dismiss = () => {
    $("#name").val("")
    $("#module").val("").trigger("change")
    $("#groups").val("").trigger("change")
    $("#due_date").val("")
    $("#flashes").empty()
}

const save = () => {
    const groups: any[] = []
    $("#groups").select2("data").forEach((g) => groups.push({ name: g.text }))
    const due = $("#due_date").val() as string
    const tc = {
        name: $("#name").val(),
        module_id: parseInt($("#module").val() as string),
        due_date: due ? moment(due, "MMMM Do YYYY, h:mm a").utc().format() : null,
        groups: groups,
    }
    api.training_campaigns.post(tc)
        .success(() => {
            dismiss()
            $("#modal").modal("hide")
            load()
            successFlash(`Training campaign "${escapeHtml(tc.name)}" created.`)
        })
        .error((data) => {
            modalError(data.responseJSON.message)
        })
}

const progressCell = (stats) => {
    const total = stats.total || 0
    const pct = total ? Math.round((100 * stats.completed) / total) : 0
    return `
      <div class="progress" style="margin-bottom:4px">
        <div class="progress-bar progress-bar-success" role="progressbar" style="width:${pct}%">${pct}%</div>
      </div>
      <small>${stats.completed}/${total} completed &middot; ${stats.started} in progress</small>`
}

const load = () => {
    $("#campaignTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.training_campaigns.get()
        .success((tcs) => {
            campaigns = tcs
            $("#loading").hide()
            if (!campaigns || campaigns.length === 0) {
                $("#emptyMessage").show()
                return
            }
            $("#campaignTable").show()
            const table = $("#campaignTable").DataTable({
                destroy: true,
                columnDefs: [{ orderable: false, targets: "no-sort" }]
            })
            table.clear()
            $.each(campaigns, (i, tc) => {
                const due = tc.due_date && !moment(tc.due_date).year().toString().startsWith("0001")
                    ? moment(tc.due_date).format("MMM Do YYYY")
                    : "&mdash;"
                table.row.add([
                    escapeHtml(tc.name),
                    due,
                    progressCell(tc.stats),
                    moment(tc.created_date).format("MMM Do YYYY"),
                    `<div class="pull-right">
                        <button class="btn btn-danger delete_button" data-campaign-id="${tc.id}">
                          <i class="fa fa-trash-o"></i>
                        </button>
                     </div>`
                ]).draw(false)
            })
        })
        .error(() => {
            $("#loading").hide()
            errorFlash("Error fetching training campaigns")
        })
}

const deleteCampaign = (id) => {
    const tc = campaigns.find(x => x.id == id)
    if (!tc) {
        return
    }
    Swal.fire({
        title: "Are you sure?",
        text: `This will delete the training campaign '${escapeHtml(tc.name)}' and its enrollments.`,
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: () => {
            return new Promise<void>((resolve, reject) => {
                api.trainingCampaignId.delete(id)
                    .success(() => resolve())
                    .error((data) => reject(data.responseJSON.message))
            }).catch(error => {
                Swal.showValidationMessage(error)
            })
        }
    }).then((result) => {
        if (result.value) {
            Swal.fire("Deleted!", "The training campaign has been deleted.", "success")
        }
        $("button:contains('OK')").on("click", () => location.reload())
    })
}

$(document).ready(function () {
    $.fn.select2.defaults.set("width", "100%")
    $.fn.select2.defaults.set("dropdownParent", $("#modal_body"))
    $("#due_date").datetimepicker({
        widgetPositioning: { vertical: "bottom" },
        showTodayButton: true,
        useCurrent: false,
        format: "MMMM Do YYYY, h:mm a"
    })
    load()
    $("#modal").on("hide.bs.modal", dismiss)
    $("#new_button").on("click", setupOptions)
    $("#modalSubmit").on("click", save)
    $("#campaignTable").on("click", ".delete_button", function () {
        deleteCampaign($(this).attr("data-campaign-id"))
    })
})
