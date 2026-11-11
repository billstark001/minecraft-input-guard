package hotkey

import (
	"slices"
	"testing"

	"minecraft-input-guard/internal/game"
)

func TestDeferredPressReplaysOnlyWhenReady(t *testing.T) {
	var d Deferred
	window := game.Window{Handle: 1}
	d.SetTarget(window)
	down := KeyEvent{Key: 'W', Scan: 0x11}
	up := down
	up.Up = true

	if !d.Event(down, true) || !d.Event(down, true) {
		t.Fatal("original and repeat reached the IME before the game was ready")
	}
	d.Flush(func(target game.Window, events []KeyEvent) int {
		if target != window || !slices.Equal(events, []KeyEvent{down}) {
			t.Fatalf("unexpected replay to %+v: %+v", target, events)
		}
		return len(events)
	})
	d.ReplayDown(down.Key, true)
	if d.Pending() || d.Event(up, false) {
		t.Fatal("release did not follow replayed key-down")
	}
}

func TestDeferredTapAndFocusLoss(t *testing.T) {
	var d Deferred
	d.SetTarget(game.Window{Handle: 1})
	down := KeyEvent{Key: 0x20, Scan: 0x39}
	up := down
	up.Up = true
	if !d.Event(down, true) || !d.Event(up, true) {
		t.Fatal("short tap escaped the gate")
	}
	d.Flush(func(_ game.Window, events []KeyEvent) int {
		if !slices.Equal(events, []KeyEvent{down, up}) {
			t.Fatalf("tap replay was incomplete: %+v", events)
		}
		return len(events)
	})

	d.Event(down, true)
	d.SetTarget(game.Window{})
	if d.Pending() || !d.Event(up, false) {
		t.Fatal("the pending input or its orphaned release escaped the game")
	}
	if d.Event(down, false) || d.Event(up, false) {
		t.Fatal("a later press outside Minecraft was intercepted")
	}
}

func TestReleaseArrivingBeforeReplayAcknowledgement(t *testing.T) {
	var d Deferred
	d.SetTarget(game.Window{Handle: 1})
	down := KeyEvent{Key: 'W', Scan: 0x11}
	up := down
	up.Up = true
	d.Event(down, true)
	d.Flush(func(_ game.Window, keys []KeyEvent) int { return len(keys) })
	if !d.Event(up, false) {
		t.Fatal("release reached Windows before its delayed press")
	}
	if late, ok := d.ReplayDown(down.Key, true); !ok || late != up {
		t.Fatalf("late release was not scheduled: %+v %t", late, ok)
	}
}
