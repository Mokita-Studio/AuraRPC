// Package core glues the preset manager to the Discord IPC client and
// exposes the apply/disconnect API consumed by the UI and the tray.
package core

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"aurarpc/internal/discord"
	"aurarpc/internal/preset"
)

// ipcClient is the subset of *discord.Client the controller drives,
// expressed as an interface so the switching logic can be unit-tested
// without a real Discord process.
type ipcClient interface {
	Run(ctx context.Context) error
	SetActivity(a discord.Activity)
	State() discord.State
}

// teardownWait bounds how long stopping the client waits for its goroutine
// to exit, so the controller never deadlocks on a hang.
const teardownWait = 2 * time.Second

// Controller owns at most one live Discord connection and recycles it when
// the Client ID changes. Discord serves a single RPC connection per
// process, so connections cannot be pooled: switching to a preset with a
// different Client ID disconnects the old one and reconnects.
type Controller struct {
	presets   *preset.Manager
	newClient func(clientID string) ipcClient

	mu            sync.Mutex
	clientID      string
	client        ipcClient
	cancel        context.CancelFunc
	done          chan struct{}
	activeID      string
	lastAppliedID string

	processStart time.Time
	connectAt    time.Time
	lastApplyAt  time.Time

	// OnLastAppliedChange fires after a successful Apply. main uses it
	// to persist the last preset id so AutoConnect can reuse it.
	OnLastAppliedChange func(id string)
}

// NewController builds a Controller backed by mgr.
func NewController(mgr *preset.Manager) *Controller {
	return &Controller{
		presets:      mgr,
		newClient:    func(cid string) ipcClient { return discord.New(cid) },
		processStart: time.Now(),
	}
}

// Apply validates p, ensures the live connection uses its Client ID and
// pushes the activity. Switching to a different Client ID reconnects.
func (c *Controller) Apply(p *preset.Preset) error {
	if err := preset.Validate(p); err != nil {
		return err
	}
	cid := EffectiveClientID(p)
	if cid == "" {
		return errors.New("core: missing client id")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	restart := c.client == nil || c.clientID != cid
	if restart {
		c.stopLocked()
		c.startLocked(cid)
		c.connectAt = time.Now()
	}
	c.lastApplyAt = time.Now()
	activity := c.buildActivityLocked(p)
	c.client.SetActivity(activity)
	c.activeID = p.ID

	notify := p.ID != "" && p.ID != c.lastAppliedID
	if notify {
		c.lastAppliedID = p.ID
	}
	cb := c.OnLastAppliedChange
	log.Printf("core.Apply: cid=%s presetID=%s restart=%v name=%q", cid, p.ID, restart, activity.Name)
	if notify && cb != nil {
		go cb(p.ID)
	}
	return nil
}

// LastAppliedID is the last preset applied in this session, kept across
// Disconnects so the tray can reconnect without opening the window.
func (c *Controller) LastAppliedID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastAppliedID
}

// Disconnect stops the live connection and clears active state.
func (c *Controller) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	log.Printf("core.Disconnect: invoked")
	c.stopLocked()
	c.activeID = ""
}

// Status returns "connected", "connecting" or "disconnected".
func (c *Controller) Status() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.client == nil {
		return "disconnected"
	}
	return c.client.State().String()
}

// ActiveID is the id of the currently applied preset, or "" if none.
func (c *Controller) ActiveID() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.activeID
}

func (c *Controller) startLocked(cid string) {
	c.clientID = cid
	c.client = c.newClient(cid)
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	done := make(chan struct{})
	c.done = done
	client := c.client
	go func() {
		_ = client.Run(ctx)
		close(done)
	}()
}

// stopLocked cancels the live client and waits for its goroutine to exit
// so the named pipe is released before a replacement connects. The wait is
// normally sub-millisecond; the timeout only guards a pathological hang so
// the controller never deadlocks.
func (c *Controller) stopLocked() {
	if c.cancel != nil {
		c.cancel()
		c.cancel = nil
	}
	if c.done != nil {
		select {
		case <-c.done:
		case <-time.After(teardownWait):
			log.Printf("core: timed out waiting for IPC client teardown")
		}
		c.done = nil
	}
	c.client = nil
	c.clientID = ""
}

// EffectiveClientID returns the trimmed Client ID sent to Discord for p.
func EffectiveClientID(p *preset.Preset) string {
	return strings.TrimSpace(p.ClientID)
}

// BuildActivity converts a Preset into the Discord IPC payload without
// session context. Use the Controller's path for runtime-relative time
// modes (sinceConnect / sincePresence / sinceStart).
func BuildActivity(p *preset.Preset) discord.Activity {
	now := time.Now()
	return buildActivityWith(p, now, now, now)
}

func (c *Controller) buildActivityLocked(p *preset.Preset) discord.Activity {
	now := time.Now()
	connect := c.connectAt
	if connect.IsZero() {
		connect = now
	}
	apply := c.lastApplyAt
	if apply.IsZero() {
		apply = now
	}
	return buildActivityWith(p, connect, apply, c.processStart)
}

func buildActivityWith(p *preset.Preset, connectAt, applyAt, processStart time.Time) discord.Activity {
	a := discord.Activity{
		Type:    activityType(p.Type),
		Name:    p.AppName,
		Details: p.Details,
		State:   p.State,
	}
	if a.Type == discord.TypeStreaming {
		a.URL = p.DetailsURL
	}
	if assets := buildAssets(p); assets != nil {
		a.Assets = assets
	}
	if ts := buildTimestamps(p, connectAt, applyAt, processStart); ts != nil {
		a.Timestamps = ts
	}
	if p.PartySize > 0 || p.PartyMax > 0 {
		a.Party = &discord.Party{Size: []int{p.PartySize, p.PartyMax}}
	}
	if btns := buildButtons(p); len(btns) > 0 {
		a.Buttons = btns
	}
	return a
}

func activityType(t preset.ActivityType) int {
	switch t {
	case preset.TypeStreaming:
		return discord.TypeStreaming
	case preset.TypeListening:
		return discord.TypeListening
	case preset.TypeWatching:
		return discord.TypeWatching
	case preset.TypeCompeting:
		return discord.TypeCompeting
	default:
		return discord.TypePlaying
	}
}

func buildAssets(p *preset.Preset) *discord.Assets {
	if p.LargeKey == "" && p.SmallKey == "" &&
		p.LargeText == "" && p.SmallText == "" &&
		p.LargeURL == "" && p.SmallURL == "" {
		return nil
	}
	return &discord.Assets{
		LargeImage: p.LargeKey,
		LargeText:  p.LargeText,
		LargeURL:   p.LargeURL,
		SmallImage: p.SmallKey,
		SmallText:  p.SmallText,
		SmallURL:   p.SmallURL,
	}
}

func buildButtons(p *preset.Preset) []discord.Button {
	var out []discord.Button
	if p.Btn1Text != "" && p.Btn1URL != "" {
		out = append(out, discord.Button{Label: p.Btn1Text, URL: p.Btn1URL})
	}
	if p.Btn2Text != "" && p.Btn2URL != "" {
		out = append(out, discord.Button{Label: p.Btn2Text, URL: p.Btn2URL})
	}
	return out
}

func buildTimestamps(p *preset.Preset, connectAt, applyAt, processStart time.Time) *discord.Timestamps {
	switch p.TimeMode {
	case preset.TimeNone, "":
		return nil
	case preset.TimeSinceConnect:
		return &discord.Timestamps{Start: connectAt.Unix()}
	case preset.TimeSincePresence:
		return &discord.Timestamps{Start: applyAt.Unix()}
	case preset.TimeSinceStart:
		return &discord.Timestamps{Start: processStart.Unix()}
	case preset.TimeLocal:
		return &discord.Timestamps{Start: time.Now().Unix()}
	case preset.TimeCustom:
		ts := &discord.Timestamps{}
		if t, err := parseLocalDateTime(p.TimeStart); err == nil {
			ts.Start = t.Unix()
		}
		if p.TimeEndOn {
			if t, err := parseLocalDateTime(p.TimeEnd); err == nil {
				ts.End = t.Unix()
			}
		}
		if ts.Start == 0 && ts.End == 0 {
			return nil
		}
		return ts
	}
	return nil
}

// parseLocalDateTime parses HTML datetime-local style values
// (YYYY-MM-DDTHH:MM) in the local timezone.
func parseLocalDateTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, errors.New("empty")
	}
	layouts := []string{"2006-01-02T15:04", "2006-01-02T15:04:05"}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("invalid datetime")
}
