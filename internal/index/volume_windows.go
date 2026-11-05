//go:build windows

package index

import (
	"syscall"
	"unsafe"

	"github.com/dvygolov/yalauncher/internal/win"
)

// diskLabels maps "C:" to the volume label Windows shows. Installers that
// create shortcuts on the drive root store the root, the real target lives in
// the shell's folder item and is found by scanning the drive label instead.
func diskLabels() (map[string]string, bool) { return win.DiskLabels(), true }

var (
	_kernel32    = syscall.NewLazyDLL("kernel32.dll")
	_pGetLogical = _kernel32.NewProc("GetLogicalDrives")
	_pGetVolume  = _kernel32.NewProc("GetVolumeInformationW")
	_            = unsafe.Pointer(nil)
)
