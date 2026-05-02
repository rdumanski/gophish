// Ambient declarations for window globals supplied at runtime by
// either base.html's inline <script> (server-rendered values) or
// vendor.min.js (concatenated third-party libraries). The Phase 4c
// TypeScript pass treats these as `any` — narrowing to real types is
// a follow-up once strict mode is on per file.

declare const user: { api_key: string; username: string }
declare const csrf_token: string

// Window interface extensions for the same server-rendered globals
// when accessed via `window.user` / `window.csrf_token`.
interface Window {
    user: { api_key: string; username: string }
    csrf_token: string
}

// jQuery plugin augmentations for the vendor libraries we ship in
// vendor.min.js. Loose typing — Phase 4c keeps them as `any` to
// unblock typecheck; strict per-plugin types are a follow-up.
interface JQuery {
    modal(action?: any, ...args: any[]): JQuery
    tooltip(action?: any, ...args: any[]): JQuery
    dataTable(opts?: any): any
    DataTable(opts?: any): any
    select2(opts?: any): JQuery
    fileupload(opts?: any): JQuery
}

interface JQueryStatic {
    fn: {
        select2: { defaults: { set(key: string, value: any): void } } & any
        dataTable: { moment(format: string): void } & any
    } & any
}

// jQuery deferred shims that upstream Gophish code uses everywhere:
// `.success()` / `.error()` were removed in jQuery 3 but the vendor
// bundle still ships jQuery 2.x where they work. Declare them as
// fluent shorthands returning the same jqXHR (or Deferred) so the
// long .success(...).error(...) chains type-check.
declare namespace JQuery {
    interface jqXHR<T = any> {
        success(callback: (data: any) => void): this
        error(callback: (data: any) => void): this
    }
    interface Deferred<TR, TJ = any, TN = any> {
        success(callback: (data: any) => void): this
        error(callback: (data: any) => void): this
    }
    interface PromiseBase<TR, TJ, TN, UR, UJ, UN, VR, VJ, VN, SR, SJ, SN> {
        success(callback: (data: any) => void): this
        error(callback: (data: any) => void): this
    }
}

// SweetAlert2 — used directly as `Swal.fire(...)` in groups, webhooks,
// landing_pages, sending_profiles, templates. The vendor build is the
// pre-bundled UMD and exposes a global Swal.
declare const Swal: any

// Highcharts — used in dashboard.ts.
declare const Highcharts: any

// CKEditor — used in autocomplete.ts.
declare const CKEDITOR: any

// d3 / topojson / Datamap — vendor-only, accessed via window globals.
declare const d3: any
declare const topojson: any
declare const Datamap: any

// PapaParse — used in groups.ts.
declare const Papa: any

// moment — vendor-bundled. jQuery DataTables.moment plugin extends
// $.fn.dataTable. We declare the namespace augmentation loosely.
declare const moment: any

// UAParser — used in campaign_results.ts.
declare const UAParser: any
