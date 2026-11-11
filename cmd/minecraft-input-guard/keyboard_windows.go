package main

import (
	"sync/atomic"
	"time"

	"minecraft-input-guard/internal/config"
	"minecraft-input-guard/internal/game"
	"minecraft-input-guard/internal/hotkey"
	"minecraft-input-guard/internal/ime"
	"minecraft-input-guard/internal/win32"
)

type inputState struct {
	window       game.Window
	mode         game.MouseMode
	readyProfile uintptr
}

// keyboard runs on the desktop's message thread. The IME monitor publishes
// readiness from its own goroutine so the low-level hook never waits on IME I/O.
type keyboard struct {
	shortcuts *hotkey.Filter
	ime       bool
	state     atomic.Pointer[inputState]
	notify    func()

	sampler      sampler
	pending      hotkey.Deferred
	replayTarget game.Window
	replayed     [256]bool
	lateReleases []hotkey.KeyEvent
}

func (k *keyboard) event(event hotkey.KeyEvent, replay bool) bool {
	if replay {
		return k.filterReplay(event)
	}
	block, mask := k.shortcuts.Event(uint8(event.Key), event.Up, win32.Modifiers, func() bool {
		return win32.MinecraftWindow(win32.Foreground()).Handle != 0
	})
	if mask {
		win32.MaskStartMenu()
	}
	if block || !k.ime || !hotkey.CompositionKey(event.Key) {
		return block
	}

	window, ready := k.scope()
	if event.Up {
		k.replayed[event.Key] = false
	}
	delay := window.Handle != 0 && (!ready || k.pending.Pending()) && !hasSystemModifier()
	block = k.pending.Event(event, delay)
	if ready && k.pending.Pending() {
		k.notify()
	}
	return block
}

func (k *keyboard) scope() (game.Window, bool) {
	window, mode := k.sampler.read(time.Now())
	if mode != game.MouseCaptured {
		window = game.Window{}
	}
	k.pending.SetTarget(window)
	if window.Handle == 0 {
		return window, false
	}
	profile := (win32.Input{}).Layout(window)
	if profile != 0 && !ime.IsManagedProfile(profile) {
		return window, true
	}
	state := k.state.Load()
	return window, profile != 0 && state != nil && state.window == window &&
		state.mode == game.MouseCaptured && state.readyProfile == profile
}

func (k *keyboard) flush() {
	if len(k.lateReleases) != 0 {
		keys := k.lateReleases
		k.lateReleases = nil
		win32.ReplayKeys(keys)
	}
	if !k.pending.Pending() {
		return
	}
	window, ready := k.scope()
	if !ready || !k.pending.Pending() {
		return
	}
	if hasSystemModifier() {
		k.pending.Discard()
		return
	}
	k.pending.Flush(func(_ game.Window, keys []hotkey.KeyEvent) int {
		if win32.Foreground() != window.Handle {
			return 0
		}
		k.replayTarget = window
		return win32.ReplayKeys(keys)
	})
}

func (k *keyboard) filterReplay(event hotkey.KeyEvent) bool {
	if event.Up {
		accepted := k.replayed[event.Key]
		k.replayed[event.Key] = false
		return !accepted
	}
	// SendInput can deliver later than the initial focus check. Drop the
	// replay here if another app or a Minecraft menu took over in between.
	window, mode := k.sampler.read(time.Now())
	accepted := window == k.replayTarget && mode == game.MouseCaptured && !hasSystemModifier()
	if release, late := k.pending.ReplayDown(event.Key, accepted); late {
		k.lateReleases = append(k.lateReleases, release)
		k.notify()
	}
	k.replayed[event.Key] = accepted
	return !accepted
}

func hasSystemModifier() bool {
	return win32.Modifiers()&(config.Win|config.Ctrl|config.Alt) != 0
}
