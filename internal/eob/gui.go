package eob

import (
	"github.com/felix/goeob/internal/gfx"
)

// GUI colour variables (set by O2106_0937 for VGA).
type guiColors struct {
	BoxFill, BoxHi, BoxLo, Hi2, HPLow, Text, Misc1, Misc2, BtnFill, BtnHi, BtnLo byte
}

// initGuiColors ports O2106_0937 (VGA branch; the EGA branch is nonvga).
func (e *Engine) initGuiColors() {
	from := e.X.OverlayBase(0x2106) + 0x0948
	to := e.X.OverlayBase(0x2106) + 0x0981
	g := func(a int) byte { v, _ := e.X.ImmStoreByte(from, to, a); return v }
	e.col = guiColors{BoxFill: g(0xf57e), BoxHi: g(0xfb4c), BoxLo: g(0xf53a), Hi2: g(0xf53b), HPLow: g(0xfb7a), Text: g(0xfb7b),
		Misc1: g(0xa9c4), Misc2: g(0xa928), BtnFill: g(0xa95a), BtnHi: g(0x8a6c), BtnLo: g(0x8a7d)}
}

// panel layout tables
const (
	dsHPBarColors  = 0x5d1c
	dsBorderX      = 0x5d1f
	dsBorderY      = 0x5d21
	dsHPX          = 0x5d24
	dsHPY          = 0x5d26
	dsPortraitX    = 0x5d2c
	dsPortraitY    = 0x5d2e
	dsHandX        = 0x5d31
	dsHandY        = 0x5d33
	dsBoxSrcX8     = 0x5d4b
	dsBoxDstX8     = 0x5d4d
	dsBoxY         = 0x5d4f
	dsStrSwapping  = 0x5d82
	dsStrHP        = 0x5d52
	dsStrHPFmt     = 0x5d55
	dsStrMiss      = 0x5d61
	dsStrHack      = 0x5d66
	dsStrCant      = 0x5d6b
	dsStrReach     = 0x5d71
	dsStrNo        = 0x5d77
	dsStrAmmo      = 0x5d7a
	dsStrNumFmt    = 0x5d7f
	charStatusAct  = 1
	charStatusLive = 2
	charStatusCons = 4
)

// charStatus ports S0d1c_0670.
func (e *Engine) charStatus(c, mask int) bool {
	if c == 6 {
		// Original bug: callers probe char 6, one past ds:f580's six chars.
		// Its Flags (+1) is ds:fb33, the high byte of stepsUntilScriptCall;
		// its HP byte (+0x1b) is ds:fb4d, filled from INF+0x12 (O210c_082f).
		flags := byte(e.StepsUntilScript >> 8)
		hp := 0
		if len(e.LevelINF) > 0x12 {
			hp = int(int8(e.LevelINF[0x12]))
		}
		return (mask&1 == 0 || flags&1 != 0) && (mask&2 == 0 || hp > -10) &&
			(mask&4 == 0 || hp > 0) && (mask&8 == 0 || flags&4 == 0) && (mask&0x10 == 0 || flags&2 == 0)
	}
	ch := &e.Chars[c]
	ok := true
	if mask&1 != 0 {
		ok = ok && ch.Flags&1 != 0
	}
	if mask&2 != 0 {
		ok = ok && ch.HPCur > -10
	}
	if mask&4 != 0 {
		ok = ok && ch.HPCur > 0
	}
	if mask&8 != 0 {
		ok = ok && ch.Flags&4 == 0
	}
	if mask&0x10 != 0 {
		ok = ok && ch.Flags&2 == 0
	}
	return ok
}

// setFont ports S1a47_0169; returns the previous font, which callers keep
// to restore it like the original's saved ds:fe4e.
func (e *Engine) setFont(f *gfx.Font) *gfx.Font {
	old := e.font
	e.font = f
	return old
}

func (e *Engine) printText(s string, x, y int, fg, bg byte) {
	e.Scr.PrintText(e.font, e.drawPage, s, x, y, fg, bg)
}

// drawCharPanel ports S082c_0974 S082c_09a6 (digit-count*3 width, inlined).
// drawCharPanel ports S1547_0991 (playfield mode).
func (e *Engine) drawCharPanel(c int) {
	if c < 0 || !e.charStatus(c, 1) { // campSelectChar passes -1 (no keyboard selection yet)
		return
	}
	ch := &e.Chars[c]
	fg, fg2 := byte(0xc), byte(0xf)
	if ch.Flags&6 != 0 {
		fg, fg2 = 8, 6
	}
	_ = fg2
	switch e.GuiMode {
	case 0:
		defer e.setFont(e.setFont(e.font6))
		di := e.u8(dsBoxSrcX8 + c&1)
		si := e.u8(dsBoxY + c>>1)
		e.Scr.CopyRegion(0x16*8, 0xa8, di*8, si, 64, 0x18, 2, 2)
		e.Scr.CopyRegion(0x1e*8, 0xa8, di*8, si+0x18, 64, 0x1a, 2, 2)
		old := e.drawPage
		e.drawPage = 2
		x := di * 8
		if int(e.SwapChar) == c {
			e.printText(e.X.Str(dsStrSwapping), x+2, si+2, 8, e.col.BoxFill)
		} else {
			e.printText(ch.NameStr(), x+2, si+2, fg, e.col.BoxFill)
		}
		e.drawPortrait(c)
		e.drawHand(c, 0)
		e.drawHand(c, 1)
		e.drawHP(c)
		e.drawCharBorder(c)
		if ch.DamageTaken > 0 {
			s := itoa(int(ch.DamageTaken))
			e.Scr.DrawShape(2, e.MiscShapes[3], x+0xd, si+0x1e, 0, 0)
			e.printText(s, x+0x22-len(s)*3, si+0x2a, 0xf, 0)
		}
		e.drawPage = old
		if old == 0 {
			e.Scr.CopyRegion(di*8, si, e.u8(dsBoxDstX8+c&1)*8, si, 64, 0x32, 2, 0)
		}
	case 1, 2:
		e.drawInvPanel(c)
	}
}

// drawPortrait ports S1547_0544.
func (e *Engine) drawPortrait(c int) {
	if !e.charStatus(c, 1) {
		return
	}
	ch := &e.Chars[c]
	var x, y int
	if e.GuiMode == 0 {
		x, y = e.u8(dsPortraitX+c&1), e.u8(dsPortraitY+c>>1)
		if e.drawPage == 0 {
			x += 0xb0
		}
	} else {
		if e.InvChar != c {
			return
		}
		x, y = 0xb5, 3
	}
	if ch.HPCur == -10 {
		e.Scr.DrawShape(e.drawPage, e.GuiShapes[3], x, y, 0, 0)
		return
	}
	if ch.EffectFlags&4 != 0 && e.GuiMode == 0 {
		e.Scr.FillRect(e.drawPage, x, y, x+0x1f, y+0x1f, 0xc)
		return
	}
	if f, ok := ch.Face.(*gfx.Shape); ok {
		e.Scr.DrawShape(e.drawPage, f, x, y, 0, 0)
	}
	if ch.HPCur < 1 {
		e.Scr.DrawShape(e.drawPage, e.GuiShapes[4], x, y, 0, 0)
	}
}

// drawHP ports S1547_033f.
func (e *Engine) drawHP(c int) {
	if !e.charStatus(c, 1) {
		return
	}
	ch := &e.Chars[c]
	if e.GuiMode != 0 && e.InvChar != c {
		return
	}
	var x, y, w, h int
	if e.GuiMode == 0 {
		x, y = e.u8(dsHPX+c&1), e.u8(dsHPY+c>>1)
		w, h = 0x26, 3
		if e.drawPage == 0 {
			x += 0xb0
		}
	} else {
		x, y, w, h = 0xfa, 0x10, 0x33, 5
	}
	if e.BarGraphs {
		cur, max := int(ch.HPCur)+10, int(ch.HPMax)+10
		col := 0
		if max/3 > cur {
			col = 1
		}
		if cur <= 10 {
			col = 2
		}
		if e.GuiMode == 0 {
			e.printText(e.X.Str(dsStrHP), x-13, y-1, 0xc, e.col.BoxFill)
		}
		e.drawBar(x, y, cur, max, w, h, byte(e.u8(dsHPBarColors+col)), e.col.HPLow)
		return
	}
	s := sprintfC(e.X.Str(dsStrHPFmt), int(ch.HPCur), int(ch.HPMax))
	if e.GuiMode == 0 {
		x -= 13
		y--
	}
	e.printText(s, x, y, 0xc, e.col.BoxFill)
}

// drawBar ports S1547_028c.
func (e *Engine) drawBar(x, y, cur, max, w, h int, col, bg byte) {
	if max < 1 {
		return
	}
	if cur < 0 {
		cur = 0
	}
	if cur > max {
		cur = max
	}
	n := w * cur / max
	if n <= 0 && cur > 0 {
		n = 1
	}
	h--
	e.Scr.DrawBox3D(e.drawPage, x-1, y-1, x+w+1, y+h+1, e.col.BoxLo, e.col.BoxHi, -1)
	if n > 0 {
		e.Scr.FillRect(e.drawPage, x, y, x+n, y+h, col)
	}
	if n < w {
		e.Scr.FillRect(e.drawPage, x+n, y, x+w, y+h, bg)
	}
}

// drawHand ports S1547_07e7.
func (e *Engine) drawHand(c, hand int) {
	ch := &e.Chars[c]
	x := e.u8(dsHandX + c&1)
	y := e.u8(dsHandY + c&6 + hand)
	if e.drawPage == 0 {
		x += 0xb0
	}
	item := int(ch.Inventory[hand])
	e.Scr.DrawBox3D(e.drawPage, x, y, x+0x1e, y+0xf, e.col.BoxHi, e.col.BoxLo, int(e.col.BoxFill))
	if e.KbdMode && c == e.InvChar && hand == e.kbdSlot {
		e.Scr.DrawRect(e.drawPage, x, y, x+0x1e, y+0xf, 0xf)
	}
	if ch.SlotStatus[hand] != 0 {
		e.drawSlotStatus(x, y, int(ch.SlotStatus[hand]))
		return
	}
	if item != 0 {
		e.drawItemIcon(e.drawPage, item, x+8, y)
	} else {
		e.Scr.DrawShape(e.drawPage, e.ItemIcons[85+hand], x+8, y, 0, 0)
	}
	if ch.DisabledSlots&(1<<hand) != 0 || !e.canUseHand(c, hand) || ch.HPCur <= 0 || ch.Flags&4 != 0 {
		e.Scr.DrawShape(e.drawPage, e.GuiShapes[5], x, y, 0, 0)
	}
}

// drawSlotStatus ports S1547_0642 (MISS/HACK/... or damage number).
func (e *Engine) drawSlotStatus(x, y, v int) {
	if v > -3 {
		e.Scr.DrawShape(e.drawPage, e.MiscShapes[4], x-1, y, 0, 0)
	} else {
		e.Scr.DrawBox3D(e.drawPage, x, y, x+0x1e, y+0xf, e.col.BtnFill, e.col.BtnHi, int(e.col.BtnLo))
	}
	var l1, l2 string
	switch v {
	case -1:
		l1 = e.X.Str(dsStrMiss)
		y++
	case -2:
		l1 = e.X.Str(dsStrHack)
	case -3:
		l1, l2 = e.X.Str(dsStrCant), e.X.Str(dsStrReach)
	case -4:
		l1, l2 = e.X.Str(dsStrNo), e.X.Str(dsStrAmmo)
	default:
		l1 = sprintfC(e.X.Str(dsStrNumFmt), v)
	}
	if l2 == "" {
		e.printText(l1, x+0x10-len(l1)*3, y+5, 0xf, 0)
	} else {
		e.printText(l1, x+0x10-len(l1)*3, y+2, 0xf, 0)
		e.printText(l2, x+0x10-len(l2)*3, y+9, 0xf, 0)
	}
}

// drawItemIcon ports S0d1c_070f.
func (e *Engine) drawItemIcon(page, item, x, y int) {
	icon := int(byte(e.Items[item].Icon))
	if icon >= len(e.ItemIcons) {
		return
	}
	e.Scr.DrawShape(page, e.ItemIcons[icon], x, y, 0, e.magicRemap(item, icon))
}

// magicRemap is the Detect Magic test of S0d1c_070f / S07a9_010c: while
// ds:7172&1 is set, magic items (flag 0x80) draw through their icon's
// 16-byte ITEMRMP table (driver entry 35, ds:80d2).
func (e *Engine) magicRemap(item, icon int) int {
	if e.Vars.Raw[10]&1 == 0 || e.Items[item].Flags&0x80 == 0 || len(e.ItemRemap) < icon*16+16 {
		return 0
	}
	copy(e.Scr.Remap16[:], e.ItemRemap[icon*16:])
	return gfx.DrawRemap
}

// drawCharBorder ports S1547_0009: plain frame, or a coloured frame for
// harmful (6) / beneficial (5) effects, dashed when both apply.
func (e *Engine) drawCharBorder(c int) {
	ch := &e.Chars[c]
	x, y := e.u8(0x5d1f+c&1), e.u8(0x5d21+c>>1)
	if e.drawPage == 0 {
		x += 0xb0
	}
	ln := func(x1, y1, x2, y2 int, col byte) { e.Scr.DrawLine(e.drawPage, x1, y1, x2, y2, col) }
	bad := ch.EffectFlags&0x302 != 0 || ch.EffectsRemainder[0] != 0 || ch.EffectsRemainder[1] != 0
	good := ch.EffectFlags&0x4c8 != 0 || e.Vars.Raw[10]&0x30 != 0
	const badCol, goodCol = 6, 5
	switch {
	case !bad && !good:
		ln(x, y, x+0x3e, y, e.col.BoxHi)
		ln(x, y+0x31, x+0x3e, y+0x31, e.col.BoxLo)
		ln(x, y, x, y+0x33, 0xc)
		ln(x+0x3f, y, x+0x3f, y+0x33, 0xc)
	case bad != good:
		col := byte(badCol)
		if good {
			col = goodCol
		}
		e.Scr.DrawRect(e.drawPage, x, y, x+0x3f, y+0x31, col)
	default:
		for o := 0; o < 0x40; o += 0x10 {
			xx := x + o
			ln(xx, y, xx+7, y, badCol)
			ln(xx+8, y+0x31, xx+0xf, y+0x31, badCol)
			ln(xx+8, y, xx+0xf, y, goodCol)
			ln(xx, y+0x31, xx+7, y+0x31, goodCol)
		}
		for o := 1; o < 0x30; o += 0xc {
			yy := y + o - 1
			ln(x, yy+1, x, yy+6, goodCol)
			ln(x+0x3f, yy+7, x+0x3f, yy+0xc, goodCol)
			ln(x, yy+7, x, yy+0xc, badCol)
			ln(x+0x3f, yy+1, x+0x3f, yy+6, badCol)
		}
	}
}

// canUseHand ports S0d1c_07db.
func (e *Engine) canUseHand(c, hand int) bool {
	ch := &e.Chars[c]
	r := int(ch.Inventory[0])
	if hand == 0 {
		return r == 0 || e.classCanUse(c, r)
	}
	l := int(ch.Inventory[1])
	if r != 0 && e.ItemTypes[e.Items[r].Type].RequiredHands == 2 {
		return false // two-handed weapon in the right hand
	}
	if l == 0 {
		return true
	}
	lt := &e.ItemTypes[e.Items[l].Type]
	if p := lt.ExtraProps & 0x7f; p > 0 && p < 4 && lt.RequiredHands != 0 {
		return false
	}
	return e.classCanUse(c, l)
}

// DrawPlayfield draws the party panel for all characters.
// DrawPlayfield ports S0d1c_0583: redraws the six character panels.
func (e *Engine) DrawPlayfield() {
	for c := 0; c < 6; c++ {
		e.drawCharPanel(c)
	}
}
