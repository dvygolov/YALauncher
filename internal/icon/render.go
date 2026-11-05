package icon

import (
	"image"
	"image/color"
	"math"
)

// Palette: the launcher is dark, so the icon is a bright tile that reads on
// both the dark taskbar and the light one.
var (
	tileTop    = color.RGBA{0x5B, 0x8D, 0xF5, 0xFF}
	tileBottom = color.RGBA{0x2A, 0x4F, 0xC8, 0xFF}
	glass      = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
)

type point struct{ x, y float64 }
type segment struct{ a, b point }

type shape struct {
	segs []segment
	r    float64
}

func (s shape) inside(x, y float64) bool {
	best := math.Inf(1)
	for _, sg := range s.segs {
		if d := distToSegment(x, y, sg.a, sg.b); d < best {
			best = d
		}
	}
	return best <= s.r
}

func distToSegment(px, py float64, a, b point) float64 {
	vx, vy := b.x-a.x, b.y-a.y
	wx, wy := px-a.x, py-a.y
	den := vx*vx + vy*vy
	t := 0.0
	if den > 0 {
		t = (wx*vx + wy*vy) / den
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}
	}
	dx, dy := px-(a.x+t*vx), py-(a.y+t*vy)
	return math.Hypot(dx, dy)
}

// Render draws the icon at the requested size: a rounded gradient tile with a
// white magnifier whose handle points down-right.
func Render(size int) *image.RGBA {
	if size < 8 {
		size = 8
	}
	ss := 4
	if size >= 96 {
		ss = 2
	}
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	inv := 1.0 / float64(ss*ss)
	n := float64(size)

	ring := shape{segs: circle(0.44, 0.42, 0.175), r: 0.04}
	handleShape := shape{segs: []segment{{a: point{0.568, 0.558}, b: point{0.775, 0.765}}}, r: 0.045}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a float64
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					px := (float64(x) + (float64(sx)+0.5)/float64(ss)) / n
					py := (float64(y) + (float64(sy)+0.5)/float64(ss)) / n
					cr, cg, cb, ca := sample(px, py, ring, handleShape)
					r += float64(cr) * float64(ca)
					g += float64(cg) * float64(ca)
					b += float64(cb) * float64(ca)
					a += float64(ca)
				}
			}
			r *= inv
			g *= inv
			b *= inv
			a *= inv
			if a <= 0.0001 {
				continue
			}
			img.SetRGBA(x, y, color.RGBA{
				R: clamp8(r / a),
				G: clamp8(g / a),
				B: clamp8(b / a),
				A: clamp8(a),
			})
		}
	}
	return img
}

func circle(cx, cy, r float64) []segment {
	const steps = 48
	out := make([]segment, 0, steps)
	prev := point{cx + r, cy}
	for i := 1; i <= steps; i++ {
		th := 2 * math.Pi * float64(i) / steps
		p := point{cx + r*math.Cos(th), cy + r*math.Sin(th)}
		out = append(out, segment{prev, p})
		prev = p
	}
	return out
}

// sample blends the tile, a soft shadow and the white magnifier.
func sample(x, y float64, ring, handle shape) (float32, float32, float32, float32) {
	if !inRounded(x, y, 0.22) {
		return 0, 0, 0, 0
	}
	c := mix(tileTop, tileBottom, smooth(0.05, 0.95, y))
	if ring.inside(x-0.016, y-0.018) || handle.inside(x-0.016, y-0.018) {
		c = mix(c, color.RGBA{0x08, 0x18, 0x48, 0xFF}, 0.35)
	}
	if ring.inside(x, y) || handle.inside(x, y) {
		c = glass
	}
	return float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, 1
}

func smooth(a, b, v float64) float64 {
	t := (v - a) / (b - a)
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func inRounded(x, y, r float64) bool {
	if x < 0 || x > 1 || y < 0 || y > 1 {
		return false
	}
	dx := math.Max(0, math.Max(r-x, x-(1-r)))
	dy := math.Max(0, math.Max(r-y, y-(1-r)))
	return math.Hypot(dx, dy) <= r
}

func mix(a, b color.RGBA, t float64) color.RGBA {
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: 255,
	}
}

func clamp8(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v + 0.5)
}
