package eob

// Spellbook menu and casting flow (ports of S0ef3_21e0, 2295, 2541, 25bd,
// 2929, 2676, 08cf, 059d, 026f).

const (
	dsMageNames     = 0x1278 // far ptrs by spell id (mage list)
	dsClericNames   = 0x12e0
	dsSpellTabBoxes = 0x5773 // 5 x {x1,y1,x2,y2}
	dsSpellTabNames = 0x5787 // 5 far ptrs
	dsStrAbortSpell = 0x592b
	dsStrAbortSel   = 0x5937
	dsStrWhichChar  = 0x57a3
	dsStrTargetOk   = 0x57c3
	dsStrTargetNone = 0x57c8
)

type spellbook struct {
	level   int  // ds:dde2
	row     int  // ds:a9c5
	caster  int  // ds:df52
	cleric  bool // ds:f43d
	list    *[30]int8
	names   int
	waiting bool // target selection active
}

// openSpellbook ports S0ef3_21e0.
func (e *Engine) openSpellbook(c int, cleric bool) {
	sb := &e.book
	sb.level = 0
	sb.caster = c
	sb.cleric = cleric
	if cleric {
		sb.names = dsClericNames
		sb.list = &e.Chars[c].ClericSpells
	} else {
		sb.names = dsMageNames
		sb.list = &e.Chars[c].MageSpells
	}
	sb.row = 0
	for sb.row < 6 && sb.list[sb.row] <= 0 {
		sb.row++
	}
	if !e.SpellbookOpen {
		e.Scr.CopyRegion(64, 122, 64, 122, 112, 55, 0, 12) // save area under the menu
	}
	e.SpellbookOpen = true
	e.drawSpellbook()
}

// drawSpellbook ports S0ef3_2295.
func (e *Engine) drawSpellbook() {
	sb := &e.book
	p := 2
	e.Scr.FillRect(p, 0x46, 0x7a, 0xaf, 0xaf, e.col.BoxFill)
	for i := 0; i < 5; i++ {
		o := dsSpellTabBoxes + i*4
		hi, lo, fill := e.col.Misc1, e.col.Misc2, e.col.Text
		if sb.level == i {
			hi, lo, fill = e.col.BoxHi, e.col.BoxLo, e.col.BoxFill
		}
		e.Scr.DrawBox3D(p, e.u8(o), e.u8(o+1), e.u8(o+2), e.u8(o+3), hi, lo, int(fill))
	}
	for i := 0; i < 5; i++ {
		e.Scr.PrintText(e.font, p, e.farStr(dsSpellTabNames+i*4), 0x49+i*0x15, 0x7c, 0xc, 0)
	}
	e.Scr.DrawBox3D(p, 0x47, 0x83, 0xaf, 0xae, e.col.BoxHi, e.col.BoxLo, int(e.col.BoxFill))
	for si := 0; si < 7; si++ {
		var v int8
		if si < 6 {
			v = sb.list[sb.level*6+si]
		}
		fg := byte(0xf)
		bg := e.col.BoxFill
		if sb.row == si {
			bg = 0xb
		}
		y := 0x84 + si*6
		switch {
		case si < 6 && v >= 0: // original also prints names[0] for empty slots
			e.Scr.PrintText(e.font, p, e.farStr(sb.names+int(v)*4), 0x49, y, fg, bg)
		case si == 6:
			s := e.X.Str(dsStrAbortSpell)
			if sb.row == 6 {
				s = e.X.Str(dsStrAbortSel)
			}
			e.Scr.PrintText(e.font, p, s, 0x49, y, fg, bg)
		}
	}
	e.Scr.CopyRegion(0x48, 0x7a, 0x48, 0x7a, 0x68, 0x35, 2, 0)
	e.Scr.DrawLine(0, 0x47, 0x7a, 0x47, 0xae, e.col.BoxLo)
}

// closeSpellbook ports S0ef3_2541.
func (e *Engine) closeSpellbook() {
	e.SpellbookOpen = false
	e.book.waiting = false
	e.Scr.CopyRegion(64, 122, 64, 122, 112, 55, 12, 0)
	e.scene.lastCompassDir = -1
	e.drawCompass()
}

// spellbookTab ports S0ef3_25bd.
func (e *Engine) spellbookTab(level int) {
	sb := &e.book
	sb.level = level
	sb.row = 0
	for sb.row < 6 && sb.list[sb.level*6+sb.row] <= 0 {
		sb.row++
	}
	e.drawSpellbook()
}

// spellbookRow ports S0ef3_2929.
// spellbookRow ports S082c_0ab4 (clamp/wrap helper, inlined).
func (e *Engine) spellbookRow(row int, ev Event) {
	sb := &e.book
	if row == 6 {
		any := false
		for i := 0; i < 6; i++ {
			if sb.list[sb.level*6+i] > 0 {
				any = true
				break
			}
		}
		if !any {
			return
		}
		d := 1
		if ev.Button == 1 {
			d = -1
		}
		for {
			sb.row += d
			if sb.row < 0 {
				sb.row = 6
			}
			if sb.row > 6 {
				sb.row = 0
			}
			if sb.row == 6 || sb.list[sb.level*6+sb.row] > 0 {
				break
			}
		}
		e.drawSpellbook()
		return
	}
	if row < 6 && sb.list[sb.level*6+row] <= 0 {
		return
	}
	if row < 6 {
		sb.row = row
	}
	if sb.row >= 6 {
		e.closeSpellbook()
		return
	}
	e.drawSpellbook()
	sp := int(sb.list[sb.level*6+sb.row])
	if sb.cleric {
		sp += 0x19
	}
	e.castSpell(sp, 0)
}

// castSpell ports S0ef3_2676.
func (e *Engine) castSpell(sp, itemSlot int) {
	e.spendTurn()
	sb := &e.book
	e.curSpell = int8(sp)
	s := &e.Spells[sp]
	c := sb.caster
	ch := &e.Chars[c]
	if s.Flags2&1 != 0 && ch.EffectFlags&4 != 0 {
		e.endInvisibility(c)
	}
	si := c
	if si > 3 {
		si -= 2
	}
	e.spellStartPos = int8(e.u8(0x1a1b + e.CurrentDirection*4 + si))
	if s.Flags&2 != 0 {
		if ch.Inventory[0] != 0 && ch.Inventory[1] != 0 {
			e.printMessageNow(e.X.Str(dsStrFreeHand))
			return
		}
		if isMagicItem(int(ch.Inventory[0])) || isMagicItem(int(ch.Inventory[1])) {
			e.printMessageNow(e.X.Str(dsStrTwoSpells))
			return
		}
	}
	if !e.castFromItemFlag {
		base := sb.level * 6
		v := sb.list[base+sb.row]
		if sb.row < 5 {
			copy(sb.list[base+sb.row:base+5], sb.list[base+sb.row+1:base+6])
		}
		sb.list[base+5] = -v
	} else {
		e.consumeItemCharge(c, itemSlot)
	}
	e.printMessage(sprintfC(e.X.Str(dsStrCasts), ch.NameStr(), s.Name))
	if s.Flags&1 != 0 {
		// target prompt ports S0ef3_059d (its S0cf7_0211 button list = sb.waiting)
		e.printMessageNow(e.X.Str(dsStrWhichChar))
		e.playSound(0x4f)
		sb.waiting = true
		return
	}
	e.spellCaster = int8(c)
	e.spellCasterChar = int8(c)
	e.spellTarget = int8(c)
	e.playSound(int(s.Sound))
	if s.Flags&6 != 0 {
		e.spellSparkle(c)
	} else if s.Flags&8 != 0 {
		e.spellSparkle(-1)
	}
	e.printMessage(e.X.Str(dsStrOk))
	if f, ok := e.spellCast[s.Cast]; ok {
		f()
	}
	if s.Flags&4 != 0 {
		e.drawCharPanel(c)
	}
	if e.castFromItemFlag {
		e.endItemCast()
	} else {
		e.closeSpellbook()
	}
}

// isMagicItem ports S0ef3_121b.
func isMagicItem(it int) bool { return it >= 0xb && it <= 0x11 }

// spellTargetClicked ports S0ef3_08cf.
func (e *Engine) spellTargetClicked(c int) {
	if !e.book.waiting {
		return
	}
	e.spellTarget = int8(c)
	if int8(c) == -1 {
		e.printMessageNow(e.X.Str(dsStrTargetNone))
	} else if e.Chars[c].Flags&1 == 0 {
		return // original keeps waiting for a target
	} else {
		ch := &e.Chars[c]
		e.printMessage(ch.NameStr())
		e.printMessage(e.X.Str(dsStrTargetOk))
		s := &e.Spells[e.curSpell]
		e.playSound(int(s.Sound))
		e.spellSparkle(c)
		e.spellCasterChar = int8(e.book.caster)
		if f, ok := e.spellCast[s.Cast]; ok {
			f()
		}
		e.drawCharPanel(c)
	}
	e.book.waiting = false
	if e.castFromItemFlag {
		e.endItemCast()
	} else {
		e.closeSpellbook()
	}
}
