package win32

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmClose          = 0x0010
	wmDestroy        = 0x0002
	wmEndSession     = 0x0016
	wmCancelMode     = 0x001F
	wmContextMenu    = 0x007B
	wmRightUp        = 0x0205
	wmTray           = 0x8001
	wmInputState     = 0x8002
	nimAdd           = 0
	nimDelete        = 2
	nimSetFocus      = 3
	nimSetVersion    = 4
	nifMessage       = 1
	nifIcon          = 2
	nifTip           = 4
	nifShowTip       = 0x80
	trayVersion      = 4
	idiApplication   = 32512
	appIcon          = 101
	wsPopup          = 0x80000000
	wsExToolWindow   = 0x00000080
	swHide           = 0
	swShowNoActivate = 4
)

var (
	createMutex           = kernel32.NewProc("CreateMutexW")
	registerClassEx       = user32.NewProc("RegisterClassExW")
	createWindowEx        = user32.NewProc("CreateWindowExW")
	defWindowProc         = user32.NewProc("DefWindowProcW")
	destroyWindow         = user32.NewProc("DestroyWindow")
	postQuitMessage       = user32.NewProc("PostQuitMessage")
	getMessage            = user32.NewProc("GetMessageW")
	translateMessage      = user32.NewProc("TranslateMessage")
	dispatchMessage       = user32.NewProc("DispatchMessageW")
	loadIcon              = user32.NewProc("LoadIconW")
	createPopupMenu       = user32.NewProc("CreatePopupMenu")
	appendMenu            = user32.NewProc("AppendMenuW")
	trackPopupMenu        = user32.NewProc("TrackPopupMenu")
	destroyMenu           = user32.NewProc("DestroyMenu")
	getCursorPos          = user32.NewProc("GetCursorPos")
	setForegroundWindow   = user32.NewProc("SetForegroundWindow")
	showWindow            = user32.NewProc("ShowWindow")
	endMenu               = user32.NewProc("EndMenu")
	windowFromPoint       = user32.NewProc("WindowFromPoint")
	registerWindowMessage = user32.NewProc("RegisterWindowMessageW")
	messageBox            = user32.NewProc("MessageBoxW")
	shellNotifyIcon       = shell32.NewProc("Shell_NotifyIconW")
)

type windowClass struct {
	Size, Style                   uint32
	Proc                          uintptr
	ClassExtra, WindowExtra       int32
	Instance, Icon, Cursor, Brush uintptr
	Menu, Name                    *uint16
	SmallIcon                     uintptr
}

type message struct {
	Window         uintptr
	Code           uint32
	WParam, LParam uintptr
	Time           uint32
	Point          Point
	Private        uint32
}

type notifyIcon struct {
	Size                uint32
	Window              uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Version             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                windows.GUID
	BalloonIcon         uintptr
}

type Desktop struct {
	Handle               uintptr
	OnInputStateChange   func()
	icon                 notifyIcon
	modernTray           bool
	menuOpen             bool
	mouseCallback        uintptr
	menuKeyboardCallback uintptr
}

func AcquireInstance() (func(), bool, error) {
	name := windows.StringToUTF16Ptr("Local\\MinecraftInputGuard")
	handle, _, err := createMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if handle == 0 {
		return nil, false, fmt.Errorf("CreateMutexW: %w", err)
	}
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(windows.Handle(handle))
		return nil, false, nil
	}
	return func() { windows.CloseHandle(windows.Handle(handle)) }, true, nil
}

func NewDesktop() (*Desktop, error) {
	d := new(Desktop)
	instance, _, _ := getModuleHandle.Call(0)
	taskbarName := windows.StringToUTF16Ptr("TaskbarCreated")
	taskbarCreated, _, _ := registerWindowMessage.Call(uintptr(unsafe.Pointer(taskbarName)))
	callback := windows.NewCallback(func(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
		if uintptr(msg) == taskbarCreated && taskbarCreated != 0 && d.icon.Window != 0 {
			d.addIcon()
			return 0
		}
		switch msg {
		case wmTray:
			if d.modernTray && uint16(lParam) == wmContextMenu {
				d.showMenu(Point{X: int32(int16(wParam)), Y: int32(int16(wParam >> 16))})
			} else if !d.modernTray && lParam == wmRightUp {
				var point Point
				getCursorPos.Call(uintptr(unsafe.Pointer(&point)))
				d.showMenu(point)
			}
			return 0
		case wmInputState:
			if d.OnInputStateChange != nil {
				d.OnInputStateChange()
			}
			return 0
		case wmCancelMode:
			if d.menuOpen {
				endMenu.Call()
			}
		case wmClose:
			if d.menuOpen {
				endMenu.Call()
			}
			destroyWindow.Call(hwnd)
			return 0
		case wmEndSession:
			if wParam != 0 {
				destroyWindow.Call(hwnd)
			}
			return 0
		case wmDestroy:
			d.notify(nimDelete)
			postQuitMessage.Call(0)
			return 0
		}
		result, _, _ := defWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
		return result
	})

	name := windows.StringToUTF16Ptr("MinecraftInputGuard")
	icon, _, _ := loadIcon.Call(instance, appIcon)
	if icon == 0 {
		icon, _, _ = loadIcon.Call(0, idiApplication)
	}
	class := windowClass{
		Size: uint32(unsafe.Sizeof(windowClass{})), Proc: callback, Instance: instance, Name: name,
		Icon: icon, SmallIcon: icon,
	}
	atom, _, err := registerClassEx.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return nil, fmt.Errorf("RegisterClassExW: %w", err)
	}
	x, y := int32(-32000), int32(-32000)
	hwnd, _, err := createWindowEx.Call(wsExToolWindow, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)),
		wsPopup, uintptr(x), uintptr(y), 1, 1, 0, 0, instance, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowExW: %w", err)
	}
	d.Handle = hwnd
	d.prepareMenuDismissal()
	d.icon = notifyIcon{
		Size: uint32(unsafe.Sizeof(notifyIcon{})), Window: hwnd, ID: 1,
		Flags: nifMessage | nifIcon | nifTip | nifShowTip, Callback: wmTray, Icon: icon, Version: trayVersion,
	}
	copy(d.icon.Tip[:], windows.StringToUTF16(appName))
	if !d.addIcon() {
		destroyWindow.Call(hwnd)
		return nil, fmt.Errorf("Shell_NotifyIconW: could not add the tray icon")
	}
	return d, nil
}

func (d *Desktop) notify(action uintptr) bool {
	result, _, _ := shellNotifyIcon.Call(action, uintptr(unsafe.Pointer(&d.icon)))
	return result != 0
}

func (d *Desktop) NotifyInputState() {
	PostMessage(d.Handle, wmInputState, 0, 0)
}

func (d *Desktop) Close() {
	PostMessage(d.Handle, wmClose, 0, 0)
}

func (d *Desktop) Run() error {
	var m message
	for {
		result, _, err := getMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		switch int32(result) {
		case -1:
			return fmt.Errorf("GetMessageW: %w", err)
		case 0:
			return nil
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&m)))
		dispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func ShowError(err error) {
	title := windows.StringToUTF16Ptr(appName)
	message := windows.StringToUTF16Ptr(err.Error())
	messageBox.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
