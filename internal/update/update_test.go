package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"v1.2.3":       "1.2.3",
		"V1.2.3":       "1.2.3",
		"1.2.3":        "1.2.3",
		"v1.2.3-beta1": "1.2.3",
		"1.2.3+build7": "1.2.3",
		"  v2.0.0  ":   "2.0.0",
	}
	for in, want := range cases {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.1.0", "1.0.0", 1},
		{"1.0.0", "1.1.0", -1},
		{"1.0.1", "1.0.0", 1},
		{"2.0.0", "1.9.9", 1},
		{"1.0", "1.0.0", 0},   // missing patch == 0
		{"1.2", "1.10.0", -1}, // numeric, not lexical
		{"garbage", "1.0.0", -1},
	}
	for _, c := range cases {
		if got := compare(c.a, c.b); got != c.want {
			t.Errorf("compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCheckNewer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","html_url":"https://example.com/r"}`))
	}))
	defer srv.Close()

	rel, newer, err := checkAt(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !newer {
		t.Error("expected newer = true")
	}
	if rel.Version != "9.9.9" || rel.URL != "https://example.com/r" {
		t.Errorf("release = %+v", rel)
	}
}

func TestCheckNotNewer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"v0.0.1","html_url":"x"}`))
	}))
	defer srv.Close()

	_, newer, err := checkAt(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if newer {
		t.Error("expected newer = false for an older tag")
	}
}

func TestState(t *testing.T) {
	s := NewState()
	if _, ok := s.Available(); ok {
		t.Fatal("new state should have no update")
	}
	s.Set(Release{Version: "1.1.0", Tag: "v1.1.0", URL: "u"})
	rel, ok := s.Available()
	if !ok || rel.Tag != "v1.1.0" {
		t.Errorf("Available() = %+v, %v", rel, ok)
	}
}
