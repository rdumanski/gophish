import { api, errorFlash, escapeHtml, modalError, successFlash, T } from './common'

let quizzes = []
let gidCounter = 0

const addOption = ($container, gid, text = "", isCorrect = false) => {
    const $row = $(`
      <div class="option-row" style="margin-bottom:6px;">
        <div class="input-group">
          <span class="input-group-addon" title="${T("quizzes.mark_correct")}">
            <input type="radio" name="correct-${gid}" class="o-correct" />
          </span>
          <input type="text" class="form-control o-text" placeholder="${T("quizzes.answer_option")}" />
          <span class="input-group-btn">
            <button type="button" class="btn btn-default remove-option"><i class="fa fa-times"></i></button>
          </span>
        </div>
      </div>`)
    $row.find(".o-text").val(text)
    if (isCorrect) {
        $row.find(".o-correct").prop("checked", true)
    }
    $container.append($row)
}

const addQuestion = (prompt = "", options: any[] = []) => {
    const gid = ++gidCounter
    const $block = $(`
      <div class="question-block panel panel-default" data-gid="${gid}" style="padding:12px;margin-bottom:12px;background:#fafafa;">
        <div class="form-group" style="margin-bottom:8px;">
          <div class="input-group">
            <input type="text" class="form-control q-prompt" placeholder="${T("quizzes.question_prompt")}" />
            <span class="input-group-btn">
              <button type="button" class="btn btn-danger remove-question" title="${T("quizzes.remove_question")}"><i class="fa fa-trash-o"></i></button>
            </span>
          </div>
        </div>
        <div class="options"></div>
        <button type="button" class="btn btn-xs btn-default add-option"><i class="fa fa-plus"></i> ${T("quizzes.add_option")}</button>
      </div>`)
    $block.find(".q-prompt").val(prompt)
    const $opts = $block.find(".options")
    $("#questions").append($block)
    if (options.length) {
        options.forEach((o) => addOption($opts, gid, o.text, o.is_correct))
    } else {
        addOption($opts, gid, "", false)
        addOption($opts, gid, "", false)
    }
}

const serializeQuestions = () => {
    const questions: any[] = []
    $("#questions .question-block").each(function () {
        const prompt = $(this).find(".q-prompt").val()
        const options: any[] = []
        $(this).find(".option-row").each(function () {
            options.push({
                text: $(this).find(".o-text").val(),
                is_correct: $(this).find(".o-correct").is(":checked"),
            })
        })
        questions.push({ prompt, options })
    })
    return questions
}

const setupModuleOptions = (selected?) => {
    return api.training_modules.get().success((modules) => {
        const $sel = $("#module").empty()
        if (!modules || modules.length === 0) {
            modalError(T("quizzes.no_modules"))
            return
        }
        $.each(modules, (i, m) => {
            $sel.append(`<option value="${m.id}">${escapeHtml(m.name)}</option>`)
        })
        if (selected) {
            $sel.val(selected)
        }
    })
}

const dismiss = () => {
    $("#title").val("")
    $("#pass_threshold").val("70")
    $("#questions").empty()
    $("#flashes").empty()
}

const save = (id) => {
    const quiz: any = {
        title: $("#title").val(),
        module_id: parseInt($("#module").val() as string),
        pass_threshold: parseInt($("#pass_threshold").val() as string),
        questions: serializeQuestions(),
    }
    const done = (msgKey) => {
        dismiss()
        $("#modal").modal("hide")
        load()
        successFlash(T(msgKey, escapeHtml(quiz.title)))
    }
    if (id != -1) {
        quiz.id = parseInt(id)
        api.quizId.put(quiz).success(() => done("quizzes.updated")).error((d) => modalError(d.responseJSON.message))
    } else {
        api.quizzes.post(quiz).success(() => done("quizzes.created")).error((d) => modalError(d.responseJSON.message))
    }
}

const editQuiz = (id) => {
    $("#modalSubmit").unbind("click").click(() => save(id))
    $("#questions").empty()
    if (id === -1) {
        $("#quizModalLabel").text(T("quizzes.new_title"))
        setupModuleOptions().success(() => { /* options ready */ })
        addQuestion()
        return
    }
    $("#quizModalLabel").text(T("quizzes.edit_title"))
    api.quizId.get(id).success((q) => {
        $("#title").val(q.title)
        $("#pass_threshold").val(q.pass_threshold)
        setupModuleOptions(q.module_id)
        ;(q.questions || []).forEach((question) => addQuestion(question.prompt, question.options || []))
        if (!q.questions || q.questions.length === 0) {
            addQuestion()
        }
    }).error(() => errorFlash(T("quizzes.fetch_one_error")))
}

const load = () => {
    $("#quizTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.quizzes.get()
        .success((qs) => {
            quizzes = qs
            $("#loading").hide()
            if (!quizzes || quizzes.length === 0) {
                $("#emptyMessage").show()
                return
            }
            $("#quizTable").show()
            const table = $("#quizTable").DataTable({
                destroy: true,
                columnDefs: [{ orderable: false, targets: "no-sort" }]
            })
            table.clear()
            $.each(quizzes, (i, q) => {
                table.row.add([
                    escapeHtml(q.title),
                    q.question_count,
                    q.pass_threshold + "%",
                    moment(q.modified_date).format("MMM Do YYYY"),
                    `<div class="pull-right" style="white-space:nowrap">
                        <button class="btn btn-primary edit_button" data-toggle="modal" data-backdrop="static" data-target="#modal" data-quiz-id="${q.id}">
                          <i class="fa fa-pencil"></i>
                        </button>
                        <button class="btn btn-danger delete_button" data-quiz-id="${q.id}">
                          <i class="fa fa-trash-o"></i>
                        </button>
                     </div>`
                ]).draw(false)
            })
        })
        .error(() => {
            $("#loading").hide()
            errorFlash(T("quizzes.fetch_error"))
        })
}

const deleteQuiz = (id) => {
    const q = quizzes.find(x => x.id == id)
    if (!q) {
        return
    }
    Swal.fire({
        title: T("quizzes.delete_title"),
        text: T("quizzes.delete_confirm", escapeHtml(q.title)),
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: T("common.delete"),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: () => new Promise<void>((resolve, reject) => {
            api.quizId.delete(id).success(() => resolve()).error((d) => reject(d.responseJSON.message))
        }).catch(error => { Swal.showValidationMessage(error) })
    }).then((result) => {
        if (result.value) {
            Swal.fire(T("quizzes.deleted_title"), T("quizzes.deleted_text"), "success")
        }
        $("button:contains('OK')").on("click", () => location.reload())
    })
}

$(document).ready(function () {
    load()
    $("#modal").on("hide.bs.modal", dismiss)
    $("#new_button").on("click", () => editQuiz(-1))
    $("#add_question").on("click", () => addQuestion())
    $("#questions").on("click", ".add-option", function () {
        const $block = $(this).closest(".question-block")
        addOption($block.find(".options"), $block.attr("data-gid"))
    })
    $("#questions").on("click", ".remove-option", function () {
        $(this).closest(".option-row").remove()
    })
    $("#questions").on("click", ".remove-question", function () {
        $(this).closest(".question-block").remove()
    })
    $("#quizTable").on("click", ".edit_button", function () {
        editQuiz($(this).attr("data-quiz-id"))
    })
    $("#quizTable").on("click", ".delete_button", function () {
        deleteQuiz($(this).attr("data-quiz-id"))
    })
})
