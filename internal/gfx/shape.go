// Package gfx implements the 320x200 8-bit screen model of the original game.
package gfx

// Shape is an encoded image as produced by the original encodeShape routine.
// Pix holds one entry per pixel: for 4-bit shapes the nibble (0..15) mapped
// through Col; for 8-bit shapes the palette index directly. A mapped value of 0
// is transparent.
type Shape struct {
	H, W8 int // height in pixels, width in units of 8 pixels
	Four  bool
	Col   [16]byte
	Pix   []byte
}

// W returns the width in pixels.
func (s *Shape) W() int { return s.W8 * 8 }

// At returns the palette colour of pixel (x,y) (0 = transparent).
func (s *Shape) At(x, y int) byte {
	v := s.Pix[y*s.W8*8+x]
	if s.Four {
		return s.Col[v&15]
	}
	return v
}

// Encode4 cuts a 4-bit shape from a 320x200 page. x and w are in 8-pixel units.
// Colours are assigned table slots in order of first appearance (max 16;
// further colours map to slot 0), exactly like the original driver.
func Encode4(page []byte, x8, y, w8, h int) *Shape {
	s := &Shape{H: h, W8: w8, Four: true, Pix: make([]byte, w8*8*h)}
	var m [256]int
	for i := range m {
		m[i] = -1
	}
	n := 0
	for yy := 0; yy < h; yy++ {
		for xx := 0; xx < w8*8; xx++ {
			c := page[(y+yy)*320+x8*8+xx]
			if m[c] < 0 {
				if n < 16 {
					s.Col[n] = c
					m[c] = n
					n++
				} else {
					m[c] = 0
				}
			}
			s.Pix[yy*w8*8+xx] = byte(m[c])
		}
	}
	return s
}

// Encode8 cuts an 8-bit shape from a page.
func Encode8(page []byte, x8, y, w8, h int) *Shape {
	s := &Shape{H: h, W8: w8, Pix: make([]byte, w8*8*h)}
	for yy := 0; yy < h; yy++ {
		copy(s.Pix[yy*w8*8:], page[(y+yy)*320+x8*8:(y+yy)*320+x8*8+w8*8])
	}
	return s
}

// Scale23 reproduces the original 2/3 shape scaler: keeps rows/pixels 0,1 of
// every 3 (pixel pairs 0-1, 3-4 of every 6).
func (s *Shape) Scale23() *Shape {
	nh := (s.H + 1) * 2 / 3
	w2 := s.W8 * 8 * 2 // width in... original works in nibbles: w8*4 bytes per row
	nw8 := w2 / 3      // ceil(w8*2/3)
	if (s.W8*2)%3 != 0 {
		nw8 = s.W8*2/3 + 1
	} else {
		nw8 = s.W8 * 2 / 3
	}
	d := &Shape{H: nh, W8: nw8, Four: s.Four, Col: s.Col, Pix: make([]byte, nw8*8*nh)}
	srcW := s.W8 * 8
	dstW := nw8 * 8
	dy := 0
	for y := 0; y < s.H && dy < nh; y++ {
		if y%3 == 2 {
			continue
		}
		dx := 0
		for x := 0; x < srcW && dx < dstW; x++ {
			if x%3 == 2 {
				continue
			}
			d.Pix[dy*dstW+dx] = s.Pix[y*srcW+x]
			dx++
		}
		dy++
	}
	return d
}
