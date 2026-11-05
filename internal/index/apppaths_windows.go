//go:build windows

package index

import (
	"path/filepath"
	"strings"

	"github.com/dvygolov/yalauncher/internal/win"
)

// scanAppPaths walks HKCU and HKLM "App Paths": every registered program
// executable, which is where installers put the exe that should be launched.
func scanAppPaths() []*Item {
	const key = `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths`
	var out []*Item
	for _, root := range []uintptr{win.HKEY_CURRENT_USER, win.HKEY_LOCAL_MACHINE} {
		for _, name := range win.SubKeys(root, key) {
			path := win.RegString(root, key+`\`+name, "")
			if path == "" {
				continue
			}
			path = strings.Trim(strings.TrimSpace(path), `"`)
			if !strings.EqualFold(filepath.Ext(path), ".exe") {
				continue
			}
			base := filepath.Base(path)
			if isUninstaller(base, "") {
				continue
			}
			out = append(out, &Item{
				Name:           strings.TrimSuffix(base, filepath.Ext(base)),
				Path:           path,
				Kind:           KindExe,
				ExecutableName: base,
				Source:         "App Paths",
			})
		}
	}
	return out
}
