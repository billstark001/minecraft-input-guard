package win32

import (
	"fmt"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
	"minecraft-input-guard/internal/game"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	imm32    = windows.NewLazySystemDLL("imm32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")

	getForegroundWindow      = user32.NewProc("GetForegroundWindow")
	getWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	getClassName             = user32.NewProc("GetClassNameW")
	getWindowText            = user32.NewProc("GetWindowTextW")
	getGUIThreadInfo         = user32.NewProc("GetGUIThreadInfo")
	postMessage              = user32.NewProc("PostMessageW")
)

type Point struct {
	X, Y int32
}

type Rect struct {
	Left, Top, Right, Bottom int32
}

func Foreground() uintptr {
	hwnd, _, _ := getForegroundWindow.Call()
	return hwnd
}

func identity(hwnd uintptr) game.Window {
	if hwnd == 0 {
		return game.Window{}
	}
	var pid uint32
	thread, _, _ := getWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return game.Window{Handle: hwnd, Thread: uint32(thread), PID: pid}
}

func IsSameWindow(w game.Window) bool {
	return w.Handle != 0 && w.Thread != 0 && w.PID != 0 && identity(w.Handle) == w
}

func MinecraftWindow(hwnd uintptr) game.Window {
	w := identity(hwnd)
	if w.Thread == 0 || w.PID == 0 {
		return game.Window{}
	}
	class := windowString(getClassName, hwnd)
	title := windowString(getWindowText, hwnd)
	if !game.Match(class, title, "javaw.exe") {
		return game.Window{}
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, w.PID)
	if err != nil {
		return game.Window{}
	}
	defer windows.CloseHandle(process)
	var image [1024]uint16
	n := uint32(len(image))
	if windows.QueryFullProcessImageName(process, 0, &image[0], &n) != nil {
		return game.Window{}
	}
	if !game.Match(class, title, filepath.Base(windows.UTF16ToString(image[:n]))) {
		return game.Window{}
	}
	return w
}

func windowString(proc *windows.LazyProc, hwnd uintptr) string {
	var text [256]uint16
	n, _, _ := proc.Call(hwnd, uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)))
	return windows.UTF16ToString(text[:n])
}

type guiThreadInfo struct {
	Size, Flags            uint32
	Active, Focus, Capture uintptr
	MenuOwner, Move, Caret uintptr
	CaretRect              Rect
}

func focusWindow(w game.Window) uintptr {
	var info guiThreadInfo
	info.Size = uint32(unsafe.Sizeof(info))
	ok, _, _ := getGUIThreadInfo.Call(uintptr(w.Thread), uintptr(unsafe.Pointer(&info)))
	if ok != 0 && info.Focus != 0 && identity(info.Focus).Thread == w.Thread {
		return info.Focus
	}
	return w.Handle
}

func PostMessage(hwnd uintptr, msg uint32, wParam, lParam uintptr) error {
	result, _, err := postMessage.Call(hwnd, uintptr(msg), wParam, lParam)
	if result == 0 {
		return fmt.Errorf("PostMessageW(0x%04X): %w", msg, err)
	}
	return nil
}
