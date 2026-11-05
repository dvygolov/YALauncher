//go:build windows

package index

// The Shell Link parser itself is plain Go (see lnk.go); this file only wires
// it into the scanner.
func init() { resolveShortcut = resolveShortcutImpl }
