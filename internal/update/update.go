// Package update implements the optional update check. It asks GitHub for
// the latest release tag, compares it with the running version and, if a
// newer one exists, exposes a link the user can open. It never downloads
// or installs anything.
package update

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CurrentVersion is the running app version, "MAJOR.MINOR.PATCH". It is a
// var so release builds can stamp it via -ldflags "-X
// aurarpc/internal/update.CurrentVersion=1.2.3".
var CurrentVersion = "1.0.0"

// releasesAPI is the GitHub "latest release" endpoint for the project,
// queried with a single unauthenticated HTTPS GET.
const releasesAPI = "https://api.github.com/repos/Mokita-Studio/AuraRPC/releases/latest"

// httpTimeout caps the whole request so a slow network never blocks the
// background goroutine for long.
const httpTimeout = 10 * time.Second

// Release describes the newest published release.
type Release struct {
	Version string // normalized, e.g. "1.1.0"
	Tag     string // raw tag, e.g. "v1.1.0"
	URL     string // html_url of the release page, for the user to open
}

// Check queries GitHub for the latest release and reports whether it is
// newer than CurrentVersion. The bool is false (with no error) when the
// running version is already current.
func Check(ctx context.Context) (Release, bool, error) {
	return checkAt(ctx, releasesAPI)
}

// checkAt is Check parameterized by endpoint so tests can point it at a
// local server.
func checkAt(ctx context.Context, endpoint string) (Release, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Release{}, false, fmt.Errorf("update: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "AuraRPC-update-check")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return Release{}, false, fmt.Errorf("update: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Release{}, false, fmt.Errorf("update: unexpected status %s", resp.Status)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Release{}, false, fmt.Errorf("update: decode: %w", err)
	}
	if payload.TagName == "" {
		return Release{}, false, fmt.Errorf("update: empty tag in response")
	}

	rel := Release{
		Version: normalize(payload.TagName),
		Tag:     payload.TagName,
		URL:     payload.HTMLURL,
	}
	return rel, compare(rel.Version, normalize(CurrentVersion)) > 0, nil
}

// normalize strips a leading "v" and any pre-release/build suffix so
// "v1.2.3-beta+5" becomes "1.2.3".
func normalize(tag string) string {
	tag = strings.TrimSpace(tag)
	tag = strings.TrimPrefix(tag, "v")
	tag = strings.TrimPrefix(tag, "V")
	if i := strings.IndexAny(tag, "-+"); i >= 0 {
		tag = tag[:i]
	}
	return tag
}

// compare returns -1, 0 or 1 comparing two normalized "X.Y.Z" versions.
// Missing or non-numeric components are treated as 0, so malformed tags
// never cause a false "update available".
func compare(a, b string) int {
	pa, pb := splitVersion(a), splitVersion(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return 1
		}
	}
	return 0
}

func splitVersion(v string) [3]int {
	var out [3]int
	for i, part := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		n, _ := strconv.Atoi(strings.TrimSpace(part))
		if n < 0 {
			n = 0
		}
		out[i] = n
	}
	return out
}

// State is a concurrent-safe holder for the result of a background check.
// The UI and tray read it; the checker goroutine writes it once.
type State struct {
	mu        sync.RWMutex
	available bool
	release   Release
}

// NewState returns an empty State (no update available).
func NewState() *State { return &State{} }

// Set records an available release. Called by the checker goroutine.
func (s *State) Set(r Release) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.available = true
	s.release = r
}

// Available reports the pending release, if any.
func (s *State) Available() (Release, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.release, s.available
}
