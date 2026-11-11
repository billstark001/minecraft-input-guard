package ime

import (
	"time"

	"minecraft-input-guard/internal/game"
)

const (
	checkInterval = 100 * time.Millisecond
	layoutTimeout = 500 * time.Millisecond
)

type Input interface {
	Exists(game.Window) bool
	Layout(game.Window) uintptr
	LocalLayouts() bool
	USLayout() uintptr
	SetLayout(game.Window, uintptr) error
	IMEMode(game.Window, uintptr) (uintptr, error)
	SetIMEMode(game.Window, uintptr, uintptr) error
}

type Guard struct {
	Input          Input
	UseLocalLayout bool
	Report         func(string, ...any)

	window      game.Window
	seenCapture bool
	nextCheck   time.Time
	change      *change
}

type change struct {
	profile   uintptr
	english   uintptr
	requested time.Time
	confirmed bool

	originalMode uintptr
	modeChanged  bool
	tries        int
}

func (g *Guard) Observe(w game.Window, mode game.MouseMode, now time.Time) {
	if w != g.window || mode == game.MouseReleased {
		g.Restore()
		g.window = w
	}
	if w.Handle == 0 || mode != game.MouseCaptured {
		g.seenCapture = false
		return
	}
	if !g.seenCapture {
		g.seenCapture = true
		return
	}
	if now.Before(g.nextCheck) {
		return
	}
	g.nextCheck = now.Add(checkInterval)
	g.update(now)
}

func (g *Guard) update(now time.Time) {
	profile := g.Input.Layout(g.window)
	if profile == 0 {
		return
	}
	if g.change != nil && g.change.english != 0 {
		switch profile {
		case g.change.english:
			g.change.confirmed = true
			return
		case g.change.profile:
			if !g.change.confirmed && now.Sub(g.change.requested) < layoutTimeout {
				return
			}
			if !g.change.confirmed {
				g.log("Minecraft did not activate the US keyboard layout; trying its IME window")
				g.beginMode(profile)
				return
			}
		}
		g.change = nil
	}
	if g.change != nil && g.change.profile != profile {
		g.change = nil
	}
	if !isEastAsian(profile) {
		return
	}
	if g.change == nil {
		if g.UseLocalLayout && g.Input.LocalLayouts() {
			english := g.Input.USLayout()
			if english != 0 {
				if err := g.Input.SetLayout(g.window, english); err == nil {
					g.change = &change{profile: profile, english: english, requested: now}
					g.log("requested US keyboard layout for Minecraft (was 0x%X)", uint32(profile))
					return
				} else {
					g.log("could not switch Minecraft's keyboard layout: %v", err)
				}
			}
		}
		g.beginMode(profile)
		return
	}
	if g.change.tries < 2 {
		g.readAndSuspendMode(false)
	}
}

func (g *Guard) beginMode(profile uintptr) {
	g.change = &change{profile: profile}
	g.readAndSuspendMode(true)
}

func (g *Guard) readAndSuspendMode(first bool) {
	c := g.change
	mode, err := g.Input.IMEMode(g.window, c.profile)
	if err != nil {
		c.tries = 2
		g.log("could not read Minecraft's IME mode: %v", err)
		return
	}
	if first {
		c.originalMode = mode
	}
	if mode == 0 {
		c.tries = 0
		return
	}
	c.tries++
	if err := g.Input.SetIMEMode(g.window, c.profile, 0); err != nil {
		g.log("could not set Minecraft's IME to direct input: %v", err)
		return
	}
	if !c.modeChanged {
		g.log("set Minecraft IME to direct input (profile 0x%X, previous mode 0x%X)", uint32(c.profile), mode)
	}
	c.modeChanged = true
}

func (g *Guard) Restore() {
	c, w := g.change, g.window
	g.change = nil
	g.seenCapture = false
	g.nextCheck = time.Time{}
	if c == nil || !g.Input.Exists(w) {
		return
	}
	current := g.Input.Layout(w)
	if c.english != 0 {
		// A posted switch may still be queued. Post the restoration behind it.
		if current == c.english || current == c.profile && !c.confirmed {
			g.restoreResult(g.Input.SetLayout(w, c.profile))
		}
	} else if c.modeChanged && c.originalMode != 0 && current == c.profile {
		g.restoreResult(g.Input.SetIMEMode(w, c.profile, c.originalMode))
	}
}

func (g *Guard) restoreResult(err error) {
	if err != nil {
		g.log("could not restore Minecraft's input state: %v", err)
	} else {
		g.log("restored or requested restoration of Minecraft's input state")
	}
}

func (g *Guard) log(format string, args ...any) {
	if g.Report != nil {
		g.Report(format, args...)
	}
}

func isEastAsian(profile uintptr) bool {
	switch profile & 0x03FF {
	case 0x04, 0x11, 0x12: // Chinese, Japanese, Korean
		return true
	default:
		return false
	}
}
