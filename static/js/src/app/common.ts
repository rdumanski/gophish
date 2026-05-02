// Shared helpers used by every page-specific app bundle. Pure ESM —
// no document.ready, no window attachments, no DOM setup. The
// loaded-on-every-page side-effects (nav highlighting, datatables
// moment plugin, tooltips, window.api compatibility shim) live in
// gophish.js, which is still loaded as a standalone script tag from
// base.html and runs before any page-specific bundle.
//
// Vendor globals ($, jQuery, ...) come from vendor.min.js loaded
// separately by base.html. They are referenced as free identifiers
// here and at runtime resolve to window.$ / window.jQuery.
//
// `user` and `csrf_token` are server-rendered by base.html into an
// inline <script> right above gophish.min.js, so they're also
// available as window globals when these modules execute.

export function errorFlash(message) {
    $('#flashes').empty()
    $('#flashes').append('<div style="text-align:center" class="alert alert-danger">' +
        '<i class="fa fa-exclamation-circle"></i> ' + message + '</div>')
}

export function successFlash(message) {
    $('#flashes').empty()
    $('#flashes').append('<div style="text-align:center" class="alert alert-success">' +
        '<i class="fa fa-check-circle"></i> ' + message + '</div>')
}

export function errorFlashFade(message, fade) {
    errorFlash(message)
    setTimeout(() => $('#flashes').empty(), fade * 1000)
}

export function successFlashFade(message, fade) {
    successFlash(message)
    setTimeout(() => $('#flashes').empty(), fade * 1000)
}

export function modalError(message) {
    $('#modal\\.flashes').empty().append('<div style="text-align:center" class="alert alert-danger">' +
        '<i class="fa fa-exclamation-circle"></i> ' + message + '</div>')
}

export function query(endpoint, method, data, async) {
    return $.ajax({
        url: '/api' + endpoint,
        async: async,
        method: method,
        data: JSON.stringify(data),
        dataType: 'json',
        contentType: 'application/json',
        beforeSend: function (xhr) {
            xhr.setRequestHeader('Authorization', 'Bearer ' + window.user.api_key)
        },
    })
}

export function escapeHtml(text) {
    return $('<div/>').text(text).html()
}

export function unescapeHtml(html) {
    return $('<div/>').html(html).text()
}

export function capitalize(string) {
    return string.charAt(0).toUpperCase() + string.slice(1)
}

// API endpoint definitions. Each section maps to a Go controller in
// controllers/api/. Keep this in sync with route registrations there.
export const api = {
    campaigns: {
        get: () => query('/campaigns/', 'GET', {}, false),
        post: (data) => query('/campaigns/', 'POST', data, false),
        summary: () => query('/campaigns/summary', 'GET', {}, false),
    },
    campaignId: {
        get: (id) => query('/campaigns/' + id, 'GET', {}, true),
        delete: (id) => query('/campaigns/' + id, 'DELETE', {}, false),
        results: (id) => query('/campaigns/' + id + '/results', 'GET', {}, true),
        complete: (id) => query('/campaigns/' + id + '/complete', 'GET', {}, true),
        summary: (id) => query('/campaigns/' + id + '/summary', 'GET', {}, true),
    },
    groups: {
        get: () => query('/groups/', 'GET', {}, false),
        post: (group) => query('/groups/', 'POST', group, false),
        summary: () => query('/groups/summary', 'GET', {}, true),
    },
    groupId: {
        get: (id) => query('/groups/' + id, 'GET', {}, false),
        put: (group) => query('/groups/' + group.id, 'PUT', group, false),
        delete: (id) => query('/groups/' + id, 'DELETE', {}, false),
    },
    templates: {
        get: () => query('/templates/', 'GET', {}, false),
        post: (template) => query('/templates/', 'POST', template, false),
    },
    templateId: {
        get: (id) => query('/templates/' + id, 'GET', {}, false),
        put: (template) => query('/templates/' + template.id, 'PUT', template, false),
        delete: (id) => query('/templates/' + id, 'DELETE', {}, false),
    },
    pages: {
        get: () => query('/pages/', 'GET', {}, false),
        post: (page) => query('/pages/', 'POST', page, false),
    },
    pageId: {
        get: (id) => query('/pages/' + id, 'GET', {}, false),
        put: (page) => query('/pages/' + page.id, 'PUT', page, false),
        delete: (id) => query('/pages/' + id, 'DELETE', {}, false),
    },
    SMTP: {
        get: () => query('/smtp/', 'GET', {}, false),
        post: (smtp) => query('/smtp/', 'POST', smtp, false),
    },
    SMTPId: {
        get: (id) => query('/smtp/' + id, 'GET', {}, false),
        put: (smtp) => query('/smtp/' + smtp.id, 'PUT', smtp, false),
        delete: (id) => query('/smtp/' + id, 'DELETE', {}, false),
    },
    IMAP: {
        get: () => query('/imap/', 'GET', {}, false),
        post: (e) => query('/imap/', 'POST', e, false),
        validate: (e) => query('/imap/validate', 'POST', e, true),
    },
    users: {
        get: () => query('/users/', 'GET', {}, true),
        post: (user) => query('/users/', 'POST', user, true),
    },
    userId: {
        get: (id) => query('/users/' + id, 'GET', {}, true),
        put: (user) => query('/users/' + user.id, 'PUT', user, true),
        delete: (id) => query('/users/' + id, 'DELETE', {}, true),
    },
    webhooks: {
        get: () => query('/webhooks/', 'GET', {}, false),
        post: (webhook) => query('/webhooks/', 'POST', webhook, false),
    },
    webhookId: {
        get: (id) => query('/webhooks/' + id, 'GET', {}, false),
        put: (webhook) => query('/webhooks/' + webhook.id, 'PUT', webhook, true),
        delete: (id) => query('/webhooks/' + id, 'DELETE', {}, false),
        ping: (id) => query('/webhooks/' + id + '/validate', 'POST', {}, true),
    },
    import_email: (req) => query('/import/email', 'POST', req, false),
    clone_site: (req) => query('/import/site', 'POST', req, false),
    send_test_email: (req) => query('/util/send_test_email', 'POST', req, true),
    reset: () => query('/reset', 'POST', {}, true),
}
