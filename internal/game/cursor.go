package game

type MouseMode uint8

const (
	MouseUnknown MouseMode = iota
	MouseReleased
	MouseCaptured
)

const (
	cursorShowing    = 0x01
	cursorSuppressed = 0x02
)

func CursorMode(flags uint32, handle uintptr, transparent bool) MouseMode {
	if flags&cursorSuppressed != 0 {
		return MouseUnknown
	}
	if flags&cursorShowing == 0 || handle == 0 || transparent {
		return MouseCaptured
	}
	return MouseReleased
}
