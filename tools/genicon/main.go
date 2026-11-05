// Command genicon renders the application icon into assets/app.ico.
//
//	go run ./tools/genicon
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/dvygolov/yalauncher/internal/icon"
)

func main() {
	out := flag.String("out", "assets", "output directory")
	preview := flag.Bool("preview", true, "also write PNG previews")
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail(err)
	}
	imgs := make([]*image.RGBA, 0, len(icon.Sizes))
	for _, s := range icon.Sizes {
		imgs = append(imgs, icon.Render(s))
	}
	path := filepath.Join(*out, "app.ico")
	if err := os.WriteFile(path, icon.EncodeICO(imgs), 0o644); err != nil {
		fail(err)
	}
	if st, err := os.Stat(path); err == nil {
		fmt.Printf("%-28s %6d bytes\n", path, st.Size())
	}
	if *preview {
		for _, s := range []int{16, 32, 64, 256} {
			f, err := os.Create(filepath.Join(*out, fmt.Sprintf("preview-%d.png", s)))
			if err != nil {
				fail(err)
			}
			if err := png.Encode(f, icon.Render(s)); err != nil {
				_ = f.Close()
				fail(err)
			}
			_ = f.Close()
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "genicon:", err)
	os.Exit(1)
}
