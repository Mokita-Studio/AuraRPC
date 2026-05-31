package preset

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateGood(t *testing.T) {
	p := &Preset{
		Name:     "OK",
		ClientID: "1234567890",
		AppName:  "Hello",
		Details:  "doing things",
		State:    "with care",
		Type:     TypePlaying,
		Display:  DisplayName,
		TimeMode: TimeSinceConnect,
		Btn1Text: "open",
		Btn1URL:  "https://example.com",
	}
	if err := Validate(p); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	good := func() *Preset {
		return &Preset{Name: "OK", ClientID: "1"}
	}
	mutate := func(fn func(*Preset)) *Preset {
		p := good()
		fn(p)
		return p
	}

	cases := map[string]*Preset{
		"empty name":      {Name: "", ClientID: "1"},
		"long name":       {Name: strings.Repeat("a", 65), ClientID: "1"},
		"no client id":    {Name: "OK"},
		"long details":    mutate(func(p *Preset) { p.Details = strings.Repeat("a", 129) }),
		"btn no url":      mutate(func(p *Preset) { p.Btn1Text = "open" }),
		"btn no label":    mutate(func(p *Preset) { p.Btn1URL = "https://x" }),
		"btn bad url":     mutate(func(p *Preset) { p.Btn1Text = "x"; p.Btn1URL = "ftp://x" }),
		"bad details url": mutate(func(p *Preset) { p.DetailsURL = "ftp://x" }),
		"bad type":        mutate(func(p *Preset) { p.Type = "dancing" }),
		"bad display":     mutate(func(p *Preset) { p.Display = "title" }),
		"bad time mode":   mutate(func(p *Preset) { p.TimeMode = "later" }),
		"party invalid":   mutate(func(p *Preset) { p.PartySize = 5; p.PartyMax = 2 }),
	}
	for name, p := range cases {
		t.Run(name, func(t *testing.T) {
			err := Validate(p)
			if err == nil {
				t.Fatalf("expected error for %q", name)
			}
			if !errors.Is(err, ErrInvalid) {
				t.Errorf("error not wrapping ErrInvalid: %v", err)
			}
		})
	}
}

func TestValidateNil(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Fatal("expected error for nil preset")
	}
}

// TestValidateAcceptsMultiByteAtLimit ensures length limits are measured
// in runes, not bytes: maxText runes of multi-byte text must pass.
func TestValidateAcceptsMultiByteAtLimit(t *testing.T) {
	p := &Preset{
		Name:     "OK",
		ClientID: "1",
		Details:  strings.Repeat("あ", 128), // 128 runes, 384 bytes
		State:    strings.Repeat("é", 128),  // 128 runes, 256 bytes
		Btn1Text: strings.Repeat("🎮", 32),  // 32 runes (supplementary BMP)
		Btn1URL:  "https://example.com",
	}
	if err := Validate(p); err != nil {
		t.Fatalf("Validate rejected text at rune limit: %v", err)
	}
}

// TestValidateRejectsRuneOverflow covers the symmetric case: 129 ASCII
// runes must fail even though each is one byte.
func TestValidateRejectsRuneOverflow(t *testing.T) {
	p := &Preset{
		Name:     "OK",
		ClientID: "1",
		Details:  strings.Repeat("a", 129),
	}
	if err := Validate(p); err == nil {
		t.Fatal("expected error for 129-rune details")
	}
}

// TestValidateRejectsSingleCharRichText ensures Details and State must
// have at least 2 runes (or be empty) for Discord to render them.
func TestValidateRejectsSingleCharRichText(t *testing.T) {
	cases := []*Preset{
		{Name: "OK", ClientID: "1", Details: "a"},
		{Name: "OK", ClientID: "1", State: "b"},
		{Name: "OK", ClientID: "1", Details: "  a  "}, // 1 rune after trim
	}
	for i, p := range cases {
		if err := Validate(p); err == nil {
			t.Errorf("case %d: expected error for single-rune rich text", i)
		}
	}
}

// TestValidateRejectsURLWithoutHost ensures checkHTTPURL requires a host:
// "https://" parses with scheme https but empty Host, which Discord
// would silently drop.
func TestValidateRejectsURLWithoutHost(t *testing.T) {
	cases := []string{
		"https://",
		"http://",
		"https:///path",
	}
	for _, raw := range cases {
		p := &Preset{Name: "OK", ClientID: "1", DetailsURL: raw}
		if err := Validate(p); err == nil {
			t.Errorf("expected error for url %q", raw)
		}
	}
}

// TestValidateTrimsBeforeMeasuring ensures leading/trailing whitespace
// does not consume rune budget — paste with stray spaces still validates.
func TestValidateTrimsBeforeMeasuring(t *testing.T) {
	core := strings.Repeat("a", 128)
	p := &Preset{
		Name:     "OK",
		ClientID: "1",
		Details:  "   " + core + "   ", // 134 bytes, 128 useful runes
	}
	if err := Validate(p); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}
