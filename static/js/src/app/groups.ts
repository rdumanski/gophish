import { api, errorFlash, escapeHtml, modalError, successFlash, T, unescapeHtml } from './common'

var groups = []

// Save attempts to POST or PUT to /groups/
function save(id) {
    var targets = []
    $.each($("#targetsTable").DataTable().rows().data(), function (i, target) {
        targets.push({
            first_name: unescapeHtml(target[0]),
            last_name: unescapeHtml(target[1]),
            email: unescapeHtml(target[2]),
            position: unescapeHtml(target[3]),
            department: unescapeHtml(target[4]),
            sub_department: unescapeHtml(target[5]),
            wydzial: unescapeHtml(target[6]),
            position_level: unescapeHtml(target[7])
        })
    })
    var group: any = {
        name: $("#name").val(),
        targets: targets
    }
    // Submit the group
    if (id != -1) {
        // If we're just editing an existing group,
        // we need to PUT /groups/:id
        group.id = id
        api.groupId.put(group)
            .success(function (data) {
                successFlash(T("groups.updated"))
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Else, if this is a new group, POST it
        // to /groups
        api.groups.post(group)
            .success(function (data) {
                successFlash(T("groups.added"))
                load()
                dismiss()
                $("#modal").modal('hide')
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#targetsTable").dataTable().DataTable().clear().draw()
    $("#name").val("")
    $("#modal\\.flashes").empty()
}

// Holds the jQuery DataTable instance for the targets table inside
// the edit-group modal. Set by edit(); read by load() and the file
// upload handler when batch-adding rows.
let targets: any
function edit(id) {
    targets = $("#targetsTable").dataTable({
        destroy: true, // Destroy any other instantiated table - http://datatables.net/manual/tech-notes/3#destroy
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }]
    })
    $("#modalSubmit").unbind('click').click(function () {
        save(id)
    })
    if (id == -1) {
        $("#groupModalLabel").text(T("groups.new_title"));
        var group = {}
    } else {
        $("#groupModalLabel").text(T("groups.edit_title"));
        api.groupId.get(id)
            .success(function (group) {
                $("#name").val(group.name)
                const targetRows: any[] = []
                $.each(group.targets, function (i, record) {
                  targetRows.push([
                      escapeHtml(record.first_name),
                      escapeHtml(record.last_name),
                      escapeHtml(record.email),
                      escapeHtml(record.position),
                      escapeHtml(record.department),
                      escapeHtml(record.sub_department),
                      escapeHtml(record.wydzial),
                      escapeHtml(record.position_level),
                      '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
                  ])
                });
                targets.DataTable().rows.add(targetRows).draw()
            })
            .error(function () {
                errorFlash(T("groups.fetch_one_error"))
            })
    }
    // Handle file uploads
    $("#csvupload").fileupload({
        url: "/api/import/group",
        dataType: "json",
        beforeSend: function (xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + user.api_key);
        },
        add: function (e, data) {
            $("#modal\\.flashes").empty()
            var acceptFileTypes = /(csv|txt)$/i;
            var filename = data.originalFiles[0]['name']
            if (filename && !acceptFileTypes.test(filename.split(".").pop())) {
                modalError(T("groups.bad_file_ext"))
                return false;
            }
            data.submit();
        },
        done: function (e, data) {
            $.each(data.result, function (i, record) {
                addTarget(
                    record.first_name,
                    record.last_name,
                    record.email,
                    record.position,
                    record.department,
                    record.sub_department,
                    record.wydzial,
                    record.position_level);
            });
            targets.DataTable().draw();
        }
    })
}

var downloadCSVTemplate = function () {
    var csvScope = [{
        'First Name': 'Example',
        'Last Name': 'User',
        'Email': 'foobar@example.com',
        'Position': 'Systems Administrator',
        'Department': 'Departament Eksploatacji',
        'Sub-Department': 'Biuro Ruchu',
        'Wydzial': 'Wydzial Nadzoru',
        'Position Level': 'Specjalista'
    }]
    var filename = 'group_template.csv'
    var csvString = Papa.unparse(csvScope, {})
    var csvData = new Blob([csvString], {
        type: 'text/csv;charset=utf-8;'
    });
    // navigator.msSaveBlob is an IE-only API not in lib.dom — cast to any.
    if ((navigator as any).msSaveBlob) {
        (navigator as any).msSaveBlob(csvData, filename);
    } else {
        var csvURL = window.URL.createObjectURL(csvData);
        var dlLink = document.createElement('a');
        dlLink.href = csvURL;
        dlLink.setAttribute('download', filename)
        document.body.appendChild(dlLink)
        dlLink.click();
        document.body.removeChild(dlLink)
    }
}


var deleteGroup = function (id) {
    var group = groups.find(function (x) {
        return x.id === id
    })
    if (!group) {
        return
    }
    Swal.fire({
        title: T("groups.delete_title"),
        text: T("groups.delete_confirm"),
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: T("common.delete_named", escapeHtml(group.name)),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise<void>(function (resolve, reject) {
                api.groupId.delete(id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                T("groups.deleted_title"),
                T("groups.deleted_text"),
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function addTarget(firstNameInput, lastNameInput, emailInput, positionInput,
    departmentInput = '', subDepartmentInput = '', wydzialInput = '', positionLevelInput = '') {
    // Create new data row.
    var email = escapeHtml(emailInput).toLowerCase();
    var newRow = [
        escapeHtml(firstNameInput),
        escapeHtml(lastNameInput),
        email,
        escapeHtml(positionInput),
        escapeHtml(departmentInput),
        escapeHtml(subDepartmentInput),
        escapeHtml(wydzialInput),
        escapeHtml(positionLevelInput),
        '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
    ];

    // Check table to see if email already exists.
    var targetsTable = targets.DataTable();
    var existingRowIndex = targetsTable
        .column(2, {
            order: "index"
        }) // Email column has index of 2
        .data()
        .indexOf(email);
    // Update or add new row as necessary.
    if (existingRowIndex >= 0) {
        targetsTable
            .row(existingRowIndex, {
                order: "index"
            })
            .data(newRow);
    } else {
        targetsTable.row.add(newRow);
    }
}

function load() {
    $("#groupTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.groups.summary()
        .success(function (response) {
            $("#loading").hide()
            if (response.total > 0) {
                groups = response.groups
                $("#emptyMessage").hide()
                $("#groupTable").show()
                var groupTable = $("#groupTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                groupTable.clear();
                const groupRows: any[] = []
                $.each(groups, function (i, group) {
                    // Auto-generated org-unit groups are rebuilt by the server,
                    // so they're badged and not hand-editable/deletable.
                    const name = group.is_auto
                        ? escapeHtml(group.name) + " <span class='label label-info'>" + T("groups.auto_badge") + "</span>"
                        : escapeHtml(group.name)
                    const actions = group.is_auto
                        ? ""
                        : "<div class='pull-right'><button class='btn btn-primary' data-toggle='modal' data-backdrop='static' data-target='#modal' onclick='edit(" + group.id + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button>\
                    <button class='btn btn-danger' onclick='deleteGroup(" + group.id + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    groupRows.push([
                        name,
                        escapeHtml(group.num_targets),
                        moment(group.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        actions
                    ])
                })
                groupTable.rows.add(groupRows).draw()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            errorFlash(T("groups.fetch_error"))
        })
}

$(document).ready(function () {
    load()
    // Setup the event listeners
    // Handle manual additions
    $("#targetForm").submit(function () {
        // Validate the form data
        var targetForm = document.getElementById("targetForm") as HTMLFormElement
        if (!targetForm.checkValidity()) {
            targetForm.reportValidity()
            return
        }
        addTarget(
            $("#firstName").val(),
            $("#lastName").val(),
            $("#email").val(),
            $("#position").val());
        targets.DataTable().draw();

        // Reset user input.
        $("#targetForm>div>input").val('');
        $("#firstName").focus();
        return false;
    });
    // Handle Deletion
    $("#targetsTable").on("click", "span>i.fa-trash-o", function () {
        targets.DataTable()
            .row($(this).parents('tr'))
            .remove()
            .draw();
    });
    $("#modal").on("hide.bs.modal", function () {
        dismiss();
    });
    $("#csv-template").click(downloadCSVTemplate)
});

// Inline-onclick references from templates/groups.html plus
// JS-string-built buttons in this file's load handler.
// Rebuild the system-managed per-org-unit groups from current recipients.
function regenerateOrgGroups() {
    api.org_groups.regenerate()
        .success(function (r) {
            successFlash(r.message)
            load()
        })
        .error(function (e) {
            errorFlash(e.responseJSON ? e.responseJSON.message : T("groups.fetch_error"))
        })
}

Object.assign(window, { deleteGroup, dismiss, edit, regenerateOrgGroups })
