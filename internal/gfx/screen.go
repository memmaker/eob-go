package gfx

// Dim is an original "screen dim" (text/clip window): X and W in 8-pixel units.
type Dim struct {
	X, Y, W, H             int
	Unk8, UnkA, UnkC, UnkE int
}

// NumPages is the number of 320x200 work pages.
const NumPages = 16

// Screen holds the original game's video pages and palette.
type Screen struct {
	Pages   [NumPages][]byte
	Pal     [768]byte // 6-bit VGA palette
	Dims    []Dim
	CurPage int
	Remap   []byte   // optional colour remap used by remapped shape draws (8-bit)
	Remap16 [16]byte // nibble remap for 4-bit shapes (driver entry 35)
	Dirty   bool
}

// New allocates a screen.
func New() *Screen {
	s := &Screen{}
	for i := range s.Pages {
		s.Pages[i] = make([]byte, 64000)
	}
	return s
}

// DrawFlags select shape drawing modes.
const (
	DrawFlip  = 1
	DrawRemap = 2
	DrawOddX  = 4 // keep odd x: S1a29_000f called directly skips the S082c wrappers' x&~1
)

// DrawShape ports S082c_063b S082c_0681 S082c_06c7 S082c_070d (plain/flip/
// remap/flip+remap via flags; 070d does not force x even, Go does).
// The dim-relative inner draws (x+dim.X*8, y+dim.Y, clip to dim) are folded in:
// The dim argument stands for the current-dim select S1a29_0189 (ports S1a29_0189).
// DrawShape also ports S1a29_000f S1a29_004d S1a29_008b S1a29_00c9.
// DrawShape draws shp at absolute (x,y) on page, clipped to dim (as the
// original: x is forced even; clipping window = dim rectangle).
func (s *Screen) DrawShape(page int, shp *Shape, x, y, dim int, flags int) {
	if shp == nil {
		return
	}
	if flags&DrawOddX == 0 && flags&(DrawFlip|DrawRemap) != DrawFlip|DrawRemap {
		x &^= 1
	}
	cx1, cy1, cx2, cy2 := 0, 0, 320, 200
	if dim >= 0 && dim < len(s.Dims) {
		d := s.Dims[dim]
		cx1, cy1 = d.X*8, d.Y
		cx2, cy2 = cx1+d.W*8, cy1+d.H
	}
	s.DrawShapeClip(page, shp, x, y, cx1, cy1, cx2, cy2, flags)
}

// DrawShapeClip draws with an explicit clip rectangle [x1,x2) x [y1,y2).
func (s *Screen) DrawShapeClip(page int, shp *Shape, x, y, x1, y1, x2, y2, flags int) {
	dst := s.Pages[page]
	w := shp.W()
	for yy := 0; yy < shp.H; yy++ {
		py := y + yy
		if py < y1 || py >= y2 || py < 0 || py >= 200 {
			continue
		}
		for xx := 0; xx < w; xx++ {
			px := x + xx
			if px < x1 || px >= x2 || px < 0 || px >= 320 {
				continue
			}
			sx := xx
			if flags&DrawFlip != 0 {
				sx = w - 1 - xx
			}
			var c byte
			if flags&DrawRemap != 0 && shp.Four {
				c = s.Remap16[shp.Pix[yy*w+sx]&15]
			} else {
				c = shp.At(sx, yy)
				if c != 0 && flags&DrawRemap != 0 && s.Remap != nil {
					c = s.Remap[c]
				}
			}
			if c == 0 {
				continue
			}
			dst[py*320+px] = c
		}
	}
	if page == 0 {
		s.Dirty = true
	}
}

// FillRect fills [x1,x2]x[y1,y2] inclusive with colour c.
func (s *Screen) FillRect(page, x1, y1, x2, y2 int, c byte) {
	p := s.Pages[page]
	for y := y1; y <= y2; y++ {
		if y < 0 || y >= 200 {
			continue
		}
		for x := x1; x <= x2; x++ {
			if x >= 0 && x < 320 {
				p[y*320+x] = c
			}
		}
	}
	if page == 0 {
		s.Dirty = true
	}
}

// CopyRegion copies a w x h pixel rectangle between pages.
// CopyRegion ports S19f3_000e (pixel units; the original takes x/w in 8px
// columns and clips the same way).
func (s *Screen) CopyRegion(sx, sy, dx, dy, w, h, src, dst int) {
	a, b := s.Pages[src], s.Pages[dst]
	tmp := make([]byte, w)
	for y := 0; y < h; y++ {
		ys, yd := sy+y, dy+y
		if ys < 0 || ys >= 200 || yd < 0 || yd >= 200 {
			continue
		}
		for x := 0; x < w; x++ {
			if sx+x >= 0 && sx+x < 320 {
				tmp[x] = a[ys*320+sx+x]
			}
		}
		for x := 0; x < w; x++ {
			if dx+x >= 0 && dx+x < 320 {
				b[yd*320+dx+x] = tmp[x]
			}
		}
	}
	if dst == 0 {
		s.Dirty = true
	}
}

// CopyPage copies a full page; ports S19ec_002a (no-op when src == dst).
func (s *Screen) CopyPage(src, dst int) {
	copy(s.Pages[dst], s.Pages[src])
	if dst == 0 {
		s.Dirty = true
	}
}

// SetPalette sets the 6-bit palette.
func (s *Screen) SetPalette(p []byte) {
	copy(s.Pal[:], p)
	s.Dirty = true
}

// RGBA converts page 0 to RGBA pixels.
func (s *Screen) RGBA(out []byte) {
	p := s.Pages[0]
	for i, c := range p {
		r, g, b := s.Pal[3*int(c)]&63, s.Pal[3*int(c)+1]&63, s.Pal[3*int(c)+2]&63
		out[4*i] = r<<2 | r>>4
		out[4*i+1] = g<<2 | g>>4
		out[4*i+2] = b<<2 | b>>4
		out[4*i+3] = 255
	}
}
