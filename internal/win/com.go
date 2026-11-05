//go:build windows

package win

import "errors"

// CoInitialize enters a single threaded apartment, which the shell helpers
// want before they can render icons off the main window thread.
func CoInitialize() error {
	r, _, _ := pCoInitializeEx.Call(0, COINIT_APARTMENTTHREADED)
	// S_FALSE (1) means "already initialised on this thread": not an error.
	if int32(r) < 0 {
		return errors.New("CoInitializeEx failed")
	}
	return nil
}

func CoUninitialize() { pCoUninitialize.Call() }

// SetThreadDesktopReady is a hook for worker threads that need a message
// queue; the launcher does not, so it is a no-op kept for symmetry.
func SetThreadDesktopReady() {}
