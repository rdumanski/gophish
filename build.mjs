// Phase 4a build: replaces gulpfile.js + webpack.config.js with a thin
// esbuild driver. Same outputs at the same paths the HTML templates
// already reference, so this is a tooling-only swap. Source files stay
// as-is (ES5, jQuery globals); the ESM/TS migration is Phase 4b/4c.

import { build } from 'esbuild'
import { mkdir, readFile, writeFile } from 'node:fs/promises'
import { dirname, join } from 'node:path'

const jsSrc = 'static/js/src'
const cssSrc = 'static/css'
const jsDist = 'static/js/dist'
const cssDist = 'static/css/dist'

const appFiles = [
    'autocomplete.ts',
    'campaign_results.ts',
    'campaigns.ts',
    'dashboard.ts',
    'groups.ts',
    'landing_pages.ts',
    'sending_profiles.ts',
    'settings.ts',
    'templates.ts',
    'gophish.ts',
    'users.ts',
    'webhooks.ts',
    'passwords.ts',
]

const vendorOrder = [
    'jquery.js',
    'bootstrap.min.js',
    'moment.min.js',
    'papaparse.min.js',
    'd3.min.js',
    'topojson.min.js',
    'datamaps.min.js',
    'jquery.dataTables.min.js',
    'dataTables.bootstrap.js',
    'datetime-moment.js',
    'jquery.ui.widget.js',
    'jquery.fileupload.js',
    'jquery.iframe-transport.js',
    'sweetalert2.min.js',
    'bootstrap-datetime.js',
    'select2.min.js',
    'core.min.js',
    'highcharts.js',
    'ua-parser.min.js',
]

const cssOrder = [
    'bootstrap.min.css',
    'main.css',
    'dashboard.css',
    'flat-ui.css',
    'dataTables.bootstrap.css',
    'font-awesome.min.css',
    'chartist.min.css',
    'bootstrap-datetime.css',
    'checkbox.css',
    'sweetalert2.min.css',
    'select2.min.css',
    'select2-bootstrap.min.css',
]

async function ensureDir(p) {
    await mkdir(p, { recursive: true })
}

// autocomplete.ts is the only app file that's still a non-module
// script: it declares top-level `var TEMPLATE_TAGS = [...]` that an
// inline page script reads as a global. Bundling it as IIFE would
// hide that global. Everything else is an ESM module and is bundled
// + IIFE-wrapped; functions called from inline HTML onclick handlers
// are re-exported onto window inside each module.
//
// Phase 4c: source files are now TypeScript. esbuild handles .ts
// natively (strips types and bundles); standalone type checking is
// `npm run typecheck` (tsc --noEmit) and runs separately.
const plainApps = ['autocomplete.ts']
const moduleApps = appFiles.filter((f) => !plainApps.includes(f))

async function buildApp() {
    await ensureDir(`${jsDist}/app`)

    // Plain (non-module) files: minify in place, no IIFE wrap, top-level
    // declarations remain page-globals.
    await build({
        entryPoints: plainApps.map((f) => `${jsSrc}/app/${f}`),
        outdir: `${jsDist}/app`,
        bundle: false,
        minify: true,
        target: 'es2017',
        platform: 'browser',
        logLevel: 'info',
        outExtension: { '.js': '.min.js' },
        loader: { '.ts': 'ts' },
    })

    // ESM files: resolve imports (mostly ./common, plus zxcvbn for
    // passwords.js), minify, IIFE-wrap. Each entry produces a
    // self-contained <name>.min.js that the corresponding HTML
    // template loads via a single <script src="..."> tag, exactly as
    // before the migration.
    await build({
        entryPoints: moduleApps.map((f) => `${jsSrc}/app/${f}`),
        outdir: `${jsDist}/app`,
        bundle: true,
        minify: true,
        target: 'es2017',
        platform: 'browser',
        format: 'iife',
        logLevel: 'info',
        outExtension: { '.js': '.min.js' },
        loader: { '.ts': 'ts' },
    })
}

async function bundleVendor() {
    await ensureDir(jsDist)
    const parts = []
    for (const f of vendorOrder) {
        const src = await readFile(`${jsSrc}/vendor/${f}`, 'utf8')
        parts.push(`/* ${f} */\n${src}`)
    }
    // Plain concatenation — vendor scripts are nearly all already
    // minified upstream, and they deliberately leak window globals
    // (jQuery, $, Highcharts, UAParser, ...) via UMD checks against
    // typeof module/exports. Running them through esbuild's minifier
    // in an ESM-typed package would have it rewrite the unresolved
    // `module`/`exports` references and corrupt those UMD branches.
    await writeFile(`${jsDist}/vendor.min.js`, parts.join('\n;\n'))
}

async function bundleCss() {
    await ensureDir(cssDist)
    const parts = []
    for (const f of cssOrder) {
        const src = await readFile(`${cssSrc}/${f}`, 'utf8')
        parts.push(`/* ${f} */\n${src}`)
    }
    // The original gulp build ran clean-css with `compatibilty: 'ie9'`
    // (sic, upstream typo). Most files in cssOrder are pre-minified;
    // running another minifier over the concat saves only a handful of
    // bytes and risks breaking selectors that the IE9 quirks-mode
    // compatibility layer protected. Plain concat preserves original
    // semantics exactly.
    await writeFile(`${cssDist}/gophish.css`, parts.join('\n'))
}

async function main() {
    const t0 = Date.now()
    await Promise.all([buildApp(), bundleVendor(), bundleCss()])
    const ms = Date.now() - t0
    console.log(`build complete in ${ms} ms`)
}

main().catch((err) => {
    console.error(err)
    process.exit(1)
})
