// Package tray renders the system tray icon and keeps its menu in sync
// with the Controller via a periodic refresh.
package tray

import (
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getlantern/systray"

	"aurarpc/internal/core"
	"aurarpc/internal/i18n"
	"aurarpc/internal/preset"
)

// maxPresetItems is the upper bound of presets shown in the tray menu.
// Slots are preallocated and shown/hidden as the list changes.
const maxPresetItems = 20

// Tray owns the tray icon and its menu.
type Tray struct {
	ctrl   *core.Controller
	mgr    *preset.Manager
	l      *i18n.Localizer
	onShow func()

	statusItem    *systray.MenuItem
	presetsHeader *systray.MenuItem
	presetItems   []*systray.MenuItem
	connectItem   *systray.MenuItem
	showItem      *systray.MenuItem
	quitItem      *systray.MenuItem

	// lastDark caches the taskbar theme so we only swap the icon on a
	// real change. atomic.Bool because refresh() can run concurrently
	// from the ticker and from click handlers.
	lastDark atomic.Bool

	// refreshMu serializes refresh() so the ticker and click handlers
	// never manipulate the native menu at the same time.
	refreshMu sync.Mutex
}

// New builds the Tray. onShow is invoked when the user clicks "Show window".
func New(ctrl *core.Controller, mgr *preset.Manager, l *i18n.Localizer, onShow func()) *Tray {
	return &Tray{ctrl: ctrl, mgr: mgr, l: l, onShow: onShow}
}

// Run starts the tray loop. Blocks until the user clicks Quit.
func (t *Tray) Run() {
	systray.Run(t.onReady, t.onExit)
}

func (t *Tray) onReady() {
	dark := systemDark()
	t.lastDark.Store(dark)
	systray.SetIcon(trayIcon(dark))
	systray.SetTitle("AuraRPC")
	systray.SetTooltip(t.l.T("tray.tooltip"))

	t.statusItem = systray.AddMenuItem(t.l.T("status.disconnected"), "")
	t.statusItem.Disable()
	systray.AddSeparator()

	t.presetsHeader = systray.AddMenuItem(t.l.T("sidebar.title"), "")
	t.presetsHeader.Disable()

	t.presetItems = make([]*systray.MenuItem, maxPresetItems)
	for i := 0; i < maxPresetItems; i++ {
		item := systray.AddMenuItem("", "")
		item.Hide()
		t.presetItems[i] = item
	}

	systray.AddSeparator()
	t.connectItem = systray.AddMenuItem(t.l.T("btn.connect"), "")

	systray.AddSeparator()
	t.showItem = systray.AddMenuItem(t.l.T("tray.show"), "")
	t.quitItem = systray.AddMenuItem(t.l.T("tray.quit"), "")

	t.refresh()

	// One goroutine per item, reading ClickedCh until Quit.
	for i, item := range t.presetItems {
		i, item := i, item
		go func() {
			for range item.ClickedCh {
				t.handlePresetClick(i)
			}
		}()
	}
	go func() {
		for range t.connectItem.ClickedCh {
			t.handleConnectToggle()
		}
	}()
	go func() {
		for range t.showItem.ClickedCh {
			log.Printf("tray: show window clicked")
			if t.onShow != nil {
				t.onShow()
			}
		}
	}()
	go func() {
		for range t.quitItem.ClickedCh {
			log.Printf("tray: quit clicked")
			systray.Quit()
		}
	}()

	go t.refreshLoop()
}

func (t *Tray) onExit() {
	log.Printf("tray: systray.Run returned, exiting process")
	t.ctrl.Disconnect()
	os.Exit(0)
}

func (t *Tray) handleConnectToggle() {
	switch t.ctrl.Status() {
	case "connected", "connecting":
		log.Printf("tray: disconnect clicked")
		t.ctrl.Disconnect()
	default:
		id := t.ctrl.LastAppliedID()
		if id == "" {
			log.Printf("tray: connect clicked but no preset has been applied yet")
			t.refresh()
			return
		}
		p, err := t.mgr.Get(id)
		if err != nil {
			log.Printf("tray: connect: preset %s not found: %v", id, err)
			t.refresh()
			return
		}
		log.Printf("tray: connect clicked, reapplying preset %q", p.Name)
		if err := t.ctrl.Apply(p); err != nil {
			log.Printf("tray: connect: apply: %v", err)
		}
	}
	t.refresh()
}

func (t *Tray) handlePresetClick(i int) {
	list := t.mgr.List()
	if i >= len(list) {
		return
	}
	p := list[i]
	if err := t.ctrl.Apply(p); err != nil {
		log.Printf("tray: apply %q: %v", p.Name, err)
		return
	}
	t.refresh()
}

func (t *Tray) refresh() {
	t.refreshMu.Lock()
	defer t.refreshMu.Unlock()

	// Re-evaluate the OS theme and swap icon only on a real change.
	if dark := systemDark(); dark != t.lastDark.Load() {
		t.lastDark.Store(dark)
		systray.SetIcon(trayIcon(dark))
	}

	list := t.mgr.List()
	activeID := t.ctrl.ActiveID()

	for i := 0; i < maxPresetItems; i++ {
		if i < len(list) {
			label := list[i].Name
			if list[i].ID == activeID {
				label = "· " + label
			}
			t.presetItems[i].SetTitle(label)
			t.presetItems[i].Show()
		} else {
			t.presetItems[i].Hide()
		}
	}

	switch t.ctrl.Status() {
	case "connected":
		t.statusItem.SetTitle(t.l.T("status.connected"))
		t.connectItem.SetTitle(t.l.T("btn.disconnect"))
	case "connecting":
		t.statusItem.SetTitle(t.l.T("status.connecting"))
		t.connectItem.SetTitle(t.l.T("btn.disconnect"))
	default:
		t.statusItem.SetTitle(t.l.T("status.disconnected"))
		t.connectItem.SetTitle(t.l.T("btn.connect"))
	}

	// Static labels are also re-evaluated so language changes propagate
	// without restarting the process.
	t.presetsHeader.SetTitle(t.l.T("sidebar.title"))
	t.showItem.SetTitle(t.l.T("tray.show"))
	t.quitItem.SetTitle(t.l.T("tray.quit"))
}

func (t *Tray) refreshLoop() {
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()
	for range tick.C {
		t.refresh()
	}
}
