// Package icon renders YALauncher's icon in pure Go: a search glyph over a
// rounded gradient tile. No image assets, no fonts, no CGO - the same code
// produces the .ico at build time and the tray icon at runtime.
package icon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/png"
)

// Sizes are the icon sizes baked into the .ico: the tray asks for 16..32 px
// depending on the monitor scale, Explorer for 48..256.
var Sizes = []int{16, 20, 24, 32, 48, 64, 128, 256}

// EncodeICO turns rendered images into a Windows .ico file. Entries up to
// 64 px are stored as classic 32 bpp bitmaps, larger ones as PNG.
func EncodeICO(images []*image.RGBA) []byte {
	type dirEntry struct {
		w, h   byte
		size   uint32
		offset uint32
	}
	entries := make([]dirEntry, 0, len(images))
	var body bytes.Buffer
	for _, img := range images {
		b := img.Bounds()
		w, h := b.Dx(), b.Dy()
		var data []byte
		if w > 64 {
			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				continue
			}
			data = buf.Bytes()
		} else {
			data = encodeDIB(img)
		}
		entries = append(entries, dirEntry{
			w: byte(w % 256), h: byte(h % 256),
			size:   uint32(len(data)),
			offset: uint32(6 + 16*len(images) + body.Len()),
		})
		body.Write(data)
	}
	le := binary.LittleEndian
	var out bytes.Buffer
	_ = binary.Write(&out, le, uint16(0))
	_ = binary.Write(&out, le, uint16(1))
	_ = binary.Write(&out, le, uint16(len(entries)))
	for _, e := range entries {
		out.WriteByte(e.w)
		out.WriteByte(e.h)
		out.WriteByte(0)
		out.WriteByte(0)
		_ = binary.Write(&out, le, uint16(1))
		_ = binary.Write(&out, le, uint16(32))
		_ = binary.Write(&out, le, e.size)
		_ = binary.Write(&out, le, e.offset)
	}
	out.Write(body.Bytes())
	return out.Bytes()
}

func encodeDIB(img *image.RGBA) []byte {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	var out bytes.Buffer
	le := binary.LittleEndian
	_ = binary.Write(&out, le, uint32(40))
	_ = binary.Write(&out, le, int32(w))
	_ = binary.Write(&out, le, int32(h*2))
	_ = binary.Write(&out, le, uint16(1))
	_ = binary.Write(&out, le, uint16(32))
	_ = binary.Write(&out, le, uint32(0))
	_ = binary.Write(&out, le, uint32(w*h*4))
	_ = binary.Write(&out, le, int32(0))
	_ = binary.Write(&out, le, int32(0))
	_ = binary.Write(&out, le, uint32(0))
	_ = binary.Write(&out, le, uint32(0))
	for y := h - 1; y >= 0; y-- {
		row := img.Pix[y*img.Stride : y*img.Stride+w*4]
		for x := 0; x < w; x++ {
			out.WriteByte(row[x*4+2])
			out.WriteByte(row[x*4+1])
			out.WriteByte(row[x*4+0])
			out.WriteByte(row[x*4+3])
		}
	}
	maskStride := ((w + 31) / 32) * 4
	mask := make([]byte, maskStride*h)
	for y := 0; y < h; y++ {
		row := img.Pix[(h-1-y)*img.Stride : (h-1-y)*img.Stride+w*4]
		for x := 0; x < w; x++ {
			if row[x*4+3] == 0 {
				mask[y*maskStride+x/8] |= 1 << (7 - uint(x%8))
			}
		}
	}
	out.Write(mask)
	return out.Bytes()
}
