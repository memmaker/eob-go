package eob

import (
	"fmt"
	"strings"
)

// Camp menu (overlay O2102). The menus are built from 9-byte button
// templates and a NUL-separated string list that live in far data
// segments outside DGROUP.

const (
	campStrSeg       = 0x2003 // string list (far ptr 0x1e8:0 after relocation)
	campBtnSeg       = 0x208e // button templates (segment 0x218 after relocation)
	dsCampMenus      = 0x628c // 5 bytes per menu: title, dim, first template, count, title colour
	dsCampKeyMap     = 0x6457 // key bytes (NUL terminated), mapped template at +2
	dsCampPortX8     = 0x62f2 // char select: portrait box x8 per char
	dsCampPortY      = 0x62f8
	dsDropBoxes      = 0x62e2 // 2 x {x1,y1,x2,y2} panel areas of chars 4/5
	dsSpellLvlMage   = 0x62af // spell level per mage spell (string)
	dsSpellLvlCleric = 0x62c9
	dsSlotsCleric    = 0x62fe // [level*5+i] spells per day
	dsSlotsWisBonus  = 0x6330 // [(wis-12)*5+i]
	dsSlotsPaladin   = 0x6358
	dsSlotsMage      = 0x6394
	dsCampFmtSpell   = 0x645b // "%-18s %1d"
	dsCampFmtRemain  = 0x6465 // "%d of %d Remaining.  "
	dsCampPlayfld    = 0x64ce // "PLAYFLD"
	dsCampInvent     = 0x64d6 // "INVENT"
	dsCampFmtPrefs   = 0x648d // 4 x "... are %-3s"
	dsClassFlags     = 0x1231 // per class: 2 mage, 4 cleric
	campPage         = 10     // backup of the 3D view while camping
)

type campState struct {
	btns     []Button
	labels   map[int]string // labels rewritten at runtime (preferences)
	titles   map[int]int    // menu titles set at runtime (ds:628c + 5*menu)
	needRest bool           // ds:fe2c
	redraw   bool           // ds:fd98
	// memorize scratch (ds:fd9a..fe2b)
	spellLevel [32]byte   // fd9a
	lists      [5][9]byte // fdba: n, spell ids
	used       [5]byte    // fde7
	count      [32]byte   // fdec (wanted)
	ready      [32]byte   // fded (already memorized)
}

func (e *Engine) campStr(i int) string {
	if s, ok := e.camp.labels[i]; ok {
		return s
	}
	return e.nthStr(e.X.SegOffset(campStrSeg, 0)-e.X.DS, i)
}

func (e *Engine) campTmpl(t, o int) int { return int(e.X.FileU8(e.X.SegOffset(campBtnSeg, t*9+o))) }

func (e *Engine) campMenu(m, o int) int { return int(e.X.I8(dsCampMenus + m*5 + o)) }

// campPrint prints (shadowed) text with '\r' line breaks.
func (e *Engine) campPrint(s string, x, y int, fg, bg byte) {
	for i, l := range strings.Split(s, "\r") {
		e.printShadow(l, x, y+i*e.font.H, fg, bg)
	}
}

// campBox ports S0d1c_0c21.
func (e *Engine) campBox(x, y, w, h int, pressed, noFill bool) {
	c1, c2 := e.col.BoxHi, e.col.BoxLo
	if pressed {
		c1, c2 = e.col.BoxFill, e.col.BoxFill
	}
	e.Scr.DrawBox3D(0, x, y, x+w-1, y+h-1, c1, c2, -1)
	fill := int(e.col.BoxFill)
	if noFill {
		fill = -1
	}
	e.Scr.DrawBox3D(0, x+1, y+1, x+w-2, y+h-2, e.col.BoxHi, e.col.BoxLo, fill)
}

// saveCampView ports O2102_1107: saves the 176x24 strip under the 3D view
// (tmpBuffer+0x4e20) that the camp menus cover.
func (e *Engine) saveCampView() { e.Scr.CopyRegion(0, 0x78, 0, 0x78, 0xb0, 0x18, 0, campPage) }

// restoreCampView ports O2102_112d.
func (e *Engine) restoreCampView() { e.Scr.CopyRegion(0, 0x78, 0, 0x78, 0xb0, 0x18, campPage, 0) }

// prefLabel ports O2102_10de.
func (e *Engine) prefLabel(on bool) string {
	if on {
		return e.campStr(0x15)
	}
	return e.campStr(0x16)
}

// drawCampMenu ports O2102_1153: background box, title and buttons.
func (e *Engine) drawCampMenu(m int) {
	if dim := e.campMenu(m, 1); dim != 0 {
		d := &e.Scr.Dims[dim]
		d.UnkA = int(e.col.BoxFill)
		e.campBox(d.X*8, d.Y, d.W*8, d.H, false, false)
	}
	title, ok := e.camp.titles[m]
	if !ok {
		title = e.campMenu(m, 0)
	}
	e.campPrint(e.campStr(title), 5, 5, byte(e.campMenu(m, 4)), 0)
	e.camp.btns = e.camp.btns[:0]
	for i := 0; i < e.campMenu(m, 3); i++ {
		t := e.campMenu(m, 2) + i
		b := Button{ID: t + 1, Key: e.campTmpl(t, 5), X: e.campTmpl(t, 1) * 8, Y: e.campTmpl(t, 2),
			W: e.campTmpl(t, 3) * 8, H: e.campTmpl(t, 4), Arg: e.campTmpl(t, 7) | e.campTmpl(t, 8)<<8}
		e.camp.btns = append(e.camp.btns, b)
		e.drawCampButton(&b, false, false, false)
	}
	e.flushInput() // O2102_13e0
}

// drawCampButton ports O2102_1551.
func (e *Engine) drawCampButton(b *Button, pressed, hl, plain bool) {
	if b == nil {
		return
	}
	if b.Arg&1 != 0 {
		e.campBox(b.X, b.Y, b.W, b.H, pressed, plain)
	}
	t := b.ID - 1
	if e.campTmpl(t, 0) == 0 {
		return
	}
	s := e.campStr(e.campTmpl(t, 0))
	x, y := 4, 3
	if b.Arg&4 != 0 {
		x, y = (b.W-len(s)*8)/2+1, (b.H-7)/2
	}
	fg := byte(0xf)
	if hl {
		fg = 6
	}
	if pressed || plain {
		e.printText(s, b.X+x, b.Y+y, fg, 0)
	} else {
		e.printShadow(s, b.X+x, b.Y+y, fg, 0)
	}
}

func (e *Engine) campButton(id int) *Button {
	for i := range e.camp.btns {
		if e.camp.btns[i].ID == id&0x7fff {
			return &e.camp.btns[i]
		}
	}
	return nil
}

// campPress animates a button press (O2102_1551 pressed, wait 5, released
// plain, highlighted if hl).
func (e *Engine) campPress(b *Button, hl bool) {
	if b == nil {
		return
	}
	e.drawCampButton(b, true, true, true)
	e.present()
	e.waitTicks(5)
	e.drawCampButton(b, false, hl, true)
}

// campInput ports the S16a5_005a poll used by the camp menus: returns
// 0x8000|id of an activated button, a raw key code or 0.
func (e *Engine) campInput() int {
	e.present()
	e.Sys.Sleep(16)
	ev, ok := e.poll()
	if !ok {
		return 0
	}
	switch ev.Kind {
	case EvQuit:
		e.quit = true
		return KeyEsc
	case EvKeyDown:
		for _, b := range e.camp.btns {
			if b.Key != 0 && b.Key&0xff == ev.Key {
				return 0x8000 | b.ID
			}
		}
		return ev.Key
	case EvMouseDown: // 0x1100 buttons fire on press (S16a5_00bd state 1)
		e.mouseX, e.mouseY = ev.X, ev.Y
		for _, b := range e.camp.btns {
			if ev.X >= b.X && ev.X < b.X+b.W && ev.Y >= b.Y && ev.Y < b.Y+b.H {
				return 0x8000 | b.ID
			}
		}
	}
	return 0
}

// inRect ports S082c_0613.
func inRect(x, y, x1, y1, x2, y2 int) bool { return x >= x1 && x <= x2 && y >= y1 && y <= y2 }

// campMessage ports O2102_13f7: shows string i in dim 11 until a key or
// click, at most 120 ticks. Also ports O212a_0b9b, the rest overlay's
// byte-identical copy.
func (e *Engine) campMessage(i int) {
	defer e.setFont(e.setFont(e.font8))
	d := &e.Scr.Dims[0xb]
	d.UnkA = int(e.col.BoxFill)
	e.campBox(d.X*8, d.Y, d.W*8, d.H, false, false)
	e.campPrint(e.campStr(i), d.X*8+5, d.Y+5, 0xf, 0)
	e.present()
	end := e.Sys.Millis() + 120*tickMillis
	for e.Sys.Millis() < end && !e.quit {
		ev, ok := e.poll()
		if ok && (ev.Kind == EvKeyDown || ev.Kind == EvMouseDown || ev.Kind == EvQuit) {
			e.quit = e.quit || ev.Kind == EvQuit
			break
		}
		e.Sys.Sleep(16)
	}
	e.flushInput() // O2102_14da/150b, O212a_0c7e/0caf
}

// campConfirm ports O2102_243a: Yes/No question, true for Yes.
func (e *Engine) campConfirm(i int) bool {
	defer e.setFont(e.setFont(e.font8))
	old := e.camp.btns
	e.camp.btns = nil
	e.drawCampMenu(5)
	d := e.Scr.Dims[0xb]
	e.campPrint(e.campStr(i), (d.X+1)*8, d.Y+4, 0xf, 0)
	sel, cur := -1, 0
	for !e.quit {
		if sel != cur {
			if sel != -1 {
				e.drawCampButton(e.campButton(sel+0x21), false, false, true)
			}
			e.drawCampButton(e.campButton(cur+0x21), false, true, true)
			sel = cur
		}
		k := e.campInput()
		// jump table at 266c; Esc is not in it. The press animates the
		// highlighted button, whichever was clicked.
		yes := false
		switch k {
		case 0x2b, 0x3d, 0x61: // enter, space
			yes = sel == 0
		case 0x8021:
			yes = true
		case 0x8022:
		case 0x5c, 0x66:
			cur ^= 1
			continue
		default:
			if !e.KbdMode {
				mx, my := e.mouse()
				for _, b := range e.camp.btns {
					if b.Arg&2 != 0 && inRect(mx, my, b.X, b.Y, b.X+b.W, b.Y+b.H) {
						cur = b.ID - 0x21
						break
					}
				}
			}
			continue
		}
		e.campPress(e.campButton(sel+0x21), true)
		e.camp.btns = old
		return yes
	}
	e.camp.btns = old
	return false
}

// campSelectChar ports O2102_0416: lets the player pick a character for
// action msg (0x17 memorize, 0x1a pray, 0x31 scribe, 0x35 drop).
// Returns -1 if cancelled or not possible.
func (e *Engine) campSelectChar(msg int) int {
	mask := 2
	if msg == 0x1a {
		mask = 4
	}
	e.flushInput() // O2102_042f
	e.camp.redraw = false
	var elig [6]bool
	n, sel := 0, -1
	for c := range e.Chars {
		ch := &e.Chars[c]
		if (ch.Flags&1 != 0 && ch.HPCur > 0 && ch.Food != 0 && e.u8(dsClassFlags+int(ch.Class))&mask != 0) ||
			(msg == 0x35 && ch.Flags&1 != 0) {
			elig[c] = true
			n++
			sel = c
		}
	}
	if n == 0 {
		switch msg {
		case 0x1a:
			e.campMessage(0x1b)
		case 0x17:
			e.campMessage(0x1c)
		case 0x31:
			e.campMessage(0x34)
		default:
			e.campMessage(0)
		}
		return -1
	}
	for c := range e.Chars {
		if elig[c] || !e.charStatus(c, 1) {
			continue
		}
		x, y := e.u8(dsCampPortX8+c)*8, e.u8(dsCampPortY+c)
		for i := 0; i < 4; i++ {
			e.Scr.DrawShape(0, e.GuiShapes[6], x+i*0x10, y, 0, 0)
		}
		e.camp.redraw = true
	}
	if n == 1 {
		if e.lowPaladin(sel) {
			e.campMessage(0x18)
			return -1
		}
		return sel
	}
	old := e.camp.btns
	e.camp.btns = nil
	e.camp.titles[3] = msg
	e.drawCampMenu(3)
	di := 0
	for di < 6 && !elig[di] {
		di++
	}
	if !e.KbdMode {
		di = -1
	}
	sel = -2
	e.setFont(e.font6)
	for sel == -2 && !e.quit {
		k := e.campInput()
		e.highlight(di)
		switch k {
		case 0x12, 0x1f, 0x5c, 0x60:
			e.highlight(-1)
			e.drawCharPanel(di)
			for {
				if di--; di < 0 {
					di = 5
				}
				if elig[di] {
					break
				}
			}
		case 0x20, 0x2e, 0x62, 0x66:
			e.highlight(-1)
			e.drawCharPanel(di)
			for {
				if di++; di > 5 {
					di = 0
				}
				if elig[di] {
					break
				}
			}
		case KeyEsc, 0x8012:
			e.setFont(e.font8)
			e.campPress(e.campButton(0x12), false)
			e.setFont(e.font6)
			sel = -1
		case 0x8013, 0x8014, 0x8015, 0x8016, 0x8017, 0x8018:
			if elig[k-0x8013] {
				sel = k - 0x8013
			}
		case 0x2b, 0x61:
			if di >= 0 {
				sel = di
			}
		}
	}
	e.highlight(-1)
	if di > 0 {
		e.drawCharPanel(di)
	}
	e.setFont(e.font8)
	e.camp.btns = old
	if sel == -1 || msg == 0x35 {
		return sel
	}
	if mask == 4 && e.lowPaladin(sel) {
		e.campMessage(0x18)
		return -1
	}
	if mask == 2 && e.findInvItem(sel, 0x1d, -1) == -1 {
		e.campMessage(0x19)
		return -1
	}
	return sel
}

func (e *Engine) lowPaladin(c int) bool {
	ch := &e.Chars[c]
	i := e.classPartIndex(int(ch.Class), 4)
	return i != -1 && ch.Level[i] <= 8
}

// findInvItem ports S0d1c_00f1: inventory slot holding an item of type
// typ (-1 any) and value v (-1 any), or -1.
func (e *Engine) findInvItem(c, typ, v int) int {
	for s, it := range e.Chars[c].Inventory {
		if it == 0 {
			continue
		}
		i := &e.Items[it]
		if (int(i.Type) == typ || typ == -1) && (int(i.Value) == v || v == -1) {
			return s
		}
	}
	return -1
}

// wearsRing ports S0d1c_0163: ring of type 0x2f with value v on a hand.
func (e *Engine) wearsRing(c, v int) bool {
	for s := 0x19; s <= 0x1a; s++ {
		it := e.Chars[c].Inventory[s]
		if it != 0 && e.Items[it].Type == 0x2f && int(e.Items[it].Value) == v {
			return true
		}
	}
	return false
}

// memorizeSlots computes the spells per level, the spell mask and the
// cleric flag (first part of O2102_097a).
func (e *Engine) memorizeSlots(c int, pray bool) (slots [5]int, mask uint32, cleric bool) {
	ch := &e.Chars[c]
	wis := int(ch.WisCur) - 12
	if wis < 0 {
		wis = 0
	}
	if !pray {
		if i := e.classPartIndex(int(ch.Class), 1); i != -1 {
			l := int(ch.Level[i]) - 1
			for j := range slots {
				slots[j] = e.u8(dsSlotsMage + l*5 + j)
			}
			mask = ch.MageSpellsAvail
		}
		if e.wearsRing(c, 1) {
			slots[3] *= 2
			slots[4] *= 2
		}
		return slots, mask, false
	}
	if i := e.classPartIndex(int(ch.Class), 2); i != -1 {
		l := int(ch.Level[i]) - 1
		for j := range slots {
			slots[j] = e.u8(dsSlotsCleric + l*5 + j)
			if slots[j] != 0 {
				slots[j] += e.u8(dsSlotsWisBonus + wis*5 + j)
			}
		}
		mask = 0x7bffff
	} else if i := e.classPartIndex(int(ch.Class), 4); i != -1 {
		l := int(ch.Level[i])
		for j := range slots {
			slots[j] = e.u8(dsSlotsPaladin + l*5 + j)
		}
		mask = 0x800ff2
	}
	return slots, mask, true
}

func (e *Engine) spellList(c int, cleric bool) *[30]int8 {
	if cleric {
		return &e.Chars[c].ClericSpells
	}
	return &e.Chars[c].MageSpells
}

// memorizeInit ports O2102_00dd: counts memorized spells and builds the
// per-level lists of learnable spells.
func (e *Engine) memorizeInit(c int, slots [5]int, mask uint32, cleric bool) {
	cs := &e.camp
	cs.spellLevel, cs.lists, cs.used, cs.count, cs.ready = [32]byte{}, [5][9]byte{}, [5]byte{}, [32]byte{}, [32]byte{}
	lv := e.X.Str(dsSpellLvlMage)
	if cleric {
		lv = e.X.Str(dsSpellLvlCleric)
	}
	copy(cs.spellLevel[:], lv)
	for _, v := range e.spellList(c, cleric) {
		s := int(v)
		if s == 0 {
			continue
		}
		if s > 0 {
			cs.ready[s-1]++
		} else {
			s = -s
		}
		cs.count[s-1]++
		if l := cs.spellLevel[s-1]; l > 0 {
			cs.used[l-1]++
		}
	}
	for b := 0; b < 32; b++ {
		di := int(cs.spellLevel[b]) - 1
		if mask&(1<<b) == 0 || di < 0 || (cleric && slots[di] == 0) {
			continue
		}
		l := &cs.lists[di]
		l[0]++
		l[l[0]] = byte(b + 1)
	}
}

// memorizeApply ports O2102_025d: writes the chosen spells back; spells
// not memorized yet are stored negative (learnt on rest).
func (e *Engine) memorizeApply(c int, cleric bool) {
	cs := &e.camp
	list := e.spellList(c, cleric)
	*list = [30]int8{}
	for i := 0; i < 32; i++ {
		for {
			if cs.count[i] < cs.ready[i] {
				cs.ready[i] = cs.count[i]
			}
			v := int8(i + 1)
			switch {
			case cs.ready[i] != 0:
				cs.ready[i]--
				cs.count[i]--
			case cs.count[i] != 0:
				cs.count[i]--
				cs.needRest = true
				v = -v
			default:
				v = 0
			}
			if v == 0 {
				break
			}
			l := int(cs.spellLevel[i]) - 1
			for s := 0; s < 6 && l >= 0; s++ {
				if list[l*6+s] == 0 {
					list[l*6+s] = v
					break
				}
			}
		}
	}
}

// drawMemorizeRow ports O2102_0881: id 0 clears the row (unless plain);
// plain rows are printed without shadow on bg 0.
func (e *Engine) drawMemorizeRow(id, row int, cleric, plain, hl bool) {
	if row < 0 {
		return
	}
	y := 0x32 + row*9
	if id == 0 {
		if !plain {
			e.Scr.FillRect(0, 6, y, 0xa8, y+8, e.col.BoxFill)
		}
		return
	}
	names := dsMageNames
	if cleric {
		names = dsClericNames
	}
	fg := byte(0xf)
	if hl {
		fg = 6
	}
	s := fmt.Sprintf("%-18s %1d", e.farStr(names+id*4), int8(e.camp.count[id-1]))
	if plain {
		e.printText(s, 8, y, fg, 0)
	} else {
		e.printShadow(s, 8, y, fg, e.col.BoxFill)
	}
}

// memorize ports O2102_097a (memorize / pray for spells menu).
func (e *Engine) memorize(c int, pray bool) {
	if c == -1 {
		return
	}
	cs := &e.camp
	slots, mask, cleric := e.memorizeSlots(c, pray)
	e.memorizeInit(c, slots, mask, cleric)
	old := cs.btns
	cs.btns = nil
	e.drawCampMenu(4)
	// si level shown, want level asked; hl row asked, drawn row shown.
	si, want, hl, drawn := -1, 0, 0, -1
	text, list := true, false
	row := func(r int) int { return int(cs.lists[si][r+1]) } // r=-1 reads the count byte
	for running := true; running && !e.quit; {
		e.highlight(c)
		if want < 0 {
			want = 4
		}
		if want > 4 {
			want = 0
		}
		if si != want {
			if si >= 0 {
				e.drawCampButton(e.campButton(si+0x1b), false, false, true)
			}
			e.drawCampButton(e.campButton(want+0x1b), false, true, true)
			hl, drawn, si, text, list = 0, -1, want, true, true
		}
		n := int(cs.lists[si][0])
		if list {
			list = false
			for r := 0; r < 8; r++ {
				e.drawMemorizeRow(row(r), r, cleric, false, false)
			}
			drawn = -1
		}
		if text {
			text = false
			e.printShadow(fmt.Sprintf(e.X.Str(dsCampFmtRemain), slots[si]-int(int8(cs.used[si])), slots[si]), 8, 0x26, 9, e.col.BoxFill)
		}
		if hl < 0 {
			hl = n - 1
		}
		if n <= hl {
			hl = 0
		}
		if hl != drawn {
			if drawn >= 0 {
				e.drawMemorizeRow(row(drawn), drawn, cleric, true, false)
			}
			if hl >= 0 {
				e.drawMemorizeRow(row(hl), hl, cleric, true, true)
			}
			drawn = hl
		}
		k := e.campInput()
		switch k {
		case 0x5c:
			k = 0x801a + si
			if si == 0 {
				k = 0x801f
			}
		case 0x66:
			k = 0x801b + (si+1)%5
		case KeyEsc:
			k = 0x801a
		default:
			mx, my := e.mouse()
			if !e.KbdMode && inRect(mx, my, 8, 0x32, 0xa8, 0x7a) {
				hl = min((my-0x32)/9, n-1)
			}
		}
		if k&0x8000 != 0 {
			e.campPress(e.campButton(k), false)
		}
		switch k {
		case 0xc, 0x42, 0x69: // remove one
			id := row(drawn)
			// drawn -1 (empty level) reads id 0: count[-1] is used[3] in the original
			cnt := &cs.used[3]
			if id > 0 {
				cnt = &cs.count[id-1]
			}
			if slots[si] != 0 && *cnt != 0 {
				*cnt--
				cs.used[si]--
				e.drawMemorizeRow(row(drawn), drawn, cleric, false, true)
				text = true
			}
		case 0xd, 0x2b, 0x3d, 0x61, 0x6a, 0x8020: // add one
			if slots[si]-int(int8(cs.used[si])) > 0 && drawn > -1 {
				cs.count[row(drawn)-1]++
				cs.used[si]++
				e.drawMemorizeRow(row(drawn), drawn, cleric, false, true)
				text = true
			}
		case 0x8019: // clear
			if cs.used[si] != 0 {
				for r := 0; r < n; r++ {
					s := row(r) - 1
					cs.used[si] -= cs.count[s]
					cs.count[s] = 0
				}
				text, list = true, true
			}
		case 0x801a:
			for i := range e.Chars { // S0d1c_0583
				e.drawCharPanel(i)
			}
			running = false
		case 0x801b, 0x801c, 0x801d, 0x801e, 0x801f:
			if want = k - 0x801b; want == si {
				e.drawCampButton(e.campButton(k), false, true, true)
			}
		case 0x5b:
			hl = 0
		case 0x5d:
			hl = n - 1
		case 0x60:
			hl = drawn - 1
		case 0x62:
			hl = drawn + 1
		}
	}
	cs.btns = old
	e.highlight(-1)
	e.setFont(e.font6)
	e.drawCharPanel(c)
	e.setFont(e.font8)
	e.memorizeApply(c, cleric)
}

// scribeScrolls ports O2102_1691.
func (e *Engine) scribeScrolls() {
	var slot, owner [32]int
	n := 0
	for k := 0; k < 32; k++ {
		for c := 0; c < 6; c++ {
			slot[k] = e.findInvItem(c, 0x22, k+1) + 1
			if slot[k] > 0 {
				owner[k] = c
				n++
				break
			}
		}
	}
	if n == 0 {
		e.campMessage(0x32)
		return
	}
	c := e.campSelectChar(0x31)
	if c == -1 {
		return
	}
	cnt := 0
	for k := 0; k < 32 && cnt < 6; k++ {
		if slot[k] == 0 {
			continue
		}
		if e.Chars[c].MageSpellsAvail&(1<<k) != 0 {
			slot[k] = 0
		} else {
			cnt++
		}
	}
	if cnt == 0 {
		e.campMessage(0x33)
		return
	}
	old := e.camp.btns
	defer func() { e.camp.btns = old }()
	var list []int
	di, sel, dirty := 0, -1, true
	for !e.quit {
		if dirty {
			list = list[:0]
			for k := 0; k < 32 && len(list) < 6; k++ {
				if slot[k] != 0 {
					list = append(list, k+1)
				}
			}
			if len(list) == 0 {
				return
			}
			e.camp.btns = nil
			e.drawCampMenu(6)
			for i, k := range list {
				e.printShadow(e.farStr(dsMageNames+k*4), 8, 0x32+i*9, 0xf, 0)
			}
			sel, di, dirty = -1, 0, false
		}
		if sel != di {
			if sel >= 0 {
				e.printText(e.farStr(dsMageNames+list[sel]*4), 8, 0x32+sel*9, 0xf, 0)
			}
			sel = di
			e.printText(e.farStr(dsMageNames+list[di]*4), 8, 0x32+di*9, 6, 0)
		}
		k := e.campInput()
		mx, my := e.mouse()
		if k == 0 && inRect(mx, my, 8, 0x32, 0xb0, 0x31+len(list)*9) {
			di = (my - 0x32) / 9
		}
		switch k {
		case 0x6e, 0x8023:
			return
		case 0x60:
			di = (di + len(list) - 1) % len(list)
		case 0x62:
			di = (di + 1) % len(list)
		case 0x8024:
			if v := (e.mouseY - 0x32) / 9; v >= 0 && v < len(list) { // click y ds:8142
				di = v
			} else {
				break
			}
			fallthrough
		case 0x2b, 0x3d, 0x61:
			sp := list[di] - 1
			o := owner[sp]
			it := e.Chars[o].Inventory[slot[sp]-1]
			e.Chars[c].MageSpellsAvail |= 1 << sp
			e.Chars[o].Inventory[slot[sp]-1] = 0
			slot[sp] = 0
			e.Items[it].Block = -1
			dirty = true
			e.drawCharPanel(o)
		}
	}
}

// dropChar ports O2102_0000.
// dropChar ports S082c_07c5 (3-way memcpy swap via temp, inlined).
func (e *Engine) dropChar(c int) {
	if c < 0 || !e.charStatus(c, 1) {
		return
	}
	e.removeChar(c)
	si := c
	if c < 4 {
		si = 4
		if e.charStatus(5, 1) {
			si = 5
		}
		e.Chars[c], e.Chars[si] = e.Chars[si], e.Chars[c]
		e.drawCharPanel(c)
	}
	o := dsDropBoxes + (si-4)*8
	e.Scr.FillRect(0, int(e.X.I16(o)), int(e.X.I16(o+2)), int(e.X.I16(o+4)), int(e.X.I16(o+6)), 0xc)
	e.selectChar(0)
	e.rebuildCharTimers()
}

// removeChar ports S0d1c_035a: drops the inventory on the floor.
func (e *Engine) removeChar(c int) {
	ch := &e.Chars[c]
	ch.Flags = 0
	pos := func() int { return e.u8(0x1a1b + e.CurrentDirection*4 + e.rollDice(1, 2, -1)) }
	for s := 0; s < 27; s++ {
		if s != 16 && ch.Inventory[s] != 0 {
			e.linkItemToBlock(e.CurrentBlock, int(ch.Inventory[s]), pos())
		}
	}
	for ch.Inventory[16] != 0 {
		it := e.unlinkListItem(&ch.Inventory[16], 0, -1)
		e.linkItemToBlock(e.CurrentBlock, it, pos())
	}
	if int(e.SelectedChar) == c { // ds:1f3a is both the selected and the inventory char
		e.SelectedChar, e.InvChar = 0, 0
	}
	e.rebuildCharTimers()
}

// openCamp ports O2102_1acd (camp button): main camp menu loop.
func (e *Engine) openCamp() {
	if e.GuiMode != 0 {
		e.closeInventory()
	}
	if e.SpellbookOpen {
		e.closeSpellbook()
	}
	if e.camp.labels == nil {
		e.camp.labels, e.camp.titles = map[int]string{}, map[int]int{}
	}
	e.camp.titles[0], e.camp.titles[2], e.camp.titles[1] = 0x38, 0x39, 0x3a
	for c := range e.Chars {
		if e.charStatus(c, 1) {
			ch := &e.Chars[c]
			ch.DamageTaken, ch.SlotStatus[0], ch.SlotStatus[1] = 0, 0, 0
			e.drawCharPanel(c)
		}
	}
	e.saveCampView()
	oldDim := e.txtDim // [bp-6], restored at 23bb
	oldFont := e.setFont(e.font8)
	e.camp.redraw = false
	redrawChars, reloadInvent := false, false
	prev, di := -1, 0
	hl, want := 0, 0
	for running := true; running && !e.quit; {
		if di == 2 {
			e.camp.labels[0x11] = fmt.Sprintf(e.X.Str(dsCampFmtPrefs), e.prefLabel(e.Opt80e8 != 0))
			e.camp.labels[0x12] = fmt.Sprintf(e.X.Str(dsCampFmtPrefs+0xf), e.prefLabel(e.Opt80e6 != 0))
			e.camp.labels[0x13] = fmt.Sprintf(e.X.Str(dsCampFmtPrefs+0x1f), e.prefLabel(e.OptF43c != 0))
			e.camp.labels[0x14] = fmt.Sprintf(e.X.Str(dsCampFmtPrefs+0x33), e.prefLabel(!e.KbdMode))
		}
		if di != -1 {
			e.drawCampMenu(di)
			if di != prev {
				want, hl = e.campMenu(di, 2)+1, 0
			}
			prev, di = di, -1
		}
		k := e.campInput() & 0x80ff
		for i := 0; e.u8(dsCampKeyMap+i) != 0; i++ {
			if e.u8(dsCampKeyMap+i) == k {
				k = e.u8(dsCampKeyMap+i+2) + 0x8001
				break
			}
		}
		if (k == 0x2b || k == 0x3d || k == 0x61) && hl != 0 {
			k = 0x8000 | hl
		}
		// any code naming a button animates it, even a raw key (1d55)
		if b := e.campButton(k); k&0x7fff != 0 && b != nil {
			e.drawCampButton(e.campButton(hl), false, false, true)
			e.campPress(b, true)
			hl, want = 0, b.ID
		}
		first, last := e.campMenu(prev, 2)+1, e.campMenu(prev, 2)+e.campMenu(prev, 3)
		switch k {
		case 0x8004:
			e.scribeScrolls()
			di = 0
		case 0x800a:
			n := 0
			for c := range e.Chars {
				if e.charStatus(c, 1) {
					n++
				}
			}
			if n < 5 {
				e.campMessage(0x2d)
			} else {
				e.dropChar(e.campSelectChar(0x35))
				e.loadCPSVia3(e.X.Str(dsCampPlayfld), 0)
				reloadInvent = true
				e.loadCPSVia3(e.X.Str(dsCampInvent), 2)
				e.saveCampView()
				e.setFont(e.font6)
				e.DrawPlayfield()
				e.setFont(e.font8)
			}
			di = 0
		case 0x8001:
			if e.restParty() {
				running = false
			} else {
				e.camp.needRest = false
			}
			redrawChars = true
			di = 0
		case 0x8002:
			e.memorize(e.campSelectChar(0x17), false)
			di = 0
		case 0x8003:
			e.memorize(e.campSelectChar(0x1a), true)
			di = 0
		case 0x8005, 0x800d, 0x800e:
			if k == 0x800d {
				e.Opt80e8 ^= 1
			} else if k == 0x800e {
				e.Opt80e6 ^= 1
			}
			di = 2
		case 0x800f:
			e.OptF43c ^= 1
			e.BarGraphs = e.OptF43c != 0
			di, redrawChars = 2, true
		case 0x8010:
			e.KbdMode = !e.KbdMode
			di, redrawChars = 2, true
		case 0x8008:
			if !e.campConfirm(0x37) {
				di = 1
				break
			}
			reloadInvent = true
			e.restoreCampView()
			e.setFont(e.font6)
			e.DrawScene(true)
			if err := e.LoadOriginalSave(e.savePath); err != nil {
				e.setFont(e.font8)
				e.saveCampView()
				e.camp.btns = nil
				e.drawCampMenu(1)
				want = e.campMenu(1, 2) + 1
				e.campMessage(0x10)
				di = 1
				break
			}
			e.saveCampView()
			e.setFont(e.font8)
			running = false
		case 0x8009:
			if e.campConfirm(0x36) {
				if e.SaveGame(e.savePath) == nil {
					e.campMessage(0xe)
				} else {
					e.campMessage(0xf)
				}
			}
			di = 1
		case 0x8006:
			di = 1
		case 0x8007, 0x800c, 0x8011:
			if k == 0x8007 && e.camp.needRest {
				e.campMessage(0x2c)
			}
			if prev == 0 {
				running = false
			} else {
				di = 0
			}
		case 0x800b:
			if e.campConfirm(0x2e) {
				e.quit = true
			}
			di = 0
		case 0x5b, 0x65:
			want = first
		case 0x5d, 0x67, 0x60, 0x62:
			if k == 0x5d || k == 0x67 {
				// the original falls through into the 0x60 handler
				want, k = last, 0x60
			}
			if hl == 0 {
				break
			}
			d := 1
			if k == 0x60 {
				d = -1
			}
			want = hl
			for {
				want += d
				if want < first {
					want = last
				}
				if want > last {
					want = first
				}
				if e.campTmpl(want-1, 7)&2 != 0 || want == hl {
					break
				}
			}
		default:
			if !e.KbdMode {
				mx, my := e.mouse()
				for _, b := range e.camp.btns {
					if b.Arg&2 != 0 && inRect(mx, my, b.X, b.Y, b.X+b.W, b.Y+b.H) {
						want = b.ID
					}
				}
			}
		}
		if e.camp.redraw || redrawChars {
			e.setFont(e.font6)
			for c := 0; c < 6; c++ {
				e.drawCharPanel(c)
				e.sortSpells(c)
			}
			e.setFont(e.font8)
			e.camp.redraw, redrawChars = false, false
		}
		if hl != want && di == -1 && running {
			e.drawCampButton(e.campButton(hl), false, false, true)
			e.drawCampButton(e.campButton(want), false, true, true)
			hl = want
		}
	}
	e.camp.btns = nil
	e.setFont(oldFont)
	e.DrawScene(false)
	e.setDim(10) // 2363: blit the redrawn 3D view (dim 10) over the camp menu
	d := e.Scr.Dims[10]
	e.Scr.CopyRegion(d.X*8, d.Y, d.X*8, d.Y, d.W*8, d.H, scenePage, 0)
	e.restoreCampView()
	if reloadInvent { // 2394: INVENT back to page 2
		e.loadCPSVia3(e.X.Str(dsCampInvent), 2)
	}
	e.setDim(oldDim) // 23bb: back to the message window
	e.present()
}

// sortSpells ports S082c_07a4 (byte swap helper, inlined).
// sortSpells ports S0ef3_000e: bubble-sorts each 6-slot row of the mage and
// cleric memorized lists; ready spells first, then cast (negated), empty last.
func (e *Engine) sortSpells(c int) {
	ch := &e.Chars[c]
	key := func(v int8) int {
		if v == 0 {
			return 0x50
		}
		if v < 0 {
			return 0x28 - int(v)
		}
		return int(v)
	}
	for _, list := range [][]int8{ch.MageSpells[:], ch.ClericSpells[:]} {
		for r := 0; r < 30; r += 6 {
			a := list[r : r+6]
			for swapped := true; swapped; {
				swapped = false
				for j := 0; j < 5; j++ {
					if key(a[j]) > key(a[j+1]) {
						a[j], a[j+1] = a[j+1], a[j]
						swapped = true
					}
				}
			}
		}
	}
}
