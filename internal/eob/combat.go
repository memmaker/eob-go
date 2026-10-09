package eob

// Party combat, damage, experience and character effect timers
// (ports of segments S0de8, S0a72, S1414 and parts of S0bd2).

const (
	dsStrHitTbl     = 0x3439 // [str]
	dsStrHitExtThr  = 0x3430 // [5]
	dsStrHitExtVal  = 0x3435
	dsStrDmgTbl     = 0x345c
	dsStrDmgExtThr  = 0x3453
	dsStrDmgExtVal  = 0x3458
	dsDexMissileTbl = 0x3474 // i16 [dex]
	dsClassGroup    = 0x116e // [class]
	dsThacDiv       = 0x3614 // [group]
	dsThacMul       = 0x3610
	dsNumClasses    = 0x115f // [class]
	dsExpTables     = 0x35bc // far ptr per class part
	dsHPDice0       = 0x174b
	dsHPDice1       = 0x175a
	dsHPDice2       = 0x1769
	dsConHPBonus    = 0x1778
	dsSaveTables    = 0x3406 // far ptr per class group
	dsSaveMaxLvl    = 0x341e
	dsSaveLvlDiv    = 0x3424
	dsSaveTypeMul   = 0x342a
	dsConSaveBonus  = 0x54f6
	dsAmmoType      = 0x552f // [itemType]
	dsStrLevelUp    = 0x55b0
	dsStrIsEffect   = 0x55da
	dsStrPartyDead  = 0x5543
	dsStrStrRestore = 0x5cbb
	dsStrPoisonTick = 0x5cdd
	dsStrParaEnd    = 0x5cfe
	dsStrCantUse    = 0x39fc
)

// strHitBonus ports S0a72_00cf.
func (e *Engine) strHitBonus(c int) int {
	ch := e.charRec(c)
	v := int(e.X.I8(dsStrHitTbl + int(ch.StrCur)))
	if ch.StrExtCur != 0 {
		for i := 0; i < 5; i++ {
			if int(ch.StrExtCur) >= int(e.X.I8(dsStrHitExtThr+i)) {
				v = int(e.X.I8(dsStrHitExtVal + i))
			}
		}
	}
	return v
}

// strDmgBonus ports S0a72_0128.
func (e *Engine) strDmgBonus(c int) int {
	ch := &e.Chars[c]
	v := int(e.X.I8(dsStrDmgTbl + int(ch.StrCur)))
	if ch.StrExtCur != 0 {
		for i := 0; i < 5; i++ {
			if int(ch.StrExtCur) >= int(e.X.I8(dsStrDmgExtThr+i)) {
				v = int(e.X.I8(dsStrDmgExtVal + i))
			}
		}
	}
	return v
}

// dexMissileBonus ports S0a72_0181.
func (e *Engine) dexMissileBonus(c int) int {
	return int(e.X.I16(dsDexMissileTbl + int(e.charRec(c).DexCur)*2))
}

// charRec returns character c. Spells cast by monsters or traps reach the
// to-hit code with c == -1; the original then reads the 243 bytes before
// the party array (ds:f48d), unused zero-initialised memory, so that case
// gets an all-zero record.
func (e *Engine) charRec(c int) *Character {
	if c < 0 {
		e.nullChar = Character{}
		return &e.nullChar
	}
	return &e.Chars[c]
}

// charThac0 ports S0a72_0522.
func (e *Engine) charThac0(c, ac int) int {
	ch := e.charRec(c)
	lvl := int(int8(ch.Level[0] - 1))
	g := e.u8(dsClassGroup + int(ch.Class))
	v := lvl / e.u8(dsThacDiv+g) * e.u8(dsThacMul+g)
	return int(int8(byte(20 - v - ac)))
}

// thac0Target ports S0a72_058c (monster to-hit target vs character AC).
func (e *Engine) thac0Target(hitChance, ac int) int { return hitChance - ac }

// rollToHit ports S0de8_0007.
func (e *Engine) rollToHit(c, mon, item int, melee bool) bool {
	t := 0
	if item != 0 {
		t = int(e.Items[item].Type)
	}
	bonus := 0
	if t >= 1 && t <= 3 {
		bonus = int(e.Items[item].Value)
	}
	if melee {
		bonus += e.strHitBonus(c)
	} else {
		bonus += e.dexMissileBonus(c)
	}
	m := &e.Monsters[mon]
	m.Flags |= 1
	target := e.charThac0(c, e.mprop(m, 0)) - bonus
	r := e.rollDice(1, 20, 0)
	if e.Vars.Raw[10]&0x30 != 0 {
		r++
	}
	if e.charRec(c).EffectFlags&0x40 != 0 {
		r++
	}
	if r > 20 {
		r = 20
	}
	if r < 1 {
		r = 1
	}
	return r >= target
}

// calcDamage ports S0de8_011a.
func (e *Engine) calcDamage(c, mon, item, typ int, strBonus bool) int {
	d := 0
	if strBonus && c != -1 {
		d = e.strDmgBonus(c)
	}
	if item != 0 {
		it := &e.ItemTypes[typ]
		if mon != -1 && e.mprop(&e.Monsters[mon], 0x0e)&1 != 0 {
			d += e.rollDice(int(it.DmgDiceL), int(it.DmgPipsL), int(it.DmgIncS))
		} else {
			d += e.rollDice(int(it.DmgDiceS), int(it.DmgPipsS), int(it.DmgIncS))
		}
		d += int(e.Items[item].Value)
	} else {
		d += e.rollDice(1, 2, 0)
	}
	if d < 0 {
		return 0
	}
	return d
}

// monsterMagicResist ports S0d1c_0bab.
func (e *Engine) monsterMagicResist(mon int) bool {
	return e.rollDice(1, 100, 0) < e.mprop(&e.Monsters[mon], 0x1a)
}

// monsterHalfDamage ports S0d1c_0beb.
func (e *Engine) monsterHalfDamage(mon, typ int) bool {
	return e.mprop(&e.Monsters[mon], 0x0e)&4 != 0 && e.ItemTypes[typ].AllowedClasses&4 == 0
}

// meleeAttack ports S0de8_0359. Returns damage, -1 miss, -2 hack, -3 can't reach.
func (e *Engine) meleeAttack(c, item int) int {
	if c > 1 {
		return -3
	}
	blk := e.calcNewBlock(e.CurrentBlock, e.CurrentDirection)
	mon := e.pickMonsterTarget(c, blk)
	if mon == -1 {
		w := e.Blocks[blk].Walls[e.scene.drawVarDown]
		if e.SpecialWallTypes[w] == 0xff {
			e.Blocks[blk].Walls[e.scene.drawVarDown]++
			e.Blocks[blk].Walls[e.scene.drawVarDown^2]++
			e.sceneUpdate = true
			return -2
		}
		return -1
	}
	if !e.rollToHit(c, mon, item, true) {
		return -1
	}
	typ := int(e.Items[item].Type) // original reads Items[0] for bare hands too
	if typ > 0x33 && typ < 0x39 && e.monsterMagicResist(mon) {
		return -1
	}
	e.attackTarget = mon
	d := e.calcDamage(c, mon, item, typ, true)
	if e.monsterHalfDamage(mon, typ) {
		d >>= 1
	}
	return d
}

// attackWithItem ports S0de8_0763.
func (e *Engine) attackWithItem(c, hand, item int) {
	typ := 0
	if item != 0 {
		typ = int(e.Items[item].Type)
	}
	dmg := 0
	if e.Chars[c].EffectFlags&4 != 0 {
		e.endInvisibility(c)
	}
	switch e.ItemTypes[typ].ExtraProps & 0x7f {
	case 1:
		dmg = e.meleeAttack(c, item)
		if dmg == 0 {
			dmg = -1
		}
		e.playSound(0x20)
	case 2:
		dmg = e.throwHandItem(c, hand, item)
	case 3:
		dmg = e.fireMissile(c, item)
		e.drawCharPanel(c)
	}
	if dmg > 0 {
		ch := &e.Chars[c]
		if e.Items[item].Flags&8 != 0 {
			v := int(ch.HPCur) + dmg
			if v >= 0x80 {
				v = 0x7f
			}
			ch.HPCur = int8(v)
			e.drawCharPanel(c)
		}
		if e.Items[item].Flags&0x10 != 0 {
			ch.Inventory[hand] = 0
		}
		e.damageMonster(&e.Monsters[e.attackTarget], dmg)
	}
	ch := &e.Chars[c]
	ch.DisabledSlots ^= 1 << hand
	ch.SlotStatus[hand] = int8(dmg)
	e.drawCharPanel(c)
	ev := hand + 2
	if dmg < -2 {
		ev = hand
	}
	e.setCharEventTimer(c, 0x12, ev, true)
}

// throwHandItem ports S0de8_0537.
func (e *Engine) throwHandItem(c, hand, item int) int {
	si := c
	if si > 3 {
		si -= 2
	}
	pos := e.u8(0x1a1b + e.CurrentDirection*4 + si)
	if e.launchItem(c, item, e.CurrentBlock, pos, e.CurrentDirection, int(e.Items[item].Type)) {
		e.playSound(0xb)
		e.Chars[c].Inventory[hand] = 0
		e.refillHand(c, hand, -1, false)
		e.sceneUpdate = true
	}
	return 0
}

// refillHand ports S0de8_0474: refill an emptied hand from quiver/belt.
func (e *Engine) refillHand(c, hand, typ int, quiver bool) {
	ch := &e.Chars[c]
	if quiver && ch.Inventory[16] != 0 {
		ch.Inventory[hand] = uint16(e.unlinkListItem(&ch.Inventory[16], 0, -1))
		return
	}
	for s := 0x18; s >= 0x16; s-- {
		it := int(ch.Inventory[s])
		if it == 0 {
			continue
		}
		if int(e.Items[it].Type) != typ || typ == -1 {
			ch.Inventory[hand] = uint16(it)
			ch.Inventory[s] = 0
			return
		}
	}
}

// fireMissile ports S0de8_05bb (bows / slings).
func (e *Engine) fireMissile(c, launcher int) int {
	ch := &e.Chars[c]
	ltype := int(e.Items[launcher].Type)
	ammo := e.u8(dsAmmoType + ltype)
	si := 0
	if ammo == 0x10 {
		if it := int(ch.Inventory[0]); it != 0 && e.Items[it].Type == 0x10 {
			si = it
			ch.Inventory[0] = 0
		} else if it := int(ch.Inventory[1]); it != 0 && e.Items[it].Type == 0x10 {
			si = it
			ch.Inventory[1] = 0
		} else if ch.Inventory[16] != 0 {
			si = e.unlinkListItem(&ch.Inventory[16], 0, -1)
		}
	} else {
		for s := 0; s < 27; s++ {
			it := int(ch.Inventory[s])
			if int(e.Items[it].Type) == ammo { // original: no empty-slot check (Items[0])
				si = it
				ch.Inventory[s] = 0
				if s < 2 {
					e.drawCharPanel(c)
				}
				break
			}
		}
	}
	if si == 0 {
		return -4
	}
	p := c
	if p > 3 {
		p -= 2
	}
	if e.launchItem(c, si, e.CurrentBlock, e.u8(0x1a1b+e.CurrentDirection*4+p), e.CurrentDirection, ltype) {
		if ltype == 7 {
			e.playSound(0x1a)
		} else {
			e.playSound(0xb)
		}
		e.sceneUpdate = true
	}
	return 0
}

// damageChar ports S0de8_09b9.
func (e *Engine) damageChar(c, dmg int) {
	if !e.charStatus(c, 3) {
		return
	}
	ch := &e.Chars[c]
	if ch.EffectsRemainder[3] != 0 {
		ch.EffectsRemainder[3] -= byte(dmg) // byte wrap, no clamp
	}
	ch.HPCur = int8(int(ch.HPCur) - dmg)
	ch.DamageTaken = byte(dmg)
	if ch.HPCur <= -10 {
		ch.HPCur = -10
		ch.EffectsRemainder[3] = 0
		ch.EffectFlags &^= 4
		ch.Flags &= 1
		e.playSound(0x16)
	} else {
		e.playSound(0x15)
	}
	if int8(ch.EffectsRemainder[0]) > 0 {
		v := int(int8(ch.EffectsRemainder[0] - byte(dmg)))
		if v <= 0 {
			ch.EffectsRemainder[0] = 0
			e.spellEffectEnds(c)
		} else {
			ch.EffectsRemainder[0] = byte(v)
		}
	}
	if e.GuiMode == 0 {
		e.drawCharPanel(c)
	} else {
		e.drawPortrait(c)
	}
	if ch.HPCur <= -10 {
		e.checkPartyDead()
	}
	e.setCharEventTimer(c, 0x12, 6, true)
}

// healChar ports S0de8_0adb.
func (e *Engine) healChar(c, v int) {
	if !e.charStatus(c, 3) {
		return
	}
	ch := &e.Chars[c]
	n := int(ch.HPCur) + v
	if n > int(ch.HPMax) {
		n = int(ch.HPMax)
	}
	ch.HPCur = int8(n)
	e.drawHP(c)
	e.drawCharPanel(c)
}

// increasePartyExperience ports S0de8_0c2b.
func (e *Engine) increasePartyExperience(v int) {
	n := 0
	for c := 0; c < 6; c++ {
		if e.charStatus(c, 3) {
			n++
		}
	}
	if n == 0 {
		return
	}
	v /= n
	for c := 0; c < 6; c++ {
		if e.charStatus(c, 3) {
			e.addCharExp(c, v)
		}
	}
}

// classPart ports S0a72_0509.
func (e *Engine) classPart(class, i int) int { return int(int8(e.X.U8(dsClassParts + class*4 + i))) }

// expForLevel ports S0a72_04b7.
func (e *Engine) expForLevel(class, part, level int) int {
	p := e.classPart(class, part)
	if p == -1 {
		return -1
	}
	off := int(e.X.U16(dsExpTables + p*4))
	o := off + level*4 - 4
	return int(e.X.U16(o)) | int(e.X.U16(o+2))<<16
}

// addCharExp ports S0de8_0b45.
func (e *Engine) addCharExp(c, v int) {
	ch := &e.Chars[c]
	cl := int(ch.Class)
	v /= e.u8(dsNumClasses + cl)
	for i := 0; i < 3; i++ {
		if e.classPart(cl, i) == -1 {
			continue
		}
		ch.Experience[i] += uint32(v)
		need := e.expForLevel(cl, i, int(ch.Level[i])+1)
		if need != -1 && int(ch.Experience[i]) > need {
			e.levelUp(c, cl, i)
		}
	}
}

// hpGain ports S0a72_01ae.
func (e *Engine) hpGain(class, con, a, b, c int) int {
	con--
	r := e.rollDice(a, e.u8(dsHPDice0+class), 0) + e.rollDice(b, e.u8(dsHPDice1+class), 0) + e.rollDice(c, e.u8(dsHPDice2+class), 0)
	cb := int(e.X.I8(dsConHPBonus + con)) // signed: -3..-1 for low CON (jle at S0a72_021c)
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

// levelUp ports S0de8_0c82.
func (e *Engine) levelUp(c, cl, part int) {
	ch := &e.Chars[c]
	ch.Level[part]++
	var g int
	switch part {
	case 0:
		g = e.hpGain(cl, int(ch.ConCur), 1, 0, 0)
	case 1:
		g = e.hpGain(cl, int(ch.ConCur), 0, 1, 0)
	case 2:
		g = e.hpGain(cl, int(ch.ConCur), 0, 0, 1)
	}
	ch.HPCur += int8(g)
	ch.HPMax += int8(g)
	msg := sprintfC(e.X.Str(dsStrLevelUp), ch.NameStr())
	e.drawCharPanel(c)
	e.printMessage(msg)
	e.playSound(0x17)
}

// savingThrowValue ports S0a72_0006.
func (e *Engine) savingThrowValue(group, level, typ int) int {
	off := int(e.X.U16(dsSaveTables + group*4))
	if mx := e.u8(dsSaveMaxLvl + group); mx < level {
		level = mx
	}
	idx := level/e.u8(dsSaveLvlDiv+group) + e.u8(dsSaveTypeMul+group)*typ
	return int(e.X.I8(off + idx))
}

// savingThrow ports S0d1c_059b. Returns true if the save succeeded.
func (e *Engine) savingThrow(c, typ int) bool {
	ch := &e.Chars[c]
	g := e.classPart(int(ch.Class), 0)
	if g == 4 || g == 5 {
		g = 0
	}
	t := e.savingThrowValue(g, int(ch.Level[0]), typ)
	race := int(ch.RaceSex) // shifted below as in the original
	di := race << 1
	if (di == 3 || di == 5) && (typ == 4 || typ == 0 || typ == 1) {
		t -= e.u8(dsConSaveBonus + int(ch.ConCur))
	}
	if di == 4 && (typ == 4 || typ == 1) {
		t -= e.u8(dsConSaveBonus + int(ch.ConCur))
	}
	return e.rollDice(1, 20, 0) >= t
}

// savingThrowEffect ports S0de8_101a.
func (e *Engine) savingThrowEffect(c, bit int, name string, saveType, dur, ev, unique int) {
	ch := &e.Chars[c]
	if int(ch.Flags)&bit != 0 && unique != 0 {
		return
	}
	if e.savingThrow(c, saveType) {
		return
	}
	ch.Flags |= byte(bit)
	e.setCharEventTimer(c, int(int16(dur*0x222)), ev, true) // 16-bit imul; original always passes unique=1
	e.printMessage(sprintfC(e.X.Str(dsStrIsEffect), ch.NameStr(), name))
}

// checkPartyDead ports S0de8_093b.
// checkPartyDead ports S082c_0b43 (as seqDelay+seqWaitAny).
func (e *Engine) checkPartyDead() {
	for c := 0; c < 6; c++ {
		if e.charStatus(c, 5) {
			return
		}
	}
	e.Scr.DrawBox3D(0, 0, 0x79, 0x13f, 0xc8, e.col.BoxHi, e.col.BoxLo, int(e.col.BoxFill))
	e.setDim(9)
	e.printMessage(e.X.Str(dsStrPartyDead))
	e.seqDelay(0x12) // S082c_0b43(1): one second, then any key
	e.seqWaitAny()
	e.flushInput()
	e.gameOver = true
	e.quit = true
}

// setCharEventTimer ports S1414_05ed.
func (e *Engine) setCharEventTimer(c, ticks, ev int, unique bool) {
	ch := &e.Chars[c]
	t := &e.Timers[6+c]
	due := e.ticks + uint32(ticks)
	if !t.Enabled {
		t.Interval = uint32(ticks)
		ch.Timers[0] = due
		ch.Events[0] = int8(ev)
		t.Enabled = true
		t.Next = due
		return
	}
	if t.Next > due {
		t.Next = due
	}
	if !unique {
		for i := 0; i < 10; i++ {
			if ch.Timers[i] == 0 {
				ch.Timers[i] = due
				ch.Events[i] = int8(ev)
				return
			}
		}
		return
	}
	di := -1
	for i := 0; i < 10; i++ {
		if di == -1 && ch.Timers[i] == 0 {
			di = i
		}
		if int(ch.Events[i]) == ev {
			di = i
			break
		}
	}
	if di >= 0 {
		ch.Timers[di] = due
		ch.Events[di] = int8(ev)
		return
	}
	// Original bug: no slot -> writes Timers[-1] (= Inventory[25..26], the
	// ring slots) and Events[-1] (= top byte of Timers[9]).
	ch.Inventory[25], ch.Inventory[26] = uint16(due), uint16(due>>16)
	ch.Timers[9] = ch.Timers[9]&0x00ffffff | uint32(byte(ev))<<24
}

// clearCharEvent ports S1414_0c2d.
func (e *Engine) clearCharEvent(c, ev int) {
	ch := &e.Chars[c]
	for i := 0; i < 10; i++ {
		if int(ch.Events[i]) == ev {
			ch.Events[i] = 0
			ch.Timers[i] = 0
		}
	}
	e.rebuildCharTimers()
}

// resetHand ports S1414_0770.
func (e *Engine) resetHand(c, mode, hand int) {
	ch := &e.Chars[c]
	if mode == 0 || mode == 2 {
		ch.DisabledSlots ^= 1 << hand
	}
	if mode != 2 {
		ch.SlotStatus[hand] = 0
	} else {
		ch.SlotStatus[hand] = 0
	}
	e.drawCharPanel(c)
}

// timerCharacter ports S1414_07ca.
func (e *Engine) timerCharacter(c int) {
	ch := &e.Chars[c]
	for i := 0; i < 10; i++ {
		due := ch.Timers[i]
		if due == 0 || due > e.ticks {
			continue
		}
		ch.Timers[i] = 0
		ev := int(ch.Events[i])
		if ev < 0 {
			e.spellExpires(-ev, c)
			continue
		}
		switch ev {
		case 0, 1:
			e.resetHand(c, 0, ev)
		case 2, 3:
			e.resetHand(c, 1, ev-2)
			d := 0x24
			if ch.EffectFlags&8 != 0 {
				d /= 2
			}
			e.setCharEventTimer(c, d, ev+2, true)
		case 4, 5:
			e.resetHand(c, 2, ev-4)
		case 6:
			ch.DamageTaken = 0
			e.drawCharPanel(c)
		case 7:
			e.printMessage(sprintfC(e.X.Str(dsStrStrRestore), ch.NameStr()))
			ch.StrCur = ch.StrMax
			ch.StrExtCur = ch.StrExtMax
			if e.GuiMode == 2 {
				e.drawInvPanel(c)
			}
		case 8:
			if ch.Flags&2 != 0 {
				if ch.EffectFlags&0x80 == 0 {
					e.damageChar(c, 5)
					e.printMessage(sprintfC(e.X.Str(dsStrPoisonTick), ch.NameStr()))
				}
				e.setCharEventTimer(c, 0x222, 8, true)
			} else {
				ch.Flags &^= 2
				e.drawCharPanel(c)
			}
		case 9:
			if ch.Flags&4 != 0 {
				e.printMessage(sprintfC(e.X.Str(dsStrParaEnd), ch.NameStr()))
				ch.Flags &^= 4
				e.drawCharPanel(c)
			}
		}
	}
	min := uint32(0xffffffff)
	for i := 0; i < 10; i++ {
		if ch.Timers[i] != 0 && ch.Timers[i] < min {
			min = ch.Timers[i]
		}
	}
	t := &e.Timers[6+c]
	if min != 0xffffffff {
		t.Interval = min - e.ticks
		t.Next = min
	} else {
		t.Interval = min
		e.enableTimer(6+c, false)
	}
}

// useItem ports S0bd2_1001 (right click on a hand / inventory use).
func (e *Engine) useItem(c, hand int) {
	if !e.charStatus(c, 0xd) {
		return
	}
	ch := &e.Chars[c]
	it := int(ch.Inventory[hand])
	h := hand // ports S0bd2_1244 (slot > 1 -> -1)
	if h > 1 {
		h = -1
	}
	if h != -1 {
		if !e.canUseHand(c, h) {
			return
		}
		if e.GuiMode == 0 && ch.DisabledSlots&(1<<h) != 0 {
			return
		}
	}
	if !e.classCanUse(c, it) {
		e.printMessage(ch.NameStr())
		e.printMessage(e.X.Str(dsStrCantUse))
		return
	}
	if it == 0 && h == -1 {
		return
	}
	typ := int(e.Items[it].Type)
	if p := e.ItemTypes[typ].ExtraProps & 0x7f; p != 5 && p != 6 { // opening the spell menu is free; castSpell spends the turn
		e.spendTurn()
	}
	switch e.ItemTypes[typ].ExtraProps & 0x7f {
	case 0, 0x10:
		e.printMessage(e.X.Str(0x3a15))
	case 1, 2, 3:
		if e.GuiMode == 0 {
			e.attackWithItem(c, hand, it)
		}
	case 5, 6:
		if e.GuiMode == 0 {
			e.openSpellbook(c, e.ItemTypes[typ].ExtraProps&0x7f == 6)
		}
	case 7:
		e.eatFood(c)
	case 4, 8, 0xc, 0xd, 0xf:
		e.printMessage(e.X.Str(0x3a3e))
	case 9:
		if e.GuiMode == 0 {
			e.castFromItem(c, int(e.Items[it].Value), h)
		}
	case 0xa:
		if e.GuiMode == 0 {
			e.castFromItem(c, int(e.Items[it].Value)+0x19, h)
		}
	case 0xb:
		e.readTextItem(int(e.Items[it].Value))
	case 0xe:
		e.drinkPotion(c, hand)
	case 0x12:
		if e.GuiMode == 0 {
			e.useWand(c, hand)
		}
	}
}

func (e *Engine) waitKey() { e.waitTicks(40) }
