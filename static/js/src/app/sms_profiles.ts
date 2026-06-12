import { api, errorFlash, escapeHtml, modalError, successFlash } from './common'

// Phase 8b: SMS Sending Profile management. Mirrors sending_profiles.ts
// (the SMTP analog) but without headers / test-email. Test-send is a
// nice-to-have deferred to Phase 8c — pre-flight needs Twilio's
// validate-credentials endpoint, which requires the same dispatcher
// shape we're about to ship for the live path.

var profiles: any[] = []

function save(idx) {
    var profile: any = {}
    profile.name = $("#name").val()
    profile.provider = $("#provider").val()
    profile.account_sid = $("#account_sid").val()
    profile.auth_token = $("#auth_token").val()
    profile.from_number = $("#from_number").val()
    profile.messaging_service_sid = $("#messaging_service_sid").val()
    if (idx != -1) {
        profile.id = profiles[idx].id
        api.SMSProfileId.put(profile)
            .success(function () {
                successFlash("SMS profile edited successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        api.SMSProfile.post(profile)
            .success(function () {
                successFlash("SMS profile added successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#modal\\.flashes").empty()
    $("#name").val("")
    $("#provider").val("twilio")
    $("#account_sid").val("")
    $("#auth_token").val("")
    $("#from_number").val("")
    $("#messaging_service_sid").val("")
    $("#modal").modal('hide')
}

var deleteProfile = function (idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the SMS profile. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(profiles[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise<void>(function (resolve, reject) {
                api.SMSProfileId.delete(profiles[idx].id)
                    .success(function () { resolve() })
                    .error(function (data) { reject(data.responseJSON.message) })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('SMS Profile Deleted!', 'This SMS profile has been deleted!', 'success')
        }
        $('button:contains("OK")').on('click', function () { location.reload() })
    })
}

function edit(idx) {
    $("#modalSubmit").unbind('click').click(function () { save(idx) })
    var profile: any = {}
    if (idx != -1) {
        $("#profileModalLabel").text("Edit SMS Profile")
        profile = profiles[idx]
        $("#name").val(profile.name)
        $("#provider").val(profile.provider || "twilio")
        $("#account_sid").val(profile.account_sid)
        $("#auth_token").val(profile.auth_token || "")
        $("#from_number").val(profile.from_number)
        $("#messaging_service_sid").val(profile.messaging_service_sid || "")
    } else {
        $("#profileModalLabel").text("New SMS Profile")
    }
}

function copy(idx) {
    $("#modalSubmit").unbind('click').click(function () { save(-1) })
    var profile: any = profiles[idx]
    $("#name").val("Copy of " + profile.name)
    $("#provider").val(profile.provider || "twilio")
    $("#account_sid").val(profile.account_sid)
    $("#auth_token").val(profile.auth_token || "")
    $("#from_number").val(profile.from_number)
    $("#messaging_service_sid").val(profile.messaging_service_sid || "")
}

function load() {
    $("#profileTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.SMSProfile.get()
        .success(function (ss) {
            profiles = ss
            $("#loading").hide()
            if (profiles.length > 0) {
                $("#profileTable").show()
                const profileTable = $("#profileTable").DataTable({
                    destroy: true,
                    columnDefs: [{ orderable: false, targets: "no-sort" }]
                })
                profileTable.clear()
                const rows: any[] = []
                $.each(profiles, function (i, profile) {
                    rows.push([
                        escapeHtml(profile.name),
                        escapeHtml(profile.provider || "twilio"),
                        escapeHtml(profile.from_number || profile.messaging_service_sid),
                        moment(profile.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Profile' onclick='edit(" + i + ")'>\
                        <i class='fa fa-pencil'></i>\
                        </button></span>\
                        <span data-toggle='modal' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Profile' onclick='copy(" + i + ")'>\
                        <i class='fa fa-copy'></i>\
                        </button></span>\
                        <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Profile' onclick='deleteProfile(" + i + ")'>\
                        <i class='fa fa-trash-o'></i>\
                        </button></div>"
                    ])
                })
                profileTable.rows.add(rows).draw()
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching SMS profiles")
        })
}

$(document).ready(function () {
    $('#modal').on('hidden.bs.modal', function () { dismiss() })
    load()
})

// Inline-onclick references from templates/sms_profiles.html plus the
// JS-string-built buttons in the table rows above.
Object.assign(window, { copy, deleteProfile, dismiss, edit })
