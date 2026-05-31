# CONFIGURATION.md

# Configuration and Data

AuraRPC saves user state in JSON files under the per-user config directory:

- **Windows:** `%APPDATA%\AuraRPC\`
- **Linux:** `$XDG_CONFIG_HOME/AuraRPC` (or `~/.config/AuraRPC`)
- **macOS:** `~/Library/Application Support/AuraRPC`

## Structure

```text
<config-dir>/AuraRPC/
    config.json          General settings (theme, language, autostart, updates)
    presets.json         Saved activity profiles
    presets.json.bak     Backup of the last save
    debug.log            Execution log (cleared on restart)
```

## config.json

```json
{
  "language": "en",
  "theme": "dark",
  "start_with_system": false,
  "start_minimized": false,
  "last_preset_id": "",
  "auto_connect": true,
  "sidebar_open": true,
  "check_updates": true
}
```
- `start_with_system` reflects the OS auto-start mechanism (the `HKCU\...\Run`
  registry key on Windows; a `.desktop` entry on Linux).
- `check_updates` enables the new-version check on launch (a single HTTPS
  request to GitHub; nothing is downloaded or installed).
- `last_preset_id` is persisted coalesced: rapid preset switching collapses
  into a single disk write instead of hammering the disk.

## presets.json

Stores the configured profiles, wrapped with an application id and schema
version for cross-version compatibility:

```json
{ "app": "aurarpc", "version": 1, "presets": [ ... ] }
```

- Unknown fields from future versions are ignored on read, and an older build
  can still read a newer file (backward and forward compatible).
- Character limits (Discord requires a minimum of 2 in details/state) are
  validated by the internal API.
- Writes are **atomic** (temp file + `rename`) and a `.bak` copy of the last
  good state is kept before overwriting.

## System Integration

- **Autostart:**
  - Windows: `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` key.
  - Linux: `~/.config/autostart/AuraRPC.desktop` (freedesktop standard).
  - macOS: not implemented yet (the toggle is a no-op).
- **Dynamic Theme (Windows):** Reads `HKCU\SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize` to adapt the tray icon to the OS light or dark mode.
- **Single-instance (Windows):** Creates the local Mutex `Local\AuraRPC-Singleton-2af1c9f4` to prevent duplicate executions in the same session. Not yet implemented on other platforms.
- **Tray icon:** ICO on Windows; PNG on Linux/macOS (what `systray` expects).