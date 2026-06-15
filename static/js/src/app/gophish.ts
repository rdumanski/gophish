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
    T,
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

// Global localization of library chrome, set before any table/chart initializes
// (this module loads before page bundles). Driven by the per-user language the
// server injects as window.user.language + the window.i18n catalog (via T).
const uiLang = ((window as any).user && (window as any).user.language) || 'en'

// DataTables UI strings (Search, paginate, "Showing N of M", "Show N entries")
// on every table. The _MENU_/_START_/_END_/_TOTAL_/_MAX_ tokens are DataTables
// placeholders and are kept verbatim in the catalog values.
if ($.fn && ($.fn as any).dataTable) {
    $.extend(true, ($.fn as any).dataTable.defaults, {
        language: {
            search: T('dt.search'),
            lengthMenu: T('dt.length_menu'),
            info: T('dt.info'),
            infoEmpty: T('dt.info_empty'),
            infoFiltered: T('dt.info_filtered'),
            zeroRecords: T('dt.zero_records'),
            emptyTable: T('dt.empty_table'),
            paginate: {
                first: T('dt.first'),
                previous: T('dt.previous'),
                next: T('dt.next'),
                last: T('dt.last'),
            },
        },
    })
}

// Polish month/weekday names for dates (moment) and charts (Highcharts). moment
// ships no pl locale in our bundle, so define a minimal one inline.
if (uiLang === 'pl') {
    const months = ['styczeń', 'luty', 'marzec', 'kwiecień', 'maj', 'czerwiec', 'lipiec', 'sierpień', 'wrzesień', 'październik', 'listopad', 'grudzień']
    const monthsShort = ['sty', 'lut', 'mar', 'kwi', 'maj', 'cze', 'lip', 'sie', 'wrz', 'paź', 'lis', 'gru']
    const weekdays = ['niedziela', 'poniedziałek', 'wtorek', 'środa', 'czwartek', 'piątek', 'sobota']
    if (typeof moment !== 'undefined') {
        moment.defineLocale('pl', {
            months: months,
            monthsShort: monthsShort,
            weekdays: weekdays,
            weekdaysShort: ['niedz.', 'pon.', 'wt.', 'śr.', 'czw.', 'pt.', 'sob.'],
            weekdaysMin: ['Nd', 'Pn', 'Wt', 'Śr', 'Cz', 'Pt', 'So'],
            ordinal: (n) => n + '.',
            meridiem: () => '',
        })
        moment.locale('pl')
    }
    if (typeof Highcharts !== 'undefined') {
        Highcharts.setOptions({ lang: { months: months, shortMonths: monthsShort, weekdays: weekdays } })
    }
}

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
