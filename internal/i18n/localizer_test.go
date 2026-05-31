package i18n

import (
	"sort"
	"testing"
)

func TestNewLoadsEmbeddedCatalogs(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	langs := l.Languages()
	codes := make([]string, len(langs))
	for i, lg := range langs {
		codes[i] = lg.Code
	}
	sort.Strings(codes)
	want := []string{"en", "es"}
	if len(codes) != len(want) {
		t.Fatalf("languages = %v, want %v", codes, want)
	}
	for i, c := range want {
		if codes[i] != c {
			t.Errorf("languages[%d] = %q, want %q", i, codes[i], c)
		}
	}
}

func TestTUsesActiveLanguage(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if got := l.T("btn.connect"); got != "Connect" {
		t.Errorf("en: got %q, want Connect", got)
	}
	if err := l.SetLanguage("es"); err != nil {
		t.Fatal(err)
	}
	if got := l.T("btn.connect"); got != "Conectar" {
		t.Errorf("es: got %q, want Conectar", got)
	}
}

func TestTFallsBackToEnglish(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.SetLanguage("es"); err != nil {
		t.Fatal(err)
	}
	l.catalogs["en"]["__test.only_in_en"] = "english-only"
	if got := l.T("__test.only_in_en"); got != "english-only" {
		t.Errorf("fallback failed: got %q", got)
	}
}

func TestTReturnsKeyWhenMissing(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if got := l.T("__nope__"); got != "__nope__" {
		t.Errorf("missing key returned %q, want the key", got)
	}
}

func TestSetLanguageRejectsUnknown(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	if err := l.SetLanguage("fr"); err == nil {
		t.Fatal("expected error for unknown lang")
	}
	if l.Lang() != FallbackLang {
		t.Errorf("Lang changed despite error: %q", l.Lang())
	}
}

func TestLanguageMetadata(t *testing.T) {
	l, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, lg := range l.Languages() {
		if lg.Label == "" || lg.Short == "" {
			t.Errorf("language %q missing label/short: %+v", lg.Code, lg)
		}
	}
}
