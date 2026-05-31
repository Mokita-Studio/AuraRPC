//go:build windows

package platform

import (
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

// Single-instance lock + IPC:
//   1. AcquireSingleInstance creates a named mutex; duplicates detect it.
//   2. The duplicate calls PingExistingInstance, which finds the live
//      instance's message window and sends WM_AURARPC_SHOW.
//   3. The original receives the message in its WndProc and pushes to
//      showCh; main consumes it and reopens the window.
//
// Mutex scope is "Local\\" (per user session) so two users can run
// AuraRPC in different sessions without colliding.

const (
	mutexName    = "Local\\AuraRPC-Singleton-2af1c9f4"
	signalClass  = "AuraRPCSignalWnd"
	wmAuraShow   = 0x0400 + 1 // WM_USER + 1
	hwndMessage  = ^uintptr(0) - 2
	errAlreadyEx = 183
)

var (
	procCreateMutexW    = kernel32.NewProc("CreateMutexW")
	procCloseHandle     = kernel32.NewProc("CloseHandle")
	procRegisterClassW  = user32.NewProc("RegisterClassW")
	procDefWindowProcW  = user32.NewProc("DefWindowProcW")
	procCreateWindowExW = user32.NewProc("CreateWindowExW")
	procFindWindowExW   = user32.NewProc("FindWindowExW")
	procGetMessageW     = user32.NewProc("GetMessageW")
	procTranslateMsg    = user32.NewProc("TranslateMessage")
	procDispatchMsgW    = user32.NewProc("DispatchMessageW")
)

// wndclassW mirrors the Win32 WNDCLASSW struct.
type wndclassW struct {
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
}

// msgW mirrors the Win32 MSG struct.
type msgW struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	PtX      int32
	PtY      int32
	LPrivate uint32
}

var (
	siShowCh    = make(chan struct{}, 1)
	siProcOnce  sync.Once
	siWndProcCB uintptr
)

// wndProc handles the message window's messages. It captures wmAuraShow
// and delegates the rest to DefWindowProcW.
func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	if msg == wmAuraShow {
		select {
		case siShowCh <- struct{}{}:
		default:
		}
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

// AcquireSingleInstance tries to acquire the per-user-session lock.
// Returns acquired=true when we are the first instance, along with a
// channel that fires whenever another instance pings us.
//
// CreateMutexW returns a valid handle even when the mutex already
// exists; the duplicate state is reported through the third return of
// proc.Call (captured immediately, unlike syscall.GetLastError()).
func AcquireSingleInstance() (acquired bool, showCh <-chan struct{}) {
	namePtr, err := syscall.UTF16PtrFromString(mutexName)
	if err != nil {
		return true, siShowCh
	}
	h, _, callErr := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if h == 0 {
		return true, siShowCh
	}
	if errno, ok := callErr.(syscall.Errno); ok && errno == syscall.Errno(errAlreadyEx) {
		procCloseHandle.Call(h)
		return false, nil
	}
	go runMessageWindow()
	return true, siShowCh
}

// runMessageWindow registers the message window class, creates the
// HWND_MESSAGE window and runs the blocking message pump.
func runMessageWindow() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	siProcOnce.Do(func() {
		siWndProcCB = syscall.NewCallback(wndProc)
	})

	classPtr, err := syscall.UTF16PtrFromString(signalClass)
	if err != nil {
		return
	}
	hInst, _, _ := procGetModuleHandleW.Call(0)
	wc := wndclassW{
		WndProc:   siWndProcCB,
		Instance:  hInst,
		ClassName: classPtr,
	}
	procRegisterClassW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classPtr)),
		0,
		0,
		0, 0, 0, 0,
		hwndMessage,
		0, hInst, 0,
	)
	if hwnd == 0 {
		return
	}
	var m msgW
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			return
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsgW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// PingExistingInstance finds the live instance's message window and
// signals it to open its UI window. FindWindowExW with HWND_MESSAGE as
// parent is required because message-only windows are not in the
// desktop tree that FindWindowW searches.
func PingExistingInstance() {
	classPtr, err := syscall.UTF16PtrFromString(signalClass)
	if err != nil {
		return
	}
	hwnd, _, _ := procFindWindowExW.Call(
		hwndMessage,
		0,
		uintptr(unsafe.Pointer(classPtr)),
		0,
	)
	if hwnd == 0 {
		return
	}
	procSendMessageW.Call(hwnd, wmAuraShow, 0, 0)
}
