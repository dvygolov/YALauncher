//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dvygolov/yalauncher/internal/win"
)

// Autostart uses the per-user Run key. The launcher never needs elevation (it
// only reads folders and starts programs as the current user), so there is no
// scheduled task to keep in sync - unlike a keyboard layout switcher, which
// must run elevated to see keys typed into elevated windows.

// syncAutostart makes the registry match the config.
func (a *App) syncAutostart() {
	exe := win.ExePath()
	if exe == "" {
		logf("autostart: cannot determine the exe path")
		return
	}
	if !a.cfg.Autostart {
		win.DeleteRunValue(appName)
		logf("autostart: disabled")
		return
	}
	if win.SetRunValue(appName, exe, "") {
		logf("autostart: HKCU Run entry set for %s", exe)
	} else {
		logf("autostart: writing the HKCU Run entry failed")
	}
}

// cleanupOldBinary removes the leftover of a previous self-update.
func cleanupOldBinary() {
	cur := win.ExePath()
	if cur == "" {
		return
	}
	if _, err := os.Stat(cur + ".old"); err == nil {
		if err := os.Remove(cur + ".old"); err == nil {
			logf("update: previous binary removed")
		}
	}
}

// addFolderDialog asks for one folder with the shell's picker. The dialog is
// COM based; SHBrowseForFolder with the modern flag is the smallest way to get
// a native window.
func (a *App) addFolderDialog() {
	dir := browseFolder("Добавить папку в индекс")
	if dir == "" {
		return
	}
	for _, f := range a.cfg.Folders {
		if strings.EqualFold(filepath.Clean(f), filepath.Clean(dir)) {
			a.balloon("Папка уже в индексе", dir, win.NIIF_INFO)
			return
		}
	}
	a.cfg.Folders = append(a.cfg.Folders, dir)
	a.saveConfig()
	a.balloon("Папка добавлена", dir+"\nИндекс перестраивается…", win.NIIF_INFO)
	go a.reindex(false)
}
