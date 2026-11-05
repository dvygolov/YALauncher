//go:build windows

package index

import (
	"path/filepath"
	"strings"

	"github.com/dvygolov/yalauncher/internal/win"
)

// ItemFromLNK is provided by the platform layer so that the rest of the
// scanner stays testable on every OS.
var resolveShortcut func(path string) (target, args, description, workingDir string)

func itemFromFile(path string, modified int64, source string) *Item {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(base))
	name := strings.TrimSuffix(base, filepath.Ext(base))
	it := &Item{
		Name:     name,
		Path:     path,
		Source:   source,
		Modified: modified,
	}
	switch ext {
	case ".exe":
		it.Kind = KindExe
		it.ExecutableName = base
		if isUninstaller(base, "") {
			return nil
		}
	case ".lnk":
		it.Kind = KindShortcut
		if resolveShortcut != nil {
			target, args, desc, _ := resolveShortcut(path)
			it.Args = args
			it.Description = desc
			if target != "" {
				it.ExecutableName = filepath.Base(target)
			}
		}
		if isUninstaller(filepath.Base(it.ExecutableName), base) {
			return nil
		}
		if it.Description != "" && strings.EqualFold(strings.TrimSpace(it.Description), it.Name) {
			it.Description = ""
		}
	case ".url":
		it.Kind = KindURL
		if u := readURLTarget(path); u != "" {
			it.Path = path
			it.ExecutableName = u
		}
	case ".appref-ms":
		it.Kind = KindAppRef
		it.ExecutableName = base
	default:
		return nil
	}
	return it
}

// readURLTarget reads the "URL=" line of an Internet Shortcut. Missing or
// broken files simply have no target.
func readURLTarget(path string) string {
	b, err := readSmallFile(path, 64<<10)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if len(line) > 4 && strings.EqualFold(line[:4], "URL=") {
			return line[4:]
		}
	}
	return ""
}

// commonUninstallerNames is copied from Flow Launcher: an exe with one of
// these names is never offered as a program.
var commonUninstallerNames = map[string]bool{
	"uninst.exe":    true,
	"unins000.exe":  true,
	"uninst000.exe": true,
	"uninstall.exe": true,
	"uninstall.lnk": true,
	"unins000.lnk":  true,
}

// commonUninstallerPrefixes is Flow Launcher's list, trimmed to the languages
// that show up on Windows here (en, ru, uk, de, fr).
var commonUninstallerPrefixes = []string{
	"uninstall", "удалить", "видалити", "deinstallieren", "désinstaller",
	"uninst", "desinstalar", "disinstallare", "deinstall", "unins",
}

func isUninstaller(exeName, lnkName string) bool {
	for _, n := range []string{exeName, lnkName} {
		if n == "" {
			continue
		}
		low := strings.ToLower(n)
		if commonUninstallerNames[low] {
			return true
		}
		for _, p := range commonUninstallerPrefixes {
			if strings.HasPrefix(low, p) && (strings.HasSuffix(low, ".exe") || strings.HasSuffix(low, ".lnk")) {
				return true
			}
		}
	}
	return false
}

const (
	csidlPrograms       = 2
	csidlCommonPrograms = 23
)

func init() { resolveShortcut = resolveShortcutImpl }

func knownFolder(id int) string { return win.KnownFolder(id) }

func readSmallFile(path string, limit int64) ([]byte, error) {
	return osReadFileLimit(path, limit)
}
