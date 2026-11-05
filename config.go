//go:build windows

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/dvygolov/yalauncher/internal/index"
	"github.com/dvygolov/yalauncher/internal/search"
)

// Config lives in %APPDATA%\YALauncher\config.json. It is rewritten by the app
// (usage counts, window preferences) and is also the place to hand-edit the
// indexed folders or the hotkey.
type Config struct {
	// Hotkey is the trigger, written like "ctrl+alt+space", "ctrl+space" or
	// "alt+`". Modifiers: ctrl, alt, shift, win. The last part is a key name.
	Hotkey string `json:"hotkey"`

	// Folders are indexed in addition to the start menu. Each entry is a
	// directory; sub folders are scanned too.
	Folders []string `json:"folders"`
	// StartMenu indexes the two Start Menu trees (the default source of both
	// Flow Launcher and this launcher).
	StartMenu bool `json:"startMenu"`
	// AppPaths indexes the registry App Paths keys.
	AppPaths bool `json:"appPaths"`
	// EnvPath indexes the folders of the PATH variable (non recursive).
	EnvPath bool `json:"envPath"`
	// Recursive descends into sub folders of Folders.
	Recursive bool `json:"recursive"`

	// Precision is "loose", "regular" or "strict" - Flow Launcher's search
	// precision setting, which trades recall for noise.
	Precision string `json:"precision"`

	Autostart  bool `json:"autostart"`
	AutoUpdate bool `json:"autoUpdate"`
	// UpdateCheckHours is how often the updater polls GitHub.
	UpdateCheckHours int `json:"updateCheckHours"`
	// ReindexMinutes is the fallback rescan interval; file system
	// notifications trigger an immediate rescan on their own.
	ReindexMinutes int `json:"reindexMinutes"`
	// HideUninstallers drops uninst.exe / uninstall.lnk style entries.
	HideUninstallers bool `json:"hideUninstallers"`

	// History keeps the exact programs the user started, newest first.
	History []string `json:"history"`
	// Frequent stores the launch counts consumed by the ranking.
	Frequent index.MostFrequent `json:"frequent"`
	// Rejected hides programs the user removed from the list.
	Rejected []string `json:"rejected"`

	// WindowShadow is a reserved switch; kept so the config reads complete
	// for anyone who wants the frame without the DWM border.
	WindowShadow bool `json:"windowShadow,omitempty"`

	InstalledExe string `json:"installedExe,omitempty"`
}

func defaultConfig() Config {
	return Config{
		Hotkey: "ctrl+alt+space",
		// The portable tools folder is indexed by default: this launcher is
		// meant to find what lives in D:\YandexDisk\Tools too.
		Folders:          []string{`D:\YandexDisk\Tools`},
		StartMenu:        true,
		AppPaths:         true,
		EnvPath:          false,
		Recursive:        true,
		Precision:        "regular",
		Autostart:        true,
		AutoUpdate:       true,
		UpdateCheckHours: 6,
		ReindexMinutes:   30,
		HideUninstallers: true,
		WindowShadow:     true,
	}
}

func dataDir() string {
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		return filepath.Join(dir, appName)
	}
	if app := os.Getenv("APPDATA"); app != "" {
		return filepath.Join(app, appName)
	}
	return appName
}

func configPath() string { return filepath.Join(dataDir(), "config.json") }

// loadConfig reads the file, filling in defaults for anything missing or
// invalid, and writes it back on first start.
func loadConfig() (Config, bool) {
	cfg := defaultConfig()
	raw, err := os.ReadFile(configPath())
	if err != nil {
		_ = cfg.save()
		return cfg, true
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		logf("config: %s is unreadable (%v), using defaults", configPath(), err)
		return defaultConfig(), false
	}
	cfg.normalize()
	return cfg, false
}

func (c *Config) normalize() {
	d := defaultConfig()
	if strings.TrimSpace(c.Hotkey) == "" {
		c.Hotkey = d.Hotkey
	}
	switch c.Precision {
	case "loose", "regular", "strict":
	default:
		c.Precision = d.Precision
	}
	if c.UpdateCheckHours <= 0 || c.UpdateCheckHours > 720 {
		c.UpdateCheckHours = d.UpdateCheckHours
	}
	if c.ReindexMinutes < 1 || c.ReindexMinutes > 24*60 {
		c.ReindexMinutes = d.ReindexMinutes
	}
	if c.Frequent.Count == nil {
		c.Frequent = *index.NewMostFrequent()
	}
}

func (c Config) precision() search.Precision {
	switch c.Precision {
	case "loose":
		return search.Loose
	case "strict":
		return search.Strict
	}
	return search.Regular
}

func (c Config) save() error {
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o755); err != nil {
		return err
	}
	tmp := configPath() + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, configPath())
}

// options turns the config into a scanner configuration.
func (c Config) options() index.Options {
	return index.Options{
		Folders:       append([]string(nil), c.Folders...),
		StartMenu:     c.StartMenu,
		AppPaths:      c.AppPaths,
		EnvPath:       c.EnvPath,
		Recursive:     c.Recursive,
		IncludeHidden: false,
	}
}
