// Package ui owns the Gio window lifecycle. The process lives in the
// tray and the window is opened on demand via Show().
package ui

import (
	"log"
	"runtime"
	"runtime/debug"
	"time"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/font/gofont"
	"gioui.org/font/opentype"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget/material"

	"golang.org/x/image/font/gofont/gomedium"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/gofont/goregular"

	"aurarpc/internal/config"
	"aurarpc/internal/core"
	"aurarpc/internal/i18n"
	"aurarpc/internal/platform"
	"aurarpc/internal/preset"
	"aurarpc/internal/ui/screens"
	"aurarpc/internal/ui/theme"
	"aurarpc/internal/update"
)

// windowTitle is the Win32 window title, also used to locate the HWND
// for caption tinting and icon assignment.
const windowTitle = "AuraRPC"

// Manager owns the single-window lifecycle. Theme and Home are created
// lazily on Show and released on Close to keep idle memory low.
type Manager struct {
	cfgDir  string
	cfg     *config.Config
	mgr     *preset.Manager
	ctrl    *core.Controller
	l       *i18n.Localizer
	updates *update.State

	th     *material.Theme
	home   *screens.Home
	showCh chan struct{}
}

// NewManager builds the manager without opening a window.
func NewManager(cfgDir string, cfg *config.Config, mgr *preset.Manager, ctrl *core.Controller, l *i18n.Localizer, updates *update.State) *Manager {
	if cfg.Theme == "" {
		cfg.Theme = "light"
	}
	theme.SetMode(cfg.Theme)
	return &Manager{
		cfgDir:  cfgDir,
		cfg:     cfg,
		mgr:     mgr,
		ctrl:    ctrl,
		l:       l,
		updates: updates,
		showCh:  make(chan struct{}, 1),
	}
}

// Show requests the window to open. No-op if one is already open or pending.
func (m *Manager) Show() {
	select {
	case m.showCh <- struct{}{}:
	default:
	}
}

// Run consumes showCh and opens a window per signal. Run in a goroutine.
func (m *Manager) Run() {
	for range m.showCh {
		log.Printf("ui: show signal received, opening window")
		if err := m.runWindow(); err != nil {
			log.Printf("ui: window loop: %v", err)
		}
		log.Printf("ui: window closed, releasing UI")
		m.releaseUI()
	}
}

func (m *Manager) initUI() {
	if m.th != nil {
		return
	}
	th := material.NewTheme()
	th.Shaper = text.NewShaper(text.WithCollection(appFonts()))
	theme.Apply(th)
	m.th = th
	m.home = screens.NewHome(m.mgr, m.ctrl, m.cfg, m.cfgDir, th, m.l, m.updates)
	m.home.OnThemeChange = m.refreshTheme
}

// appFonts returns only the Go font faces the UI actually renders: regular
// and medium for text, plus mono for Client IDs and technical values. This
// avoids loading the full collection (12 faces) when 3 are used. Falls back
// to the regular face if a face fails to parse.
func appFonts() []font.FontFace {
	var faces []font.FontFace
	for _, ttf := range [][]byte{goregular.TTF, gomedium.TTF, gomono.TTF} {
		if ff, err := opentype.ParseCollection(ttf); err == nil {
			faces = append(faces, ff[0])
		}
	}
	if len(faces) == 0 {
		return gofont.Regular()
	}
	return faces
}

func (m *Manager) releaseUI() {
	m.th = nil
	m.home = nil
	runtime.GC()
	debug.FreeOSMemory()
}

func (m *Manager) refreshTheme() {
	if m.th != nil {
		theme.Apply(m.th)
	}
	m.tintTitlebar()
}

// tintTitlebar reapplies the native caption color with the active theme.
func (m *Manager) tintTitlebar() {
	platform.TintTitlebar(windowTitle, theme.Chrome, theme.TextPrimary, theme.Mode() == "dark")
}

func (m *Manager) runWindow() error {
	m.initUI()

	win := new(app.Window)
	win.Option(
		app.Title(windowTitle),
		app.Size(unit.Dp(1080), unit.Dp(720)),
		app.MinSize(unit.Dp(900), unit.Dp(560)),
	)
	// Both helpers poll for the HWND in the background, so it is safe
	// to call them before the window actually exists.
	m.tintTitlebar()
	platform.SetWindowIcon(windowTitle)

	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(750 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				win.Invalidate()
			}
		}
	}()
	defer close(stop)

	var ops op.Ops
	for {
		switch e := win.Event().(type) {
		case app.DestroyEvent:
			return e.Err
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)
			m.home.Layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
