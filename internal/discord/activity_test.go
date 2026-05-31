package discord

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestActivityOmitsEmpty(t *testing.T) {
	b, err := json.Marshal(Activity{})
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "{}" {
		t.Errorf("empty Activity = %s, want {}", b)
	}
}

func TestActivityMarshal(t *testing.T) {
	a := Activity{
		Details:    "Hacking",
		State:      "On a Tuesday",
		Assets:     &Assets{LargeImage: "code", LargeText: "VS Code"},
		Timestamps: &Timestamps{Start: 1700000000},
		Buttons:    []Button{{Label: "Repo", URL: "https://example.com"}},
	}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)

	for _, want := range []string{
		`"details":"Hacking"`,
		`"state":"On a Tuesday"`,
		`"large_image":"code"`,
		`"large_text":"VS Code"`,
		`"start":1700000000`,
		`"label":"Repo"`,
		`"url":"https://example.com"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in %s", want, s)
		}
	}
	if strings.Contains(s, `"party"`) {
		t.Errorf("party should be omitted: %s", s)
	}
}
