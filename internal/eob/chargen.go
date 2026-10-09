package eob

import "github.com/felix/goeob/internal/gfx"

// Character generation (overlay O20f6).

const (
	cgTmplSeg   = 0x206b // button templates: x8,y,w,h,key (8 bytes)
	cgImgSeg    = 0x2080 // button background rects on page 2: x8,y,w8,h
	cgBtnSeg    = 0x2084 // 17 buttons x 9: label rect x8,y,w8,h, label dx,dy, image, x8,y
	numFaces    = 0x2c
	dsCgStatTag = 0x115d // far ptrs, stat abbreviations at index 8..13
	dsCgNameX   = 0x5e44
	dsCgNameY   = 0x5e4c
	dsCgBoxX    = 0x5e54
	dsCgBoxY    = 0x5e5c
	dsCgUsed    = 0x5eac
	dsCgRaceMod = 0x5eb1
	dsCgLevels  = 0x5ef9
	dsCgRaceMin = 0x5f35
	dsCgRaceMax = 0x5f7d
	dsCgClsMin  = 0x5fc5
	dsCgNoExStr = 0x116e
	dsCgRectX   = 0x6079
	dsCgRectY   = 0x607d
	dsCgRaceCls = 0x6081
	dsCgClsAlgn = 0x6099
	dsCgItems   = 0x60d5
	dsCgClsItem = 0x6101
	dsClassFlag = 0x1231
	dsRaceStrs  = 0x11ad
	dsClassStrs = 0x11dd
	dsAlignStrs = 0x1240
	dsHlIdx     = 0x54f2
	dsHlRects   = 0x5482 // x1,y1,x2,y2 words
	dsHlCols    = 0x1ce7 // NUL terminated colour cycle
)

type chargen struct {
	slot             int // ds:dcb0
	sex              int // ds:fc8a
	used             [4]int
	faces            [numFaces]*gfx.Shape
	frames           [10]*gfx.Shape
	btnShapes        [17]*gfx.Shape
	min, max         [7]int // ds:dd04 / ds:dc98 (6: hit points)
	blinkSlot, frame int
	lastFrame        int
	nextBlink        uint32
}

func (e *Engine) cgBtn(b, i int) int { return int(e.X.FileU8(e.X.SegOffset(cgBtnSeg, b*9+i))) }
func (e *Engine) cgImg(n, i int) int { return int(e.X.FileU8(e.X.SegOffset(cgImgSeg, n*4+i))) }
func (e *Engine) u32(o int) uint32   { return uint32(e.X.U16(o)) | uint32(e.X.U16(o+2))<<16 }
func (e *Engine) named(c int) bool   { return e.Chars[c].Name[0] != 0 }

// cgInit ports O20f6_2547 (and O20f6_0000): loads CHARGEN* and cuts shapes.
func (e *Engine) cgInit() {
	e.mustCPS(e.X.Str(0x6272), 5)
	for i := range e.cg.faces {
		e.cg.faces[i] = gfx.Encode8(e.Scr.Pages[5], (i%10)*4, (i/10)*32, 4, 32)
	}
	e.mustCPS(e.X.Str(0x627b), 0)
	e.mustCPS(e.X.Str(0x6283), 5)
	for i := range e.cg.frames {
		e.cg.frames[i] = gfx.Encode8(e.Scr.Pages[5], i*4, 0, 4, 32)
	}
	for b := range e.cg.btnShapes {
		if e.cgBtn(b, 2) != 0 {
			e.cg.btnShapes[b] = gfx.Encode8(e.Scr.Pages[5], e.cgBtn(b, 0), e.cgBtn(b, 1), e.cgBtn(b, 2), e.cgBtn(b, 3))
		}
	}
	// O20f6_263a: CHARGENB (page 3 in the original) -> page 4 -> page 2;
	// cgDrawButton cuts button backgrounds (PLAY, REROLL...) from it.
	e.Scr.CopyPage(5, 2)
	e.Scr.CopyRegion(0x90, 0x40, 0, 0, 160, 128, 0, 2) // clean info panel
	for i := range e.cg.used {
		e.cg.used[i] = int(e.X.I8(dsCgUsed + i))
	}
	e.cg.blinkSlot = int(e.X.U8(dsCgUsed + 4))
	e.drawPage = 0
}

// cgButtons ports O20f6_042f: builds n buttons from template first.
func (e *Engine) cgButtons(first, n int) []Button {
	r := make([]Button, n)
	for i := range r {
		t := e.X.SegOffset(cgTmplSeg, (first+i)*8)
		f := e.X.File[t:]
		r[i] = Button{ID: i + 1, Key: int(f[4]), Key2: int(f[4]) + 0x100, Flags: 0x1100, X: int(f[0]) * 8, Y: int(f[1]), W: int(f[2]), H: int(f[3])}
	}
	return r
}

// cgInput ports O20f6_00f7 (S16a5_005a + button list): 0x8000|id for an
// activated button, else the raw key code (0 if none).
func (e *Engine) cgInput(btns []Button) int {
	e.present()
	e.Sys.Sleep(16)
	ev, ok := e.Sys.PollEvent()
	if !ok {
		return 0
	}
	switch ev.Kind {
	case EvQuit:
		panic(errQuit)
	case EvKeyDown:
		for _, b := range btns {
			if b.Key != 0 && b.Key&0xff == ev.Key {
				return 0x8000 | b.ID
			}
		}
		return ev.Key
	case EvMouseUp:
		e.mouseX, e.mouseY = ev.X, ev.Y
		for _, b := range btns {
			if ev.X >= b.X && ev.X < b.X+b.W && ev.Y >= b.Y && ev.Y < b.Y+b.H {
				return 0x8000 | b.ID
			}
		}
	}
	return 0
}

// cgDrawButton ports O20f6_0128. mode 0: draw to screen, 2: into the info
// panel backup on page 2 only.
func (e *Engine) cgDrawButton(b, state, mode int) {
	if b >= len(e.cg.btnShapes) {
		return
	}
	n := e.cgBtn(b, 6) + state
	dx, dy := 0x14, 0
	if mode != 0 {
		dx, dy = e.cgBtn(b, 7)+2, e.cgBtn(b, 8)-0x40
	}
	w, h := e.cgImg(n, 2)*8, e.cgImg(n, 3)
	e.Scr.CopyRegion(e.cgImg(n, 0)*8, e.cgImg(n, 1), dx*8, dy, w, h, 2, 2)
	if e.cgBtn(b, 2) != 0 {
		e.Scr.DrawShape(2, e.cg.btnShapes[b], dx*8+e.cgBtn(b, 4), dy+e.cgBtn(b, 5), 0, 0)
	}
	if mode == 2 {
		return
	}
	e.Scr.CopyRegion(0x14*8, 0, e.cgBtn(b, 7)*8, e.cgBtn(b, 8), w, h, 2, 0)
}

// cgPress ports O20f6_037a: button press animation.
func (e *Engine) cgPress(b int) {
	e.cgDrawButton(b, 1, 0)
	e.playSound(0x4c)
	e.present()
	e.waitTicks(5)
	e.cgDrawButton(b, 0, 0)
}

// cgPanel copies the clean info panel to its work copy on page 2.
func (e *Engine) cgPanel() { e.Scr.CopyRegion(0, 0, 160, 0, 160, 128, 2, 2) }

// cgShowPanel copies the info panel work copy to the screen.
func (e *Engine) cgShowPanel() { e.Scr.CopyRegion(160, 0, 0x90, 0x40, 160, 128, 2, 0) }

// cgPlayButton ports O20f6_0504.
func (e *Engine) cgPlayButton() {
	for i := 0; i < 4; i++ {
		if !e.named(i) {
			e.cgDrawButton(0xe, 0, 0)
			return
		}
	}
	e.drawPage = 2
	e.printShadow(e.X.Str(0x613d), 0xa8, 0x3d, 0xf, 0)
	e.drawPage = 0
	e.Scr.CopyRegion(0xa8, 0x3d, 0x98, 0x7d, 0x88, 0x28, 2, 0)
	e.cgDrawButton(0xf, 0, 0)
}

// cgPanelText ports O20f6_1071 / O20f6_10f6 (info panel message).
func (e *Engine) cgPanelText(s, y int) {
	e.cgPanel()
	e.drawPage = 2
	e.printShadow(e.X.Str(s), 0xa8, y, 0xf, 0)
	e.drawPage = 0
	e.cgShowPanel()
}

// cgPrintName ports O20f6_05ba.
func (e *Engine) cgPrintName(c, n int, col byte) {
	defer e.setFont(e.setFont(e.font6))
	x, y := e.i16(dsCgNameX+c*2), e.i16(dsCgNameY+c*2)
	e.printText(e.X.Str(0x618c), x, y, 0xc, 0xc)
	e.printText(e.Chars[c].NameStr(), x+(0x3c-e.font.W*n)/2, y, col, 0)
}

// strengthStr ports S0a72_0468.
func (e *Engine) strengthStr(str, ext int) string {
	if ext > 0 {
		if ext == 100 {
			ext = 0
		}
		return sprintfC(e.X.Str(dsStatFmtExt), str, ext)
	}
	return sprintfC(e.X.Str(dsStatFmt), str)
}

// cgDrawSheet ports O20f6_065b. mode 1: roll buttons, 2: view buttons,
// 3: modify buttons, 4: no portrait.
func (e *Engine) cgDrawSheet(c, mode int) {
	ch := &e.Chars[c]
	e.cgPanel()
	e.drawPage = 2
	if f, ok := ch.Face.(*gfx.Shape); ok && mode != 4 {
		e.Scr.DrawShape(2, f, 0xe0, 2, 0, 0)
	}
	center := func(s string, y int) { e.printShadow(s, (0x14-len(s))*4+0xa0, y, 0xf, 0) }
	center(ch.NameStr(), 0x23)
	center(e.farStr(dsRaceStrs+int(ch.RaceSex)*4), 0x2d)
	center(e.farStr(dsClassStrs+int(ch.Class)*4), 0x36)
	for i := 8; i < 14; i++ {
		e.printShadow(e.farStr(dsCgStatTag+i*4), 0xa3, i*8, 0xf, 0)
	}
	e.printShadow(e.X.Str(0x6197), 0xf8, 0x40, 0xf, 0)
	e.printShadow(sprintfC(e.X.Str(0x61a1), e.strengthStr(int(ch.StrCur), int(ch.StrExtCur)),
		int(ch.IntCur), int(ch.WisCur), int(ch.DexCur), int(ch.ConCur), int(ch.ChaCur)), 0xc0, 0x40, 0xf, 0)
	e.printShadow(sprintfC(e.X.Str(0x61b3), int(ch.AC), int(ch.HPMax)), 0x118, 0x40, 0xf, 0)
	f := 0x61b9
	if ch.Level[1] > 0 {
		f = 0x61bc
	}
	if ch.Level[2] > 0 {
		f = 0x61c2
	}
	e.printShadow(sprintfC(e.X.Str(f), int(ch.Level[0]), int(ch.Level[1]), int(ch.Level[2])), 0x118, 0x50, 0xf, 0)
	var btns []int
	switch mode {
	case 1:
		btns = []int{4, 7, 8, 6}
	case 2:
		btns = []int{0x10, 9}
	case 3:
		btns = []int{0xa, 0xb, 9}
	}
	for _, b := range btns {
		e.cgDrawButton(b, 0, 2)
	}
	e.drawPage = 0
	e.cgShowPanel()
}

// cgDrawFace ports O20f6_11ba: arg > 0x30 draws empty box arg-0x32,
// otherwise the current character's portrait in its box.
func (e *Engine) cgDrawFace(a int) {
	e.highlight(-1)
	if a > 0x30 {
		e.cgDrawButton(a-0x32, 0, 0)
		return
	}
	s := e.cg.slot
	if f, ok := e.Chars[s].Face.(*gfx.Shape); ok {
		e.Scr.DrawShape(0, f, e.i16(dsCgBoxX+s*2), e.i16(dsCgBoxY+s*2)+1, 0, 0)
	}
}

// cgBlink ports O20f6_1224: animated frame on the active box.
func (e *Engine) cgBlink() {
	c := &e.cg
	if c.blinkSlot != c.slot {
		c.nextBlink = 0
		c.blinkSlot = c.slot
	}
	if now := e.Sys.Millis() / tickMillis; c.nextBlink < now {
		if c.frame++; c.frame > 9 {
			c.frame = 0
		}
		c.nextBlink = now + 2
	}
	if c.lastFrame == c.frame {
		return
	}
	e.Scr.CopyRegion(c.slot*32, 0x80, 0x120, 0x80, 32, 32, 2, 2)
	e.Scr.DrawShape(2, c.frames[c.frame], 0x120, 0x80, 0, 0)
	e.Scr.CopyRegion(0x120, 0x80, e.i16(dsCgBoxX+c.slot*2)&^7, e.i16(dsCgBoxY+c.slot*2)+1, 32, 32, 2, 0)
	c.lastFrame = c.frame
}

// roll4d6 ports O20f6_1177: 4d6, lowest die dropped.
func (e *Engine) roll4d6() int {
	lo, sum := 10, 0
	for i := 0; i < 4; i++ {
		v := e.rollDice(1, 6, 0)
		sum += v
		if v < lo {
			lo = v
		}
	}
	return (sum - lo) & 0xff
}

// cgStatLimits ports O20f6_1ea9.
func (e *Engine) cgStatLimits() {
	ch := &e.Chars[e.cg.slot]
	race, cls := int(ch.RaceSex)>>1, int(ch.Class)
	for i := 0; i < 6; i++ {
		e.cg.min[i] = max(e.i16(dsCgClsMin+cls*12+i*2), e.i16(dsCgRaceMin+race*12+i*2))
		e.cg.max[i] = e.i16(dsCgRaceMax + race*12 + i*2)
		if e.u8(dsCgNoExStr+cls) != 0 {
			e.cg.max[0] = 18
		}
	}
}

// cgRollStats ports O20f6_1bf2.
func (e *Engine) cgRollStats(c int) {
	ch := &e.Chars[c]
	cls := int(ch.Class)
	for i := 0; i < 3; i++ {
		ch.Level[i] = e.X.U8(dsCgLevels + cls*4 + i)
		ch.Experience[i] = uint32(5000 / e.u8(dsCgLevels+cls*4+3))
	}
	e.cgStatLimits()
	var v [6]int
	race := int(int8(ch.RaceSex)) >> 1
	for i := range v {
		v[i] = e.roll4d6() + e.i16(dsCgRaceMod+race*12+i*2)
		if v[i] < e.cg.min[i] {
			v[i] = e.cg.min[i]
		}
		if i == 0 && v[0] == 18 {
			v[0] |= e.rollDice(1, 100, 0) << 8
		}
		if v[i] > e.cg.max[i] {
			v[i] = e.cg.max[i]
		}
	}
	ch.StrCur, ch.StrMax = int8(v[0]), int8(v[0])
	ch.StrExtCur, ch.StrExtMax = int8(v[0]>>8), int8(v[0]>>8)
	ch.IntCur, ch.IntMax = int8(v[1]), int8(v[1])
	ch.WisCur, ch.WisMax = int8(v[2]), int8(v[2])
	ch.DexCur, ch.DexMax = int8(v[3]), int8(v[3])
	ch.ConCur, ch.ConMax = int8(v[4]), int8(v[4])
	ch.ChaCur, ch.ChaMax = int8(v[5]), int8(v[5])
	ch.AC = int8(e.i16(dsDexAC+int(int8(v[3]))*2) + 10)
	hp := int8(e.hpGain(cls, int(ch.ConCur), int(ch.Level[0]), int(ch.Level[1]), int(ch.Level[2])))
	ch.HPCur, ch.HPMax = hp, hp
}

// cgModify ports O20f6_1add (modify attributes).
func (e *Engine) cgModify() {
	ch := &e.Chars[e.cg.slot]
	vals := []*int8{&ch.StrCur, &ch.IntCur, &ch.WisCur, &ch.DexCur, &ch.ConCur, &ch.ChaCur, &ch.HPCur}
	var none int8 // the original passes a null pointer
	e.cgDrawSheet(e.cg.slot, 3)
	for si := 0; si >= 0 && si <= 6; {
		ext := &none
		if si == 0 {
			ext = &ch.StrExtCur
		}
		si = e.cgModifyStat(si, vals[si], ext)
		e.cgDrawSheet(e.cg.slot, 3)
		if si == -2 {
			break
		}
		if si < 0 {
			si = 6
		}
		si %= 7
	}
	e.cgDrawSheet(e.cg.slot, 1)
}

// cgModifyStat ports O20f6_135d: edits stat si (6 = hit points). Returns
// the next stat to edit or -2 when done.
func (e *Engine) cgModifyStat(si int, val, ext *int8) int {
	btns := e.cgButtons(0x1f, 10)
	b := btns[si]
	ch := &e.Chars[e.cg.slot]
	x8 := (b.X + 0x20) / 8
	draw := func(f int) {
		s := sprintfC(e.X.Str(f+3), int(*val))
		if si == 0 {
			s = sprintfC(e.X.Str(f), e.strengthStr(int(*val), int(*ext)))
		}
		e.Scr.CopyRegion((x8-0x12)*8, b.Y-0x40, x8*8, b.Y, 40, b.H, 2, 0)
		e.printShadow(s, b.X+0x20, b.Y, 6, 0)
	}
	e.cgDrawSheet(e.cg.slot, 3)
	draw(0x6227)
	hpRange := func() {
		cls, con, l := int(ch.Class), int(ch.ConCur), ch.Level
		e.cg.max[6] = e.hpMaxRoll(cls, con, int(l[0]), int(l[1]), int(l[2]))
		e.cg.min[6] = e.hpMinRoll(cls, con, int(l[0]), int(l[1]), int(l[2]))
	}
	if si == 6 {
		hpRange()
	}
	show := func(old, cur int8, f, y int) {
		if old != cur {
			e.Scr.CopyRegion(0x78, y-0x40, 0x108, y, 40, 8, 2, 0)
			e.printShadow(sprintfC(e.X.Str(f), int(cur)), 0x108, y, 0xf, 0)
		}
	}
	exv, di, changed := int(*ext), si, false
	for !changed {
		v := int(*val)
		e.cgBlink()
		r := e.cgInput(btns)
		switch r {
		case KeyKP8:
			di--
			di %= 7
			changed = true
		case KeyKP2:
			di++
			di %= 7
			changed = true
		case KeyEqual, KeyKP6, KeyKPSub, 0x8008: // the original swaps keypad +/-
			e.cgPress(0xa)
			v++
		case KeyMinus, KeyKP4, KeyKPAdd, 0x8009:
			e.cgPress(0xb)
			v--
		case KeyKP5, KeyEsc, 0x800a:
			e.cgPress(9)
			changed, r = true, -1
		default:
			if r&0x8000 != 0 {
				if r = r&0xff - 1; r != si {
					di, changed = r, true
				}
			}
		}
		if v == int(*val) {
			if changed && r == -1 {
				di = -2
			}
			// ponytail: the original then reprints the field in white at
			// x*8 (off screen); cgModify redraws the sheet anyway.
			continue
		}
		mx := e.cg.max[si]
		if si == 0 {
			for v > 18 {
				v--
				exv++
			}
			for exv > 0 && v < 18 {
				v++
				exv--
			}
			mx &= 0xff
		}
		v = min(max(e.cg.min[si], v), mx)
		*val = int8(v)
		if si == 0 {
			if v < 18 || e.cg.max[0] < 19 {
				exv = 0
			}
			*ext = int8(min(exv, 100))
		}
		if si == 6 {
			ch.HPMax = int8(v)
		}
		draw(0x622d)
		switch si {
		case 4:
			old := ch.HPCur
			hpRange()
			hp := int8(min(max(int(ch.HPCur), e.cg.min[6]), e.cg.max[6]))
			ch.HPCur, ch.HPMax = hp, hp
			show(old, hp, 0x6233, 0x88)
		case 3:
			old := ch.AC
			ch.AC = int8(e.i16(dsDexAC+v*2) + 10)
			show(old, ch.AC, 0x6236, 0x80)
		}
	}
	return di
}

// hpConDiv is the constitution bonus and multi-class division shared by
// S0a72_01ae, S0a72_02b3 and S0a72_03a4, applied to the dice total r.
func (e *Engine) hpConDiv(class, con, a, b, c, r int) int {
	cb := int(e.X.I8(dsConHPBonus + con - 1))
	if cb > 2 && e.u8(dsHPDice0+class) < 10 {
		cb = 2
	}
	r += cb * a
	if b != 0 && e.u8(dsHPDice1+class) != 0 {
		r += cb * b
	}
	if c != 0 && e.u8(dsHPDice2+class) != 0 {
		r += cb * c
	}
	if e.u8(dsHPDice2+class) != 0 {
		r /= 3
	} else if e.u8(dsHPDice1+class) != 0 {
		r /= 2
	}
	return r
}

// hpMaxRoll ports S0a72_02b3: hit points with every die at its maximum.
func (e *Engine) hpMaxRoll(class, con, a, b, c int) int {
	return e.hpConDiv(class, con, a, b, c, a*e.u8(dsHPDice0+class)+b*e.u8(dsHPDice1+class)+c*e.u8(dsHPDice2+class))
}

// hpMinRoll ports S0a72_03a4: hit points with every die at 1 (at least 1).
func (e *Engine) hpMinRoll(class, con, a, b, c int) int {
	return max(1, e.hpConDiv(class, con, a, b, c, a+b+c))
}

// highlight ports S0d1c_0454: keyboard selection frame i (rects ds:5482)
// cycling through the colours at ds:1ce7, one step per tick; -1 erases.
func (e *Engine) highlight(i int) {
	now := e.Sys.Millis() / tickMillis
	rect := func(c int) {
		o := dsHlRects + e.hl*8
		e.Scr.DrawRect(0, e.i16(o), e.i16(o+2), e.i16(o+4), e.i16(o+6), byte(c))
	}
	if e.hl != i {
		if e.hl != -1 {
			rect(0xc)
		}
		e.hl, e.hlCol, e.hlNext = i, 0, now
		return
	}
	if i == -1 || now <= e.hlNext {
		return
	}
	if e.u8(dsHlCols+e.hlCol) == 0 {
		e.hlCol = 0
	}
	rect(e.u8(dsHlCols + e.hlCol))
	e.hlCol++
	e.hlNext = now + 1
}

// cgNextFace ports O20f6_0d5b: next portrait not used by another
// character and matching the chosen sex.
func (e *Engine) cgNextFace(si, dir int) int {
	for {
		if si < 0 {
			si = numFaces - 1
		}
		si %= numFaces
		bad := false
		for i := 0; i < 4; i++ {
			if e.named(i) && e.cg.used[i] == si {
				bad = true
			}
		}
		if (e.cg.sex != 0 && si < 0x1a) || (e.cg.sex == 0 && si > 0x1c) {
			bad = true
		}
		if !bad {
			return si
		}
		si += dir
	}
}

// cgSelectFace ports O20f6_0a5e.
func (e *Engine) cgSelectFace() {
	var shown [4]int
	btns := e.cgButtons(0x15, 6)
	start := 0
	if e.cg.sex != 0 {
		start = 0x1a
	}
	e.cgDrawSheet(e.cg.slot, 4)
	e.cgDrawButton(0xc, 0, 0)
	e.cgDrawButton(0xd, 0, 0)
	e.highlight(-1)
	start = e.cgNextFace(start, 1)
	sel, row := -1, 1
	for sel == -1 {
		first := start
		for i := range shown {
			shown[i] = start
			e.Scr.DrawShape(0, e.cg.faces[start], 0xb0+i*32, 0x42, 0, 0)
			start = e.cgNextFace(start+1, 1)
		}
		start = first
		r := 0
		for r == 0 {
			e.cgBlink()
			k := e.cgInput(btns)
			if e.KbdMode {
				e.highlight(row + 10)
			}
			switch k {
			case KeyEnter, KeyKP5:
				r = 3
			case KeyKP4, 0x8001:
				e.cgPress(0xc)
				r = 1
			case KeyKP6, 0x8002:
				e.cgPress(0xd)
				r = 2
			case 0x8003, 0x8004, 0x8005, 0x8006:
				row, r = k&0xff-3, 3
			}
		}
		if e.KbdMode {
			e.highlight(-1)
		}
		switch r {
		case 1:
			if row == 0 || !e.KbdMode {
				start = e.cgNextFace(start-1, -1)
			} else {
				row--
			}
		case 2:
			if row == 3 || !e.KbdMode {
				start = e.cgNextFace(start+1, 1)
			} else {
				row++
			}
		case 3:
			sel = row
		}
	}
	e.highlight(-1)
	e.cgBlink()
	p := shown[sel]
	e.cg.used[e.cg.slot] = p
	ch := &e.Chars[e.cg.slot]
	ch.Portrait, ch.Face = int8(p), e.cg.faces[p]
	e.cgDrawSheet(e.cg.slot, 1)
}

// cgStatsScreen ports O20f6_0ddc (roll / modify / faces / keep).
func (e *Engine) cgStatsScreen() {
	e.cgSelectFace()
	btns := e.cgButtons(0x1b, 4)
	for {
		e.cgBlink()
		redraw := true
		switch e.cgInput(btns) {
		case 0x8001:
			e.cgPress(4)
			e.cgBlink()
			e.cgRollStats(e.cg.slot)
		case 0x8002:
			e.cgPress(7)
			e.cgModify()
		case 0x8003:
			e.cgPress(8)
			e.cgSelectFace()
		case KeyKP5, 0x8004:
			e.cgPress(6)
			e.highlight(e.cg.slot + 6)
			e.highlight(-1)
			return
		default:
			redraw = false
		}
		if redraw {
			e.cgDrawSheet(e.cg.slot, 1)
		}
	}
}

// cgMenuTitle prepares the info panel for a selection menu.
func (e *Engine) cgMenuTitle(s int) {
	e.Scr.CopyRegion(0, 0, 0x90, 0x40, 160, 128, 2, 0)
	e.printShadow(e.X.Str(s), 0x93, 0x43, 9, 0)
}

// cgSelectRace ports O20f6_1f66.
func (e *Engine) cgSelectRace() int {
	e.drawPage = 0
	x, y := e.u8(dsCgRectX+e.cg.slot), e.u8(dsCgRectY+e.cg.slot)
	e.Scr.DrawRect(0, x, y, x+0x20, y+0x21, 0xc)
	e.cgMenuTitle(0x623f)
	e.Scr.Dims[menuDimBase+1].Unk8 = 0
	e.drawMenu(1, dsRaceStrs, 0xffffffff, 0, 0)
	for {
		if r := e.pollMenu(1, dsRaceStrs, 0xffffffff, 0); r != -1 {
			e.cg.sex = r % 2
			return r
		}
		e.cgBlink()
	}
}

// cgSelect ports the common part of O20f6_207a (class) and O20f6_2238
// (alignment): returns the string index or KeyEsc to go back.
func (e *Engine) cgSelect(m, title, strs int, mask uint32, n, backX int, escBack bool) int {
	d := &e.Scr.Dims[menuDimBase+m]
	d.H, d.Unk8 = menuCount(n, mask, 0), 0
	e.cgBlink()
	e.cgMenuTitle(title)
	e.cgDrawButton(5, 0, 0)
	e.drawMenu(m, strs, mask, 0, 0)
	for {
		e.cgBlink()
		r := e.pollMenu(m, strs, mask, 0)
		k := e.menuKey
		if (escBack && (k == KeyEsc || k == 0x32)) || ((k == 0x41 || k == 0x42) && inBox(e.mouseX, e.mouseY, backX, 0xab, 0x12f, 0xbb)) {
			e.cgPress(5)
			return KeyEsc
		}
		if r != -1 {
			return r
		}
	}
}

// cgSelectClass ports O20f6_207a.
func (e *Engine) cgSelectClass(raceSex int) int {
	mask := ^uint32(0)
	for i := 0; i < 4; i++ {
		if e.named(i) && e.Chars[i].Alignment >= 6 {
			mask = ^uint32(4) // no paladins with an evil party member
		}
	}
	return e.cgSelect(2, 0x624c, dsClassStrs, e.u32(dsCgRaceCls+raceSex/2*4)&mask, 0xf, 0x108, false)
}

// cgSelectAlign ports O20f6_2238.
func (e *Engine) cgSelectAlign(cls int) int {
	mask := ^uint32(0)
	for i := 0; i < 4; i++ {
		if e.named(i) && e.Chars[i].Class == 2 {
			mask = ^uint32(0x1c0) // no evil alignments with a paladin
		}
	}
	return e.cgSelect(3, 0x625a, dsAlignStrs, e.u32(dsCgClsAlgn+cls*4)&mask, 9, 0x109, true)
}

// inputString ports S082c_000b (line editor): typed letters are upper
// cased and inserted at the cursor; backspace, delete and the left/right
// arrows edit; returns the length, or -1 on Esc.
func (e *Engine) inputString(buf []byte, x8, y, maxLen int, fg, bg, cur byte) int {
	n := len(cstr(buf))
	if n > maxLen {
		n = maxLen
		buf[maxLen] = 0
	}
	pos := n
	if pos >= maxLen {
		pos--
	}
	e.Scr.CopyRegion((x8-1)*8, y, 0, 0xbf, (maxLen+2)*8, 9, 0, 2)
	draw := func() {
		e.Scr.CopyRegion(0, 0xbf, (x8-1)*8, y, (maxLen+2)*8, 9, 2, 0)
		e.printShadow(cstr(buf), x8*8, y, fg, bg)
		c := byte(' ')
		if pos < n {
			c = buf[pos]
		}
		// ponytail: the original redraws the cursor every 2 ticks with the
		// same colours, so it is drawn once here.
		e.printShadow(string(c), (x8+pos)*8, y, fg, cur)
	}
	draw()
	for {
		var si int
		for si == 0 {
			e.present()
			e.Sys.Sleep(16)
			ev, ok := e.Sys.PollEvent()
			switch {
			case !ok:
			case ev.Kind == EvQuit:
				panic(errQuit)
			case ev.Kind != EvKeyDown || ev.Ascii > 0xff:
			case ev.Ascii != 0:
				si = int(ev.Ascii)
			case ev.Key == KeyEnter:
				si = 0xd
			case ev.Key == KeyEsc:
				si = 0x1b
			case ev.Key == KeyBack:
				si = 8
			}
		}
		if si == 0xad || si == 0x7f {
			si = 0x7e
		}
		switch {
		case si == 8:
			if pos < n {
				if pos > 0 {
					copy(buf[pos-1:maxLen], buf[pos:maxLen])
					pos--
					n--
					buf[n] = 0
				}
			} else if n > 0 {
				n--
				buf[n] = 0
				pos--
			}
		case si == 0xb5:
			if pos > 0 {
				pos--
			}
		case si == 0xb3:
			if pos < n && pos < maxLen-1 {
				pos++
			}
		case si == 0x7e:
			if buf[pos] != 0 {
				copy(buf[pos:], buf[pos+1:n+1])
				n--
			}
		case si == 0x1b || si == 0xd || si < 0x20 || si > 0x7e:
		case pos == 0 && si == ' ':
		default:
			if si >= 'a' && si <= 'z' {
				si -= 0x20
			}
			if pos < n {
				copy(buf[pos+1:maxLen+1], buf[pos:maxLen])
				buf[pos] = byte(si)
				pos++
				if n == maxLen {
					buf[maxLen] = 0
				}
			} else {
				if pos == maxLen {
					pos--
					n--
				}
				buf[pos] = byte(si)
				pos++
				buf[pos] = 0
			}
			n = min(n+1, maxLen)
			pos = min(pos, maxLen-1)
		}
		draw()
		switch si {
		case 0xd:
			return n
		case 0x1b:
			return -1
		}
	}
}

// cgCreate ports O20f6_23ec: race, class, alignment, stats, portrait, name.
func (e *Engine) cgCreate() {
	e.setDim(2)
	ch := &e.Chars[e.cg.slot]
	for step := 0; step != 3; {
		back := false
		switch step {
		case 0:
			ch.RaceSex = byte(e.cgSelectRace())
		case 1:
			v := e.cgSelectClass(int(ch.RaceSex))
			ch.Class, back = byte(v), v == KeyEsc
		case 2:
			v := e.cgSelectAlign(int(ch.Class))
			ch.Alignment, back = byte(v), v == KeyEsc
		}
		if step++; back {
			step -= 2
		}
	}
	e.cgRollStats(e.cg.slot)
	e.cgStatsScreen()
	ch.Name[0] = 0
	for ch.Name[0] == 0 {
		e.cgDrawFace(0)
		e.cgDrawSheet(e.cg.slot, 0)
		e.printShadow(e.X.Str(0x626c), 0x95, 0x64, 9, 0)
		n := e.inputString(ch.Name[:], 0x18, 0x64, 10, 0xf, 0, 8)
		e.cgPrintName(e.cg.slot, n, 2)
	}
}

// cgView ports O20f6_0ee5: shows existing characters (delete / play).
func (e *Engine) cgView() bool {
	btns := e.cgButtons(0, 7)
	play, done := false, false
	e.highlight(-1)
	e.cgDrawSheet(e.cg.slot, 2)
	for e.named(e.cg.slot) && !done {
		e.highlight(e.cg.slot + 6)
		si := e.cg.slot
		switch k := e.cgInput(btns); k {
		case 0x8001, 0x8002, 0x8003, 0x8004:
			si = k&0xff - 1
		case 0x8005:
			play, done = true, true
		case KeyEnter, KeySpace, KeyKP5, KeyEsc, 0x8006:
			e.cgPress(9)
			done = true
		case 0x8007:
			e.cgPress(0x10)
			e.Chars[si].Name[0] = 0
			e.cgPrintName(si, 1, 0xc)
			e.cgDrawFace(si + 0x32)
		case KeyKP4, KeyKP6:
			si ^= 1
		case KeyKP8, KeyKP2:
			si ^= 2
		}
		if done {
			e.highlight(-1)
		}
		if !e.named(si) {
			done = true
		}
		if si != e.cg.slot {
			e.cg.slot = si
			e.highlight(-1)
			if !done {
				e.cgDrawSheet(si, 2)
			}
		}
	}
	return play
}

// cgGiveItems ports O20f6_26c6: starting equipment.
func (e *Engine) cgGiveItems() {
	e.Chars[0].Inventory[2] = uint16(e.duplicateItem(0x23))
	for c := 0; c < 4; c++ {
		ch := &e.Chars[c]
		ch.Flags, ch.Food, ch.ID = 1, 100, byte(c)
		ch.Inventory[3] = uint16(e.duplicateItem(0xa))
		for i := 0; i < 4; i++ {
			k := int(e.X.I8(dsCgClsItem + int(ch.Class)*4 + i))
			it, si := e.i16(dsCgItems+k*4), e.i16(dsCgItems+k*4+2)
			// ponytail: k == -1 makes the original read the item table tail
			// (item 511, slot 0); guarded here as "no item".
			if it == -1 || it >= len(e.Items) {
				continue
			}
			if si < 0 {
				si = 0
				if ch.Inventory[si] != 0 {
					si = 1
				}
				if ch.Inventory[si] != 0 {
					si = 2
				}
			}
			if si != 2 && ch.Inventory[si] != 0 {
				continue
			}
			if it == 5 && ch.RaceSex>>1 == 3 {
				it = 0x24
			}
			n := e.duplicateItem(it)
			if si == 2 {
				for ch.Inventory[si] != 0 {
					si++
				}
			}
			ch.Inventory[si] = uint16(n)
		}
		e.recalcArmorClass(c)
	}
}

// chargen ports O20f6_28f9: the party creation screen.
func (e *Engine) chargen() {
	e.setFont(e.font8)       // O2106_08f5
	defer e.setFont(e.font6) // O2106_0909
	e.cgInit()
	e.setDim(2)
	e.cgPanelText(0x61cb, 0x10)
	e.cgPlayButton()
	btns := e.cgButtons(0, 5)
	e.playSound(0x14)
	for play := false; !play; {
		e.highlight(e.cg.slot + 6)
		k := e.cgInput(btns)
		switch k {
		case KeyKP4, KeyKP6:
			e.cg.slot ^= 1
		case KeyKP8, KeyKP2:
			e.cg.slot ^= 2
		}
		if k != 0 {
			e.highlight(-1)
		}
		if k&0x8000 != 0 {
			if id := k&0xf - 1; id == 4 {
				play = true
			} else {
				e.cg.slot, k = id, KeyEnter
			}
		}
		if k == KeyKP5 || k == KeyEnter || k == KeySpace {
			e.highlight(-1)
			if e.named(e.cg.slot) {
				old := e.cg.slot
				play = e.cgView()
				if !e.named(e.cg.slot) && old != e.cg.slot {
					e.cgCreate()
				}
			} else {
				e.cgCreate()
			}
			e.cgPanelText(0x61cb, 0x10)
			e.cgPlayButton()
		}
		for i := 0; i < 4 && play; i++ {
			play = e.named(i)
		}
	}
	e.cgPress(0xf)
	e.cgPanelText(0x6207, 0x20)
	e.present()
	e.cgFinish()
}

// cgFinish is the tail of O20f6_28f9: starting items, mage spells, maxima.
func (e *Engine) cgFinish() {
	e.cgGiveItems()
	for c := 0; c < 4; c++ { // O20f6_280d
		if e.u8(dsClassFlag+int(e.Chars[c].Class))&2 != 0 {
			e.Chars[c].MageSpellsAvail = 0x26c
		}
	}
	for c := 0; c < 4; c++ { // O20f6_284a
		ch := &e.Chars[c]
		ch.StrMax, ch.StrExtMax, ch.IntMax, ch.WisMax = ch.StrCur, ch.StrExtCur, ch.IntCur, ch.WisCur
		ch.DexMax, ch.ConMax, ch.ChaMax, ch.HPMax = ch.DexCur, ch.ConCur, ch.ChaCur, ch.HPCur
	}
}
