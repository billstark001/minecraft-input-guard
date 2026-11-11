package win32

import (
	"errors"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmClose        = 0x0010
	wmDestroy      = 0x0002
	wmEndSession   = 0x0016
	wmRightUp      = 0x0205
	wmTray         = 0x8001
	wmInputState   = 0x8002
	nimAdd         = 0
	nimDelete      = 2
	nifMessage     = 1
	nifIcon        = 2
	nifTip         = 4
	idiApplication = 32512
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	tpmNoNotify    = 0x0080
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
	Handle             uintptr
	OnInputStateChange func()
	icon               notifyIcon
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
			d.notify(nimAdd)
			return 0
		}
		switch msg {
		case wmTray:
			if lParam == wmRightUp {
				d.showMenu()
			}
			return 0
		case wmInputState:
			if d.OnInputStateChange != nil {
				d.OnInputStateChange()
			}
			return 0
		case wmClose:
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
	class := windowClass{Size: uint32(unsafe.Sizeof(windowClass{})), Proc: callback, Instance: instance, Name: name}
	atom, _, err := registerClassEx.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return nil, fmt.Errorf("RegisterClassExW: %w", err)
	}
	hwnd, _, err := createWindowEx.Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(name)),
		0, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("CreateWindowExW: %w", err)
	}
	d.Handle = hwnd
	icon, _, _ := loadIcon.Call(0, idiApplication)
	d.icon = notifyIcon{
		Size: uint32(unsafe.Sizeof(notifyIcon{})), Window: hwnd, ID: 1,
		Flags: nifMessage | nifIcon | nifTip, Callback: wmTray, Icon: icon,
	}
	copy(d.icon.Tip[:], windows.StringToUTF16("Minecraft Input Guard"))
	if !d.notify(nimAdd) {
		destroyWindow.Call(hwnd)
		return nil, fmt.Errorf("Shell_NotifyIconW: could not add the tray icon")
	}
	return d, nil
}

func (d *Desktop) notify(action uintptr) bool {
	result, _, _ := shellNotifyIcon.Call(action, uintptr(unsafe.Pointer(&d.icon)))
	return result != 0
}

func (d *Desktop) showMenu() {
	menu, _, _ := createPopupMenu.Call()
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)
	label := windows.StringToUTF16Ptr("Exit")
	appendMenu.Call(menu, 0, 1, uintptr(unsafe.Pointer(label)))
	var point Point
	getCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	setForegroundWindow.Call(d.Handle)
	choice, _, _ := trackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd|tpmNoNotify,
		uintptr(point.X), uintptr(point.Y), 0, d.Handle, 0)
	PostMessage(d.Handle, 0, 0, 0)
	if choice == 1 {
		d.Close()
	}
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
	title := windows.StringToUTF16Ptr("Minecraft Input Guard")
	message := windows.StringToUTF16Ptr(err.Error())
	messageBox.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), 0x10)
}
