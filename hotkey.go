//go:build windows

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dvygolov/yalauncher/internal/win"
)

// Hotkey is a parsed trigger: the modifier bits, the virtual key and the
// scan code (needed to tell right Ctrl from the left one when the user asks
// to distinguish them - Windows does not, so the VK is what matters).
type Hotkey struct {
	Ctrl, Alt, Shift, Win bool
	VK                    uint16
	Name                  string
}

// ParseHotkey understands "ctrl+alt+space", "win+space", "alt+`",
// "ctrl+shift+f1", "f13" and single letters.
func ParseHotkey(s string) (Hotkey, error) {
	var hk Hotkey
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return hk, fmt.Errorf("пустой хоткей")
	}
	parts := strings.Split(s, "+")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		last := i == len(parts)-1
		if !last {
			switch part {
			case "ctrl", "control":
				hk.Ctrl = true
			case "alt":
				hk.Alt = true
			case "shift":
				hk.Shift = true
			case "win", "super", "meta":
				hk.Win = true
			default:
				return hk, fmt.Errorf("неизвестный модификатор %q", part)
			}
			continue
		}
		vk, ok := keyByName(part)
		if !ok {
			return hk, fmt.Errorf("неизвестная клавиша %q", part)
		}
		hk.VK = vk
	}
	if hk.VK == 0 {
		return hk, fmt.Errorf("в хоткее нет клавиши, только модификаторы")
	}
	hk.Name = hk.String()
	return hk, nil
}

func keyByName(name string) (uint16, bool) {
	named := map[string]uint16{
		"space": win.VK_SPACE, "spacebar": win.VK_SPACE,
		"enter": win.VK_RETURN, "return": win.VK_RETURN,
		"tab": win.VK_TAB, "esc": win.VK_ESCAPE, "escape": win.VK_ESCAPE,
		"backspace": win.VK_BACK, "insert": win.VK_INSERT, "delete": win.VK_DELETE,
		"home": win.VK_HOME, "end": win.VK_END, "pageup": win.VK_PRIOR, "pagedown": win.VK_NEXT,
		"left": win.VK_LEFT, "right": win.VK_RIGHT, "up": win.VK_UP, "down": win.VK_DOWN,
		"`": win.VK_OEM_3, "~": win.VK_OEM_3, "ё": win.VK_OEM_3,
		"-": win.VK_OEM_MINUS, "=": win.VK_OEM_PLUS,
		"[": win.VK_OEM_4, "]": win.VK_OEM_6, "\\": win.VK_OEM_5,
		";": win.VK_OEM_1, "'": win.VK_OEM_7, ",": win.VK_OEM_COMMA, ".": win.VK_OEM_PERIOD, "/": win.VK_OEM_2,
		"pause": win.VK_PAUSE, "printscreen": win.VK_SNAPSHOT, "apps": win.VK_APPS,
		"num0": win.VK_NUMPAD0, "divide": win.VK_DIVIDE, "multiply": win.VK_MULTIPLY,
		"subtract": win.VK_SUBTRACT, "add": win.VK_ADD, "decimal": win.VK_DECIMAL,
	}
	if vk, ok := named[name]; ok {
		return vk, true
	}
	if strings.HasPrefix(name, "f") {
		if n, err := strconv.Atoi(name[1:]); err == nil && n >= 1 && n <= 24 {
			return uint16(win.VK_F1 + n - 1), true
		}
	}
	if len(name) == 1 {
		c := name[0]
		if c >= 'a' && c <= 'z' {
			return uint16('A') + uint16(c-'a'), true
		}
		if c >= '0' && c <= '9' {
			return uint16('0') + uint16(c-'0'), true
		}
	}
	return 0, false
}

// String renders the hotkey in the canonical form used in the config.
func (hk Hotkey) String() string {
	var b strings.Builder
	if hk.Ctrl {
		b.WriteString("ctrl+")
	}
	if hk.Alt {
		b.WriteString("alt+")
	}
	if hk.Shift {
		b.WriteString("shift+")
	}
	if hk.Win {
		b.WriteString("win+")
	}
	b.WriteString(keyName(hk.VK))
	return b.String()
}

func keyName(vk uint16) string {
	rev := map[uint16]string{
		win.VK_SPACE: "space", win.VK_RETURN: "enter", win.VK_TAB: "tab",
		win.VK_ESCAPE: "esc", win.VK_BACK: "backspace", win.VK_INSERT: "insert",
		win.VK_DELETE: "delete", win.VK_HOME: "home", win.VK_END: "end",
		win.VK_PRIOR: "pageup", win.VK_NEXT: "pagedown", win.VK_LEFT: "left",
		win.VK_RIGHT: "right", win.VK_UP: "up", win.VK_DOWN: "down",
		win.VK_OEM_3: "`", win.VK_OEM_MINUS: "-", win.VK_OEM_PLUS: "=",
		win.VK_OEM_4: "[", win.VK_OEM_6: "]", win.VK_OEM_5: "\\",
		win.VK_OEM_1: ";", win.VK_OEM_7: "'", win.VK_OEM_COMMA: ",",
		win.VK_OEM_PERIOD: ".", win.VK_OEM_2: "/", win.VK_PAUSE: "pause",
		win.VK_SNAPSHOT: "printscreen", win.VK_APPS: "apps",
	}
	if s, ok := rev[vk]; ok {
		return s
	}
	if vk >= win.VK_F1 && vk <= win.VK_F24 {
		return fmt.Sprintf("f%d", int(vk-win.VK_F1)+1)
	}
	if vk >= 'A' && vk <= 'Z' {
		return strings.ToLower(string(rune(vk)))
	}
	if vk >= '0' && vk <= '9' {
		return string(rune(vk))
	}
	return fmt.Sprintf("vk(0x%02x)", vk)
}

// matches reports whether an event is the press of this hotkey. The caller
// has already folded the event into the modifier state.
func (hk Hotkey) matches(ev win.KeyEvent, mods modifierState) bool {
	if ev.VK != uint32(hk.VK) || !ev.Down {
		return false
	}
	return mods.ctrl == hk.Ctrl && mods.alt == hk.Alt &&
		mods.shift == hk.Shift && mods.win == hk.Win
}

// isModifier reports whether the key is one of the four modifiers.
func isModifier(vk uint32) bool {
	switch vk {
	case win.VK_CONTROL, win.VK_MENU, win.VK_SHIFT, win.VK_LWIN, win.VK_RWIN,
		0xA0, 0xA1, 0xA2, 0xA3, 0xA4, 0xA5:
		return true
	}
	return false
}

type modifierState struct {
	ctrl, alt, shift, win bool
}

func modifierStateFrom(vk uint32, down bool, st modifierState) modifierState {
	switch vk {
	case win.VK_CONTROL, 0xA2, 0xA3: // VK_LCONTROL, VK_RCONTROL
		st.ctrl = down
	case win.VK_MENU, 0xA4, 0xA5: // VK_LMENU, VK_RMENU
		st.alt = down
	case win.VK_SHIFT, 0xA0, 0xA1: // VK_LSHIFT, VK_RSHIFT
		st.shift = down
	case win.VK_LWIN, win.VK_RWIN:
		st.win = down
	}
	return st
}
