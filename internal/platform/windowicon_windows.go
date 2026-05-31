//go:build windows

package platform

import (
	"syscall"
	"time"
	"unsafe"
)

// Win32 constants for WM_SETICON / LoadImage.
const (
	wmSetIcon = 0x0080

	iconSmall = 0
	iconBig   = 1

	imageIcon     = 1
	lrDefaultSize = 0x00000040
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procLoadImageW       = user32.NewProc("LoadImageW")
	procSendMessageW     = user32.NewProc("SendMessageW")
	procGetSystemMetrics = user32.NewProc("GetSystemMetrics")
)

const (
	smCXSMICON = 49
	smCYSMICON = 50
	smCXICON   = 11
	smCYICON   = 12
)

// SetWindowIcon attaches the resource ID 1 icon (embedded by the .syso)
// to the window with the given title via WM_SETICON. Covers the caption
// icon and the taskbar / alt-tab icon. If the .syso is missing, the
// LoadImage call fails silently and the default OS icon stays.
func SetWindowIcon(title string) {
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
		hInst, _, _ := procGetModuleHandleW.Call(0)
		if hInst == 0 {
			return
		}

		smallW := getSystemMetric(smCXSMICON)
		smallH := getSystemMetric(smCYSMICON)
		bigW := getSystemMetric(smCXICON)
		bigH := getSystemMetric(smCYICON)

		// MAKEINTRESOURCE(1): integer passed as a pointer; Win32 reads
		// the high bits to distinguish numeric IDs from string names.
		const resID = 1
		hSmall := loadIcon(hInst, resID, smallW, smallH)
		hBig := loadIcon(hInst, resID, bigW, bigH)

		if hSmall != 0 {
			procSendMessageW.Call(hwnd, wmSetIcon, iconSmall, hSmall)
		}
		if hBig != 0 {
			procSendMessageW.Call(hwnd, wmSetIcon, iconBig, hBig)
		}
	}()
}

func loadIcon(hInst uintptr, resID uintptr, w, h int) uintptr {
	flags := uintptr(0)
	if w == 0 || h == 0 {
		flags |= lrDefaultSize
	}
	h2, _, _ := procLoadImageW.Call(
		hInst,
		resID,
		uintptr(imageIcon),
		uintptr(w),
		uintptr(h),
		flags,
	)
	return h2
}

func getSystemMetric(idx int) int {
	v, _, _ := procGetSystemMetrics.Call(uintptr(idx))
	return int(int32(v))
}

// Anchor unsafe so goimports keeps the import even if every direct use
// disappears in the future.
var _ = unsafe.Sizeof(uintptr(0))
