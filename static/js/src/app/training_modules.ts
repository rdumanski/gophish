import { api, errorFlash, escapeHtml, modalError, successFlash } from './common'

let modules = []

// Show the HTML editor for inline modules and the URL field for
// video/external modules.
const toggleContentFields = () => {
    const type = $("#content_type").val()
    if (type === "html") {
        $("#html_group").show()
        $("#url_group").hide()
    } else {
        $("#html_group").hide()
        $("#url_group").show()
    }
}

const dismiss = () => {
    $("#name").val("")
    $("#description").val("")
    $("#content_type").val("html")
    $("#html").val("")
    $("#url").val("")
    toggleContentFields()
    $("#flashes").empty()
}

const saveModule = (id) => {
    const m: any = {
        name: $("#name").val(),
        description: $("#description").val(),
        content_type: $("#content_type").val(),
        html: $("#html").val(),
        url: $("#url").val(),
    }
    if (id != -1) {
        m.id = parseInt(id)
        api.trainingModuleId.put(m)
            .success(function () {
                dismiss()
                load()
                $("#modal").modal("hide")
                successFlash(`Training module "${escapeHtml(m.name)}" updated successfully!`)
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        api.training_modules.post(m)
            .success(function () {
                dismiss()
                load()
                $("#modal").modal("hide")
                successFlash(`Training module "${escapeHtml(m.name)}" created successfully!`)
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

const load = () => {
    $("#moduleTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.training_modules.get()
        .success((ms) => {
            modules = ms
            $("#loading").hide()
            if (!modules || modules.length === 0) {
                $("#emptyMessage").show()
                return
            }
            $("#moduleTable").show()
            const moduleTable = $("#moduleTable").DataTable({
                destroy: true,
                columnDefs: [{ orderable: false, targets: "no-sort" }]
            })
            moduleTable.clear()
            $.each(modules, (i, m) => {
                moduleTable.row.add([
                    escapeHtml(m.name),
                    escapeHtml(m.content_type),
                    escapeHtml(m.description || ""),
                    moment(m.modified_date).format("MMMM Do YYYY, h:mm:ss a"),
                    `
                      <div class="pull-right">
                        <button class="btn btn-primary edit_button" data-toggle="modal" data-backdrop="static" data-target="#modal" data-module-id="${m.id}">
                          <i class="fa fa-pencil"></i>
                        </button>
                        <button class="btn btn-danger delete_button" data-module-id="${m.id}">
                          <i class="fa fa-trash-o"></i>
                        </button>
                      </div>
                    `
                ]).draw()
            })
        })
        .error(() => {
            $("#loading").hide()
            errorFlash("Error fetching training modules")
        })
}

const editModule = (id) => {
    $("#modalSubmit").unbind("click").click(() => {
        saveModule(id)
    })
    if (id !== -1) {
        $("#moduleModalLabel").text("Edit Training Module")
        api.trainingModuleId.get(id)
            .success(function (m) {
                $("#name").val(m.name)
                $("#description").val(m.description)
                $("#content_type").val(m.content_type)
                $("#html").val(m.html)
                $("#url").val(m.url)
                toggleContentFields()
            })
            .error(function () {
                errorFlash("Error fetching training module")
            })
    } else {
        $("#moduleModalLabel").text("New Training Module")
        toggleContentFields()
    }
}

const deleteModule = (id) => {
    const m = modules.find(x => x.id == id)
    if (!m) {
        return
    }
    Swal.fire({
        title: "Are you sure?",
        text: `This will delete the training module '${escapeHtml(m.name)}'`,
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete",
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise<void>((resolve, reject) => {
                api.trainingModuleId.delete(id)
                    .success(() => resolve())
                    .error((data) => reject(data.responseJSON.message))
            })
                .catch(error => {
                    Swal.showValidationMessage(error)
                })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire("Training Module Deleted!", "The training module has been deleted!", "success")
        }
        $("button:contains('OK')").on("click", function () {
            location.reload()
        })
    })
}

$(document).ready(function () {
    load()
    $("#modal").on("hide.bs.modal", function () {
        dismiss()
    })
    $("#new_button").on("click", function () {
        editModule(-1)
    })
    $("#content_type").on("change", toggleContentFields)
    $("#moduleTable").on("click", ".edit_button", function () {
        editModule($(this).attr("data-module-id"))
    })
    $("#moduleTable").on("click", ".delete_button", function () {
        deleteModule($(this).attr("data-module-id"))
    })
})
