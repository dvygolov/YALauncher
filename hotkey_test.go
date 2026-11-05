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
