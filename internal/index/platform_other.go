//go:build !windows

package index

import "os"

// On non-Windows hosts the scanner still works for tests and for browsing a
// mirrored folder tree: shortcuts simply have no target to resolve.
var resolveShortcut func(path string) (target, args, description, workingDir string)

func itemFromFile(path string, modified int64, source string) *Item {
	return nil
}

func scanAppPaths() []*Item { return nil }

func knownFolder(id int) string { return "" }

const (
	csidlPrograms       = 2
	csidlCommonPrograms = 23
)

func readSmallFile(path string, limit int64) ([]byte, error) {
	return os.ReadFile(path)
}
