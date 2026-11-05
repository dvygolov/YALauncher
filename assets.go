//go:build windows

package main

import _ "embed"

// The icon is compiled into the exe so the tray never depends on a file next
// to the binary.
//
//go:embed assets/app.ico
var iconData []byte
