package i18n

import (
	"strings"
	"testing"
)

func TestTranslate(t *testing.T) {
	if got := T("en", "nav.dashboard"); got != "Dashboard" {
		t.Errorf("en nav.dashboard = %q, want Dashboard", got)
	}
	if got := T("pl", "nav.dashboard"); got != "Pulpit" {
		t.Errorf("pl nav.dashboard = %q, want Pulpit", got)
	}
	// Unknown key falls back to the key itself.
	if got := T("pl", "does.not.exist"); got != "does.not.exist" {
		t.Errorf("missing key = %q, want the key", got)
	}
	// Unknown language falls back to English.
	if got := T("xx", "nav.dashboard"); got != "Dashboard" {
		t.Errorf("unknown lang = %q, want English fallback", got)
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{"pl": "pl", "en": "en", "xx": "en", "": "en"} {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFromAcceptLanguage(t *testing.T) {
	cases := map[string]string{
		"pl-PL,pl;q=0.9,en;q=0.8": "pl",
		"en-US,en;q=0.9":          "en",
		"fr-FR,fr;q=0.9":          "en", // unsupported -> default
		"de,fr;q=0.5,pl;q=0.3":    "pl", // first supported in list wins
		"":                        "en",
		"pl":                      "pl",
	}
	for h, want := range cases {
		if got := FromAcceptLanguage(h); got != want {
			t.Errorf("FromAcceptLanguage(%q) = %q, want %q", h, got, want)
		}
	}
}

// TestCatalogParity guards that every key exists in both locales, so a string
// is never localized in one language and missing in the other.
func TestCatalogParity(t *testing.T) {
	ensure()
	en, pl := catalogs["en"], catalogs["pl"]
	for k := range en {
		if _, ok := pl[k]; !ok {
			t.Errorf("key %q present in en but missing in pl", k)
		}
	}
	for k := range pl {
		if _, ok := en[k]; !ok {
			t.Errorf("key %q present in pl but missing in en", k)
		}
	}
}

func TestCatalogJSON(t *testing.T) {
	js := string(CatalogJSON("pl"))
	if !strings.Contains(js, "Pulpit") {
		t.Errorf("pl catalog JSON missing Polish value: %s", js)
	}
	// English base keys are present even if not overridden.
	if !strings.Contains(js, "nav.dashboard") {
		t.Errorf("catalog JSON missing key nav.dashboard")
	}
}
