package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"time"

	"minecraft-input-guard/internal/config"
	"minecraft-input-guard/internal/game"
	"minecraft-input-guard/internal/hotkey"
	"minecraft-input-guard/internal/ime"
	"minecraft-input-guard/internal/version"
	"minecraft-input-guard/internal/win32"
)

const sampleInterval = 25 * time.Millisecond

func main() {
	var configPath, logPath string
	var observe, showVersion bool
	flag.StringVar(&configPath, "config", "", "path to JSON configuration (default: config.json beside the executable)")
	flag.StringVar(&logPath, "log", "", "append diagnostic messages to this file")
	flag.BoolVar(&observe, "observe", false, "report mouse and input state without changing input")
	flag.BoolVar(&showVersion, "version", false, "print the application version and exit")
	flag.Parse()

	if showVersion {
		fmt.Println(version.Number())
		return
	}

	logger := log.New(os.Stderr, "", log.Ltime)
	if logPath != "" {
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			win32.ShowError(err)
			return
		}
		defer file.Close()
		logger.SetOutput(file)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if observe {
		observeOnly(ctx, logger)
		return
	}
	optional := configPath == ""
	if optional {
		executable, err := os.Executable()
		if err != nil {
			win32.ShowError(err)
			return
		}
		configPath = filepath.Join(filepath.Dir(executable), "config.json")
	}
	options, err := config.Load(configPath, optional)
	if err == nil {
		err = run(ctx, options, logger)
	}
	if err != nil {
		logger.Print(err)
		win32.ShowError(err)
	}
}

func run(ctx context.Context, options config.Options, logger *log.Logger) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	release, first, err := win32.AcquireInstance()
	if err != nil {
		return err
	}
	if !first {
		logger.Print("Minecraft Input Guard is already running")
		return nil
	}
	defer release()

	desktop, err := win32.NewDesktop()
	if err != nil {
		return err
	}
	keys := &keyboard{
		shortcuts: hotkey.New(options.Blocked, win32.KeyDown),
		ime:       options.IME, notify: desktop.NotifyInputState,
	}
	desktop.OnInputStateChange = keys.flush
	if len(options.Blocked) != 0 || options.IME {
		removeHook, err := win32.InstallKeyboardHook(keys.event)
		if err != nil {
			desktop.Close()
			_ = desktop.Run()
			return err
		}
		defer removeHook()
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if options.IME {
			monitor(ctx, &ime.Guard{
				Input: win32.Input{}, UseLocalLayout: options.UseLocalLayout, Report: logger.Printf,
			}, keys)
		}
	}()
	go func() {
		<-ctx.Done()
		desktop.Close()
	}()
	logger.Printf("running (IME: %t, blocked shortcuts: %d)", options.IME, len(options.Blocked))
	err = desktop.Run()
	cancel()
	<-done
	return err
}

type sampler struct {
	cursor win32.CursorDetector
	last   uintptr
	window game.Window
	until  time.Time
}

func (s *sampler) read(now time.Time) (game.Window, game.MouseMode) {
	hwnd := win32.Foreground()
	if hwnd != s.last || !now.Before(s.until) {
		s.last = hwnd
		s.window = win32.MinecraftWindow(hwnd)
		s.until = now.Add(time.Second)
	}
	if s.window.Handle == 0 {
		return s.window, game.MouseUnknown
	}
	return s.window, s.cursor.Mode()
}

func monitor(ctx context.Context, guard *ime.Guard, keys *keyboard) {
	defer guard.Restore()
	tick := time.NewTicker(sampleInterval)
	defer tick.Stop()
	var state sampler
	var previous inputState
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			window, mode := state.read(now)
			guard.Observe(window, mode, now)
			snapshot := inputState{window: window, mode: mode, readyProfile: guard.ReadyProfile()}
			if snapshot != previous {
				previous = snapshot
				keys.state.Store(&snapshot)
				keys.notify()
			}
		}
	}
}

func observeOnly(ctx context.Context, logger *log.Logger) {
	input := win32.Input{}
	logger.Printf("Windows per-window layouts: %t; US keyboard layout: 0x%X", input.LocalLayouts(), uint32(input.USLayout()))
	logger.Print("read-only observation; Ctrl+C to stop")
	tick := time.NewTicker(sampleInterval)
	defer tick.Stop()
	var state sampler
	var last string
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tick.C:
			window, mode := state.read(now)
			status := "Minecraft is not in the foreground"
			if window.Handle != 0 {
				name := [...]string{"unknown", "released", "captured"}[mode]
				clip, _ := win32.CursorClip()
				status = fmt.Sprintf("Minecraft HWND 0x%X, mouse %s, layout 0x%X, clip %v",
					window.Handle, name, uint32(input.Layout(window)), clip)
			}
			if last != status {
				logger.Print(status)
				last = status
			}
		}
	}
}
