package eob

// Port of INTRO.EXE (the separate intro program). Its code is the FBOV overlay
// segment 0e96 (tools/intro_listing.py -> intro.lst); entry O0e96_0000.
// File names and animation tables are read from INTRO.EXE's data segment at
// runtime; images/palettes/text strips come from the *.CMP / *.COL files.
//
// Driver model (VGA path, S0892_0471): pages alias in pairs 0=1, 2=3, 4=5, 6=7.
// Timing: ds:2326 is a countdown decremented by the INT 1Ch handler (18.2 Hz);
// sequences set it, draw a frame, then wait for it to reach 0 (O0e96_0fd9).

import (
	"encoding/binary"
	"errors"

	"github.com/felix/goeob/internal/exe"
	"github.com/felix/goeob/internal/gfx"
	"github.com/felix/goeob/internal/res"
)

// INTRO.EXE DGROUP offsets.
const (
	inStrTitleE   = 0x1d42
	inStrTitleV   = 0x1d4e
	inStrWestCol  = 0x1d5a
	inStrText     = 0x1d67
	inStrWtrdp2C  = 0x1d70
	inStrWtrdp1   = 0x1d7b
	inStrWtrdp2   = 0x1d86
	inStrWtrdp3   = 0x1d91
	inStrOrb      = 0x1d9c
	inStrTowr     = 0x1da4
	inStrTowrCol  = 0x1db1
	inStrTowr2    = 0x1dbe
	inStrKing     = 0x1dcb
	inStrAdventur = 0x1dd4
	inStrHands    = 0x1de1
	inStrWtrdp3b  = 0x1deb
	inStrOrbCol   = 0x1df6
	inStrOrb2     = 0x1dfe
	inStrTunnel   = 0x1e06
	inStrZoomCol  = 0x1e11
	inStrAvalanch = 0x1e1e
	inStrZoomtunl = 0x1e2b
	inStrWestCmp  = 0x1e5d
	inStrWestCol2 = 0x1e6a

	inCreditsWait0 = 0x1d28 // u16
	inCreditsWait  = 0x1d2a // 4 x u16
	inCreditsNames = 0x1d32 // 4 x far ptr
	inWaterX8      = 0x1c2a // 31 bytes
	inWaterY8      = 0x1c49
	inWaterDrawX   = 0x1c68 // 31 x i16
	inWaterDrawY   = 0x1ca6
	inWaterW8      = 0x1cc5
	inKingX8       = 0x1ce4 // 4 x i16
	inKingW8       = 0x1cec // 4 x i16
	inKingDY       = 0x1cf4 // 8 x i16
	inAvalSX8      = 0x1d04 // 6 bytes each
	inAvalSY       = 0x1d0a
	inAvalW8       = 0x1d10
	inAvalH        = 0x1d16
	inAvalDX8      = 0x1d1c
	inAvalDY       = 0x1d22
	inRndSeed      = 0x2310 // 3 bytes (S0cda_0006)
	inCRandSeed    = 0x2610 // u32 Borland rand seed
)

var errIntroAbort = errors.New("intro aborted")

type intro struct {
	e        *Engine
	x        *exe.EXE
	s        *gfx.Screen
	cur      int    // ds:1fa1 current draw page
	deadline uint32 // ds:2326 countdown, as an absolute time
	rs       [3]byte
	cseed    uint32
}

// playIntro ports INTRO.EXE (O0e96_0000). Esc aborts the whole intro, any
// other key ends the current pause (as O0e96_0fd9).
func (e *Engine) playIntro() {
	x, err := exe.Load(e.dir + "/INTRO.EXE")
	if err != nil {
		return
	}
	in := &intro{e: e, x: x, s: e.Scr}
	copy(in.rs[:], x.Bytes(inRndSeed, 3))
	in.cseed = binary.LittleEndian.Uint32(x.File[x.DS+inCRandSeed:])
	defer func() {
		if r := recover(); r != nil && r != errIntroAbort { //nolint:errorlint
			panic(r)
		}
	}()
	in.credits()
	in.main()
}

// ---- primitives -------------------------------------------------------------

func pg(p int) int { return p &^ 1 }

func (in *intro) str(o int) string { return in.x.Str(o) }

func (in *intro) present() { in.e.Sys.Present(in.s.Pages[0], in.s.Pal[:]) }

// set loads the countdown (ds:2326 = n ticks).
func (in *intro) set(n int) { in.deadline = in.e.Sys.Millis() + uint32(n*tickMillis) }

// wait ports O0e96_0fd9.
func (in *intro) wait() {
	in.present()
	for in.e.Sys.Millis() < in.deadline {
		for {
			ev, ok := in.e.poll()
			if !ok {
				break
			}
			if ev.Kind == EvQuit || (ev.Kind == EvKeyDown && ev.Key == KeyEsc) {
				in.e.quit = ev.Kind == EvQuit
				panic(errIntroAbort)
			}
			if ev.Kind == EvKeyDown {
				in.deadline = 0
			}
		}
		in.rnd() // the original spins S0cda_0006 while idle (CPU-speed dependent count)
		in.e.Sys.Sleep(5)
	}
}

// delay ports O0e96_1025.
func (in *intro) delay(n int) { in.set(n); in.wait() }

func (in *intro) music(n int) { in.e.playMusic(n) }

// loadPal ports O0e96_2a4e (VGA): 768-byte .COL, colour 0 forced black.
func (in *intro) loadPal(o int) {
	b, err := in.e.ReadFile(in.str(o))
	if err != nil || len(b) < 768 {
		return
	}
	p := append([]byte(nil), b[:768]...)
	p[0], p[1], p[2] = 0, 0, 0
	in.s.SetPalette(p)
}

// loadCMP ports O0e96_2b7f: the raw file is read into page tmp, then
// decompressed into page dst.
func (in *intro) loadCMP(name string, tmp, dst int) {
	b, err := in.e.ReadFile(name)
	if err != nil {
		return
	}
	if len(b) > 2 {
		copy(in.s.Pages[pg(tmp)], b[2:])
	}
	if c, err := res.DecodeCPS(b); err == nil {
		copy(in.s.Pages[pg(dst)], c.Data)
	}
}

// copyPage ports S097f_002a.
func (in *intro) copyPage(src, dst int) {
	if pg(src) != pg(dst) {
		in.s.CopyPage(pg(src), pg(dst))
	}
}

// copy8 ports S0986_000e (x and w in 8-pixel units).
func (in *intro) copy8(sx, sy, dx, dy, w, h, src, dst int) {
	w, h = min(w, 40), min(h, 200)
	if sx < 0 {
		dx -= sx
		w += sx
		sx = 0
	}
	if sx >= 40 || dx >= 40 {
		return
	}
	if dx < 0 {
		sx -= dx
		w += dx
		dx = 0
	}
	if sy < 0 {
		dy -= sy
		h += sy
		sy = 0
	}
	if dy < 0 {
		sy -= dy
		h += dy
		dy = 0
	}
	if w <= 0 || h <= 0 {
		return
	}
	in.s.CopyRegion(sx*8, sy, dx*8, dy, w*8, h, pg(src), pg(dst))
}

func (in *intro) fill(x1, y1, x2, y2 int, c byte) { in.s.FillRect(pg(in.cur), x1, y1, x2, y2, c) }
func (in *intro) clearPage()                      { clear(in.s.Pages[pg(in.cur)]) }

// clearText ports O0e96_0faa: blanks the text strip on page 0.
func (in *intro) clearText() {
	c := in.cur
	in.cur = 0
	in.fill(0, 0xa8, 0x13f, 0xc7, 0xc)
	in.cur = c
}

func (in *intro) encode(x8, y, w8, h int) *gfx.Shape {
	return gfx.Encode8(in.s.Pages[pg(in.cur)], x8, y, w8, h)
}

func (in *intro) draw(p int, shp *gfx.Shape, x, y int) {
	in.s.DrawShapeClip(pg(p), shp, x, y, 0, 0, 320, 200, 0)
}

// drawFlip ports S0998_0046 (driver entry 24, mirrored blit).
func (in *intro) drawFlip(p int, shp *gfx.Shape, x, y int) {
	in.s.DrawShapeClip(pg(p), shp, x, y, 0, 0, 320, 200, gfx.DrawFlip)
}

// rnd ports S0cda_0006 (Westwood 24-bit random byte generator).
func (in *intro) rnd() int {
	s := &in.rs
	al := s[0] >> 2
	cf := s[0] >> 1 & 1
	c2 := s[2] >> 7
	s[2] = s[2]<<1 | cf
	c1 := s[1] >> 7
	s[1] = s[1]<<1 | c2
	cf = c1 ^ 1
	al = al - s[0] - cf
	s[0] = s[0]>>1 | (al&1)<<7
	return int(s[0] ^ s[1])
}

// random ports S0a15_0009 (Borland rand() scaled to [lo,hi]).
func (in *intro) random(lo, hi int) int {
	for {
		in.cseed = in.cseed*0x015a4e35 + 1
		r := int64(in.cseed >> 16 & 0x7fff)
		if v := lo + int(r*int64(hi-lo+1)/0x7fff); v <= hi {
			return v
		}
	}
}

// dissolve ports O0e96_2c0d: copies a rectangle pixel by pixel in random order.
func (in *intro) dissolve(x, y, w, h, src, dst int) {
	px, py := make([]int, w), make([]int, h)
	for i := range px {
		px[i] = i
	}
	for i := range py {
		py[i] = i
	}
	for i := range px {
		r := in.random(0, w-1)
		px[r], px[i] = px[i], px[r]
	}
	for i := range py {
		r := in.random(0, h-1)
		py[r], py[i] = py[i], py[r]
	}
	a, b := in.s.Pages[pg(src)], in.s.Pages[pg(dst)]
	for j := 0; j < h; j++ {
		di := j
		for i := 0; i < w; i++ {
			o := (y+py[di])*320 + x + px[i]
			if di++; di >= h {
				di = 0
			}
			b[o] = a[o]
		}
		// ponytail: the original draws straight into VRAM with no pacing; show
		// progress every few rows at ~1 s total, tune if real-hw speed matters.
		if dst == 0 && j%4 == 3 {
			in.present()
			in.e.Sys.Sleep(20)
		}
	}
}

// lineWipe ports O0e96_29c9: overdraws the current page with a grid of
// colour 12 lines (8 passes x 2).
func (in *intro) lineWipe() {
	p := pg(in.cur)
	for k := 0; k < 2; k++ {
		for j := 0; j < 8; j++ {
			if j&1 != 0 {
				for x := j + k; x < 0x140; x += 8 {
					in.s.DrawLine(p, x, 0, x, 0xc7, 0xc)
				}
			} else {
				for y := j + k; y < 0xc8; y += 8 {
					in.s.DrawLine(p, 0, y, 0x13f, y, 0xc)
				}
			}
			if p == 0 { // ponytail: unpaced in the original (VRAM), same caveat as dissolve
				in.present()
				in.e.Sys.Sleep(15)
			}
		}
	}
}

// dither ports O0e96_09ad: thins out an image on page 3 (mode 1 checkerboard,
// 2 every other row/pixel, 3 every third row/pixel).
func (in *intro) dither(sx, sy, dx, dy, w, h, mode int) {
	p := in.s.Pages[2]
	s, d := sy*320+sx, dy*320+dx
	switch mode {
	case 1:
		for ; h > 0; h -= 2 {
			for i := 0; i < w; i += 2 {
				p[d+i], p[d+i+1] = p[s+i], 0
			}
			s, d = s+320, d+320
			for i := 0; i < w; i += 2 {
				p[d+i], p[d+i+1] = 0, p[s+i+1]
			}
			s, d = s+320, d+320
		}
	case 2:
		for ; h > 0; h -= 2 {
			for i := 0; i < w; i += 2 {
				p[d+i], p[d+i+1] = p[s+i], 0
			}
			d += 320
			for i := 0; i < w; i++ {
				p[d+i] = 0
			}
			d += 320
			s += 640
		}
	case 3:
		for k := 0; k < h; k++ {
			i := 0
			if k%3 == 0 {
				for ; i < w-3; i += 3 {
					p[d+i], p[d+i+1], p[d+i+2] = p[s+i], 0, 0
				}
			}
			for ; i < w; i++ {
				p[d+i] = 0
			}
			s, d = s+320, d+320
		}
	}
}

// zoom ports O0e96_1041: grows a window (in 8x8 cells) from (x,y) to (tx,ty)
// with size tw x th, copying from page 2 at (sx,sy) and erasing the margins
// l,t,r,b with blank screen column 39 / row 0xc0.
func (in *intro) zoom(x, y, tx, ty, sx, sy, tw, th, l, t, r, b int) {
	right, bottom := l+r, min(t+b, 0x15)
	w, h := 1, 1
	sign := func(v int) int {
		if v < 0 {
			return -1
		} else if v > 0 {
			return 1
		}
		return 0
	}
	for done := false; !done; {
		in.set(1)
		in.copy8(sx, sy*8, x, y*8, w, h*8, 2, 0)
		if l < x {
			in.copy8(0x27, 0, l, 0, 1, 0xb0, 0, 0)
		}
		if t < y {
			in.copy8(0, 0xc0, 0, t*8, 0x28, 8, 0, 0)
		}
		if tx+tw <= right {
			in.copy8(0x27, 0, right, 0, 1, 0xb0, 0, 0)
		}
		if ty+th <= bottom {
			in.copy8(0, 0xc0, 0, bottom*8, 0x28, 8, 0, 0)
		}
		if x == tx && y == ty && w == tw && h == th && l >= tx && t >= ty && tx+tw >= right && ty+th >= bottom {
			done = true
		}
		x += sign(tx - x)
		y += sign(ty - y)
		if w != tw {
			w = min(w+2, tw)
		}
		if h != th {
			h = min(h+2, th)
		}
		l = min(l+1, tx)
		t = min(t+1, ty)
		if tx+tw < right {
			right--
		}
		if ty+th < bottom {
			bottom--
		}
		in.wait()
	}
}

func (in *intro) u8(o int) int  { return int(in.x.U8(o)) }
func (in *intro) i16(o int) int { return int(in.x.I16(o)) }

// waterFrames encodes the 31 water-glint shapes and the 3 walking-figure
// frames (figure column x8 = 5*i + fx) from page 2.
func (in *intro) waterFrames(fx int) (water [31]*gfx.Shape, fig [3]*gfx.Shape) {
	for i := range water {
		w := in.u8(inWaterW8 + i)
		water[i] = in.encode(in.u8(inWaterX8+i), in.u8(inWaterY8+i)*8+0x88, w, w*8)
	}
	for i := range fig {
		fig[i] = in.encode(i*5+fx, 0x98, 5, 0x20)
	}
	return
}

func (in *intro) waterGlint(water [31]*gfx.Shape) {
	for k := 0; k < 5; k++ {
		d := in.rnd() % 31
		in.draw(0, water[d], in.i16(inWaterDrawX+2*d)-0x50, in.u8(inWaterDrawY+d)+0x18)
	}
	if in.rnd()&7 == 0 {
		if in.rnd()&1 != 0 {
			in.music(5)
		} else {
			in.music(0xe)
		}
	}
}

func figFrame(si int) int {
	if d := si & 3; d != 3 {
		return d
	}
	return 1
}

// ---- sequences ----------------------------------------------------------------

// credits ports O0e96_2a99.
func (in *intro) credits() {
	in.loadCMP(in.str(inStrWestCmp), 5, 3)
	in.clearPage()
	in.loadPal(inStrWestCol2)
	in.copyPage(3, 0)
	in.music(1)
	in.set(in.i16(inCreditsWait0))
	in.wait()
	for i := 0; i < 4; i++ {
		in.set(in.i16(inCreditsWait + 2*i))
		in.loadCMP(in.str(in.i16(inCreditsNames+4*i)), 5, 3)
		in.copyPage(3, 4)
		in.dissolve(0, 0x32, 0x140, 0x66, 4, 0)
		in.wait()
	}
	in.set(0x32)
}

// main ports O0e96_003a.
func (in *intro) main() {
	in.cur = 0
	in.set(0x78)
	in.loadCMP(in.str(inStrTitleV), 3, 5)
	in.loadPal(inStrWestCol)
	in.music(2)
	in.copyPage(5, 2)
	in.dissolve(0, 0, 0x140, 0xc8, 2, 0)
	in.loadCMP(in.str(inStrText), 3, 5)
	in.copyPage(5, 6)
	in.wait()
	in.tower()
	in.orb()
	in.waterdeepEntry()
	in.king()
	in.hands()
	in.waterdeepExit()
	in.tunnel()
	in.lineWipe()
	in.present()
}

// tower ports O0e96_0b20.
func (in *intro) tower() {
	di := 0x20
	in.lineWipe()
	in.loadPal(inStrTowrCol)
	in.loadCMP(in.str(inStrTowr2), 5, 3)
	in.cur = 2
	shp := in.encode(0, 0, 0x10, 0x38)
	in.copyPage(3, 4)
	in.clearPage()
	for si := 0; si < 0xc8; si += 0x40 {
		in.copy8(0x10, 0x68, 0xc, si, 0x10, 0x40, 4, 2)
	}
	in.fill(0, 0xb8, 0x13f, 0xc7, 0xc)
	c := in.cur
	in.cur = 0
	in.lineWipe()
	in.cur = c
	in.clearPage()
	for si := 0; si < 0xc8; si += 0x40 {
		in.copy8(0x10, 0x68, 0, si, 0x10, 0x40, 4, 2)
	}
	in.cur = 0
	for si := 0; si < 0x40; si += 2 {
		in.set(2)
		in.copy8(0, 0x8e-si, 0xc, 0, 0x10, si+1, 4, 0)
		in.copy8(0, 0, 0xc, si+1, 0x10, 0xa7-si, 2, 0)
		if si == 0 {
			in.copy8(0, 0, 0, 0xa8, 0x28, 0x20, 6, 0)
		}
		in.wait()
	}
	for si := 0; si < 0x18; si += 2 {
		in.set(2)
		in.copy8(0, 0x4f-si, 0xc, 0, 3, si+0x41, 4, 0)
		in.copy8(0xd, 0x4f-si, 0x19, 0, 3, si+0x41, 4, 0)
		in.copy8(3, 0x6e, 0xf, si+0x1f, 0xa, 0x22, 4, 0)
		in.copy8(0x13, 0, 0xf, di, 0xa, si+1, 4, 0)
		in.copy8(0, 0, 0xc, si+0x41, 0x10, 0x67-si, 2, 0)
		in.wait()
	}
	for si := 0; si < 0x38; si += 2 {
		in.set(2)
		in.copy8(0, 0x38, 0xc, si, 3, 0x36, 4, 0)
		in.copy8(0xd, 0x38, 0x19, si, 3, 0x36, 4, 0)
		in.copy8(0, 0x6e, 0xc, si+0x36, 0x10, 0x22, 4, 0)
		in.cur = 2
		in.fill(0x80, 0, 0xff, si+1, 0xc)
		in.cur = 0
		if si < di {
			in.copy8(0x13, 0, 0xf, di, 0xa, si+0x19, 4, 0)
		} else {
			in.copy8(0x13, si+1, 0xf, di+si+1, 0xa, 0x17, 4, 0)
			in.copy8(0x13, 0, 0x13, di, 0xa, si+1, 4, 2)
		}
		in.draw(2, shp, 0x80, si-0x37)
		in.copy8(0x10, 0, 0xc, 0, 0x10, si+1, 2, 0)
		in.copy8(0, 0, 0xc, si+0x59, 0x10, 0x4f-si, 2, 0)
		in.wait()
	}
	in.copy8(0, 0x20, 0, 0xa8, 0x28, 0x20, 6, 0)
	in.set(0x41)
}

// orb ports O0e96_06e2.
func (in *intro) orb() {
	in.loadCMP(in.str(inStrOrb), 5, 3)
	in.cur = 2
	orb := in.encode(0, 0, 0x14, 0x88)
	in.loadCMP(in.str(inStrTowr), 5, 3)
	var glow [4]*gfx.Shape
	glow[3] = in.encode(0x10, 0, 0x10, 0x68)
	in.fill(0, 0, 0x7f, 0x67, 0xc)
	for si := 1; si < 4; si++ {
		in.dither(0x80, 0, 0, 0, 0x80, 0x68, si)
		glow[3-si] = in.encode(0, 0, 0x10, 0x68)
	}
	in.fill(0, 0, 0x9f, 0x87, 0xc)
	in.cur = 0
	in.copyPage(3, 4)
	in.wait()
	in.clearPage()
	in.music(6)
	frame := func(si int) {
		if si >= 0 {
			in.draw(2, glow[si], 0x10, 0x10)
		}
		in.draw(2, orb, 0, 0)
		in.copy8(0, 0, 0xa, 0x18, 0x14, 0x88, 2, 0)
		in.wait()
	}
	for si := -1; si < 4; si++ {
		in.set(3)
		frame(si)
	}
	in.copy8(0, 0x40, 0, 0xa8, 0x28, 0x10, 6, 0)
	in.delay(0x28)
	in.music(6)
	for si := 3; si > -2; si-- {
		in.set(3)
		in.cur = 2
		in.fill(0x10, 0x10, 0x8f, 0x77, 0xc)
		in.cur = 0
		frame(si)
	}
	in.set(0x28)
}

// waterdeepEntry ports O0e96_015d.
func (in *intro) waterdeepEntry() {
	in.loadPal(inStrWtrdp2C)
	in.cur = 2
	in.loadCMP(in.str(inStrWtrdp1), 5, 3)
	var s [4]*gfx.Shape
	s[3] = in.encode(0, 0, 0x14, 0x88)
	for si := 1; si < 4; si++ {
		in.dither(0, 0, 0, 0, 0xa0, 0x88, si)
		s[3-si] = in.encode(0, 0, 0x14, 0x88)
	}
	in.copyPage(3, 4)
	in.wait()
	in.clearText()
	in.music(6)
	for si := 0; si < 4; si++ {
		in.set(3)
		in.draw(0, s[si], 0x50, 0x18)
		in.wait()
	}
	in.copy8(0, 0x50, 0, 0xa8, 0x28, 0x10, 6, 0)
	in.cur = 2
	in.set(0x32)
	var v [3]*gfx.Shape
	v[0] = in.encode(0x14, 0, 0x14, 0x88)
	in.loadCMP(in.str(inStrWtrdp2), 5, 3)
	v[1] = in.encode(0, 0, 0x14, 0x88)
	v[2] = in.encode(0x14, 0, 0x14, 0x88)
	in.loadCMP(in.str(inStrWtrdp3), 5, 3)
	in.cur = 2
	water, fig := in.waterFrames(0)
	in.copyPage(3, 4)
	for si := 0; si < 3; si++ {
		in.wait()
		in.fill(0, 0, 0x9f, 0x87, 0xc)
		in.draw(2, v[si], 0, 0)
		in.copy8(0, 0, 0xa, 0x18, 0x14, 0x88, 2, 0)
		in.set(3)
	}
	in.set(4)
	in.copy8(0, 0, 0xa, 0x18, 0x14, 0x88, 4, 0)
	in.wait()
	in.set(4)
	in.copy8(0x14, 0, 0xa, 0x18, 0x14, 0x88, 4, 0)
	in.clearText()
	in.wait()
	in.copy8(0, 0xb8, 5, 0xb8, 0x1d, 0x10, 4, 0)
	x, y := 0x108, 0xb
	for si := 0; si <= 0x46; si++ {
		in.set(3)
		in.copy8((x-2)>>3, y-2, 0, 0, 6, 0x24, 4, 4)
		in.draw(4, fig[figFrame(si)], x, y)
		in.copy8((x-2)>>3, y-2, (x-0x52)>>3, y+0x16, 6, 0x24, 4, 0)
		in.copy8(0, 0, (x-2)>>3, y-2, 6, 0x24, 4, 4)
		x--
		y++
		in.waterGlint(water)
		in.wait()
	}
	in.cur = 0
}

// king ports O0e96_1265.
func (in *intro) king() {
	in.loadCMP(in.str(inStrKing), 5, 3)
	in.copyPage(3, 4)
	x, di, h, w := 0xf, 0xe, 1, 1
	for si := 0; si < 10; si++ {
		in.set(1)
		in.copy8(x, di*8, x, di*8, w, h*8, 4, 0)
		if x > 6 {
			x--
		}
		if di > 0 {
			di -= 2
		}
		if w += 3; w+x > 0x22 {
			w = 0x22 - x
		}
		if h += 3; di+h > 0x17 {
			h = 0x17 - di
		}
		in.wait()
	}
	in.set(0x19)
	in.loadCMP(in.str(inStrAdventur), 5, 3)
	in.cur = 2
	var shp [4]*gfx.Shape
	var ys, fr [4]int
	for i := 0; i < 4; i++ {
		shp[i] = in.encode(in.i16(inKingX8+2*i), 0, in.i16(inKingW8+2*i), 0x62)
		ys[i] = (in.rnd()&3)*8 + 0xb4
		fr[i] = i * 5 & 3
	}
	in.cur = 0
	in.wait()
	in.copy8(0, 0, 0, 0, 0x28, 0xc8, 0, 4)
	for done := false; !done; {
		done = true
		in.set(2)
		for i := 0; i < 4; i++ {
			if ys[i] <= 0x52 {
				continue
			}
			fr[i] = (fr[i] + 1) & 7
			ys[i] = max(ys[i]+in.i16(inKingDY+2*fr[i]), 0x52)
			if ys[i] >= 0xb4 {
				done = false
				continue
			}
			x8, w8, y := in.i16(inKingX8+2*i), in.i16(inKingW8+2*i), ys[i]
			in.copy8(x8+8, y-2, 0, y-2, w8, 0xb6-y, 4, 4)
			in.draw(4, shp[i], 0, y)
			in.copy8(0, y-2, x8+8, y-2, w8, 0xb6-y, 4, 0)
			done = false
		}
		if in.rnd()&3 == 0 {
			in.music(7)
		}
		in.wait()
	}
	in.copy8(0, 0x60, 0, 0xa0, 0x28, 0x20, 6, 0)
	in.set(0x46)
}

// hands ports O0e96_15f9.
func (in *intro) hands() {
	in.cur = 2
	s1 := in.encode(0, 0x8c, 0x15, 0x3c)
	s2 := in.encode(0x15, 0x8c, 0xc, 0x3c)
	in.loadCMP(in.str(inStrHands), 3, 5)
	in.wait()
	in.clearText()
	in.fill(0, 0, 0xbf, 0x3f, 0x9d)
	in.draw(2, s1, 0, 4)
	in.draw(2, s2, 0x97, 4)
	in.zoom(0x19, 8, 0x12, 4, 3, 0, 0x15, 8, 6, 0, 0x1c, 0x17)
	in.copy8(0, 0x80, 0, 0xb0, 0x28, 0x10, 6, 0)
	in.delay(0xf)
	in.music(0xb)
	for si := -0x16; si <= 0x14; si += 4 {
		in.set(1)
		in.fill(0, 0, 0xa7, 0x3f, 0x9d)
		in.draw(2, s1, si, 4)
		in.draw(2, s2, 0x7f-(si+0x16), 4)
		in.copy8(0, 0, 0x12, 0x20, 0x15, 0x40, 2, 0)
		in.wait()
	}
	in.music(0xa)
	in.delay(0xf)

	in.cur = 4
	s1 = in.encode(0x11, 0, 0xb, 0x78)
	s2 = in.encode(0x1c, 0x70, 1, 0x1f)
	s3 := in.encode(9, 0x8a, 0xe, 0x36)
	in.cur = 2
	in.fill(0, 0, 0x87, 0x3f, 0x9d)
	in.draw(2, s1, 0x20, -0x50)
	in.draw(2, s2, 0x28, -0x10)
	in.zoom(0x12, 0x10, 0xa, 0xc, 0, 0, 0x11, 8, 0x11, 3, 0x19, 0xa)
	in.delay(0xf)
	for si := -0x50; si <= 0; si += 4 {
		in.set(1)
		in.fill(0, 0, 0x87, 0x3f, 0x9d)
		in.draw(2, s1, 0x20, si)
		in.draw(2, s2, 0x28, si+0x40)
		in.copy8(0, 0, 0xa, 0x60, 0x11, 0x40, 2, 0)
		in.wait()
	}
	in.music(0xc)
	in.delay(5)
	for si := 0; si > -0x36; si -= 4 {
		in.set(1)
		in.fill(0, 0, 0x87, 0x3f, 0x9d)
		in.draw(2, s3, 0xc, si+0x40)
		in.draw(2, s1, 0x20, si)
		in.copy8(0, 0, 0xa, 0x60, 0x11, 0x40, 2, 0)
		in.wait()
	}
	in.delay(0xf)

	in.cur = 4
	s1 = in.encode(0, 0, 0x11, 0x88)
	s2 = in.encode(0, 0x88, 9, 0x30)
	in.cur = 2
	in.fill(0, 0, 0x8f, 0x5f, 0x9d)
	in.draw(2, s1, -0x38, -0x38)
	in.draw(2, s2, 0x34, 0x31)
	in.zoom(9, 6, 0, 0, 0, 0, 0x12, 0xc, 8, 0xb, 0x15, 0xa)
	in.delay(0xf)
	in.music(0xb)
	for si := -0x38; si <= -8; si += 4 {
		in.set(1)
		in.fill(0, 0, 0x8f, 0x5f, 0x9d)
		in.draw(2, s1, si, si)
		k := 0
		if si == -8 {
			k = 3
		}
		in.draw(2, s2, 0x34+k, 0x31+k)
		in.copy8(0, 0, 0, 0, 0x12, 0x60, 2, 0)
		in.wait()
	}
	in.music(0xa)
	in.delay(0xf)
	in.delay(0xf)

	in.cur = 4
	s1 = in.encode(0x1c, 0, 0xb, 0x28)
	s2 = in.encode(0x1c, 0x28, 0xa, 0x48)
	in.cur = 2
	in.fill(0, 0, 0x57, 0x70, 0x9d)
	in.draw(2, s2, 0, 0x5a)
	in.zoom(0x14, 0xd, 0xf, 6, 0, 0, 0xb, 0xe, 0, 0, 0x18, 0x10)
	in.delay(0xf)
	di := 0x5a
	for si := -0x28; si <= 0; si += 4 {
		in.set(1)
		in.fill(0, 0, 0x57, 0x70, 0x9d)
		in.draw(2, s2, 0, di)
		in.copy8(0, 0, 0xf, 0x30, 0xb, 0x70, 2, 0)
		in.wait()
		di -= 5
	}
	in.music(0xd)
	for si := -0x28; si <= 0; si += 4 {
		in.set(1)
		in.fill(0, 0, 0x57, 0x27, 0x9d)
		in.draw(2, s1, 0, si)
		in.copy8(0, 0, 0xf, 0x30, 0xb, 0x28, 2, 0)
		in.wait()
	}
	in.set(0x30)
}

// waterdeepExit ports O0e96_1df3.
func (in *intro) waterdeepExit() {
	in.cur = 2
	in.loadCMP(in.str(inStrWtrdp3b), 5, 3)
	water, fig := in.waterFrames(0xf)
	door := in.encode(0x1f, 0x88, 5, 0x20)
	in.copyPage(3, 4)
	in.copy8(0, 0, 0, 0x88, 6, 0x24, 4, 4)
	in.wait()
	in.clearText()
	in.copy8(0x14, 0, 0xa, 0x18, 0x14, 0x88, 4, 0)
	x, y := 0x8c, 0x80
	in.wait()
	for si := 0; si <= 0x46; si++ {
		in.set(3)
		sx := max((x-2)>>3, 0x14)
		sy := min(y-2, 0x62)
		in.copy8(sx, sy, 0, 0, 6, 0x24, 4, 4)
		in.draw(4, fig[figFrame(si)], x, y)
		in.draw(4, door, 0xa0, 0x68)
		in.copy8(sx, sy, sx-10, sy+0x18, 6, 0x24, 4, 0)
		in.copy8(0, 0, sx, sy, 6, 0x24, 4, 4)
		x++
		y--
		in.waterGlint(water)
		in.wait()
	}
	in.cur = 0
	in.clearText()
	in.set(0x14)
	in.copy8(0, 0x88, 0, 0, 6, 0x24, 4, 4)
	in.loadPal(inStrOrbCol)
	in.loadCMP(in.str(inStrOrb2), 3, 5)
	in.copyPage(5, 2)
	in.set(6)
	in.lineWipe()
	in.wait()
	in.copy8(0, 0x90, 0, 0xb8, 0x28, 0x10, 6, 0)
	dy, sx, sy := 0, 0, 0x88
	for si := 0; si < 0x13; si++ {
		in.set(1)
		in.copy8(sx, sy, 0xa, dy+0x10, 0x14, 8, 2, 0)
		sy += 8
		dy += 8
		if si == 6 {
			sx, sy = 0x14, 0
		}
		in.wait()
	}
	in.music(3)
	in.set(0x3c)
	for si := 0; si < 0x38; si++ {
		in.copy8(0, si+0x88, 0xa, 0x10, 0x14, 0x38-si, 2, 0)
		in.copy8(0x14, 0, 0xa, 0x48-si, 0x14, 0x98-(0x38-si), 2, 0)
		in.wait()
		in.set(1)
	}
	for si := 1; si <= 0x30; si++ {
		in.set(1)
		in.copy8(0x14, si, 0xa, 0x10, 0x14, 0x98, 2, 0)
		in.wait()
	}
	in.set(0x1e)
	in.loadCMP(in.str(inStrTunnel), 3, 5)
	in.copyPage(5, 2)
	in.wait()
	in.cur = 0
	in.fill(0, 0x10, 0x13f, 0x1f, 0xc)
	in.fill(0, 0x88, 0x13f, 0xc7, 0xc)
	in.copy8(0, 0, 0xa, 0x20, 0x14, 0x78, 2, 0)
	in.set(0x32)
	in.loadPal(inStrZoomCol)
}

// tunnel ports O0e96_23a3.
func (in *intro) tunnel() {
	in.cur = 4
	r := in.encode(0x14, 0, 0x14, 0x78)
	l := in.encode(0, 0, 0x14, 0x78)
	in.drawFlip(4, r, 0xa0, 0)
	in.drawFlip(4, l, 0, 0)
	in.wait()
	for si := 0; si < 3; si++ {
		in.set(8)
		in.copy8(0, 0, 0xa, 0x20, 0x14, 0x78, 4, 0)
		in.music(7)
		in.wait()
		in.set(8)
		in.copy8(0, 0, 0xa, 0x20, 0x14, 0x78, 2, 0)
		in.music(7)
		in.wait()
	}
	in.copy8(0, 0xa0, 0, 0xb8, 0x28, 0x10, 6, 0)
	in.delay(0x12)
	in.set(5)
	in.copy8(0x14, 0, 0xa, 0x20, 0x14, 0x78, 2, 0)
	in.wait()
	in.set(5)
	in.copy8(0, 0x7a, 0xa, 0x20, 0x14, 0x3c, 2, 0)
	in.copy8(0x14, 0x7a, 0xa, 0x5c, 0x14, 0x3c, 2, 0)
	in.wait()
	in.set(5)
	in.copy8(0x14, 0, 0xa, 0x20, 0x14, 0x78, 4, 0)
	for si := 0; si < 6; si++ {
		in.copy8(si*6, 0xb9, 7, si*8+0x18, 6, 8, 2, 2)
	}
	in.wait()
	in.set(0x28)
	in.copy8(0, 0, 0xa, 0x20, 0x14, 0x78, 2, 0)
	in.loadCMP(in.str(inStrAvalanch), 5, 3)
	in.copyPage(3, 4)
	in.wait()
	in.set(0x10)
	in.copy8(0x21, 0, 0x11, 0x38, 6, 0x30, 4, 0)
	in.music(8)
	in.copy8(0, 0, 0, 0, 0x28, 0xb8, 0, 2)
	in.wait()
	in.music(4)
	shake := func() {
		in.set(1)
		in.copy8(0xa, (in.rnd()&7)+0x19, 0xa, 0x18, 0x14, 0x90, 2, 0)
	}
	for si := 0; si < 0x1e; si++ {
		in.wait()
		if si == 0 {
			in.clearText()
		}
		shake()
	}
	in.music(9)
	for si := 0; si < 6; si++ {
		in.set(1)
		in.copy8(in.u8(inAvalSX8+si), in.u8(inAvalSY+si), in.u8(inAvalDX8+si), in.u8(inAvalDY+si),
			in.u8(inAvalW8+si), in.u8(inAvalH+si), 4, 2)
		for di := 0; di < 4; di++ {
			in.wait()
			shake()
		}
	}
	in.copy8(0, 0, 0, 0, 0x28, 0xa8, 2, 0)
	in.set(0x28)
	in.loadCMP(in.str(inStrZoomtunl), 5, 3)
	in.copyPage(3, 4)
	in.wait()
	in.music(6)
	in.set(2)
	in.copy8(0, 0, 0xa, 0x20, 0x14, 0x78, 4, 0)
	in.wait()
	in.set(2)
	in.copy8(0x14, 0, 0xa, 0x20, 0x14, 0x78, 4, 0)
	in.wait()
	in.copy8(0, 0x78, 0xa, 0x1e, 0x14, 0x40, 4, 0)
	in.copy8(0x14, 0x78, 0xa, 0x5e, 0x14, 0x40, 4, 0)
	in.cur = 0
	in.copy8(0, 0xb0, 0, 0xb8, 0x28, 0x10, 6, 0)
	in.delay(0x32)
}

// PlayIntro is the exported entry point (used by cmd/shot -intro).
func (e *Engine) PlayIntro() { e.playIntro() }
