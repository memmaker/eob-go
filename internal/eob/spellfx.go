package eob

// Spell visual effects (S0ef3 particle animators, sparkles).

const (
	dsExplType     = 0x55e6 // i8 per missile item: colour table / explosion kind
	dsExplStart    = 0x55ef // drawSceneShapes start index per dim map value
	dsExplColours  = 0x5615 // far ptrs to 0-terminated colour tables
	dsExplFloorY   = 0x579b // particle floor y per scale
	dsSparkFrames  = 0x5621 // portrait sparkle: frame bitmask
	dsSparkMask    = 0x5629
	dsSparkShift   = 0x562d
	dsSparkXY      = 0x5631 // 4 x {dx, dy}
	dsSparkBoxX8   = 0x5639
	dsSparkBoxY    = 0x5645
	dsWandFrames   = 0x564b // Silvias: 11 u32 frame masks
	dsWandMask     = 0x5677 // 16 u32
	dsWandShift    = 0x56b7
	dsWandX        = 0x56c7
	dsWandY        = 0x56d7
	dsConeColours  = 0x5737 // u16 colours, 0-terminated
	dsFxRndState   = 0x8484
	fxMaxParticles = 0x96
)

func (e *Engine) getPixel(page, x, y int) byte {
	if x < 0 || x >= 320 || y < 0 || y >= 200 {
		return 0
	}
	return e.Scr.Pages[page][y*320+x]
}

func (e *Engine) putPixel(x, y int, c byte) {
	if x >= 0 && x < 320 && y >= 0 && y < 200 {
		e.Scr.Pages[0][y*320+x] = c
	}
}

// fxFrame shows page 0 and delays n ticks (skipped without a System).
func (e *Engine) fxFrame(n int) {
	if e.Sys != nil {
		e.present()
		e.waitTicks(n)
	}
}

// waitRetrace ports S1a16_0027 (S1de4_000d(1): one VGA vertical retrace,
// 70 Hz in mode 13h), called n times; shows page 0 first.
func (e *Engine) waitRetrace(n int) {
	if e.Sys != nil {
		e.present()
		e.Sys.Sleep(n * 1000 / 70)
	}
}

// rndRange ports S1aaa_0005.
func (e *Engine) rndRange(lo, hi int) int {
	for {
		v := e.rand()*(hi-lo+1)/0x7fff + lo
		if uint16(v) <= uint16(hi) {
			return v
		}
	}
}

// rnd8 ports S1d8c_0000 (3-byte shift register RNG).
func (e *Engine) rnd8() byte {
	if e.fxRnd == nil {
		e.fxRnd = e.X.Bytes(dsFxRndState, 3)
	}
	s := e.fxRnd
	al := s[0] >> 2
	cf := s[0] >> 1 & 1
	c2 := s[2] >> 7
	s[2] = s[2]<<1 | cf
	c1 := s[1] >> 7
	s[1] = s[1]<<1 | c2
	al = al - s[0] - (c1 ^ 1)
	s[0] = al<<7 | s[0]>>1
	return s[0] ^ s[1]
}

// spellObjectExplode ports S0ef3_00cd.
func (e *Engine) spellObjectExplode(o *FlyingObject) {
	tbl := int(e.X.U16(dsExplColours + int(e.X.I8(dsExplType+int(o.Item)))*4))
	si := 0
	for si < 18 && e.scene.visIndex[si] != int(o.CurBlock) {
		si++
	}
	if si > 17 || si == 15 {
		return
	}
	di := int(e.T.DimMap[si])
	e.setLevelShapesDim(si, 5)
	if e.scene.shpDmX2 < e.scene.shpDmX1 {
		return
	}
	o.Enable = 0
	e.DrawScene(true)
	o.Enable = 2
	e.Scr.FillRect(scenePage, 0, 0, 0xb0, 0x78, 0)
	e.drawSceneShapes(e.u8(dsExplStart + di))
	e.setLevelShapesDim(si, 5)
	cx := int(e.T.ShapeCoords[(si*5+4)*2]) + 0x58
	e.playSound(0x23)
	if di < 3 {
		e.explodeFx(3-di, 0x93, 0x23, 0x14, 7, tbl, cx, 0x30)
	} else if di == 3 { // jump table cs:0267 covers 0..3 only
		e.explodeFx(0, 0x1cc, 0x32, 0x14, 4, tbl, cx, 0x30)
	}
}

// explodeFx ports S0ef3_2a53: bouncing particles drawn on page 0 wherever
// the foreground mask on page 2 is empty.
func (e *Engine) explodeFx(scale, spread, n, speed, shift, tbl, cx, cy int) {
	floor := int(e.X.I16(dsExplFloorY + scale*2))
	if scale != 0 {
		scale--
	}
	if n > fxMaxParticles {
		n = fxMaxParticles
	}
	d := e.Scr.Dims[5]
	x1, y1 := d.X*8, d.Y
	x2, y2 := x1+d.W*8, d.Y+d.H-1
	in := func(x, y int) bool { return x >= x1 && x <= x2 && y >= y1 && y <= y2 }
	px, py, vx, vy := make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	under, dc, col := make([]byte, n), make([]int, n), make([]int, n)
	for i := 0; i < n; i++ {
		vx[i] = e.rndRange(0, spread) - spread>>1
		vy[i] = e.rndRange(0, spread) - spread>>1 - spread>>(8-shift)
		dc[i] = e.rndRange(0x400/speed, 0x800/speed)
		col[i] = scale << 8
	}
	pos := func(i int) (int, int) {
		x, y := px[i]>>6>>scale+cx, py[i]>>6>>scale+cy
		if y > floor {
			y = floor
		}
		return x, y
	}
	for state := 2; state != 0; {
		if state != 2 {
			for i := n - 1; i >= 0; i-- {
				if x, y := pos(i); in(x, y) {
					e.putPixel(x, y, under[i])
				}
			}
		}
		state = 0
		for i := 0; i < n; i++ {
			if vx[i] > 0 {
				vx[i]--
			} else {
				vx[i]++
			}
			px[i] += vx[i]
			vy[i] += 5
			py[i] += vy[i]
			col[i] += dc[i]
			x, y := px[i]>>6>>scale+cx, py[i]>>6>>scale+cy
			if y >= floor || y < 0 {
				vy[i] = -(vy[i] >> 1)
			}
			if x >= 0x114 || x < -100 {
				vx[i] = -(vx[i] >> 1)
			}
			if y > floor {
				y = floor
			}
			mask := e.getPixel(scenePage, x, y)
			under[i] = e.getPixel(0, x, y)
			if c := e.X.U8(tbl + col[i]>>8); c != 0 {
				state = 1
				if mask == 0 && in(x, y) {
					e.putPixel(x, y, c)
				}
			} else {
				dc[i] = 0
			}
		}
		e.waitRetrace(2) // two S1a16_0027 calls
	}
}

// coneFx ports S0ef3_2ebc: particles converging / spreading from the view
// centre (cone of cold).
func (e *Engine) coneFx(n, radius, accel, speed, tbl int) {
	if n > fxMaxParticles {
		n = fxMaxParticles
	}
	radius <<= 6
	const cx, cy = 0x58, 0x30
	px, py, vx, vy := make([]int, n), make([]int, n), make([]int, n), make([]int, n)
	under, dc, col, delay := make([]byte, n), make([]int, n), make([]int, n), make([]int, n)
	for i := 0; i < n; i++ {
		r := e.rndRange(radius>>2, radius)
		dist, step := 0, 0
		for dist < r {
			step += accel
			dist += step
		}
		switch e.rnd8() & 3 {
		case 0:
			px[i], py[i], vx[i], vy[i] = 0x20, dist, step, 0
		case 1:
			px[i], py[i], vx[i], vy[i] = dist, 0x20, 0, step
		case 2:
			px[i], py[i], vx[i], vy[i] = 0x20, -dist, step, 0
		case 3:
			px[i], py[i], vx[i], vy[i] = -dist, 0x20, 0, step
		}
		if e.rnd8()&1 != 0 {
			vx[i], vy[i] = -vx[i], -vy[i]
		}
		dc[i] = e.rndRange(0x400/speed, 0x800/speed)
		delay[i] = e.rndRange(0, speed>>2)
	}
	slow := accel>>1 + accel>>2 + accel>>3
	toward := func(p, v int) int {
		if p > 0 {
			if v > 0 {
				return v - accel
			}
			return v - slow
		}
		if v < 0 {
			return v + accel
		}
		return v + slow
	}
	for state, frame := 2, 0; state != 0; frame++ {
		if state != 2 {
			for i := n - 1; i >= 0; i-- {
				e.putPixel(px[i]>>6+cx, py[i]>>6+cy, under[i])
			}
		}
		state = 0
		for i := 0; i < n; i++ {
			if delay[i] != 0 {
				delay[i]--
			} else {
				vx[i] = toward(px[i], vx[i])
				vy[i] = toward(py[i], vy[i])
				px[i] += vx[i]
				py[i] += vy[i]
				col[i] += dc[i]
			}
			x, y := px[i]>>6+cx, py[i]>>6+cy
			var mask byte
			if speed>>2 <= frame {
				mask = e.getPixel(scenePage, x, y)
			}
			under[i] = e.getPixel(0, x, y)
			if c := e.X.U16(tbl + col[i]>>8*2); c != 0 {
				state = 1
				if mask == 0 && delay[i] == 0 {
					e.putPixel(x, y, byte(c))
				}
			} else {
				dc[i] = 0
			}
		}
		e.waitRetrace(1)
	}
}

// sparkleFrame ports S0ef3_05bd.
func (e *Engine) sparkleFrame(c, f int) {
	e.drawCharPanel(c)
	for i := 0; i < 4; i++ {
		v := (e.u8(dsSparkFrames+f) & e.u8(dsSparkMask+i)) >> e.u8(dsSparkShift+i)
		if v == 0 {
			continue
		}
		x, y := 0xb5, 3
		if e.GuiMode == 0 {
			x, y = int(e.X.U16(dsSparkBoxX8+c*2))*8, int(e.X.I8(dsSparkBoxY+c))
		}
		x += int(e.X.I8(dsSparkXY+i*2)) - 8
		y += int(e.X.I8(dsSparkXY + i*2 + 1))
		e.Scr.DrawShape(0, e.GuiShapes[v-1], x, y, 0, 0)
	}
}

// spellSparkle ports S0ef3_0665 (portrait sparkle animation).
func (e *Engine) spellSparkle(c int) {
	from, to := c, c
	if c == -1 {
		from, to = 0, 5
	}
	for f := 0; f < 8; f++ {
		for i := from; i <= to; i++ {
			if e.Chars[i].Flags&1 != 0 && (e.GuiMode == 0 || e.InvChar == i) {
				e.sparkleFrame(i, f)
			}
		}
		e.fxFrame(2)
	}
	for i := from; i <= to; i++ {
		e.drawCharPanel(i)
	}
}

// wandSparkle ports S0ef3_06e7 (Wand of Silvias / scene sparkles).
func (e *Engine) wandSparkle() {
	e.Scr.CopyRegion(0, 0, 0, 0, 176, 120, 0, scenePage)
	bg := append([]byte(nil), e.Scr.Pages[scenePage]...)
	u32 := func(o int) uint32 { return uint32(e.X.U16(o)) | uint32(e.X.U16(o+2))<<16 }
	for f := 0; f < 11; f++ {
		copy(e.Scr.Pages[scenePage], bg)
		for i := 0; i < 16; i++ {
			if v := u32(dsWandFrames+f*4) & u32(dsWandMask+i*4) >> e.u8(dsWandShift+i); v != 0 {
				e.Scr.DrawShape(scenePage, e.GuiShapes[v-1], e.u8(dsWandX+i), e.u8(dsWandY+i), 0, 0)
			}
		}
		e.Scr.CopyRegion(0, 0, 0, 0, 176, 120, scenePage, 0)
		e.fxFrame(2)
	}
	// The original restores only the 16 saved 16x16 blocks, onto page 0;
	// page 2 keeps the last frame.
	for i := 0; i < 16; i++ {
		x, y := e.u8(dsWandX+i)&^7, e.u8(dsWandY+i)
		for r := y; r < y+16 && r < 200; r++ {
			copy(e.Scr.Pages[0][r*320+x:r*320+x+16], bg[r*320+x:r*320+x+16])
		}
	}
}
