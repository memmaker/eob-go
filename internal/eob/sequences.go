package eob

import "github.com/felix/goeob/internal/gfx"

// Sequences: NPC encounters, portals, Xanathar's death and the finale, text
// items (overlay O211e).

const (
	dsSeqShapeRects = 0x7b2b // 19 x {x8,y,w8,h} in OUTTAKE
	dsSeqNPC2Shape  = 0x7b77 // 3 x 6 bytes: resurrected NPC shape, x... (O211e_00a1)
	dsSeqBtnX       = 0x7b89
	dsSeqBtnY       = 0x7b94
	dsSeqCancel     = 0x7b9f
	dsSeqYesNo      = 0x7b23
	dsSeqMore       = 0x7cb7
	dsSeqSixChars   = 0x7cbc
	dsSeqJoins      = 0x7d0e
	dsSeqLuck       = 0x7d77
	dsSeqWhichRes   = 0x7d9e
	dsPortalScript  = 0x7c32
	dsNPCChars      = 0x7298 // NPC character templates (0xf3 bytes each)
	seqGfxPage      = 3      // OUTTAKE/PORTAL/XDEATH are decoded to page 3
	seqTmpPage      = 4
	seqViewPage     = 6
)

// seqLabels reads n far string pointers.
func (e *Engine) seqLabels(o, n int) []string {
	r := make([]string, n)
	for i := range r {
		r[i] = e.farStr(o + i*4)
	}
	return r
}

// seqPoll is one step of the modal input loop (S1aad_05d3): presents the
// screen and returns a key code, 0x41/0x42 for mouse clicks (with position),
// 0 for nothing and -1 when the window was closed.
func (e *Engine) seqPoll() (int, int, int) {
	ev, ok := e.poll()
	if !ok {
		e.present()
		e.Sys.Sleep(16)
		return 0, 0, 0
	}
	switch ev.Kind {
	case EvQuit:
		e.quit = true
		return -1, 0, 0
	case EvKeyDown:
		return ev.Key, 0, 0
	case EvMouseDown:
		return 0x40 + ev.Button, ev.X, ev.Y
	}
	return 0, 0, 0
}

// seqWaitAny waits for any key or click (S1aad_0480 + S1aad_05d3).
func (e *Engine) seqWaitAny() {
	e.flushInput()
	for {
		if k, _, _ := e.seqPoll(); k != 0 {
			return
		}
	}
}

// seqDelay ports S082c_0425.
func (e *Engine) seqDelay(ticks int) {
	e.present()
	e.waitTicks(ticks)
}

// loadTextDat loads TEXT.DAT (ds:df4e).
func (e *Engine) loadTextDat() {
	b, err := e.FS.Read(e.X.Str(0x7f0f))
	if err != nil {
		panic(err)
	}
	e.textDat = b
}

// drawSeqShape ports O211e_0000: an OUTTAKE shape bottom-centred at x,y.
func (e *Engine) drawSeqShape(idx, x, y int) {
	o := dsSeqShapeRects + idx*4
	w8, h := e.u8(o+2), e.u8(o+3)
	shp := gfx.Encode4(e.Scr.Pages[seqGfxPage], e.u8(o), e.u8(o+1), w8, h)
	e.Scr.DrawShape(0, shp, x-w8*8/2, y-h, 5, 0)
}

// drawSeqNPC ports O211e_00a1.
func (e *Engine) drawSeqNPC(id int) {
	e.Scr.CopyRegion(0, 0, 0, 0, 176, 120, seqViewPage, 0)
	shp := []int{2, 0, 3, 7, 6, 0x12, 0x11, 4}
	switch {
	case id > 7:
	case id == 5:
		e.drawSeqShape(0x12, 0x58, 0x58)
	case (id == 1 || id == 2) && e.seqNPC != -1:
		e.drawSeqShape(shp[id], 0x3c, 0x68)
		if id == 1 {
			e.drawSeqShape(5, 0x74, 0x68)
			break
		}
		n := e.seqNPC
		e.drawSeqShape(e.u8(dsSeqNPC2Shape+n), 0x74, 0x68)
		e.drawSeqShape(e.u8(dsSeqNPC2Shape+6+n), 0x74, e.u8(dsSeqNPC2Shape+12+n))
	default:
		e.drawSeqShape(shp[id], 0x58, 0x68)
	}
}

// seqClearText ports O211e_021c.
func (e *Engine) seqClearText() {
	e.clearDim()
	e.Scr.FillRect(0, 1, 0x7a, 0x13e, 0xc7, e.col.BoxFill)
}

// seqMore ports O211e_025f: MORE button.
func (e *Engine) seqMore() {
	x1, y1, x2, y2 := 0x11d, 0xbd, 0x13b, 0xc5
	if e.textItemMode {
		x1, y1, x2, y2 = 0x8b, 0xa2, 0xa9, 0xaa
	}
	e.Scr.DrawBox3D(0, x1, y1, x2, y2, e.col.BoxHi, e.col.BoxLo, int(e.col.BoxFill))
	e.Scr.PrintText(e.font, 0, e.X.Str(dsSeqMore), x1+4, y1+2, 0xf, 0)
	e.flushInput() // O211e_02e2
	for {
		k, mx, my := e.seqPoll()
		if k == KeyEnter || k == 0x34 || k == KeySpace || k < 0 || ((k == 0x41 || k == 0x42) && inRect(mx, my, x1, y1, x2, y2)) {
			break
		}
	}
	e.Scr.FillRect(0, x1, y1, x2, y2, e.col.BoxFill)
	e.clearDim()
}

// seqPrint ports O211e_03a9: prints TEXT.DAT string n (1-based).
func (e *Engine) seqPrint(n int, more bool) {
	o := int(le16(e.textDat[(n-1)*2:]))
	e.printMessage(cstr(e.textDat[o:]))
	if more {
		e.seqMore()
	}
}

// seqButton ports O211e_03ef.
func (e *Engine) seqButton(x, y int, fg byte, s string) {
	e.Scr.DrawBox3D(0, x, y, x+0x5e, y+8, e.col.BoxHi, e.col.BoxLo, int(e.col.BoxFill))
	e.Scr.PrintText(e.font, 0, s, x+0x30-len(s)*3, y+2, fg, 0)
}

// seqChoice ports O211e_0455: answer buttons (layout slots start..), returns
// the chosen index.
func (e *Engine) seqChoice(start, count int, labels []string) int {
	y0 := e.Scr.Dims[e.txtDim].Y + (e.txtLine+1)*e.font.H + 4
	pos := func(i int) (int, int) { return e.u8(dsSeqBtnX + start + i), y0 + int(e.X.I8(dsSeqBtnY+start+i)) }
	hit := func(x, y int) int {
		for i := 0; i < count; i++ {
			bx, by := pos(i)
			if inRect(x, y, bx, by, bx+0x5e, by+8) {
				return i
			}
		}
		return -1
	}
	sel, drawn, res := 0, -1, -1
	for res == -1 {
		if drawn != sel {
			for i := 0; i < count; i++ {
				x, y := pos(i)
				fg := byte(0xf)
				if i == sel {
					fg = 9
				}
				e.seqButton(x, y, fg, labels[i])
			}
			drawn = sel
		}
		k, cx, cy := e.seqPoll()
		if i := hit(e.mouse()); i >= 0 {
			sel = i
		}
		switch k {
		case KeyEnter, KeySpace, KeyKP5:
			res = sel
		case 0x41, 0x42:
			res = hit(cx, cy)
		case KeyKP4:
			if sel--; sel < 0 {
				sel = min(count-1, 2)
			}
		case KeyKP6:
			if sel++; sel > 2 || sel > count-1 {
				sel = 0
			}
		case KeyKP2:
			if sel+3 < count {
				sel += 3
			} else if sel-3 > 0 {
				sel -= 3
			}
			fallthrough
		case KeyKP8:
			if sel-3 > 0 {
				sel -= 3
			} else if sel+3 < count {
				sel += 3
			}
		case -1:
			res = count - 1
		}
	}
	e.seqClearText()
	return res
}

// dropAtParty drops item at a random front position of the party block.
func (e *Engine) dropAtParty(it int) {
	e.linkItemToBlock(e.CurrentBlock, it, e.u8(0x1a1b+e.CurrentDirection*4+e.rollDice(1, 2, -1)))
}

// seqDropItem ports O211e_08d2.
func (e *Engine) seqDropItem(tmpl int) { e.dropAtParty(e.duplicateItem(tmpl)) }

// deleteItemsOfType ports S0d1c_01ca: removes all items of a type (and
// value, -1 = any) from the living party and the mouse cursor.
func (e *Engine) deleteItemsOfType(typ, val int) bool {
	found := false
	for c := 0; c < 6; c++ {
		if !e.charStatus(c, 1) {
			continue
		}
		for {
			s := -1
			for i, it := range e.Chars[c].Inventory {
				t := e.Items[it]
				if it != 0 && (int(t.Type) == typ || typ == -1) && (int(t.Value) == val || val == -1) {
					s = i
					break
				}
			}
			if s < 0 {
				break
			}
			e.Items[e.Chars[c].Inventory[s]].Block = -1
			e.Chars[c].Inventory[s] = 0
			found = true
			if e.GuiMode == 0 && s < 2 && c < 5 {
				e.drawHand(c, s)
			}
			if e.GuiMode == 1 && e.InvChar == c {
				e.drawInvSlotNow(s)
			}
			if int(e.Items[e.HandItem].Type) == typ {
				e.Items[e.HandItem].Block = -1
				e.setHandItem(0)
			}
		}
	}
	return found
}

// dropCharacter ports S0d1c_035a: removes c from the party, dropping all items.
func (e *Engine) dropCharacter(c int) {
	ch := &e.Chars[c]
	ch.Flags = 0
	for s, it := range ch.Inventory {
		if s != 0x10 && it != 0 {
			e.dropAtParty(int(it))
		}
	}
	for ch.Inventory[0x10] != 0 {
		it := e.unlinkListItem(&ch.Inventory[0x10], 0, -1)
		if it == 0 {
			break
		}
		e.dropAtParty(it)
	}
	if e.InvChar == c { // ds:1f3a is both the selected and the inventory char
		e.InvChar, e.SelectedChar = 0, 0
	}
	e.rebuildCharTimers()
}

// seqMakeRoom ports O211e_067b: lets the player drop a character if the
// party is full. Returns false on cancel.
func (e *Engine) seqMakeRoom() bool {
	n := 0
	labels := make([]string, 7)
	for c := range e.Chars {
		n += int(e.Chars[c].Flags & 1)
		labels[c] = e.Chars[c].NameStr()
	}
	labels[6] = e.X.Str(dsSeqCancel)
	if n <= 5 {
		return true
	}
	e.seqClearText()
	e.printMessage(e.X.Str(dsSeqSixChars))
	c := e.seqChoice(2, 7, labels)
	if c == 6 {
		return false
	}
	e.dropCharacter(c)
	return true
}

// addNPC ports O211e_070e: copies NPC template npc into the first free slot.
func (e *Engine) addNPC(npc int) {
	c := 0
	for i := range e.Chars {
		if e.Chars[i].Flags&1 == 0 {
			c = i
			break
		}
	}
	ch := &e.Chars[c]
	ch.unmarshal(e.X.Bytes(dsNPCChars+npc*charSize, charSize))
	e.recalcArmorClass(c)
	for i := 0; i < 25; i++ {
		if ch.Inventory[i] != 0 {
			ch.Inventory[i] = uint16(e.duplicateItem(int(ch.Inventory[i])))
		}
	}
	ch.Face = gfx.Encode8(e.Scr.Pages[seqGfxPage], npc*4, 0xa0, 4, 0x20)
}

// npcJoin ports O211e_0828: asks whether NPC npc may join.
func (e *Engine) npcJoin(npc, text, accept, reject int) {
	e.seqClearText()
	e.seqPrint(text, false)
	switch e.seqChoice(0, 2, e.seqLabels(dsSeqYesNo, 2)) {
	case 0:
		if accept != -1 {
			e.seqPrint(accept, true)
		} else {
			e.printMessage(sprintfC(e.X.Str(dsSeqJoins), cstr(e.X.Bytes(dsNPCChars+npc*charSize+2, 11))))
			e.seqMore()
		}
		e.deleteItemsOfType(0x21, npc+1)
		if e.seqMakeRoom() {
			e.addNPC(npc)
		}
	case 1:
		e.seqPrint(reject, true)
	}
}

func (e *Engine) setGlobalFlag(bit int)      { e.LevelFlags[12] |= 1 << uint(bit) }
func (e *Engine) hasGlobalFlag(bit int) bool { return e.LevelFlags[12]&(1<<uint(bit)) != 0 }

// seqNPC0 ports O211e_0924 (wounded dwarf).
func (e *Engine) seqNPC0() {
	for {
		e.seqClearText()
		e.seqPrint(8, false)
		var r int
		if e.hasGlobalFlag(13) {
			if r = e.seqChoice(0, 2, e.seqLabels(0x7bb2, 2)); r == 1 {
				r = 2
			}
		} else {
			r = e.seqChoice(2, 3, e.seqLabels(0x7ba6, 3))
		}
		switch r {
		case 0:
			e.npcJoin(6, 0xc, 0x17, 2)
			e.setGlobalFlag(14)
		case 1:
			e.seqPrint(1, true)
			e.setGlobalFlag(13)
			continue
		}
		return
	}
}

// seqNPC1Proposal ports O211e_09c5.
func (e *Engine) seqNPC1Proposal() {
	ask := true
	if !e.hasGlobalFlag(15) {
		e.setGlobalFlag(15)
		e.seqPrint(3, false)
		switch e.seqChoice(0, 2, e.seqLabels(0x7bba, 2)) {
		case 0:
			e.seqPrint(4, false)
		case 1:
			ask = false
		}
	} else {
		e.seqPrint(0xd, false)
	}
	r := 1
	if ask {
		r = e.seqChoice(0, 2, e.seqLabels(0x7bc2, 2))
	}
	if r != 0 {
		e.seqPrint(5, true)
		return
	}
	for i := 0; i < 6; i++ {
		e.seqDropItem(0x37)
	}
	e.seqDropItem(0x3e)
	e.setGlobalFlag(16)
	e.seqPrint(6, true)
	e.npcJoin(7, 7, 0x1d, 0x1e)
}

// seqNPC1Elf ports O211e_0ab1.
func (e *Engine) seqNPC1Elf() bool {
	c := 0
	for ; c < 6; c++ {
		if e.Chars[c].Flags&1 != 0 && e.Chars[c].Portrait == -9 {
			break
		}
	}
	if c == 6 {
		return false
	}
	e.seqPrint(0x19, true)
	e.seqPrint(0x1a, true)
	e.setGlobalFlag(19)
	return true
}

// seqNPC1Item ports O211e_0b16.
func (e *Engine) seqNPC1Item() bool {
	if !e.deleteItemsOfType(6, -1) {
		return false
	}
	e.seqPrint(0x1c, true)
	e.seqDropItem(0x20)
	e.setGlobalFlag(20)
	return true
}

// seqNPC1 ports O211e_0b58 (dwarf king).
func (e *Engine) seqNPC1() {
	any := false
	if !e.hasGlobalFlag(16) {
		e.seqNPC1Proposal()
		any = true
	}
	if !e.hasGlobalFlag(19) && e.seqNPC1Elf() {
		any = true
	}
	if !e.hasGlobalFlag(20) && e.seqNPC1Item() {
		any = true
	}
	if !any {
		e.printMessage(e.X.Str(dsSeqLuck))
		e.seqMore()
	}
}

// seqDeadList ports O211e_0bbe: dead party members (c) and NPC bones (-value).
func (e *Engine) seqDeadList() []int {
	var l []int
	for c := range e.Chars {
		if e.Chars[c].Flags&1 != 0 && e.Chars[c].HPCur == -10 {
			l = append(l, c)
		}
	}
	for c := range e.Chars {
		if e.Chars[c].Flags&1 == 0 {
			continue
		}
		for s := 0; s < 25; s++ {
			if it := e.Chars[c].Inventory[s]; it != 0 && e.ItemTypes[e.Items[it].Type].ExtraProps&0x7f == 8 {
				l = append(l, -int(e.Items[it].Value))
			}
		}
	}
	return l
}

// seqHealParty ports O211e_0c9d.
func (e *Engine) seqHealParty() {
	n := e.rollDice(1, 3, 2)
	for c := 0; n != 0 && c < 6; {
		ch := &e.Chars[c]
		if !e.charStatus(c, 3) {
			c++
			continue
		}
		ch.Flags &= 0xf9
		ch.EffectFlags &^= 0x80
		e.clearCharEvent(c, -0x22)
		if ch.HPCur >= ch.HPMax {
			c++
			continue
		}
		n--
		if h := e.rollDice(1, 8, 9); int(ch.HPCur)+h > int(ch.HPMax) {
			ch.HPCur = ch.HPMax
		} else {
			ch.HPCur += int8(h)
		}
	}
}

// seqResurrect ports O211e_0dbb.
func (e *Engine) seqResurrect(l []int) {
	e.seqClearText()
	e.printMessage(e.X.Str(dsSeqWhichRes))
	labels := make([]string, len(l))
	for i, v := range l {
		if v < 0 {
			labels[i] = cstr(e.X.Bytes(dsNPCChars+(-v-1)*charSize+2, 11))
		} else {
			labels[i] = e.Chars[v].NameStr()
		}
	}
	v := l[e.seqChoice(2, len(l), labels)]
	if v >= 0 {
		e.Chars[v].HPCur = e.Chars[v].HPMax
		return
	}
	npc := -v - 1
	e.deleteItemsOfType(0x21, npc+1)
	e.seqNPC = npc
	e.drawSeqNPC(2)
	e.npcJoin(npc, npc*2+0x20, -1, npc*2+0x21)
}

// seqNPC2 ports O211e_0eb9 (cleric).
func (e *Engine) seqNPC2() {
	if !e.hasGlobalFlag(16) {
		e.seqPrint(0x18, true)
		return
	}
	if e.hasGlobalFlag(17) {
		e.seqPrint(0xb, true)
		return
	}
	e.seqPrint(9, false)
	l := e.seqDeadList()
	var r int
	if len(l) != 0 {
		r = e.seqChoice(2, 3, e.seqLabels(0x7bd2, 3))
	} else if r = e.seqChoice(0, 2, e.seqLabels(0x7bca, 2)); r == 1 {
		r = 2
	}
	switch r {
	case 0:
		e.seqHealParty()
		e.setGlobalFlag(17)
	case 1:
		e.seqResurrect(l)
		e.setGlobalFlag(17)
	}
}

// seqNPC3 ports O211e_0f77.
func (e *Engine) seqNPC3() {
	e.seqPrint(0x12, false)
	switch e.seqChoice(0, 2, e.seqLabels(0x7bde, 2)) {
	case 0:
		e.setGlobalFlag(27)
		e.setGlobalFlag(22)
		for i := range e.Monsters {
			if e.Monsters[i].Mode == 8 {
				e.Monsters[i].Mode = 5
			}
		}
	case 1:
		if e.deleteItemsOfType(0x31, -1) {
			e.seqPrint(0x14, true)
			e.setGlobalFlag(22)
		} else {
			e.seqPrint(0x13, true)
		}
	}
}

// seqNPC4 ports O211e_102c.
func (e *Engine) seqNPC4() {
	e.seqPrint(0xe, false)
	switch e.seqChoice(2, 3, e.seqLabels(0x7be6, 3)) {
	case 0:
		e.setGlobalFlag(21)
	case 1:
		e.seqPrint(0xf, true)
	}
	e.setGlobalFlag(23)
}

// seqNPC5 ports O211e_1087.
func (e *Engine) seqNPC5() {
	e.seqPrint(0x15, true)
	e.setGlobalFlag(24)
}

// seqNPC6 ports O211e_10a6 (drow ambush: surrender drops hand items).
func (e *Engine) seqNPC6() {
	e.seqPrint(0x10, false)
	if e.seqChoice(0, 2, e.seqLabels(0x7bf2, 2)) == 0 {
		e.seqPrint(0x11, true)
		for c := range e.Chars {
			for s := 0; s < 2; s++ {
				it := int(e.Chars[c].Inventory[s])
				if t := e.Items[it].Type; it != 0 && (t < 0x33 || t > 0x38) {
					e.Chars[c].Inventory[s] = 0
					e.dropAtParty(it)
				}
			}
		}
	}
	e.setGlobalFlag(25)
}

// seqNPC7 ports O211e_119e.
func (e *Engine) seqNPC7() {
	e.seqPrint(0x16, false)
	switch e.seqChoice(2, 3, e.seqLabels(0x7bfa, 3)) {
	case 0:
		e.npcJoin(8, 0x1b, 0x2c, 0x2d)
	case 1:
		e.seqPrint(0x1f, true)
	default:
		return
	}
	e.setGlobalFlag(26)
}

// dissolve ports O211e_1e1b: random pixel transition of a rectangle.
func (e *Engine) dissolve(x, y, w, h, src, dst int) {
	px, py := make([]int, w), make([]int, h)
	for i := range px {
		px[i] = i
	}
	for i := range py {
		py[i] = i
	}
	for i := range px {
		j := e.rndRange(0, w-1)
		px[i], px[j] = px[j], px[i]
	}
	for i := range py {
		j := e.rndRange(0, h-1)
		py[i], py[j] = py[j], py[i]
	}
	s, d := e.Scr.Pages[src], e.Scr.Pages[dst]
	// The original has no delay: its speed is the CPU's. Each pixel costs
	// ~120 instructions (36 in the loop + 42 in each of driver entries 11
	// and 22, getPixel/putPixel), paced here at the bundled dosbox_eob1.conf
	// cycles=auto rate for real mode (3000 cycles/ms).
	const pixelCycles, cyclesPerMs = 120, 3000
	slept := 0
	for r := 0; r < h; r++ {
		k := r
		for i := 0; i < w; i++ {
			o := (y+py[k])*320 + x + px[i]
			d[o] = s[o]
			if k++; k >= h {
				k = 0
			}
		}
		e.present()
		t := (r + 1) * w * pixelCycles / cyclesPerMs
		e.Sys.Sleep(t - slept)
		slept = t
	}
}

// seqPortal ports O211e_1204 (PORTALA/PORTALB).
func (e *Engine) seqPortal() {
	e.mustCPS(e.X.Str(0x7e43), seqGfxPage)
	pg := e.Scr.Pages[seqGfxPage]
	var l, r, top [5]*gfx.Shape
	for i := 0; i < 5; i++ {
		l[i] = gfx.Encode4(pg, i*3, 0, 3, 0x4b)
		r[i] = gfx.Encode4(pg, i*3, 0x50, 3, 0x4b)
		top[i] = gfx.Encode4(pg, 0xf, i*0x12, 0xf, 0x12)
	}
	door := gfx.Encode4(pg, 0x1e, 0, 8, 0x4d)
	e.mustCPS(e.X.Str(0x7e4b), seqGfxPage)
	e.playSound(0x21)
	e.playSound(0x13)
	frame := func(page, i int) {
		e.Scr.DrawShape(page, top[i], 0x1c, 9, 0, 0)
		e.Scr.DrawShape(page, l[i], 0x22, 0x1c, 0, 0)
		e.Scr.DrawShape(page, r[i], 0x78, 0x1c, 0, 0)
	}
	e.Scr.CopyRegion(24, 0, 24, 0, 0x90, 0x68, 0, seqTmpPage)
	frame(seqTmpPage, 0)
	e.Scr.DrawShape(seqTmpPage, door, 0x38, 0x1b, 0, 0)
	e.dissolve(0x18, 0, 0x90, 0x68, seqTmpPage, 0)
	e.seqDelay(0x1e)
	for p := dsPortalScript; e.X.I8(p) > -1; {
		si := e.u8(p)
		p++
		frame(0, si)
		if si == 1 && e.u8(p-3) == 0 {
			e.playSound(0x18)
			e.playSound(0x56)
		}
		si = e.u8(p)
		p++
		if si != 0 {
			si--
			e.Scr.CopyRegion(si%5*64, si/5*0x4d, 56, 0x1b, 64, 0x4d, seqGfxPage, 0)
		} else {
			e.Scr.DrawShape(0, door, 0x38, 0x1b, 0, 0)
		}
		if si == 1 {
			e.playSound(0x1f)
		}
		if si == 3 && e.u8(p-2) == 3 {
			e.playSound(0x5a)
		}
		e.seqDelay(2)
	}
}

// seqFinale ports O211e_160a: closing text, Beholder Bonus password.
func (e *Engine) seqFinale() {
	e.loadTextDat()
	e.playSound(0x14)
	e.seqMode, e.textItemMode = true, true
	e.Scr.Dims[0xc].UnkA = int(e.col.BoxFill)
	e.setDim(0xc)
	e.Scr.DrawBox3D(0, 0, 0, 0xaf, 0xae, e.col.BoxHi, e.col.BoxLo, int(e.col.BoxFill))
	e.seqPrint(0x33, true)
	if e.LevelFlags[12]&0x1ffe == 0x1ffe {
		e.printMessage(e.X.Str(0x7e5c))
		for c := 0; c < 4; c++ {
			e.printMessage(e.X.Str(0x7edb))
			if e.Chars[c].Flags&1 != 0 {
				e.printMessage(e.Chars[c].NameStr())
			}
		}
		sum := 0
		for c := range e.Chars {
			if e.Chars[c].Flags&1 != 0 {
				for _, b := range []byte(e.Chars[c].NameStr()) {
					sum += int(int8(b)) * int(int8(b))
				}
			}
		}
		e.printMessage(sprintfC(e.X.Str(0x7edd), sum&0xffff))
		e.seqDelay(10)
		e.seqWaitAny()
	}
	e.seqMode = false
}

// runFinale is the Xanathar kill in S08e6_0ec3: O211e_160a, then exit.
func (e *Engine) runFinale() {
	e.seqFinale()
	e.gameOver, e.quit = true, true
}

// seqXanatharDeath ports O211e_17c8 (XDEATH1-3), followed by the finale.
func (e *Engine) seqXanatharDeath() {
	pg := e.Scr.Pages[seqGfxPage]
	e.mustCPS(e.X.Str(0x7eef), seqGfxPage)
	var s [4]*gfx.Shape
	for i := range s {
		s[i] = gfx.Encode8(pg, i%2*0xe, i/2*0x58, 0xe, 0x58)
	}
	e.mustCPS(e.X.Str(0x7ef7), seqGfxPage)
	eye := gfx.Encode8(pg, 0x16, 0, 0x10, 0x5f)
	e.mustCPS(e.X.Str(0x7eff), seqGfxPage)
	for i := 0; i < 10; i++ {
		switch i {
		case 2:
			e.playSound(0x48)
		case 4, 6:
			e.playSound(0x36)
		}
		e.playSound(0x22)
		if i < 6 {
			e.Scr.CopyRegion(i%3*0x68, i/3*0x58, 32, 10, 0x68, 0x58, seqGfxPage, 0)
		} else {
			e.playSound(0x2a)
			e.Scr.DrawShape(0, s[i-6], 0x20, 10, 0, 0)
		}
		e.seqDelay(4)
	}
	e.Scr.CopyRegion(0, 0, 0, 0, 176, 120, 0, seqViewPage)
	e.Scr.Dims[5].Y = 8
	for i := 0; i < 19; i++ {
		e.playSound(0x77)
		e.Scr.CopyRegion(0, 0, 0, 0, 176, 120, seqViewPage, seqTmpPage)
		e.Scr.DrawShape(seqTmpPage, eye, 0x18, i*5-0x5a, 5, 0)
		e.Scr.CopyRegion(0, 0, 0, 0, 176, 120, seqTmpPage, 0)
		e.seqDelay(2)
	}
	e.playSound(5)
	e.Scr.Dims[5].Y = 0
	e.seqDelay(0x3c)
	e.runFinale() // O211e_160a + S07c8_058a
}

// seqRestore is the common epilogue of O211e_1a82 / O211e_1cdd.
func (e *Engine) seqRestore() {
	e.seqMode = false
	e.loadPlayfield()
	e.DrawPlayfield()
	e.DrawScene(true)
	e.setDim(7)
}

// runSequence ports O211e_1a82 (script opcode 0xe6).
func (e *Engine) runSequence(id int) {
	e.seqNPC, e.textItemMode = -1, false
	if e.GuiMode != 0 {
		e.closeInventory()
	}
	if e.SpellbookOpen {
		e.closeSpellbook()
	}
	e.DrawScene(true)
	switch id {
	case 8:
		e.seqPortal()
		e.setDim(7)
		return
	case 9:
		e.seqXanatharDeath()
		return
	case 10:
		return // O2106_09ed: copy protection, intentionally not ported
	}
	e.mustCPS(e.X.Str(0x7f07), seqGfxPage)
	e.Scr.CopyRegion(0, 0, 0, 0, 176, 120, 0, seqViewPage)
	defer e.setFont(e.setFont(e.font6)) // O211e_1b73 / 1c22
	e.drawSeqNPC(id)
	e.loadTextDat()
	e.Scr.DrawBox3D(0, 0, 0x79, 0x13f, 0xc8, e.col.BoxHi, e.col.BoxLo, int(e.col.BoxFill))
	e.seqMode = true
	e.Scr.Dims[9].UnkA = int(e.col.BoxFill)
	e.setDim(9)
	if id < 8 {
		[]func(){e.seqNPC0, e.seqNPC1, e.seqNPC2, e.seqNPC3, e.seqNPC4, e.seqNPC5, e.seqNPC6, e.seqNPC7}[id]()
	}
	e.seqRestore()
}

// readTextItem ports O211e_1cdd: shows a TEXT.DAT page (readable items).
func (e *Engine) readTextItem(v int) {
	e.textItemMode = true
	if e.GuiMode != 0 {
		e.closeInventory()
	}
	if e.SpellbookOpen {
		e.closeSpellbook()
	}
	e.loadTextDat()
	e.seqMode = true
	e.Scr.Dims[0xc].UnkA = int(e.col.BoxFill)
	e.setDim(0xc)
	e.Scr.DrawBox3D(0, 0, 0, 0xaf, 0xae, e.col.BoxHi, e.col.BoxLo, int(e.col.BoxFill))
	e.seqPrint(v, true)
	e.seqRestore()
}

// scriptStealItem ports S122e_0e9b (script opcode 0xf9): moves a random
// small item of character c (-1 = random) to blk/pos.
func (e *Engine) scriptStealItem(c, blk, pos int) {
	if c == -1 {
		c = e.rollDice(1, 6, -1)
	}
	for e.Chars[c].Flags == 0 {
		if c++; c > 5 {
			c = 0
		}
	}
	if e.GuiMode != 0 && e.InvChar == c {
		return
	}
	it, s, k := 0, e.rollDice(1, 0x1b, 0), 0
	for ; k < 0x1b; k++ {
		// ponytail: s == 0x1b reads past the inventory in the original; skipped.
		if (e.GuiMode != 0 || s > 1) && s != 0x10 && s < 0x1b {
			if it = int(e.Chars[c].Inventory[s]); it != 0 && e.T.ItemShapeMap[byte(e.Items[it].Icon)] >= 0xf {
				break
			}
		}
		if s++; s >= 0x1b {
			s = 0
		}
	}
	if k < 0x1b && it != 0 {
		e.Chars[c].Inventory[s] = 0
		e.linkItemToBlock(blk, it, pos)
	}
}
