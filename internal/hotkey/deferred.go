package hotkey

import "minecraft-input-guard/internal/game"

type KeyEvent struct {
	Key, Scan uint16
	Extended  bool
	Up        bool
}

type delivery uint8

const (
	idle delivery = iota
	passed
	deferred
	replaying
	releasedBeforeReplay
	discarded
)

// Deferred holds composition keys until the game is ready to accept them.
// A key release follows the disposition of its original press.
type Deferred struct {
	target game.Window
	queue  []KeyEvent
	keys   [256]delivery
	late   [256]KeyEvent
}

func (d *Deferred) SetTarget(target game.Window) {
	if d.target != target {
		d.Discard()
		d.target = target
	}
}

func (d *Deferred) Event(event KeyEvent, delay bool) bool {
	if !CompositionKey(event.Key) {
		return false
	}
	state := d.keys[event.Key]
	if event.Up {
		switch state {
		case replaying:
			d.late[event.Key] = event
			d.keys[event.Key] = releasedBeforeReplay
			return true
		case releasedBeforeReplay:
			return true
		case deferred:
			d.queue = append(d.queue, event)
		}
		d.keys[event.Key] = idle
		return state == deferred || state == discarded
	}
	if state != idle {
		return state != passed
	}
	if !delay {
		d.keys[event.Key] = passed
		return false
	}
	// Leave room for a release for every queued press.
	if len(d.queue) >= 64 {
		d.keys[event.Key] = discarded
		return true
	}
	d.queue = append(d.queue, event)
	d.keys[event.Key] = deferred
	return true
}

func (d *Deferred) Pending() bool { return len(d.queue) != 0 }

// Flush is called only while the captured game window is ready.
func (d *Deferred) Flush(replay func(game.Window, []KeyEvent) int) {
	if !d.Pending() {
		return
	}
	for key, state := range d.keys {
		if state == deferred {
			d.keys[key] = replaying
		}
	}
	queue := d.queue
	d.queue = nil
	count := replay(d.target, queue)
	if count == len(queue) {
		return
	}

	var inserted [256]bool
	for _, event := range queue[:count] {
		inserted[event.Key] = !event.Up
	}
	for _, event := range queue[count:] {
		if d.keys[event.Key] == replaying && !inserted[event.Key] {
			d.keys[event.Key] = discarded
		}
		// If Windows inserted a tap's down but rejected its queued up, release it.
		if event.Up && inserted[event.Key] {
			replay(d.target, []KeyEvent{event})
			inserted[event.Key] = false
		}
	}
}

// ReplayDown acknowledges delivery in the low-level hook. A physical key-up
// can already be in the input queue when SendInput appends the replayed down.
func (d *Deferred) ReplayDown(key uint16, accepted bool) (KeyEvent, bool) {
	switch d.keys[key] {
	case replaying:
		d.keys[key] = discarded
		if accepted {
			d.keys[key] = passed
		}
	case releasedBeforeReplay:
		d.keys[key] = idle
		return d.late[key], accepted
	}
	return KeyEvent{}, false
}

func (d *Deferred) Discard() {
	for key, state := range d.keys {
		if state == deferred {
			d.keys[key] = discarded
		}
	}
	d.queue = d.queue[:0]
}

func CompositionKey(key uint16) bool {
	switch {
	case key >= 0x30 && key <= 0x39, key >= 0x41 && key <= 0x5A:
		return true
	case key >= 0x60 && key <= 0x6F, key >= 0xBA && key <= 0xC0, key >= 0xDB && key <= 0xDF:
		return true
	}
	switch key {
	case 0x08, 0x0D, 0x20, 0xE2: // Backspace, Enter, Space, OEM_102
		return true
	default:
		return false
	}
}
