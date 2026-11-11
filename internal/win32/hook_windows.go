package win32

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
	"minecraft-input-guard/internal/config"
)

const (
	whKeyboardLL   = 13
	llkhfUp        = 0x80
	maskKey        = 0xE8
	maskExtra      = 0x4D49474D
	inputKeyboard  = 1
	keyeventfKeyUp = 2
)

var (
	setWindowsHookEx    = user32.NewProc("SetWindowsHookExW")
	unhookWindowsHookEx = user32.NewProc("UnhookWindowsHookEx")
	callNextHookEx      = user32.NewProc("CallNextHookEx")
	getAsyncKeyState    = user32.NewProc("GetAsyncKeyState")
	sendInput           = user32.NewProc("SendInput")
	getModuleHandle     = kernel32.NewProc("GetModuleHandleW")
)

type keyboardEvent struct {
	Key, Scan, Flags, Time uint32
	Extra                  uintptr
}

func InstallKeyboardHook(filter func(key uint8, up bool) bool) (func(), error) {
	callback := windows.NewCallback(func(code int32, message uintptr, event *keyboardEvent) uintptr {
		if code == 0 && event.Key < 256 && event.Extra != maskExtra &&
			filter(uint8(event.Key), event.Flags&llkhfUp != 0) {
			return 1
		}
		result, _, _ := callNextHookEx.Call(0, uintptr(code), message, uintptr(unsafe.Pointer(event)))
		return result
	})
	instance, _, _ := getModuleHandle.Call(0)
	hook, _, err := setWindowsHookEx.Call(whKeyboardLL, callback, instance, 0)
	if hook == 0 {
		return nil, fmt.Errorf("SetWindowsHookExW: %w", err)
	}
	return func() { unhookWindowsHookEx.Call(hook) }, nil
}

func KeyDown(key uint8) bool {
	result, _, _ := getAsyncKeyState.Call(uintptr(key))
	return result&0x8000 != 0
}

func Modifiers() config.Modifiers {
	var value config.Modifiers
	if KeyDown(0x5B) || KeyDown(0x5C) {
		value |= config.Win
	}
	if KeyDown(0x11) {
		value |= config.Ctrl
	}
	if KeyDown(0x12) {
		value |= config.Alt
	}
	if KeyDown(0x10) {
		value |= config.Shift
	}
	return value
}

type keyboardInput struct {
	Key, Scan   uint16
	Flags, Time uint32
	Extra       uintptr
}

type mouseInput struct {
	X, Y              int32
	Data, Flags, Time uint32
	Extra             uintptr
}

type input struct {
	Type uint32
	Key  keyboardInput
	Pad  [unsafe.Sizeof(mouseInput{}) - unsafe.Sizeof(keyboardInput{})]byte
}

func MaskStartMenu() {
	// An unused virtual key keeps an intercepted Win chord from becoming a bare Win tap.
	events := [...]input{
		{Type: inputKeyboard, Key: keyboardInput{Key: maskKey, Extra: maskExtra}},
		{Type: inputKeyboard, Key: keyboardInput{Key: maskKey, Flags: keyeventfKeyUp, Extra: maskExtra}},
	}
	sendInput.Call(uintptr(len(events)), uintptr(unsafe.Pointer(&events[0])), unsafe.Sizeof(events[0]))
}
