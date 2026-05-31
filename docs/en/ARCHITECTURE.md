# ARCHITECTURE.md

# Architecture

This document details the internal structure of AuraRPC and the design rules for keeping components isolated.

## Overview

AuraRPC is a native desktop process (Windows; Linux/macOS port in progress) that:
1. Manages a system tray icon and a Gio interface.
2. Communicates with the local Discord client via a named pipe (Windows) or Unix socket (Linux/macOS).
3. Saves data locally in the per-user config directory.

There are no servers or account validations. The only network egress is an
optional update check (one HTTPS request to GitHub).

## Tech Stack

| Component | Technology | Reason |
| --------- | ---------- | ------ |
| **Language** | Go 1.23 | Single binary, minimal RAM consumption. |
| **UI** | Gio v0.9 | Immediate mode GPU rendering; no heavy web engines. |
| **IPC Client** | Native (`os` + `net`) | Exact framing control; avoids `cgo`. |
| **Tray** | `getlantern/systray` | Wrapped and managed internally in `tray.Tray`. |
| **PE Resources** | `tc-hib/winres` | Generates the `.syso` with icons and DPI-aware manifest at build time. |
| **Data** | `encoding/json` | Auditable plain files, no SQLite or extra libraries. |

## Package Structure

```text
cmd/app/                 Entry point and compiled resources (.syso)
internal/
    core/                Central controller (Presets <-> Discord)
    discord/             IPC Client (Handshake, transmissions, reconnection)
    preset/              Data models, validation, and JSON storage
    config/              User preferences (coalesced/atomic writes)
    update/              Optional new-version check (GitHub)
    fsutil/              Atomic file writes (temp + rename)
    i18n/                Embedded translations
    platform/            Per-OS APIs (paths, autostart, single-instance, open URL)
    tray/                Menu and adaptive icon
    ui/                  Window manager and GUI (Gio)
tools/buildres/          PE resource injection tool
```

OS-specific code is isolated with `_windows.go` / `_linux.go` / `_other.go`
suffixes and build tags; shared code has no suffix.

## Dependency Direction

The architecture requires unidirectional dependencies to ensure modularity:

```text
ui -> core -> discord
ui -> preset -> config
tray -> core
* -> i18n
* -> platform
```
- `discord` handles transport only. It is unaware of the UI or presets.
- `core` orchestrates the state between data and the network.
- The interface (`ui`) communicates with Discord exclusively through `core`.

## Concurrency Model (Goroutines)

The system isolates blocking tasks into independent routines:
1. **Main:** Gio event loop (UI). Must never block.
2. **Tray:** Blocked OS thread waiting for tray icon interactions.
3. **Client:** Network loop keeping the IPC connection open in the background, with a blocking reader that detects disconnects immediately.
4. **Single-Instance (Windows):** Blocked native thread listening for `WM_USER+1` messages from duplicate processes.
5. **Refresh:** 1s ticker to dynamically update the tray icon theme (serialized with a mutex).
6. **Update check:** Short-lived goroutine that queries GitHub once at startup and exits.

## Interface Management (Gio)

Gio operates in immediate mode (`Layout(gtx)` runs at 60 FPS):
- To prevent garbage collector (GC) pressure, repetitive elements are pre-allocated in memory during view initialization.
- The theme exposes its colors and metrics as package variables for global and instant mode switching.