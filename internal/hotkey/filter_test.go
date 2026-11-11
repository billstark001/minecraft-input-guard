package hotkey

import (
	"testing"

	"minecraft-input-guard/internal/config"
)

func TestFilterPreservesKeyPairsAcrossFocusChanges(t *testing.T) {
	filter := New([]config.Chord{{Key: 0xBE, Modifiers: config.Win}}, func(uint8) bool { return false })
	for _, event := range []struct {
		key              uint8
		up, inside       bool
		mods             config.Modifiers
		wantBlock, start bool
	}{
		{0x20, false, true, 0, false, false},
		{0xBE, false, false, config.Win, false, false},
		{0xBE, false, true, config.Win, false, false},
		{0xBE, true, true, 0, false, false},
		{0xBE, false, true, config.Win, true, true},
		{0xBE, false, false, 0, true, false},
		{0xBE, true, false, 0, true, false},
		{0xBE, false, true, config.Win | config.Ctrl, false, false},
	} {
		block, start := filter.Event(event.key, event.up, func() config.Modifiers { return event.mods },
			func() bool { return event.inside })
		if block != event.wantBlock || start != event.start {
			t.Errorf("event %+v: got block=%t start=%t", event, block, start)
		}
	}
}
