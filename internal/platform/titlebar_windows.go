//go:build windows

package platform

import (
	"image/color"
	"syscall"
	"time"
	"unsafe"
)

// DWM attributes used to tint the native caption. CAPTION_COLOR,
// TEXT_COLOR and BORDER_COLOR require Windows 11 22000+; the dark-mode
// flag works on Windows 10 1809+. Unsupported attributes return
// E_INVALIDARG and are simply ignored.
const (
	dwmwaUseImmersiveDarkMode    = 20
	dwmwaUseImmersiveDarkModeOld = 19
	dwmwaCaptionColor            = 35
	dwmwaTextColor               = 36
	dwmwaBorderColor             = 34
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	dwmapi               = syscall.NewLazyDLL("dwmapi.dll")
	procFindWindowW      = user32.NewProc("FindWindowW")
	procDwmSetWindowAttr = dwmapi.NewProc("DwmSetWindowAttribute")
)

// colorRef converts an NRGBA value to the Win32 COLORREF format (0x00BBGGRR).
func colorRef(c color.NRGBA) uint32 {
	return uint32(c.B)<<16 | uint32(c.G)<<8 | uint32(c.R)
}

// findWindowByTitle returns the HWND of a top-level window with the
// exact title, or 0 if none exists.
func findWindowByTitle(title string) uintptr {
	ptr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(ptr)))
	return hwnd
}

func setAttr(hwnd uintptr, attr uint32, val uint32) {
	procDwmSetWindowAttr.Call(
		hwnd,
		uintptr(attr),
		uintptr(unsafe.Pointer(&val)),
		unsafe.Sizeof(val),
	)
}

// TintTitlebar paints the native caption with the given background and
// text colors. dark switches Windows to dark-mode chrome icons. Polls
// for the window by title in a goroutine before applying. Safe to call
// before the window exists and again after theme changes.
func TintTitlebar(title string, caption, text color.NRGBA, dark bool) {
	go func() {
		var hwnd uintptr
		for i := 0; i < 80; i++ {
			hwnd = findWindowByTitle(title)
			if hwnd != 0 {
				break
			}
			time.Sleep(40 * time.Millisecond)
		}
		if hwnd == 0 {
			return
		}
		darkFlag := uint32(0)
		if dark {
			darkFlag = 1
		}
		setAttr(hwnd, dwmwaUseImmersiveDarkMode, darkFlag)
		setAttr(hwnd, dwmwaUseImmersiveDarkModeOld, darkFlag)
		cap := colorRef(caption)
		setAttr(hwnd, dwmwaCaptionColor, cap)
		setAttr(hwnd, dwmwaBorderColor, cap)
		setAttr(hwnd, dwmwaTextColor, colorRef(text))
	}()
}
