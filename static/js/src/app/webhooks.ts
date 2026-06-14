import { api, errorFlash, escapeHtml, modalError, successFlash, T } from './common'

let webhooks = [];

const dismiss = () => {
    $("#name").val("");
    $("#url").val("");
    $("#secret").val("");
    $("#is_active").prop("checked", false);
    $("#flashes").empty();
};

const saveWebhook = (id) => {
    let wh: any = {
        name: $("#name").val(),
        url: $("#url").val(),
        secret: $("#secret").val(),
        is_active: $("#is_active").is(":checked"),
    };
    if (id != -1) {
        wh.id = parseInt(id);
        api.webhookId.put(wh)
            .success(function(data) {
                dismiss();
                load();
                $("#modal").modal("hide");
                successFlash(T("webhooks.updated", escapeHtml(wh.name)));
            })
            .error(function(data) {
                modalError(data.responseJSON.message)
            })
    } else {
        api.webhooks.post(wh)
            .success(function(data) {
                load();
                dismiss();
                $("#modal").modal("hide");
                successFlash(T("webhooks.created", escapeHtml(wh.name)));
            })
            .error(function(data) {
                modalError(data.responseJSON.message)
            })
    }
};

const load = () => {
    $("#webhookTable").hide();
    $("#loading").show();
    api.webhooks.get()
        .success((whs) => {
            webhooks = whs;
            $("#loading").hide()
            $("#webhookTable").show()
            let webhookTable = $("#webhookTable").DataTable({
                destroy: true,
                columnDefs: [{
                    orderable: false,
                    targets: "no-sort"
                }]
            });
            webhookTable.clear();
            $.each(webhooks, (i, webhook) => {
                webhookTable.row.add([
                    escapeHtml(webhook.name),
                    escapeHtml(webhook.url),
                    escapeHtml(webhook.is_active),
                    `
                      <div class="pull-right">
                        <button class="btn btn-primary ping_button" data-webhook-id="${webhook.id}">
                          ${T("webhooks.ping")}
                        </button>
                        <button class="btn btn-primary edit_button" data-toggle="modal" data-backdrop="static" data-target="#modal" data-webhook-id="${webhook.id}">
                          <i class="fa fa-pencil"></i>
                        </button>
                        <button class="btn btn-danger delete_button" data-webhook-id="${webhook.id}">
                          <i class="fa fa-trash-o"></i>
                        </button>
                      </div>
                    `
                ]).draw()
            })
        })
        .error(() => {
            errorFlash(T("webhooks.fetch_error"))
        })
};

const editWebhook = (id) => {
    $("#modalSubmit").unbind("click").click(() => {
        saveWebhook(id);
    });
    if (id !== -1) {
        $("#webhookModalLabel").text(T("webhooks.edit_title"))
        api.webhookId.get(id)
          .success(function(wh) {
              $("#name").val(wh.name);
              $("#url").val(wh.url);
              $("#secret").val(wh.secret);
              $("#is_active").prop("checked", wh.is_active);
          })
          .error(function () {
              errorFlash(T("webhooks.fetch_one_error"))
          });
    } else {
        $("#webhookModalLabel").text(T("webhooks.new_title"))
    }
};

const deleteWebhook = (id) => {
    var wh = webhooks.find(x => x.id == id);
    if (!wh) {
        return;
    }
    Swal.fire({
        title: T("webhooks.delete_title"),
        text: T("webhooks.delete_confirm", escapeHtml(wh.name)),
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: T("common.delete"),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise<void>((resolve, reject) => {
                api.webhookId.delete(id)
                    .success((msg) => {
                        resolve()
                    })
                    .error((data) => {
                        reject(data.responseJSON.message)
                    })
            })
            .catch(error => {
                Swal.showValidationMessage(error)
              })
        }
    }).then(function(result) {
        if (result.value) {
            Swal.fire(
                T("webhooks.deleted_title"),
                T("webhooks.deleted_text"),
                "success"
            );
        }
        $("button:contains('OK')").on("click", function() {
            location.reload();
        })
    })
};

const pingUrl = (btn, whId) => {
    dismiss();
    btn.disabled = true;
    api.webhookId.ping(whId)
        .success(function(wh) {
            btn.disabled = false;
            successFlash(T("webhooks.ping_success", escapeHtml(wh.name)));
        })
        .error(function(data) {
            btn.disabled = false;
            var wh = webhooks.find(x => x.id == whId);
            if (!wh) {
                return
            }
            errorFlash(T("webhooks.ping_failed", escapeHtml(wh.name), escapeHtml(data.responseJSON.message)))
        });
};

$(document).ready(function() {
    load();
    $("#modal").on("hide.bs.modal", function() {
        dismiss();
    });
    $("#new_button").on("click", function() {
        editWebhook(-1);
    });
    $("#webhookTable").on("click", ".edit_button", function(e) {
        editWebhook($(this).attr("data-webhook-id"));
    });
    $("#webhookTable").on("click", ".delete_button", function(e) {
        deleteWebhook($(this).attr("data-webhook-id"));
    });
    $("#webhookTable").on("click", ".ping_button", function(e) {
        pingUrl(e.currentTarget, e.currentTarget.dataset.webhookId);
    });
});
