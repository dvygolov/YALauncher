//go:build windows

package main

import (
	"fmt"

	"github.com/dvygolov/yalauncher/internal/win"
)

func main() {
	fmt.Println("IsElevated:", win.IsElevated())
}
