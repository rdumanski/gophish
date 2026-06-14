// Package i18n is a tiny, zero-dependency message catalog for the admin UI and
// learner portal. Catalogs are plain JSON (key -> string) embedded at build
// time and shared by both the Go templates (via the T method on templateParams)
// and the browser bundles (the active catalog is injected into each page as
// window.i18n, so there is a single source of truth and no Go/JS drift).
//
// Lookups fall back: requested language -> English -> the key itself. Optional
// args are applied with fmt.Sprintf, so messages may use %s/%d placeholders.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

// DefaultLang is the fallback language and the base every other catalog
// overlays.
const DefaultLang = "en"

var (
	catalogs map[string]map[string]string
	once     sync.Once
)

func load() {
	catalogs = map[string]map[string]string{}
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		b, err := localeFS.ReadFile("locales/" + name)
		if err != nil {
			continue
		}
		m := map[string]string{}
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		catalogs[strings.TrimSuffix(name, ".json")] = m
	}
}

func ensure() { once.Do(load) }

// Supported returns the available language codes, sorted.
func Supported() []string {
	ensure()
	out := make([]string, 0, len(catalogs))
	for lang := range catalogs {
		out = append(out, lang)
	}
	sort.Strings(out)
	return out
}

// Normalize returns lang if a catalog exists for it, else DefaultLang.
func Normalize(lang string) string {
	ensure()
	if lang != "" {
		if _, ok := catalogs[lang]; ok {
			return lang
		}
	}
	return DefaultLang
}

// FromAcceptLanguage picks a supported language from an Accept-Language header
// (e.g. "pl-PL,pl;q=0.9,en;q=0.8"). It scans the listed tags in order and
// returns the first whose primary subtag has a catalog, defaulting to English.
// Used for pre-auth / recipient-facing pages where there's no stored preference.
func FromAcceptLanguage(header string) string {
	ensure()
	for _, part := range strings.Split(header, ",") {
		tag := strings.TrimSpace(part)
		if i := strings.Index(tag, ";"); i >= 0 {
			tag = tag[:i]
		}
		primary := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
		if _, ok := catalogs[primary]; ok && primary != "" {
			return primary
		}
	}
	return DefaultLang
}

// T returns the message for key in lang, falling back to English then the key.
func T(lang, key string, args ...interface{}) string {
	ensure()
	msg := ""
	if m, ok := catalogs[lang][key]; ok && m != "" {
		msg = m
	} else if m, ok := catalogs[DefaultLang][key]; ok && m != "" {
		msg = m
	} else {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(msg, args...)
	}
	return msg
}

// CatalogJSON returns the merged catalog for lang (English base overlaid with
// the requested language) as JSON, for injection into a page as window.i18n.
// template.JS marks it safe to emit unescaped inside a <script> block.
func CatalogJSON(lang string) template.JS {
	ensure()
	merged := map[string]string{}
	for k, v := range catalogs[DefaultLang] {
		merged[k] = v
	}
	for k, v := range catalogs[lang] {
		if v != "" {
			merged[k] = v
		}
	}
	b, err := json.Marshal(merged)
	if err != nil {
		return template.JS("{}")
	}
	return template.JS(b)
}
