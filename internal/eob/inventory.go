package eob

import "fmt"

const (
	dsInvSlotX     = 0x17ce // u16 per slot
	dsInvSlotY     = 0x1804 // byte per slot
	dsInvSlotType  = 0x3875 // 0 normal, 1 quiver, 2 small/hand
	dsInvBoxSize   = 0x3890
	dsFoodBarCols  = 0x5d29
	dsStrDead      = 0x5d8b
	dsStrUncons    = 0x5d90
	dsStrPoisSlow  = 0x5d9c
	dsStrPoisoned  = 0x5daa
	dsStrParalyzed = 0x5db3
	dsStrCharInfo  = 0x5dbd
	dsStrAC        = 0x5dcc
	dsStrExp       = 0x5dd8
	dsStrLvl       = 0x5ddc
	dsRaceNames    = 0x11ad // far ptrs
	dsClassNames   = 0x11dd
	dsAlignNames   = 0x1240
	dsStatNames    = 0x1195
	dsClassPartNm  = 0x1219
	dsClassParts   = 0x35d4 // [class*4 + i]
	dsStatFmtExt   = 0x36a4
	dsStatFmt      = 0x36ac
	dsInfoBoxX1    = 0x5d39
	dsInfoBoxY1    = 0x5d3f
	dsInfoBoxX2    = 0x5d42
	dsInfoBoxY2    = 0x5d48
)

func (e *Engine) farStr(o int) string { return e.X.Str(int(e.X.U16(o))) }

// printShadow ports S082c_0898 (text with dark shadow).
func (e *Engine) printShadow(s string, x, y int, fg, bg byte) {
	e.printText(s, x-1, y, 0xc, bg)
	e.printText(s, x, y+1, 0xc, 0)
	e.printText(s, x-1, y+1, 0xc, 0)
	e.printText(s, x, y, fg, 0)
}

// drawInvPanel ports the inventory / stats branches of S1547_0991.
func (e *Engine) drawInvPanel(c int) {
	if e.InvChar != c {
		return
	}
	ch := &e.Chars[c]
	fg2 := byte(0xf)
	if ch.Flags&6 != 0 {
		fg2 = 6
	}
	e.Scr.CopyRegion(176, 0, 0, 0, 144, 168, 2, 2) // backup clean panel to page 2 x=0
	e.drawPage = 2
	e.drawPortrait(c)
	e.printShadow(ch.NameStr(), 0xdb, 6, fg2, e.col.BoxFill)
	e.drawHP(c)
	e.drawFood(c)
	if e.GuiMode == 1 {
		y := 0x9e
		switch {
		case int(ch.HPCur)+10 == 0:
			e.printShadow(e.X.Str(dsStrDead), 0xf7, y, 6, e.col.Text)
		case uint(int(ch.HPCur)+9) < 10:
			e.printShadow(e.X.Str(dsStrUncons), 0xe2, y, 6, e.col.Text)
		case ch.Flags&2 != 0 && ch.EffectFlags&0x80 != 0:
			e.printShadow(e.X.Str(dsStrPoisSlow), 0xdc, y, 6, e.col.Text)
		case ch.Flags&2 != 0:
			e.printShadow(e.X.Str(dsStrPoisoned), 0xeb, y, 6, e.col.Text)
		case ch.Flags&4 != 0:
			e.printShadow(e.X.Str(dsStrParalyzed), 0xe8, y, 6, e.col.Text)
		}
		for i := 0; i < 27; i++ {
			e.drawInvSlot(i, false, 2)
		}
		e.drawInvSlot(16, true, 2)
		e.drawInvSlot(0, true, 2)
	} else {
		e.drawStats(c)
	}
	e.drawPage = 0 // the original forces ds:8115=0, not the caller's page
	e.Scr.CopyRegion(176, 0, 176, 0, 144, 168, 2, 0)
	e.Scr.CopyRegion(0, 0, 176, 0, 144, 168, 2, 2) // restore clean panel
}

// drawFood ports S1547_04a0.
func (e *Engine) drawFood(c int) {
	if e.GuiMode == 0 || !e.charStatus(c, 1) || e.InvChar != c {
		return
	}
	ch := &e.Chars[c]
	lvl := 0
	if ch.Food < 0x21 {
		lvl = 1
	}
	if ch.Food < 0x14 {
		lvl = 2
	}
	e.drawBar(0xfa, 0x19, int(ch.Food), 100, 0x33, 5, byte(e.u8(dsFoodBarCols+lvl)), e.col.HPLow)
}

// drawInvSlot ports S0bd2_0917.
func (e *Engine) drawInvSlot(slot int, frame bool, page int) {
	oldPage := e.drawPage
	e.drawPage = page
	defer func() { e.drawPage = oldPage }()
	x := int(e.X.U16(dsInvSlotX + slot*2))
	y := e.u8(dsInvSlotY + slot)
	ch := &e.Chars[e.InvChar]
	it := int(ch.Inventory[slot])
	t := e.u8(dsInvSlotType + slot)
	if frame {
		fill := int(e.col.BoxFill)
		if t == 1 {
			fill = -1
		}
		sz := e.u8(dsInvBoxSize + t)
		e.Scr.DrawBox3D(page, x-1, y-1, x+sz, y+sz, e.col.BoxHi, e.col.BoxLo, fill)
		if t == 1 {
			e.Scr.FillRect(page, 0xe3, 0x41, 0xee, 0x45, 0xc)
			n := e.countQuiver(e.InvChar)
			nx := 0xe3
			if n < 10 {
				nx = 0xe9
			}
			e.printInt(n, nx, 0x41, 0xf, 0)
		}
	}
	if it != 0 && t != 1 {
		if t == 2 {
			e.drawItemIcon(page, it, x-4, y-4)
		} else {
			e.drawItemIcon(page, it, x, y)
		}
	}
}

// countQuiver counts items in the quiver list (slot 16 is a list head).
func (e *Engine) countQuiver(c int) int {
	h := int(e.Chars[c].Inventory[16])
	if h == 0 {
		return 0
	}
	n, si, first := 0, h, true
	for si != h || first {
		first = false
		n++
		si = int(e.Items[si].Prev)
	}
	return n
}

// drawStats ports the character info branch of S1547_0991 (gui mode 2).
func (e *Engine) drawStats(c int) {
	ch := &e.Chars[c]
	for i := 0; i < 3; i++ {
		e.Scr.FillRect(2, int(e.X.U16(dsInfoBoxX1+i*2)), e.u8(dsInfoBoxY1+i), int(e.X.U16(dsInfoBoxX2+i*2)), e.u8(dsInfoBoxY2+i), e.col.Text)
	}
	e.printShadow(e.X.Str(dsStrCharInfo), 0xb7, 0x2a, 0xf, e.col.Text)
	e.printText(e.farStr(dsClassNames+int(ch.Class)*4), 0xb7, 0x37, 0xc, e.col.Text)
	e.printText(e.farStr(dsAlignNames+int(ch.Alignment)*4), 0xb7, 0x3e, 0xc, e.col.Text)
	e.printText(e.farStr(dsRaceNames+int(ch.RaceSex)*4), 0xb7, 0x45, 0xc, e.col.Text)
	for i := 0; i < 6; i++ {
		e.printText(e.farStr(dsStatNames+i*4), 0xb7, 0x52+i*7, 0xc, e.col.Text)
	}
	e.printText(e.X.Str(dsStrAC), 0xb7, 0x7c, 0xc, e.col.Text)
	e.printText(e.X.Str(dsStrExp), 0xef, 0x8a, 0xc, e.col.Text)
	e.printText(e.X.Str(dsStrLvl), 0x116, 0x8a, 0xc, e.col.Text)
	var str string
	if ch.StrExtCur > 0 {
		ext := int(ch.StrExtCur)
		if ext == 100 {
			ext = 0
		}
		str = sprintfC(e.X.Str(dsStatFmtExt), int(ch.StrCur), ext)
	} else {
		str = sprintfC(e.X.Str(dsStatFmt), int(ch.StrCur))
	}
	e.printText(str, 0x113, 0x52, 0xf, e.col.Text)
	vals := []int8{ch.IntCur, ch.WisCur, ch.DexCur, ch.ConCur, ch.ChaCur, ch.AC}
	for i, v := range vals {
		e.printInt(int(v), 0x113, 0x59+i*7, 0xf, e.col.Text)
	}
	for i := 0; i < 3; i++ {
		p := int(int8(e.X.U8(dsClassParts + int(ch.Class)*4 + i)))
		if p == -1 {
			continue
		}
		y := 0x91 + i*7
		e.printText(e.farStr(dsClassPartNm+p*4), 0xb7, y, 0xc, e.col.Text)
		ex := fmt.Sprint(ch.Experience[i])
		e.printLong(int(ch.Experience[i]), 0xf7-len(ex)*3, y, 0xf, e.col.Text)
		lv := itoa(int(ch.Level[i]))
		e.printText(lv, 0x11e-len(lv)*3, y, 0xf, e.col.Text)
	}
}

// toggleCharInfo ports S0acd_018e (inventory <-> character info).
func (e *Engine) toggleCharInfo() {
	if e.GuiMode == 1 {
		e.GuiMode = 2
	} else {
		e.GuiMode = 1
	}
	e.drawInvPanel(e.InvChar)
}

// openInventory ports S0acd_0034 (portrait click); the save+redraw is
// the inlined S0bd2_0abe: ports S0bd2_0abe.
// A key press (ds:fc82) opens the current character without the status test.
func (e *Engine) openInventory(c int) {
	if e.btnKey == 0 && !e.charStatus(c, 1) {
		return
	}
	e.GuiMode = 1
	if e.btnKey == 0 {
		e.InvChar, e.SelectedChar = c, int8(c)
	}
	e.Scr.CopyRegion(176, 0, 176, 0, 144, 168, 0, 13) // save playfield panel
	e.drawInvPanel(c)
}

// closeInventory ports S0acd_01e3: restores the party panel.
func (e *Engine) closeInventory() {
	e.GuiMode = 0
	e.Scr.CopyRegion(176, 0, 176, 0, 144, 168, 13, 0)
	e.DrawPlayfield()
}

const (
	dsStrCantWear = 0x396e
	dsStrCantPut  = 0x3991
)

// canEquip ports S0bd2_0af2.
func (e *Engine) canEquip(it, c, slot int) bool {
	ch := &e.Chars[c]
	if slot == 0x11 && it != 0 && !e.classCanUse(c, it) {
		e.printMessage(sprintfC(e.X.Str(dsStrCantWear), ch.NameStr()))
		return false
	}
	if cur := int(ch.Inventory[slot]); e.Items[cur].Flags&0x20 != 0 {
		return false
	}
	mask := uint16(0xffff)
	if it != 0 {
		mask = e.itemType(it).InvFlags
	}
	if e.X.U16(dsSlotMask+slot*2)&mask == 0 {
		e.printMessage(e.X.Str(dsStrCantPut))
		return false
	}
	return true
}

// clickedInvSlot ports S0bd2_0ba0.
func (e *Engine) clickedInvSlot(slot int) {
	c := e.InvChar
	ch := &e.Chars[c]
	cur := int(ch.Inventory[slot])
	h := e.HandItem
	if !e.canEquip(h, c, slot) {
		return
	}
	if slot != 16 {
		e.setHandItem(cur)
		ch.Inventory[slot] = uint16(h)
		e.drawInvSlotNow(slot)
		e.recalcArmorClass(c)
		if slot < 2 || slot > 16 { // hands and worn gear; backpack moves are free
			e.spendTurn()
		}
		return
	}
	if h == 0 {
		it := e.unlinkListItem(&ch.Inventory[16], 0, -1)
		e.drawInvSlotNow(slot)
		e.setHandItem(it)
	} else {
		e.linkItem(&ch.Inventory[16], -2, h, 0)
		e.drawInvSlotNow(slot)
		e.setHandItem(0)
	}
}

func (e *Engine) drawInvSlotNow(slot int) { e.drawInvPanel(e.InvChar) }

// classCanUse ports S0bd2_11fd.
func (e *Engine) classCanUse(c, it int) bool {
	if it == 0 {
		return true
	}
	return e.ItemTypes[e.Items[it].Type].AllowedClasses&e.X.U8(0x1231+int(e.Chars[c].Class)) != 0
}

// invNextChar ports S0acd_008c / S0acd_00eb. Keys (ds:fc82) pick a char
// directly: 008c F1/F2 -> 0/1 unchecked; 00eb F3/F4 -> 2/3, F5/F6 (arg 2)
// -> 4/5, kept only if active.
func (e *Engine) invNextChar(d, arg int) {
	old := e.InvChar
	switch {
	case e.btnKey != 0 && d < 0:
		e.InvChar = e.btnKey - 1
	case e.btnKey != 0:
		e.InvChar = e.btnKey + 1
		if arg == 2 {
			e.InvChar += 2
		}
	default:
		for {
			e.InvChar = (e.InvChar + d + 6) % 6
			if e.charStatus(e.InvChar, 1) {
				break
			}
		}
	}
	if d > 0 && !e.charStatus(e.InvChar, 1) {
		e.InvChar = old
		return
	}
	e.SelectedChar = int8(e.InvChar)
	e.drawInvPanel(e.InvChar)
}

// printInt ports S1aa6_000f: itoa(v, 10) into a local buffer, then [0x80ca] printText.
func (e *Engine) printInt(v, x, y int, fg, bg byte) { e.printText(itoa(v), x, y, fg, bg) }

// printLong ports S1ba4_000c: ltoa(v, 10) then [0x80ca] printText.
func (e *Engine) printLong(v, x, y int, fg, bg byte) { e.printText(itoa(v), x, y, fg, bg) }
