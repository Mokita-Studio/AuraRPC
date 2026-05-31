// Package main is the AuraRPC entry point. It wires up the controller,
// system tray and Gio window manager, then blocks on app.Main.
package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"gioui.org/app"

	"aurarpc/internal/config"
	"aurarpc/internal/core"
	"aurarpc/internal/i18n"
	"aurarpc/internal/platform"
	"aurarpc/internal/preset"
	"aurarpc/internal/tray"
	"aurarpc/internal/ui"
	"aurarpc/internal/update"
)

func main() {
	// Tune GC for a long-idle tray app: more aggressive collection and a
	// 40 MB soft memory ceiling.
	debug.SetGCPercent(50)
	debug.SetMemoryLimit(40 << 20)

	// Single-instance lock: a duplicate process pings the live one and exits.
	acquired, showCh := platform.AcquireSingleInstance()
	if !acquired {
		platform.PingExistingInstance()
		return
	}

	dir, err := platform.AppDataDir()
	if err != nil {
		log.Fatalf("appdata: %v", err)
	}

	// Redirect global logger to %APPDATA%\AuraRPC\debug.log, truncated each run.
	if logFile, err := os.OpenFile(filepath.Join(dir, "debug.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		log.SetOutput(logFile)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		defer logFile.Close()
	}

	log.Printf("=== AuraRPC start ===")

	cfg, err := config.Load(dir)
	if err != nil {
		log.Printf("config load: %v (continuing with defaults)", err)
		cfg = config.Default()
	}

	loc, err := i18n.New()
	if err != nil {
		log.Fatalf("i18n: %v", err)
	}
	if cfg.Language != "" && cfg.Language != loc.Lang() {
		if err := loc.SetLanguage(cfg.Language); err != nil {
			log.Printf("i18n: %v (falling back to %s)", err, loc.Lang())
			cfg.Language = loc.Lang()
		}
	}
	if cfg.Language == "" {
		cfg.Language = loc.Lang()
	}

	mgr := preset.NewManager(dir)
	if err := mgr.Load(); err != nil {
		log.Printf("preset load: %v (continuing with empty list)", err)
	}

	ctrl := core.NewController(mgr)
	// Persist last applied preset so AutoConnect has something to use.
	// The saver coalesces rapid preset switching into a single, ordered
	// disk write while keeping the in-memory value current.
	lastPresetSaver := config.NewLastPresetSaver(dir, cfg, 800*time.Millisecond, func(err error) {
		log.Printf("config save last preset: %v", err)
	})
	ctrl.OnLastAppliedChange = lastPresetSaver.Set
	// Update notifications: a single background check that surfaces a link
	// when a newer release exists. Nothing is downloaded or installed.
	updates := update.NewState()

	uiMgr := ui.NewManager(dir, cfg, mgr, ctrl, loc, updates)
	tr := tray.New(ctrl, mgr, loc, uiMgr.Show)

	if cfg.CheckUpdates {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			rel, newer, err := update.Check(ctx)
			if err != nil {
				log.Printf("update check: %v", err)
				return
			}
			if newer {
				log.Printf("update available: %s (%s)", rel.Tag, rel.URL)
				updates.Set(rel)
			} else {
				log.Printf("update check: up to date (%s)", update.CurrentVersion)
			}
		}()
	}

	// AutoConnect: reapply last preset in the background.
	if cfg.AutoConnect && cfg.LastPresetID != "" {
		if p, err := mgr.Get(cfg.LastPresetID); err != nil {
			log.Printf("auto-connect: preset %s not found: %v", cfg.LastPresetID, err)
		} else if err := ctrl.Apply(p); err != nil {
			log.Printf("auto-connect: apply: %v", err)
		} else {
			log.Printf("auto-connect: applied preset %q", p.Name)
		}
	}

	// Tray owns the process lifetime: its onExit calls os.Exit.
	go tr.Run()

	// Window manager opens a window per Show() signal.
	go uiMgr.Run()

	// Surface "show window" pings from duplicate-launch attempts.
	go func() {
		for range showCh {
			log.Printf("singleton: foreign instance pinged us, showing window")
			uiMgr.Show()
		}
	}()

	if !cfg.StartMinimized {
		uiMgr.Show()
	}

	app.Main()
}
