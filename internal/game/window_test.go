package game

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		class, title, executable string
		want                     bool
	}{
		{"SDL_app", "Minecraft* 26.3 - 单人游戏", "javaw.exe", true},
		{"GLFW30", "Minecraft 1.16.5", "Javaw.EXE", true},
		{"LWJGL", "Minecraft 1.7.10", "java.exe", true},
		{"GLFW30", "Minecraft Launcher", "MinecraftLauncher.exe", false},
		{"Qt6112QWindowIcon", "Prism Launcher", "prismlauncher.exe", false},
		{"GLFW30", "Another Game", "javaw.exe", false},
	}
	for _, c := range cases {
		if got := Match(c.class, c.title, c.executable); got != c.want {
			t.Errorf("Match(%q, %q, %q) = %t, want %t", c.class, c.title, c.executable, got, c.want)
		}
	}
}
