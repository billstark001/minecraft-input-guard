# Minecraft Input Guard

A Windows tray utility for Minecraft Java Edition. It temporarily puts the game's input method into direct input while Minecraft captures the mouse, and blocks selected shortcuts while a Minecraft game window is in the foreground. It runs as a separate process and uses the game's Windows window and cursor state.

## Run

Build and open `bin/MinecraftInputGuard.exe`. Right-click **Minecraft Input Guard** in the notification area and select **Exit** to stop it. Close any earlier standalone emoji-shortcut blocker before running this utility.

The default shortcut is `Win + .`. Some keyboards emit this chord for `Fn + Space`, so both trigger the same rule in Minecraft. Plain Space, a bare Win key, and the chord in other applications pass through normally.

While you control the camera, the utility detects the captured cursor. Opening chat or another screen that releases the mouse restores the previous input state. This also restores input on focus loss or normal exit. The window filter recognizes SDL3, GLFW, and LWJGL 2 Minecraft windows when they run under `javaw.exe` or `java.exe` with a Minecraft title.

### Input methods

- In Windows' session-wide input mode, Microsoft Pinyin switches temporarily to half-width direct input, and Microsoft Japanese IME switches temporarily to its closed/direct mode. The active input profile stays selected. The utility remembers the previous conversion or open state.
- If Windows already has **Let me use a different input method for each app window** enabled and the US keyboard layout is installed, the default is to switch Minecraft to that layout temporarily. This keeps IME shortcuts out of gameplay altogether. Set `useLocalLayout` to `false` to prefer direct mode instead. The utility never enables the Windows setting itself.
- Cursor capture is an approximation of gameplay state. A GUI that keeps the mouse captured can continue to use direct input. Legacy Minecraft clients also need their own working support for receiving committed IME text.

## Configure

Copy `config.example.json` to `config.json` beside the executable, then edit it and restart the utility. To keep the file elsewhere, pass `-config <path>`.

```json
{
  "ime": true,
  "useLocalLayout": true,
  "blocked": [
    { "virtualKey": 190, "modifiers": ["Win"] }
  ]
}
```

`virtualKey` is a decimal [Windows virtual-key code](https://learn.microsoft.com/en-us/windows/win32/inputdev/virtual-key-codes). The recognized modifiers are `Win`, `Ctrl`, `Alt`, and `Shift`. Rules match the exact modifier set. Set `blocked` to `[]` to disable shortcut filtering, or set `ime` to `false` to disable input-method management.

## Build and test

Use Go 1.24 or newer on Windows. The Makefile selects Go 1.27.4 by default. Run these targets with GNU Make, or with `mingw32-make` if that is its executable name on your system.

| Command | Action |
| --- | --- |
| `make fmt` | Format Go source |
| `make vet` | Run static checks |
| `make test` | Run unit tests |
| `make check` | Run static checks and tests |
| `make build` | Build the tray executable in `bin/` |
| `make all` | Run checks and build |
| `make run` | Run from source with console diagnostics |
| `make observe` | Observe Minecraft focus, cursor state, and layout without changing input |
| `make clean` | Remove the built executable |

Use `-log <path>` with the built executable to record startup and input-state transitions for troubleshooting. Diagnostics do not log individual keystrokes.
