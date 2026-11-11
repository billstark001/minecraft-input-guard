package win32

import (
	"fmt"
	"unsafe"

	"minecraft-input-guard/internal/game"
)

const (
	spiGetThreadLocalInputSettings = 0x104E
	wmInputLangChangeRequest       = 0x0050
	wmIMEControl                   = 0x0283
	imcGetConversionMode           = 0x0001
	imcSetConversionMode           = 0x0002
	imcGetOpenStatus               = 0x0005
	imcSetOpenStatus               = 0x0006
	smtoBlock                      = 0x0001
	smtoAbortIfHung                = 0x0002
	smtoErrorOnExit                = 0x0020
)

var (
	getKeyboardLayout      = user32.NewProc("GetKeyboardLayout")
	getKeyboardLayoutList  = user32.NewProc("GetKeyboardLayoutList")
	systemParametersInfo   = user32.NewProc("SystemParametersInfoW")
	sendMessageTimeout     = user32.NewProc("SendMessageTimeoutW")
	immGetDefaultIMEWindow = imm32.NewProc("ImmGetDefaultIMEWnd")
)

type Input struct{}

func (Input) Exists(w game.Window) bool { return IsSameWindow(w) }

func (Input) Layout(w game.Window) uintptr {
	hkl, _, _ := getKeyboardLayout.Call(uintptr(w.Thread))
	return hkl
}

func (Input) LocalLayouts() bool {
	var enabled int32
	ok, _, _ := systemParametersInfo.Call(spiGetThreadLocalInputSettings, 0, uintptr(unsafe.Pointer(&enabled)), 0)
	return ok != 0 && enabled != 0
}

func (Input) USLayout() uintptr {
	n, _, _ := getKeyboardLayoutList.Call(0, 0)
	if n == 0 || n > 256 {
		return 0
	}
	layouts := make([]uintptr, n)
	got, _, _ := getKeyboardLayoutList.Call(n, uintptr(unsafe.Pointer(&layouts[0])))
	for _, layout := range layouts[:got] {
		if uint32(layout) == 0x04090409 {
			return layout
		}
	}
	return 0
}

func (p Input) SetLayout(w game.Window, hkl uintptr) error {
	if !p.LocalLayouts() {
		return fmt.Errorf("Windows is using a session-wide input layout")
	}
	if !IsSameWindow(w) {
		return fmt.Errorf("Minecraft window has closed")
	}
	return PostMessage(focusWindow(w), wmInputLangChangeRequest, 0, hkl)
}

func (p Input) IMEMode(w game.Window, profile uintptr) (uintptr, error) {
	command := uintptr(imcGetOpenStatus)
	if profile&0x03FF == 0x04 {
		// Microsoft Pinyin stays open in both Chinese and English modes.
		command = imcGetConversionMode
	}
	return p.imeControl(w, profile, command, 0)
}

func (p Input) SetIMEMode(w game.Window, profile, mode uintptr) error {
	command := uintptr(imcSetOpenStatus)
	if profile&0x03FF == 0x04 {
		command = imcSetConversionMode
	}
	result, err := p.imeControl(w, profile, command, mode)
	if err == nil && result != 0 {
		return fmt.Errorf("IME rejected mode 0x%X (result 0x%X)", mode, result)
	}
	return err
}

func (p Input) imeControl(w game.Window, profile, command, argument uintptr) (uintptr, error) {
	if !IsSameWindow(w) || p.Layout(w) != profile {
		return 0, fmt.Errorf("Minecraft window or input profile changed")
	}
	hwnd, _, _ := immGetDefaultIMEWindow.Call(focusWindow(w))
	if hwnd == 0 || identity(hwnd).Thread != w.Thread {
		return 0, fmt.Errorf("Minecraft has no thread-local IME window")
	}

	// WM_IME_CONTROL is synchronous-only. Keep it off the keyboard hook thread
	// and bound the time spent waiting for the game's message loop.
	var value uintptr
	ok, _, err := sendMessageTimeout.Call(hwnd, wmIMEControl, command, argument,
		smtoBlock|smtoAbortIfHung|smtoErrorOnExit, 35, uintptr(unsafe.Pointer(&value)))
	if ok == 0 {
		return 0, fmt.Errorf("WM_IME_CONTROL(0x%X): %w", command, err)
	}
	return value, nil
}
