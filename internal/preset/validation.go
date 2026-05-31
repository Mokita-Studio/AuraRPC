package preset

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"
)

// ErrInvalid wraps every validation failure. Concrete errors are built
// with fmt.Errorf("...: %w", ErrInvalid).
var ErrInvalid = errors.New("preset: invalid")

// Length limits in runes (Discord counts characters, not bytes).
const (
	maxName       = 64
	maxText       = 128
	maxButtonText = 32
	// minRichText is the minimum length Discord requires to render
	// details/state. Shorter strings are silently dropped on the server.
	minRichText = 2
)

// Validate checks a preset against the Discord Rich Presence limits.
// Called by Upsert and by Controller before applying. Lengths are
// measured in runes after TrimSpace.
func Validate(p *Preset) error {
	if p == nil {
		return fmt.Errorf("%w: nil preset", ErrInvalid)
	}
	name := strings.TrimSpace(p.Name)
	if name == "" || utf8.RuneCountInString(name) > maxName {
		return fmt.Errorf("%w: name length must be 1-%d", ErrInvalid, maxName)
	}
	if strings.TrimSpace(p.ClientID) == "" {
		return fmt.Errorf("%w: client_id is required", ErrInvalid)
	}
	if err := checkLen("app_name", p.AppName, maxText); err != nil {
		return err
	}
	if err := checkRichText("details", p.Details); err != nil {
		return err
	}
	if err := checkRichText("state", p.State); err != nil {
		return err
	}
	if err := checkLen("large_text", p.LargeText, maxText); err != nil {
		return err
	}
	if err := checkLen("small_text", p.SmallText, maxText); err != nil {
		return err
	}
	if err := checkLen("btn1_text", p.Btn1Text, maxButtonText); err != nil {
		return err
	}
	if err := checkLen("btn2_text", p.Btn2Text, maxButtonText); err != nil {
		return err
	}
	if err := checkHTTPURL("details_url", p.DetailsURL); err != nil {
		return err
	}
	if err := checkHTTPURL("state_url", p.StateURL); err != nil {
		return err
	}
	if err := checkHTTPURL("large_url", p.LargeURL); err != nil {
		return err
	}
	if err := checkHTTPURL("small_url", p.SmallURL); err != nil {
		return err
	}
	if err := checkButton("btn1", p.Btn1Text, p.Btn1URL); err != nil {
		return err
	}
	if err := checkButton("btn2", p.Btn2Text, p.Btn2URL); err != nil {
		return err
	}
	if p.PartySize < 0 || p.PartyMax < 0 || p.PartySize > p.PartyMax {
		return fmt.Errorf("%w: invalid party size %d/%d", ErrInvalid, p.PartySize, p.PartyMax)
	}
	switch p.Type {
	case "", TypePlaying, TypeStreaming, TypeListening, TypeWatching, TypeCompeting:
	default:
		return fmt.Errorf("%w: unknown activity type %q", ErrInvalid, p.Type)
	}
	switch p.Display {
	case "", DisplayName, DisplayDetails, DisplayState:
	default:
		return fmt.Errorf("%w: unknown display field %q", ErrInvalid, p.Display)
	}
	switch p.TimeMode {
	case "", TimeNone, TimeSinceConnect, TimeSincePresence, TimeSinceStart, TimeLocal, TimeCustom:
	default:
		return fmt.Errorf("%w: unknown time mode %q", ErrInvalid, p.TimeMode)
	}
	return nil
}

// checkLen ensures value, after TrimSpace, does not exceed max runes.
func checkLen(field, value string, max int) error {
	trimmed := strings.TrimSpace(value)
	if utf8.RuneCountInString(trimmed) > max {
		return fmt.Errorf("%w: %s > %d chars", ErrInvalid, field, max)
	}
	return nil
}

// checkRichText applies checkLen and the Discord minimum-length rule;
// empty values are allowed and mean "skip this field".
func checkRichText(field, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	n := utf8.RuneCountInString(trimmed)
	if n < minRichText {
		return fmt.Errorf("%w: %s needs ≥%d chars or be empty", ErrInvalid, field, minRichText)
	}
	if n > maxText {
		return fmt.Errorf("%w: %s > %d chars", ErrInvalid, field, maxText)
	}
	return nil
}

// checkHTTPURL ensures raw is a well-formed http(s) URL with a non-empty
// host. Discord silently drops the entire activity for malformed URLs.
func checkHTTPURL(field, raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%w: %s must be http(s)", ErrInvalid, field)
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("%w: %s must include a host", ErrInvalid, field)
	}
	return nil
}

func checkButton(field, label, raw string) error {
	label = strings.TrimSpace(label)
	raw = strings.TrimSpace(raw)
	if label == "" && raw == "" {
		return nil
	}
	if label == "" || raw == "" {
		return fmt.Errorf("%w: %s requires both label and url", ErrInvalid, field)
	}
	return checkHTTPURL(field+"_url", raw)
}
