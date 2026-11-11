package game

import (
	"strings"
)

type Window struct {
	Handle uintptr
	Thread uint32
	PID    uint32
}

func Match(class, title, executable string) bool {
	switch class {
	case "SDL_app", "GLFW30", "LWJGL":
	default:
		return false
	}
	if title != "Minecraft" && !strings.HasPrefix(title, "Minecraft ") && !strings.HasPrefix(title, "Minecraft*") {
		return false
	}
	return strings.EqualFold(executable, "javaw.exe") || strings.EqualFold(executable, "java.exe")
}
