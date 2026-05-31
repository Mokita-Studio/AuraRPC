// Package i18n loads JSON translation catalogs embedded in the binary and
// exposes Localizer.T for every user-visible string.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

//go:embed translations/*.json
var translationsFS embed.FS

// FallbackLang is the language used when a key is missing.
const FallbackLang = "en"

// Language describes a loaded language: ISO code plus display labels.
type Language struct {
	Code  string
	Label string
	Short string
}

// Localizer holds the loaded catalogs and the active language. Safe for
// concurrent use; the active language can change at runtime.
type Localizer struct {
	mu        sync.RWMutex
	lang      string
	catalogs  map[string]map[string]string
	languages []Language
}

// New loads every catalog under translations/*.json. Fails if the fallback
// catalog is not present.
func New() (*Localizer, error) {
	l := &Localizer{
		catalogs: make(map[string]map[string]string),
	}

	entries, err := fs.ReadDir(translationsFS, "translations")
	if err != nil {
		return nil, fmt.Errorf("i18n: read translations dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		code := strings.TrimSuffix(e.Name(), ".json")
		data, err := fs.ReadFile(translationsFS, filepath.ToSlash(filepath.Join("translations", e.Name())))
		if err != nil {
			return nil, fmt.Errorf("i18n: read %s: %w", e.Name(), err)
		}
		var catalog map[string]string
		if err := json.Unmarshal(data, &catalog); err != nil {
			return nil, fmt.Errorf("i18n: parse %s: %w", e.Name(), err)
		}
		l.catalogs[code] = catalog
		l.languages = append(l.languages, Language{
			Code:  code,
			Label: firstNonEmpty(catalog["lang.label"], code),
			Short: firstNonEmpty(catalog["lang.short"], strings.ToUpper(code)),
		})
	}

	if _, ok := l.catalogs[FallbackLang]; !ok {
		return nil, fmt.Errorf("i18n: fallback %q not loaded", FallbackLang)
	}
	sort.Slice(l.languages, func(i, j int) bool { return l.languages[i].Code < l.languages[j].Code })
	l.lang = FallbackLang
	return l, nil
}

// SetLanguage switches the active language; returns an error if the
// catalog is not loaded.
func (l *Localizer) SetLanguage(lang string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.catalogs[lang]; !ok {
		return fmt.Errorf("i18n: language %q not loaded", lang)
	}
	l.lang = lang
	return nil
}

// Lang returns the active ISO 639-1 code.
func (l *Localizer) Lang() string {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.lang
}

// Languages returns all loaded languages with their label and short code.
func (l *Localizer) Languages() []Language {
	l.mu.RLock()
	defer l.mu.RUnlock()
	out := make([]Language, len(l.languages))
	copy(out, l.languages)
	return out
}

// T resolves key against the active catalog, then the fallback. Returns
// the key itself if missing in both. args are passed to fmt.Sprintf.
func (l *Localizer) T(key string, args ...any) string {
	l.mu.RLock()
	catalog := l.catalogs[l.lang]
	fallback := l.catalogs[FallbackLang]
	l.mu.RUnlock()

	s, ok := catalog[key]
	if !ok {
		s, ok = fallback[key]
		if !ok {
			return key
		}
	}
	if len(args) == 0 {
		return s
	}
	return fmt.Sprintf(s, args...)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
