package win32

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
	"minecraft-input-guard/internal/config"
	"minecraft-input-guard/internal/hotkey"
)

const (
	whKeyboardLL      = 13
	llkhfUp           = 0x80
	llkhfExtended     = 0x01
	maskKey           = 0xE8
	maskExtra         = 0x4D49474D
	replayExtra       = 0x4D494752
	inputKeyboard     = 1
	keyeventfKeyUp    = 2
	keyeventfExtended = 1
	keyeventfScancode = 8
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

func InstallKeyboardHook(filter func(hotkey.KeyEvent, bool) bool) (func(), error) {
	callback := windows.NewCallback(func(code int32, message uintptr, event *keyboardEvent) uintptr {
		if code == 0 && event.Key < 256 && event.Extra != maskExtra {
			key := hotkey.KeyEvent{
				Key: uint16(event.Key), Scan: uint16(event.Scan),
				Extended: event.Flags&llkhfExtended != 0, Up: event.Flags&llkhfUp != 0,
			}
			if filter(key, event.Extra == replayExtra) {
				return 1
			}
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

func ReplayKeys(keys []hotkey.KeyEvent) int {
	if len(keys) == 0 {
		return 0
	}
	inputs := make([]input, len(keys))
	for i, event := range keys {
		key := keyboardInput{Key: event.Key, Extra: replayExtra}
		if event.Scan != 0 {
			key.Key = 0
			key.Scan = event.Scan
			key.Flags = keyeventfScancode
		}
		if event.Extended {
			key.Flags |= keyeventfExtended
		}
		if event.Up {
			key.Flags |= keyeventfKeyUp
		}
		inputs[i] = input{Type: inputKeyboard, Key: key}
	}
	count, _, _ := sendInput.Call(uintptr(len(inputs)), uintptr(unsafe.Pointer(&inputs[0])), unsafe.Sizeof(inputs[0]))
	return int(count)
}
