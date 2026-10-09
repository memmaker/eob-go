package eob

// eatFood ports S0acd_07e0.
func (e *Engine) eatFood(c int) {
	ch := &e.Chars[c]
	if !e.charStatus(c, 5) {
		e.printMessage(sprintfC(e.X.Str(0x376a), ch.NameStr()))
		return
	}
	it := e.HandItem
	if t := e.Items[it].Type; t != 0x1f && t != 0x31 {
		if it != 0 {
			e.printMessage(e.X.Str(0x378c))
		}
		return
	}
	ch.Food = byte(min(int(ch.Food)+int(e.Items[it].Value), 100))
	e.Items[it].Block = -1
	e.setHandItem(0)
	e.drawFood(c)
	e.playSound(9)
}

// castFromItem ports S0ef3_28ed.
func (e *Engine) castFromItem(c, spell, slot int) {
	e.savedCaster, e.savedCleric = e.book.caster, e.book.cleric
	e.castFromItemFlag = true
	e.castItemSlot = slot
	e.book.caster = c
	e.book.cleric = spell > 0x19
	e.castSpell(spell, slot)
}

// endItemCast ports S0ef3_026f.
func (e *Engine) endItemCast() {
	c := e.book.caster
	e.Chars[c].DisabledSlots ^= 1 << e.castItemSlot
	e.setCharEventTimer(c, 0x12, e.castItemSlot+2, true)
	e.drawCharPanel(c)
	e.book.caster, e.book.cleric = e.savedCaster, e.savedCleric
	e.castFromItemFlag = false
}

// consumeItemCharge ports S0ef3_2604: scrolls vanish, wands lose a charge.
func (e *Engine) consumeItemCharge(c, slot int) {
	it := int(e.Chars[c].Inventory[slot])
	switch e.Items[it].Type {
	case 0x22, 0x23:
		e.deleteInventoryItem(c, slot)
	case 0x30:
		if e.Items[it].Flags&0x3f == 1 {
			e.deleteInventoryItem(c, slot)
		} else {
			e.Items[it].Flags--
		}
	}
}

// drinkPotion ports S0bd2_0d2d.
func (e *Engine) drinkPotion(c, slot int) {
	ch := &e.Chars[c]
	v := e.deleteInventoryItem(c, slot)
	e.playSound(10)
	switch v {
	case 1: // giant strength
		e.spellSparkle(c)
		ch.StrCur, ch.StrExtCur = 22, 0
		e.setCharEventTimer(c, (e.rollDice(1, 4, 0)+4)*0x222, 7, true)
	case 2: // healing
		e.spellSparkle(c)
		e.healChar(c, e.rollDice(2, 4, 2))
	case 3: // extra healing
		e.spellSparkle(c)
		e.healChar(c, e.rollDice(3, 8, 3))
	case 4: // poison
		e.savingThrowEffect(c, 2, e.X.Str(0x39b0), 0, 1, 8, 1)
		ch.EffectFlags &^= 0x80
	case 5: // vitality
		e.spellSparkle(c)
		ch.Food = 100
		if e.GuiMode != 0 {
			e.drawCharPanel(c)
		}
	case 6: // speed
		e.spellSparkle(c)
	case 7: // invisibility
		e.spellSparkle(c)
		ch.EffectFlags |= 4
		e.drawCharPanel(c)
	case 8: // cure poison
		e.spellSparkle(c)
		e.curePoison(c)
	}
	if v == 4 && ch.Flags&2 != 0 {
		return
	}
	e.printMessage(sprintfC(e.X.Str(0x39b9), ch.NameStr(), e.nthStr(0x3893, v)))
}

// useWand ports S0bd2_0ee8.
func (e *Engine) useWand(c, slot int) {
	v := int(e.Items[e.Chars[c].Inventory[slot]].Value)
	if v == 0 {
		e.printMessage(e.X.Str(0x39c7))
		return
	}
	if v != 5 {
		e.castFromItem(c, int(e.u8(0x38fa+v)), slot)
		return
	}
	// Wand of Silvias: push the monsters in front back one block.
	d := e.CurrentDirection
	b1 := e.calcNewBlock(e.CurrentBlock, d)
	b2 := e.calcNewBlock(b1, d)
	e.playSound(0x62)
	e.wandSparkle()
	if e.WllWallFlags[e.Blocks[b2].Walls[d^2]]&4 == 0 || e.Blocks[b2].Flags&7 != 0 || e.Blocks[b1].Flags&7 == 0 {
		e.printMessage(e.X.Str(0x39f0))
		return
	}
	for i := range e.Monsters {
		if int(e.Monsters[i].Block) == b1 {
			e.placeMonster(&e.Monsters[i], b2, -1)
			e.sceneUpdate = true
		}
	}
}
