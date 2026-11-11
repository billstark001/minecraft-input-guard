package hotkey

import "minecraft-input-guard/internal/config"

type Filter struct {
	rules map[config.Chord]struct{}
	keys  map[uint8]press
}

type press struct {
	down, blocked bool
}

func New(rules []config.Chord, held func(uint8) bool) *Filter {
	filter := &Filter{rules: make(map[config.Chord]struct{}), keys: make(map[uint8]press)}
	for _, rule := range rules {
		filter.rules[rule] = struct{}{}
		filter.keys[rule.Key] = press{down: held(rule.Key)}
	}
	return filter
}

// Event keeps the disposition of one key press through repeats and release.
func (f *Filter) Event(key uint8, up bool, modifiers func() config.Modifiers, inMinecraft func() bool) (block, maskStart bool) {
	state, watched := f.keys[key]
	if !watched {
		return false, false
	}
	if up {
		f.keys[key] = press{}
		return state.blocked, false
	}
	if state.down {
		return state.blocked, false
	}

	active := modifiers()
	_, matched := f.rules[config.Chord{Key: key, Modifiers: active}]
	block = matched && inMinecraft()
	f.keys[key] = press{down: true, blocked: block}
	return block, block && active&config.Win != 0
}
