# Patches

Reference patches for dependencies. **None of these are currently applied to
AuraRPC** — they are documented here for future use, in case the issue they
address turns out to be common. Applying one means forking the dependency and
adding a `replace` directive (see each entry).

---

## fyne-systray-windows-foreground  ·  status: documented, not applied

**Target:** `fyne.io/systray` (file `systray_windows.go`, function `showMenu`).
**Fixes:** Windows tray menu failing to appear under a foreground lock
(`ForegroundLockTimeout` ≠ 0). See [`docs/en/TRAY.md`](../docs/en/TRAY.md),
failure mode **A**.

`fyne.io/systray` calls `SetForegroundWindow` before `TrackPopupMenu`, but
Windows denies that call when another process holds the foreground lock, so the
popup silently never shows. The fix forces the foreground with the documented
`AttachThreadInput` pattern and posts `WM_NULL` afterwards (KB135788) so the
menu dismisses correctly.

### The change

Replace `showMenu` in `systray_windows.go` with this version (only the
foreground handling and the trailing `WM_NULL` are new):

```go
func (t *winTray) showMenu() error {
	if !wt.isReady() {
		return ErrTrayNotReadyYet
	}

	const (
		TPM_BOTTOMALIGN = 0x0020
		TPM_LEFTALIGN   = 0x0000
		WM_NULL         = 0x0000
	)
	p := point{}
	res, _, err := pGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	if res == 0 {
		return err
	}

	// Force this window to the foreground before opening the popup. A plain
	// SetForegroundWindow is denied by Windows while another process holds the
	// foreground lock (ForegroundLockTimeout != 0), and TrackPopupMenu then
	// silently shows nothing. Briefly attaching our input thread to the
	// foreground window's thread lets the call through.
	u32 := windows.NewLazySystemDLL("user32.dll")
	k32 := windows.NewLazySystemDLL("kernel32.dll")
	getForegroundWindow := u32.NewProc("GetForegroundWindow")
	getWindowThreadProcessId := u32.NewProc("GetWindowThreadProcessId")
	attachThreadInput := u32.NewProc("AttachThreadInput")
	getCurrentThreadId := k32.NewProc("GetCurrentThreadId")

	fg, _, _ := getForegroundWindow.Call()
	cur, _, _ := getCurrentThreadId.Call()
	fgThread, _, _ := getWindowThreadProcessId.Call(fg, 0)
	if fg != 0 && fgThread != cur {
		attachThreadInput.Call(cur, fgThread, 1) // attach
		pSetForegroundWindow.Call(uintptr(t.window))
		attachThreadInput.Call(cur, fgThread, 0) // detach
	} else {
		pSetForegroundWindow.Call(uintptr(t.window))
	}

	res, _, err = pTrackPopupMenu.Call(
		uintptr(t.menus[0]),
		TPM_BOTTOMALIGN|TPM_LEFTALIGN,
		uintptr(p.X),
		uintptr(p.Y),
		0,
		uintptr(t.window),
		0,
	)

	// KB135788: posting a benign message makes the menu dismiss properly when
	// the user clicks outside it.
	pPostMessage.Call(uintptr(t.window), WM_NULL, 0, 0)

	if res == 0 {
		return err
	}
	return nil
}
```

It reuses procs already declared by the library (`pGetCursorPos`,
`pSetForegroundWindow`, `pTrackPopupMenu`, `pPostMessage`) and the already
imported `golang.org/x/sys/windows`, so no other edits are needed.

### How to apply

1. Fork `fyne.io/systray` (e.g. `github.com/Mokita-Studio/systray`).
2. Replace `showMenu` in `systray_windows.go` with the version above; commit and tag.
3. Point AuraRPC at the fork in `go.mod`:
   ```
   replace fyne.io/systray => github.com/Mokita-Studio/systray <tag>
   ```
4. `go mod tidy && go build ./...`. The import path in the code stays
   `fyne.io/systray` — only the module source changes.

### Upstream

This is the standard Win32 fix the library is missing; the goal is to submit it
as a pull request to `fyne.io/systray`. Once merged and released, drop the fork
and the `replace` directive.
