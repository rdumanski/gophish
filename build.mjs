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
    'autocomplete.js',
    'campaign_results.js',
    'campaigns.js',
    'dashboard.js',
    'groups.js',
    'landing_pages.js',
    'sending_profiles.js',
    'settings.js',
    'templates.js',
    'gophish.js',
    'users.js',
    'webhooks.js',
    'passwords.js',
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

// Files that use ES6 `import` and need bundling (zxcvbn etc. resolved
// from node_modules). Historically handled by webpack.config.js.
const bundledApps = ['passwords.js']
const plainApps = appFiles.filter((f) => !bundledApps.includes(f))

async function buildApp() {
    await ensureDir(`${jsDist}/app`)

    // Plain files: minify only, no bundling, no IIFE wrapper. Top-level
    // `var` declarations remain page-globals exactly like the gulp+uglify
    // pipeline produced. IIFE-wrapping these would dead-code-eliminate
    // unreferenced top-level data tables (e.g. autocomplete.js's
    // TEMPLATE_TAGS, used from inline page scripts).
    await build({
        entryPoints: plainApps.map((f) => `${jsSrc}/app/${f}`),
        outdir: `${jsDist}/app`,
        bundle: false,
        minify: true,
        target: 'es2017',
        platform: 'browser',
        logLevel: 'info',
        outExtension: { '.js': '.min.js' },
    })

    // Bundled files: resolve npm imports + minify + IIFE-wrap.
    // passwords.js pulls in zxcvbn (~800KB — same size as the previous
    // webpack output; it's the English password dictionary).
    if (bundledApps.length > 0) {
        await build({
            entryPoints: bundledApps.map((f) => `${jsSrc}/app/${f}`),
            outdir: `${jsDist}/app`,
            bundle: true,
            minify: true,
            target: 'es2017',
            platform: 'browser',
            format: 'iife',
            logLevel: 'info',
            outExtension: { '.js': '.min.js' },
        })
    }
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
