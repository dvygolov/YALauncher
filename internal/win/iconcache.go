//go:build windows

package win

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"unsafe"
)

// ToImage converts the premultiplied BGRA pixels of a MemDC into an image.
func (m *MemDC) ToImage() *image.RGBA {
	if m == nil || m.Width <= 0 || m.Height <= 0 {
		return nil
	}
	img := image.NewRGBA(image.Rect(0, 0, int(m.Width), int(m.Height)))
	for y := int32(0); y < m.Height; y++ {
		for x := int32(0); x < m.Width; x++ {
			off := (y*m.Width + x) * 4
			b := *(*byte)(unsafe.Pointer(m.Pixels + uintptr(off)))
			g := *(*byte)(unsafe.Pointer(m.Pixels + uintptr(off+1)))
			r := *(*byte)(unsafe.Pointer(m.Pixels + uintptr(off+2)))
			a := *(*byte)(unsafe.Pointer(m.Pixels + uintptr(off+3)))
			img.SetRGBA(int(x), int(y), color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	return img
}

// SavePNG writes the surface of a MemDC as a PNG file.
func (m *MemDC) SavePNG(path string) error {
	img := m.ToImage()
	if img == nil {
		return os.ErrInvalid
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// LoadPNG reads a PNG into a MemDC of the same size.
func LoadPNG(path string) *MemDC {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil
	}
	w, h := int32(img.Bounds().Dx()), int32(img.Bounds().Dy())
	dc := NewMemDC(w, h)
	if dc == nil {
		return nil
	}
	for y := int32(0); y < h; y++ {
		for x := int32(0); x < w; x++ {
			r, g, b, a := img.At(int(x), int(y)).RGBA()
			off := (y*w + x) * 4
			*(*byte)(unsafe.Pointer(dc.Pixels + uintptr(off))) = byte(b / 257)
			*(*byte)(unsafe.Pointer(dc.Pixels + uintptr(off+1))) = byte(g / 257)
			*(*byte)(unsafe.Pointer(dc.Pixels + uintptr(off+2))) = byte(r / 257)
			*(*byte)(unsafe.Pointer(dc.Pixels + uintptr(off+3))) = byte(a / 257)
		}
	}
	return dc
}
