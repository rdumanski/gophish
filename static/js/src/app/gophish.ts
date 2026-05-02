// Loaded on every authenticated page from base.html, before any
// page-specific bundle. Two responsibilities:
//   1. Re-export the helpers from common.mjs onto window so that
//      anything that still relies on globals (inline templates,
//      dev console, future code) keeps working.
//   2. Wire up the document.ready setup that's intentionally global:
//      sidebar nav highlighting, datatables moment integration,
//      tooltip activation.
import {
    api,
    capitalize,
    errorFlash,
    errorFlashFade,
    escapeHtml,
    modalError,
    query,
    successFlash,
    successFlashFade,
    unescapeHtml,
} from './common'

Object.assign(window, {
    api,
    capitalize,
    errorFlash,
    errorFlashFade,
    escapeHtml,
    modalError,
    query,
    successFlash,
    successFlashFade,
    unescapeHtml,
})

$(document).ready(function () {
    // Sidebar nav: mark the link matching the current path as active.
    const path = location.pathname
    $('.nav-sidebar li').each(function () {
        const $this = $(this)
        if ($this.find('a').attr('href') === path) {
            $this.addClass('active')
        }
    })
    $.fn.dataTable.moment('MMMM Do YYYY, h:mm:ss a')
    $('[data-toggle="tooltip"]').tooltip()
})
