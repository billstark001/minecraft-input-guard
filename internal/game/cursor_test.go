package game

import "testing"

func TestCursorMode(t *testing.T) {
	for _, c := range []struct {
		flags       uint32
		handle      uintptr
		transparent bool
		want        MouseMode
	}{
		{0, 10, false, MouseCaptured},
		{1, 0, false, MouseCaptured},
		{1, 10, true, MouseCaptured},
		{1, 10, false, MouseReleased},
		{2, 0, false, MouseUnknown},
	} {
		if got := CursorMode(c.flags, c.handle, c.transparent); got != c.want {
			t.Errorf("case %+v: got %d", c, got)
		}
	}
}
