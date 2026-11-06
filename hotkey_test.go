//go:build windows

package main

import "testing"

func TestParseHotkey(t *testing.T) {
	good := []string{"ctrl+alt+space", "ctrl+space", "alt+`", "win+space", "f13", "ctrl+shift+f1"}
	for _, s := range good {
		hk, err := ParseHotkey(s)
		if err != nil {
			t.Fatalf("ParseHotkey(%q): %v", s, err)
		}
		if hk.VK == 0 {
			t.Fatalf("ParseHotkey(%q) has no key", s)
		}
		if hk.String() != s {
			t.Errorf("round trip %q -> %q", s, hk.String())
		}
	}
	for _, s := range []string{"", "ctrl", "ctrl+nonsense"} {
		if _, err := ParseHotkey(s); err == nil {
			t.Errorf("ParseHotkey(%q) should fail", s)
		}
	}
}

func TestLauncherHandlesKey(t *testing.T) {
	for _, vk := range []uint32{0x41, 0x5A, 0x30, 0x39, 0x20, 0x0D, 0x1B, 0x26, 0x28} {
		if !launcherHandlesKey(vk) {
			t.Errorf("vk=%#x should be handled by the launcher", vk)
		}
	}
	for _, vk := range []uint32{0x2C /* PrntScreen */, 0x70 /* F1 */, 0x87 /* F24 */, 0x13 /* Pause */} {
		if launcherHandlesKey(vk) {
			t.Errorf("vk=%#x must NOT be handled", vk)
		}
	}
}
