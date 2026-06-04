# Tray icon and Windows menu reliability

The system tray icon is provided by `fyne.io/systray`. Two interactions:

- **Left-click** → opens the AuraRPC window (registered with `SetOnTapped`).
  This path does **not** use the native popup menu.
- **Right-click** → shows the context menu (preset quick-switch,
  connect/disconnect, show window, quit) via the OS `TrackPopupMenu`.

## Known Windows failure modes

Two distinct, environment-specific problems can affect the tray. They are
different and have different fixes.

### A. The menu does not display (foreground lock)

`TrackPopupMenu` only shows a popup when the owning window is the foreground
window. The library calls `SetForegroundWindow` first, but Windows **rejects**
that call when another process holds the foreground lock
(`HKCU\Control Panel\Desktop\ForegroundLockTimeout` ≠ 0, set by some games and
utilities). The popup then silently fails to appear.

Mitigations:

- **Left-click still opens the window** — it bypasses the menu entirely, so the
  app stays reachable. This is the shipped mitigation.
- A deeper fix exists (force the foreground with the documented
  `AttachThreadInput` pattern, then post `WM_NULL`, KB135788, so the menu
  dismisses correctly). It lives **inside** `fyne.io/systray`, so it is **not
  bundled** with AuraRPC — applying it would mean patching/forking the
  dependency. It is documented in [`patches/`](../../patches/README.md) for
  anyone who needs it; in practice the left-click path makes it optional.

### B. Clicks are not delivered to the app

On some setups the shell does not forward tray click messages to the app at all
— **neither** left nor right click fires. Typical causes:

- The icon lives in the hidden-icons overflow flyout, and/or
- A taskbar/shell modifier (ExplorerPatcher, StartAllBack, 7+ Taskbar Tweaker,
  Rainmeter, …) intercepting tray callbacks.

This is outside the application's control (the click never reaches our code).
Workarounds for the affected user:

- Pin the icon to the taskbar: *Settings → Personalization → Taskbar → other
  system tray icons → AuraRPC on*.
- Restart `explorer.exe`, or reboot.
- Open AuraRPC from its Start-menu / desktop shortcut: launching it again brings
  the already-running instance's window to the front (single-instance ping).

## The foreground patch (documented, not currently applied)

AuraRPC ships **without** this patch — the left-click path already keeps the app
reachable, and bundling the fix would require patching/forking the dependency.
[`patches/`](../../patches/README.md) keeps the exact change and how to apply it
(fork `fyne.io/systray`, patch `showMenu`, point to it with a `replace` in
`go.mod`), in case failure mode A turns out to be common. The cleaner long-term
path is to submit it upstream to `fyne.io/systray`.
