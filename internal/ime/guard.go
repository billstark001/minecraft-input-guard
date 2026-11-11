package ime

import (
	"time"

	"minecraft-input-guard/internal/game"
)

const (
	checkInterval = 100 * time.Millisecond
	layoutTimeout = 500 * time.Millisecond
	retryInterval = 250 * time.Millisecond
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

	window       game.Window
	seenCapture  bool
	readyProfile uintptr
	nextCheck    time.Time
	change       *change
}

type change struct {
	profile   uintptr
	english   uintptr
	requested time.Time
	confirmed bool

	originalMode uintptr
	modeTouched  bool
	modeKnown    bool
	warned       bool
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
			g.readyProfile = profile
			return
		case g.change.profile:
			if !g.change.confirmed && now.Sub(g.change.requested) < layoutTimeout {
				return
			}
			if !g.change.confirmed {
				g.log("Minecraft did not activate the US keyboard layout; trying its IME window")
				g.beginMode(profile, now)
				return
			}
		}
		g.change = nil
	}
	if g.change != nil && g.change.profile != profile {
		g.change = nil
	}
	if !IsManagedProfile(profile) {
		g.readyProfile = profile
		return
	}
	if g.change == nil {
		g.readyProfile = 0
		if g.UseLocalLayout && g.Input.LocalLayouts() {
			english := g.Input.USLayout()
			if english != 0 {
				if err := g.Input.SetLayout(g.window, english); err == nil {
					g.change = &change{profile: profile, english: english, requested: now}
					g.nextCheck = now.Add(25 * time.Millisecond)
					g.log("requested US keyboard layout for Minecraft (was 0x%X)", uint32(profile))
					return
				} else {
					g.log("could not switch Minecraft's keyboard layout: %v", err)
				}
			}
		}
		g.beginMode(profile, now)
		return
	}
	g.readAndSuspendMode(now)
}

func (g *Guard) beginMode(profile uintptr, now time.Time) {
	g.change = &change{profile: profile}
	g.readAndSuspendMode(now)
}

func (g *Guard) readAndSuspendMode(now time.Time) {
	c := g.change
	mode, err := g.Input.IMEMode(g.window, c.profile)
	if err != nil {
		if !c.warned {
			g.log("could not read Minecraft's IME mode; will retry: %v", err)
			c.warned = true
		}
		g.nextCheck = now.Add(retryInterval)
		return
	}
	if !c.modeKnown {
		c.originalMode = mode
		c.modeKnown = true
	}
	if mode == 0 {
		g.readyProfile = c.profile
		c.tries = 0
		c.warned = false
		return
	}
	g.readyProfile = 0
	if c.tries == 2 {
		if !c.warned {
			g.log("Minecraft's IME still reports composition mode; will retry")
			c.warned = true
		}
		c.tries = 0
		g.nextCheck = now.Add(retryInterval)
		return
	}
	c.tries++
	first := !c.modeTouched
	c.modeTouched = true
	if err := g.Input.SetIMEMode(g.window, c.profile, 0); err != nil {
		if !c.warned {
			g.log("could not set Minecraft's IME to direct input; will retry: %v", err)
			c.warned = true
		}
		g.nextCheck = now.Add(retryInterval)
	} else {
		g.nextCheck = now.Add(25 * time.Millisecond)
		if first {
			g.log("set Minecraft IME to direct input (profile 0x%X, previous mode 0x%X)", uint32(c.profile), mode)
		}
	}
}

func (g *Guard) Restore() {
	c, w := g.change, g.window
	g.change = nil
	g.readyProfile = 0
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
	} else if c.modeTouched && c.originalMode != 0 && current == c.profile {
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

func (g *Guard) ReadyProfile() uintptr { return g.readyProfile }

func IsManagedProfile(profile uintptr) bool {
	switch profile & 0x03FF {
	case 0x04, 0x11, 0x12: // Chinese, Japanese, Korean
		return true
	default:
		return false
	}
}
