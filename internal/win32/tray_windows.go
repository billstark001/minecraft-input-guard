package win32

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	appName        = "Minecraft Input Guard"
	menuAbout      = 1
	menuExit       = 2
	mfGrayed       = 0x0001
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	tpmNoNotify    = 0x0080
	whMouseLL      = 14
)

func (d *Desktop) addIcon() bool {
	if !d.notify(nimAdd) {
		return false
	}
	d.modernTray = d.notify(nimSetVersion)
	return true
}

func (d *Desktop) showMenu(point Point) {
	if d.menuOpen {
		return
	}
	menu, _, _ := createPopupMenu.Call()
	if menu == 0 {
		return
	}
	defer destroyMenu.Call(menu)
	addMenuItem(menu, mfGrayed, 0, appName)
	addMenuItem(menu, 0, menuAbout, "&About")
	addMenuItem(menu, 0, menuExit, "E&xit")

	if point == (Point{}) {
		getCursorPos.Call(uintptr(unsafe.Pointer(&point)))
	}
	d.menuOpen = true
	showWindow.Call(d.Handle, swShowNoActivate)
	setForegroundWindow.Call(d.Handle)
	var outsideClickHook, menuKeyboardHook uintptr
	if Foreground() != d.Handle {
		// Windows may deny activation when Explorer's overflow UI owns focus.
		// While tracking this menu, observe outside clicks without consuming them.
		instance, _, _ := getModuleHandle.Call(0)
		outsideClickHook, _, _ = setWindowsHookEx.Call(whMouseLL, d.mouseCallback, instance, 0)
		menuKeyboardHook, _, _ = setWindowsHookEx.Call(whKeyboardLL, d.menuKeyboardCallback, instance, 0)
	}
	choice, _, _ := trackPopupMenu.Call(menu, tpmRightButton|tpmReturnCmd|tpmNoNotify,
		uintptr(point.X), uintptr(point.Y), 0, d.Handle, 0)
	if outsideClickHook != 0 {
		unhookWindowsHookEx.Call(outsideClickHook)
	}
	if menuKeyboardHook != 0 {
		unhookWindowsHookEx.Call(menuKeyboardHook)
	}
	d.menuOpen = false
	PostMessage(d.Handle, 0, 0, 0)
	showWindow.Call(d.Handle, swHide)
	d.notify(nimSetFocus)

	switch choice {
	case menuAbout:
		showAbout()
	case menuExit:
		d.Close()
	}
}

func addMenuItem(menu, flags, id uintptr, text string) {
	label := windows.StringToUTF16Ptr(text)
	appendMenu.Call(menu, flags, id, uintptr(unsafe.Pointer(label)))
}

func (d *Desktop) prepareMenuDismissal() {
	thread := identity(d.Handle).Thread
	d.mouseCallback = windows.NewCallback(func(code int32, message uintptr, point *Point) uintptr {
		if code == 0 && d.menuOpen {
			switch message {
			case 0x0201, 0x0204, 0x0207, 0x020B: // Mouse button down.
				hwnd := windowAt(*point)
				if identity(hwnd).Thread != thread || windowString(getClassName, hwnd) != "#32768" {
					PostMessage(d.Handle, wmCancelMode, 0, 0)
				}
			}
		}
		result, _, _ := callNextHookEx.Call(0, uintptr(code), message, uintptr(unsafe.Pointer(point)))
		return result
	})
	d.menuKeyboardCallback = windows.NewCallback(func(code int32, message uintptr, event *keyboardEvent) uintptr {
		if code == 0 && d.menuOpen && event.Key == 0x1B && event.Flags&llkhfUp == 0 {
			PostMessage(d.Handle, wmCancelMode, 0, 0)
			return 1
		}
		result, _, _ := callNextHookEx.Call(0, uintptr(code), message, uintptr(unsafe.Pointer(event)))
		return result
	})
}

func windowAt(point Point) uintptr {
	if unsafe.Sizeof(uintptr(0)) == 4 {
		result, _, _ := windowFromPoint.Call(uintptr(point.X), uintptr(point.Y))
		return result
	}
	value := uint64(uint32(point.X)) | uint64(uint32(point.Y))<<32
	result, _, _ := windowFromPoint.Call(uintptr(value))
	return result
}

func showAbout() {
	title := windows.StringToUTF16Ptr("About " + appName)
	body := windows.StringToUTF16Ptr(appName + "\r\n\r\nA Windows utility for Minecraft Java Edition.\r\n" +
		"It pauses IME composition during gameplay and blocks configured shortcuts.")
	messageBox.Call(0, uintptr(unsafe.Pointer(body)), uintptr(unsafe.Pointer(title)), 0x10040)
}
