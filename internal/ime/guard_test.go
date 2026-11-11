package ime

import (
	"errors"
	"testing"
	"time"

	"minecraft-input-guard/internal/game"
)

const (
	us       = 0x04090409
	pinyin   = 0x08040804
	japanese = 0x04110411
)

type fakeInput struct {
	layout, pendingLayout  uintptr
	mode                   uintptr
	local, exists          bool
	delay, ignoreLayout    bool
	changes, closes, opens int
	modeFailures           int
}

func (f *fakeInput) Exists(game.Window) bool    { return f.exists }
func (f *fakeInput) Layout(game.Window) uintptr { return f.layout }
func (f *fakeInput) LocalLayouts() bool         { return f.local }
func (f *fakeInput) USLayout() uintptr          { return us }
func (f *fakeInput) IMEMode(game.Window, uintptr) (uintptr, error) {
	if f.modeFailures > 0 {
		f.modeFailures--
		return 0, errors.New("game thread is busy")
	}
	return f.mode, nil
}
func (f *fakeInput) SetLayout(_ game.Window, layout uintptr) error {
	if !f.local {
		return errors.New("layout is global")
	}
	f.changes++
	switch {
	case f.delay:
		f.pendingLayout = layout
	case !f.ignoreLayout:
		f.layout = layout
	}
	return nil
}
func (f *fakeInput) SetIMEMode(_ game.Window, _, mode uintptr) error {
	f.mode = mode
	if mode == 0 {
		f.closes++
	} else {
		f.opens++
	}
	return nil
}

var gameWindow = game.Window{Handle: 1, Thread: 2, PID: 3}

func capture(g *Guard, at time.Time) {
	g.Observe(gameWindow, game.MouseCaptured, at)
	g.Observe(gameWindow, game.MouseCaptured, at.Add(25*time.Millisecond))
}

func TestSuppressAndRestoreBothIMEsWithoutGlobalLayoutChanges(t *testing.T) {
	for _, c := range []struct{ profile, mode uintptr }{{pinyin, 1025}, {japanese, 1}} {
		f := &fakeInput{layout: c.profile, mode: c.mode, exists: true}
		g := Guard{Input: f, UseLocalLayout: true}
		now := time.Now()
		capture(&g, now)
		if f.mode != 0 || f.closes != 1 || f.changes != 0 || f.layout != c.profile {
			t.Fatalf("profile %#x was not suspended in place: %+v", c.profile, f)
		}
		g.Observe(gameWindow, game.MouseReleased, now.Add(time.Second))
		if f.mode != c.mode || f.opens != 1 || f.layout != c.profile {
			t.Fatalf("profile %#x was not restored: %+v", c.profile, f)
		}
	}
}

func TestDirectInputRemainsDirectAfterBlur(t *testing.T) {
	f := &fakeInput{layout: japanese, exists: true}
	g := Guard{Input: f}
	now := time.Now()
	capture(&g, now)
	f.mode = 1
	g.Observe(gameWindow, game.MouseCaptured, now.Add(time.Second))
	g.Observe(game.Window{}, game.MouseUnknown, now.Add(2*time.Second))
	if f.mode != 0 || f.opens != 0 || f.closes != 1 {
		t.Fatalf("original direct input was not preserved: %+v", f)
	}
}

func TestRestoreOriginalLocalLayoutOnExit(t *testing.T) {
	f := &fakeInput{layout: japanese, local: true, exists: true}
	g := Guard{Input: f, UseLocalLayout: true}
	now := time.Now()
	capture(&g, now)
	if f.layout != us {
		t.Fatalf("gameplay layout is %#x, want %#x", f.layout, us)
	}
	g.Observe(gameWindow, game.MouseCaptured, now.Add(time.Second))
	g.Restore()
	if f.layout != japanese || f.changes != 2 {
		t.Fatalf("original layout was not restored: %+v", f)
	}
}

func TestReleaseBeforePostedSwitchIsDelivered(t *testing.T) {
	f := &fakeInput{layout: pinyin, local: true, exists: true, delay: true}
	g := Guard{Input: f, UseLocalLayout: true}
	now := time.Now()
	capture(&g, now)
	g.Observe(gameWindow, game.MouseReleased, now.Add(50*time.Millisecond))
	if f.pendingLayout != pinyin || f.changes != 2 {
		t.Fatalf("restoration was not queued after the pending change: %+v", f)
	}
}

func TestRejectedLayoutRequestFallsBackToIME(t *testing.T) {
	f := &fakeInput{layout: pinyin, local: true, exists: true, mode: 1025, ignoreLayout: true}
	g := Guard{Input: f, UseLocalLayout: true}
	now := time.Now()
	capture(&g, now)
	g.Observe(gameWindow, game.MouseCaptured, now.Add(time.Second))
	if f.mode != 0 || f.closes != 1 {
		t.Fatalf("IME fallback was not activated: %+v", f)
	}
}

func TestRestoreDoesNotOverwriteAnotherProfile(t *testing.T) {
	f := &fakeInput{layout: pinyin, local: true, exists: true}
	g := Guard{Input: f, UseLocalLayout: true}
	capture(&g, time.Now())
	f.layout = japanese
	g.Restore()
	if f.layout != japanese || f.changes != 1 {
		t.Fatalf("manually selected profile was overwritten: %+v", f)
	}
}

func TestRetryWhenWorldLoadingDelaysIMEQuery(t *testing.T) {
	f := &fakeInput{layout: pinyin, mode: 1025, exists: true, modeFailures: 1}
	g := Guard{Input: f}
	now := time.Now()
	capture(&g, now)
	if f.closes != 0 {
		t.Fatalf("IME mode changed without a successful snapshot: %+v", f)
	}
	g.Observe(gameWindow, game.MouseCaptured, now.Add(time.Second))
	if f.mode != 0 || f.closes != 1 {
		t.Fatalf("IME mode was not suspended after the game became responsive: %+v", f)
	}
	g.Observe(gameWindow, game.MouseReleased, now.Add(2*time.Second))
	if f.mode != 1025 {
		t.Fatalf("original IME mode was not restored: %+v", f)
	}
}
