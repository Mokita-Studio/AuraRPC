package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"aurarpc/internal/discord"
	"aurarpc/internal/preset"
)

// fakeClient is an in-memory ipcClient for testing the controller's
// connect/switch logic without a real Discord process.
type fakeClient struct {
	mu         sync.Mutex
	activities []discord.Activity
}

func (f *fakeClient) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (f *fakeClient) SetActivity(a discord.Activity) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.activities = append(f.activities, a)
}

func (f *fakeClient) State() discord.State { return discord.StateConnected }

// newFakeController returns a controller whose connections are fakeClients
// and a pointer to a counter of how many clients were created (i.e. how
// many reconnects happened).
func newFakeController(t *testing.T) (*Controller, *int) {
	t.Helper()
	created := 0
	c := NewController(preset.NewManager(t.TempDir()))
	c.newClient = func(string) ipcClient {
		created++
		return &fakeClient{}
	}
	return c, &created
}

func TestApplyRestartsOnNewClientID(t *testing.T) {
	c, created := newFakeController(t)
	defer c.Disconnect()

	if err := c.Apply(&preset.Preset{ID: "a", Name: "A", ClientID: "111"}); err != nil {
		t.Fatal(err)
	}
	if err := c.Apply(&preset.Preset{ID: "b", Name: "B", ClientID: "222"}); err != nil {
		t.Fatal(err)
	}
	// A different Client ID must disconnect the old client and start a new
	// one (Discord serves a single RPC connection per process).
	if *created != 2 {
		t.Fatalf("created %d clients, want 2 (switching Client ID reconnects)", *created)
	}
}

func TestApplySameClientReuses(t *testing.T) {
	c, created := newFakeController(t)
	defer c.Disconnect()

	p := &preset.Preset{ID: "a", Name: "A", ClientID: "111"}
	for i := 0; i < 3; i++ {
		if err := c.Apply(p); err != nil {
			t.Fatal(err)
		}
	}
	if *created != 1 {
		t.Fatalf("created %d clients, want 1 (same Client ID reuses the connection)", *created)
	}
}

func TestDisconnectStops(t *testing.T) {
	c, _ := newFakeController(t)
	if err := c.Apply(&preset.Preset{ID: "a", Name: "A", ClientID: "111"}); err != nil {
		t.Fatal(err)
	}
	c.Disconnect()

	if c.Status() != "disconnected" {
		t.Errorf("status after Disconnect = %q, want disconnected", c.Status())
	}
	if c.ActiveID() != "" {
		t.Errorf("activeID after Disconnect = %q, want empty", c.ActiveID())
	}
}

func TestEffectiveClientID(t *testing.T) {
	if got := EffectiveClientID(&preset.Preset{ClientID: "  abc  "}); got != "abc" {
		t.Errorf("trim failed: %q", got)
	}
	if got := EffectiveClientID(&preset.Preset{}); got != "" {
		t.Errorf("empty client id: got %q", got)
	}
}

func TestBuildActivityCoreFields(t *testing.T) {
	p := &preset.Preset{
		AppName:   "name",
		Details:   "d",
		State:     "s",
		Type:      preset.TypeListening,
		LargeKey:  "li",
		LargeText: "lt",
		PartySize: 2,
		PartyMax:  4,
		Btn1Text:  "L",
		Btn1URL:   "https://x",
	}
	a := BuildActivity(p)
	if a.Type != 2 {
		t.Errorf("type = %d, want 2 (listening)", a.Type)
	}
	if a.Name != "name" || a.Details != "d" || a.State != "s" {
		t.Errorf("text fields: %+v", a)
	}
	if a.Assets == nil || a.Assets.LargeImage != "li" || a.Assets.LargeText != "lt" {
		t.Errorf("assets: %+v", a.Assets)
	}
	if a.Party == nil || a.Party.Size[0] != 2 || a.Party.Size[1] != 4 {
		t.Errorf("party: %+v", a.Party)
	}
	if len(a.Buttons) != 1 || a.Buttons[0].Label != "L" {
		t.Errorf("buttons: %+v", a.Buttons)
	}
}

func TestBuildActivityTimestamps(t *testing.T) {
	if a := BuildActivity(&preset.Preset{TimeMode: preset.TimeNone}); a.Timestamps != nil {
		t.Errorf("none should produce nil timestamps: %+v", a.Timestamps)
	}
	a := BuildActivity(&preset.Preset{TimeMode: preset.TimeLocal})
	if a.Timestamps == nil || a.Timestamps.Start == 0 {
		t.Errorf("local should set Start: %+v", a.Timestamps)
	}
}

func TestBuildActivityCustomTime(t *testing.T) {
	p := &preset.Preset{
		TimeMode:  preset.TimeCustom,
		TimeStart: "2026-01-15T10:30",
		TimeEndOn: true,
		TimeEnd:   "2026-01-15T12:30",
	}
	a := BuildActivity(p)
	if a.Timestamps == nil {
		t.Fatal("custom should produce timestamps")
	}
	if a.Timestamps.End <= a.Timestamps.Start {
		t.Errorf("End must be after Start: %+v", a.Timestamps)
	}
	expected, _ := time.ParseInLocation("2006-01-02T15:04", "2026-01-15T10:30", time.Local)
	if a.Timestamps.Start != expected.Unix() {
		t.Errorf("start = %d, want %d", a.Timestamps.Start, expected.Unix())
	}
}

func TestBuildActivityStreamingURL(t *testing.T) {
	p := &preset.Preset{
		Type:       preset.TypeStreaming,
		DetailsURL: "https://twitch.tv/example",
	}
	a := BuildActivity(p)
	if a.URL != "https://twitch.tv/example" {
		t.Errorf("streaming URL: got %q", a.URL)
	}
}
