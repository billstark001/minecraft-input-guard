package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	Win Modifiers = 1 << iota
	Ctrl
	Alt
	Shift
)

type Modifiers uint8

type Chord struct {
	Key       uint8
	Modifiers Modifiers
}

type Options struct {
	IME            bool
	UseLocalLayout bool
	Blocked        []Chord
}

type fileOptions struct {
	IME            bool        `json:"ime"`
	UseLocalLayout bool        `json:"useLocalLayout"`
	Blocked        []fileChord `json:"blocked"`
}

type fileChord struct {
	Key       uint8    `json:"virtualKey"`
	Modifiers []string `json:"modifiers"`
}

func Load(path string, optional bool) (Options, error) {
	raw := fileOptions{
		IME:            true,
		UseLocalLayout: true,
		Blocked:        []fileChord{{Key: 0xBE, Modifiers: []string{"Win"}}},
	}
	file, err := os.Open(path)
	if err != nil {
		if optional && os.IsNotExist(err) {
			return compile(raw)
		}
		return Options{}, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return Options{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Options{}, fmt.Errorf("%s: expected one JSON object", path)
	}
	return compile(raw)
}

func compile(raw fileOptions) (Options, error) {
	options := Options{IME: raw.IME, UseLocalLayout: raw.UseLocalLayout}
	seen := make(map[Chord]bool)
	for i, rule := range raw.Blocked {
		if rule.Key < 0x08 || rule.Key == 0xFF || isModifier(rule.Key) {
			return Options{}, fmt.Errorf("blocked[%d]: virtualKey must be a non-modifier keyboard key", i)
		}

		chord := Chord{Key: rule.Key}
		for _, name := range rule.Modifiers {
			modifier, ok := modifiers[strings.ToLower(name)]
			if !ok || chord.Modifiers&modifier != 0 {
				return Options{}, fmt.Errorf("blocked[%d]: invalid or repeated modifier %q", i, name)
			}
			chord.Modifiers |= modifier
		}
		if seen[chord] {
			return Options{}, fmt.Errorf("blocked[%d]: duplicate shortcut", i)
		}
		seen[chord] = true
		options.Blocked = append(options.Blocked, chord)
	}
	return options, nil
}

var modifiers = map[string]Modifiers{"win": Win, "ctrl": Ctrl, "alt": Alt, "shift": Shift}

func isModifier(key uint8) bool {
	switch key {
	case 0x10, 0x11, 0x12, 0x5B, 0x5C, 0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5:
		return true
	default:
		return false
	}
}
