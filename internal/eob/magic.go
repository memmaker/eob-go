package eob

import "github.com/felix/goeob/internal/gfx"

// Magic system: spell table, casting, spell effects and missiles
// (port of segment S0ef3 and helpers). All values come from EOB.EXE.

const (
	dsSpells        = 0x1344 // 53 x 19 bytes
	spellSize       = 0x13
	dsMissileDefs   = 0x56e7 // [spell, range, flags, shape] x 12
	dsTouchCanCast  = 0x5717 // [char] front rank flag
	dsBurnShape     = 0x571b
	dsBurnX         = 0x5721
	dsBurnY         = 0x572d
	dsIceNeighbours = 0x5733
	dsACSlots       = 0x553e
	dsAreaPick      = 0x32bb
	dsConeBlocks    = 0x5763 // far ptr per direction to 7 block offsets
	dsDexAC         = 0x349e
	dsClassMask     = 0x1231
	dsStrHighAC     = 0x5830
	dsStrDispelled  = 0x5859
	dsStrBlessed    = 0x587a
	dsStrAidFails   = 0x5899
	dsStrNoEffect1  = 0x58af
	dsStrNoEffect2  = 0x58bb
	dsStrNoEffect3  = 0x58c6
	dsStrDisint     = 0x58d1
	dsStrDeathSpell = 0x58ed
	dsStrExpires    = 0x57d9
	dsStrFrontRank  = 0x5809
	dsStrMissed     = 0x57f1
	dsStrCasts      = 0x599d
	dsStrFreeHand   = 0x5943
	dsStrTwoSpells  = 0x596e
	dsStrOk         = 0x59ab
)

// Spell is an original spell definition.
type Spell struct {
	Name   string
	Flags  byte
	Cast   uint32
	Hit    uint32
	Expire uint32
	Sound  byte
	Flags2 byte
}

func (e *Engine) loadSpells() {
	e.Spells = make([]Spell, 53)
	for i := range e.Spells {
		o := dsSpells + i*spellSize
		x := e.X
		s := &e.Spells[i]
		if n := int(x.U16(o)); n != 0 {
			s.Name = x.Str(n)
		}
		s.Flags = x.U8(o + 4)
		s.Cast = uint32(x.U16(o+7))<<16 | uint32(x.U16(o+5))
		s.Hit = uint32(x.U16(o+0xb))<<16 | uint32(x.U16(o+9))
		s.Expire = uint32(x.U16(o+0xf))<<16 | uint32(x.U16(o+0xd))
		s.Sound = x.U8(o + 0x11)
		s.Flags2 = x.U8(o + 0x12)
	}
	sp := func(off int) uint32 { return cbAddr(0x0ef3, off) }
	// Inline per-spell callbacks (ds:1344 table): castMissile ports S0ef3_10d2 S0ef3_12c2
	// (also ports) S0ef3_13fd S0ef3_144a S0ef3_150f S0ef3_1609 S0ef3_17b7 S0ef3_1a0f S0ef3_2011;
	// empty casts ports S0ef3_1315 S0ef3_18d3 S0ef3_1fc8 S0ef3_1fcd; healChar casts ports
	// (also ports) S0ef3_1b20 S0ef3_1ecc S0ef3_1fd2; touchAttack casts ports S0ef3_1b43 S0ef3_1ef3 S0ef3_1ff9.
	e.spellCast = map[uint32]func(){
		sp(0x0f0b): e.spArmor, sp(0x0f99): e.spBurningHands, sp(0x1069): e.spDetectMagic,
		sp(0x10d2): func() { e.castMissile(0) }, sp(0x1122): e.spShield, sp(0x1197): e.spShockingGrasp,
		sp(0x127f): e.spInvisibility, sp(0x12c2): func() { e.castMissile(1) }, sp(0x1315): func() {},
		sp(0x131a): e.spDispel, sp(0x13fd): func() { e.castMissile(2) }, sp(0x144a): func() { e.castMissile(3) },
		sp(0x1497): e.spHaste, sp(0x150f): func() { e.castMissile(4) }, sp(0x15d0): e.spInvis10,
		sp(0x1609): func() { e.castMissile(5) }, sp(0x1656): e.spVampiricTouch, sp(0x1702): e.spFear,
		sp(0x17b7): func() { e.castMissile(6) }, sp(0x1865): e.spStoneskin, sp(0x18d3): func() {},
		sp(0x18d8): e.spConeOfCold, sp(0x1a0f): func() { e.castMissile(7) }, sp(0x1ad0): e.spBless,
		sp(0x1b20): func() { e.healChar(int(e.spellTarget), e.rollDice(1, 8, 0)) },
		sp(0x1b43): func() { e.touchAttack(1, 8, 0) }, sp(0x1b5a): e.spProtEvil, sp(0x1ba3): e.spAid,
		sp(0x1d38): e.spFlameBlade, sp(0x1c75): e.spSlowPoison, sp(0x1dc0): e.spCreateFood,
		sp(0x1ddf): e.spVestment, sp(0x1e3e): e.spPrayer, sp(0x1e7e): e.spRemoveParalysis,
		sp(0x1ecc): func() { e.healChar(int(e.spellTarget), e.rollDice(2, 8, 1)) },
		sp(0x1ef3): func() { e.touchAttack(2, 8, 1) }, sp(0x1f0b): e.spNeutralizePoison,
		sp(0x1f33): e.spProtEvil10, sp(0x1fc8): func() {}, sp(0x1fcd): func() {},
		sp(0x1fd2): func() { e.healChar(int(e.spellTarget), e.rollDice(3, 8, 3)) },
		sp(0x1ff9): func() { e.touchAttack(3, 8, 3) }, sp(0x2011): func() { e.castMissile(8) },
		sp(0x2050): e.spRaiseDead, sp(0x21bc): e.spLayOnHands,
	}
	// spellAreaHit wrappers ports S0ef3_10f2 S0ef3_12e3 S0ef3_141e S0ef3_146b S0ef3_162a
	// (also ports) S0ef3_2032 S0ef3_208a.
	e.spellHit = map[uint32]func(o *FlyingObject, blk int) bool{
		sp(0x10f2): func(o *FlyingObject, b int) bool {
			return e.spellAreaHit(o, 1, 4, 1, (e.mageLevel(int(int8(o.U11)))-1)>>1)
		},
		sp(0x12e3): func(o *FlyingObject, b int) bool { return e.spellAreaHit(o, 2, 4, 0, e.mageLevel(int(int8(o.U11)))/3) },
		sp(0x141e): func(o *FlyingObject, b int) bool { return e.spellAreaHit(o, 1, 6, 0, e.mageLevel(int(int8(o.U11)))) },
		sp(0x146b): func(o *FlyingObject, b int) bool { return e.spellAreaHit(o, 5, 6, 0, e.mageLevel(int(int8(o.U11)))) },
		sp(0x1530): func(o *FlyingObject, b int) bool { return e.holdMonsters(o, 1) },
		sp(0x162a): func(o *FlyingObject, b int) bool { return e.spellAreaHit(o, 1, 6, 0, e.mageLevel(int(int8(o.U11)))) },
		sp(0x17d8): e.iceStormHit,
		sp(0x1a30): func(o *FlyingObject, b int) bool { return e.holdMonsters(o, 2) },
		sp(0x2032): func(o *FlyingObject, b int) bool { return e.spellAreaHit(o, 6, 8, 0, 0) },
		sp(0x208a): func(o *FlyingObject, b int) bool { return e.spellAreaHit(o, 0, 0, 0xc, 1) },
		sp(0x20a8): e.disintegrateHit,
		sp(0x2145): e.deathSpellHit,
	}
	// Expire callbacks ports S0ef3_0f86 S0ef3_10ae S0ef3_1176 S0ef3_14f9 S0ef3_16dc
	// (also ports) S0ef3_1894 S0ef3_1b13 S0ef3_1b8d S0ef3_1c45 S0ef3_1cd2 S0ef3_1e6c S0ef3_1f8f.
	e.spellEnd = map[uint32]func(c int){
		sp(0x0f86): func(c int) { e.recalcArmorClass(c) },
		sp(0x10ae): func(c int) { e.Vars.Raw[10] &^= 1; e.refreshCursor() },
		sp(0x1176): func(c int) { e.Chars[c].EffectFlags &^= 2; e.recalcArmorClass(c) },
		sp(0x1233): e.removeMagicHandItems,
		sp(0x12a1): e.endInvisibility,
		sp(0x14f9): func(c int) { e.Chars[c].EffectFlags &^= 8 },
		sp(0x16dc): func(c int) {
			ch := &e.Chars[c]
			if ch.HPCur > ch.HPMax {
				ch.HPCur = ch.HPMax
			}
			e.removeMagicHandItems(c)
		},
		sp(0x1894): func(c int) { e.Chars[c].EffectsRemainder[1] = 0 },
		sp(0x1b13): func(c int) { e.Vars.Raw[10] &^= 0x10 },
		sp(0x1b8d): func(c int) { e.Chars[c].EffectFlags &^= 0x200 },
		sp(0x1c45): func(c int) {
			ch := &e.Chars[c]
			if ch.EffectsRemainder[3] != 0 {
				ch.HPCur -= int8(ch.EffectsRemainder[3])
				ch.EffectsRemainder[3] = 0
			}
			ch.EffectFlags &^= 0x40
		},
		sp(0x1cd2): func(c int) { e.Chars[c].EffectFlags &^= 0x80 },
		sp(0x1e1d): func(c int) { e.Chars[c].EffectFlags &^= 0x100; e.recalcArmorClass(int(e.spellCasterChar)) }, // ports S0ef3_1e1d
		sp(0x1e6c): func(c int) { e.Vars.Raw[10] &^= 0x20; e.DrawPlayfield() },
		sp(0x1f8f): func(c int) {
			for i := 0; i < 6; i++ {
				e.Chars[i].EffectFlags &^= 0x200
			}
			e.DrawPlayfield()
		},
	}
}

// mageLevel ports S0ef3_0498.
func (e *Engine) mageLevel(c int) int {
	if e.castFromItemFlag {
		return 9
	}
	if c == -1 {
		if e.CurrentLevel < 7 {
			return 5
		}
		return 9
	}
	p := e.classPartIndex(int(e.Chars[c].Class), 1)
	if p > -1 {
		return int(e.Chars[c].Level[p])
	}
	return 1
}

// clericLevel ports S0ef3_04f7.
func (e *Engine) clericLevel(c int) int {
	if e.castFromItemFlag {
		return 9
	}
	if c == -1 {
		if e.CurrentLevel < 7 {
			return 5
		}
		return 9
	}
	ch := &e.Chars[c]
	if p := e.classPartIndex(int(ch.Class), 2); p > -1 {
		return int(ch.Level[p])
	}
	if p := e.classPartIndex(int(ch.Class), 4); p > -1 && ch.Level[p] > 8 {
		return int(ch.Level[p]) - 8
	}
	return 1
}

// classPartIndex ports S0a72_007b: index (0..2) of the class part whose
// id equals typ, or -1.
func (e *Engine) classPartIndex(class, typ int) int {
	for i := 0; i < 3; i++ {
		if e.classPart(class, i) == typ {
			return i
		}
	}
	return -1
}

// setSpellTimer ports S0ef3_0302: ticks = (x+z)*y for the caster.
func (e *Engine) setSpellTimer(spell, x, y, z int, unique bool) {
	e.setCharEventTimer(int(e.spellTarget), x*y+z*y, -spell, unique)
}

// castMissile ports the cast callbacks that call S0ef3_097b.
func (e *Engine) castMissile(m int) {
	e.launchSpellObject(int(e.spellCasterChar), m, e.CurrentBlock, int(e.spellStartPos), e.CurrentDirection)
}

// launchSpellObject ports S0ef3_097b.
func (e *Engine) launchSpellObject(c, m, block, pos, dir int) {
	slot := -1
	for i := range e.FlyingObjects {
		if e.FlyingObjects[i].Enable == 0 {
			slot = i
			break
		}
	}
	if slot < 0 {
		return
	}
	o := &e.FlyingObjects[slot]
	d := dsMissileDefs + m*4
	sp := e.u8(d)
	*o = FlyingObject{Enable: 2, U2: 1, Flags: e.X.U8(d + 2), Direction: byte(dir), Range: e.X.U8(d + 1),
		CurBlock: uint16(block), CurPos: byte(pos), Item: uint16(m), ShapeIndex: e.X.U8(d + 3), U11: byte(c),
		Callback: e.Spells[sp].Hit}
	e.sceneUpdate = true
}

func (e *Engine) spellObjectHit(o *FlyingObject, nb, sp int) bool {
	if h, ok := e.spellHit[o.Callback]; ok {
		return h(o, nb)
	}
	return false
}

// spellAreaHit ports S0ef3_0a3f.
func (e *Engine) spellAreaHit(o *FlyingObject, times, pips, mod, mult int) bool {
	f := o.Flags
	hitParty := f&0x10 != 0
	f1, f2, auto, save := f&1 != 0, f&2 != 0, f&4 != 0, f&8 != 0
	if mult <= 0 {
		mult = 1
	}
	hit := false
	caster := int(int8(o.U11))
	blk := int(o.CurBlock)
	if e.Blocks[blk].Flags&7 != 0 && (caster >= 0 || hitParty) {
		for _, mi := range e.monsterList(blk, int(o.CurPos), int(o.Direction), f2, f1) {
			d := int(int8(e.rollDice(times, pips, mod) * mult)) // kept in a signed byte
			if !e.rollToHit(caster, mi, 0, false) && !auto {
				continue
			}
			if save {
				d = e.monsterSaveHalf(&e.Monsters[mi], d)
			}
			hit = true
			if !e.monsterMagicResist(mi) {
				e.damageMonster(&e.Monsters[mi], d)
			}
		}
	} else if blk == e.CurrentBlock && (caster < 0 || hitParty) {
		if f2 {
			for c := 0; c < 6; c++ {
				if e.Chars[c].Flags&1 == 0 {
					continue
				}
				if !e.monsterRollHit(&e.Monsters[0], c) && !auto {
					continue
				}
				d := int(int8(e.rollDice(times, pips, mod) * mult))
				if save {
					d = int(int8(e.charSaveHalf(c, d)))
				}
				hit = true
				e.damageChar(c, d)
			}
		} else {
			sp := e.u8(dsDscItemPosIndex + e.CurrentDirection*4 + int(o.CurPos&3))
			c := sp
			if sp > 2 && (e.charStatus(5, 1) || e.charStatus(6, 1)) && e.rollDice(1, 2, -1) != 0 {
				c = sp + 2
			}
			if o.Item == 0 && e.Chars[c%6].EffectFlags&2 != 0 {
				hit = true
			} else if e.Chars[c%6].Flags&1 != 0 {
				if e.monsterRollHit(&e.Monsters[0], c%6) || auto {
					d := int(int8(e.rollDice(times, pips, mod) * mult))
					if save {
						d = int(int8(e.charSaveHalf(c%6, d)))
					}
					hit = true
					e.damageChar(c%6, d)
				}
			}
		}
	}
	if hit && o.Flags&0x40 != 0 {
		e.spellObjectExplode(o)
	} else if o.Item == 5 {
		hit = false
	}
	return hit
}

// charSaveHalf ports S0de8_0f98 (including the original's comparison against
// the 0/1 result of the saving throw).
func (e *Engine) charSaveHalf(c, d int) int {
	s := 0
	if e.savingThrow(c, 4) {
		s = 1
	}
	if e.rollDice(1, 20, 0) >= s {
		d >>= 1
	}
	return d
}

// monsterSaveHalf ports S0de8_0fca.
func (e *Engine) monsterSaveHalf(m *Monster, d int) int {
	if e.rollDice(1, 20, 0) >= e.savingThrowValue(0, int(int8(e.mprop(m, 2))), 4) {
		d >>= 1
	}
	return d
}

// monsterList ports S08e6_17fd (monsters hit at a block / sub position).
func (e *Engine) monsterList(block, pos, dir int, all, center bool) []int {
	var r []int
	if all {
		for i := range e.Monsters {
			if int(e.Monsters[i].Block) == block {
				r = append(r, i)
			}
		}
		return r
	}
	if center {
		if v := e.monsterAtPos(block, pos, dir); v >= 0 {
			r = append(r, v)
		}
		return r
	}
	for i := range e.Monsters {
		m := &e.Monsters[i]
		if int(m.Block) == block && (int(m.Pos) == pos || m.Pos == 4) {
			r = append(r, i)
			break
		}
	}
	return r
}

// monsterAtPos ports S08e6_1775: the monster exactly at pos, else the one
// whose sub-position ranks first in the ds:32c3 priority list.
func (e *Engine) monsterAtPos(block, pos, dir int) int {
	best, rank := -1, 5
	tab := 0x32c3 + dir*16 + pos*4
	for i := range e.Monsters {
		m := &e.Monsters[i]
		if int(m.Block) != block {
			continue
		}
		if int(m.Pos) == pos {
			return i
		}
		for j := 0; j < 4; j++ {
			if int(m.Pos) == e.u8(tab+j) && j < rank {
				rank, best = j, i
			}
		}
	}
	return best
}

// holdMonsters ports S0ef3_1530 / S0ef3_1a30.
func (e *Engine) holdMonsters(o *FlyingObject, typeBit int) bool {
	r := false
	for _, mi := range e.monsterList(int(o.CurBlock), int(o.CurPos), int(o.Direction), true, true) {
		if e.monsterMagicResist(mi) {
			continue
		}
		m := &e.Monsters[mi]
		r = true
		if int(e.X.U16(dsMonsterProps+int(m.Type)*monsterPropSize+0xf))&typeBit != 0 {
			m.Mode = 10
			m.SpellStatusLeft = 0xf
		}
	}
	return r
}

// iceStormHit ports S0ef3_17d8.
func (e *Engine) iceStormHit(o *FlyingObject, b int) bool {
	lvl := e.mageLevel(int(int8(o.U11)))
	r := e.spellAreaHit(o, 1, 6, 0, lvl)
	if r {
		for i := 0; i < 4; i++ {
			save := o.CurBlock
			o.CurBlock = uint16((int(o.CurBlock) + int(e.X.I8(dsIceNeighbours+i))) & 0x3ff)
			e.spellAreaHit(o, 1, 6, 0, lvl)
			o.CurBlock = save
		}
	}
	return r
}

// disintegrateHit ports S0ef3_20a8.
func (e *Engine) disintegrateHit(o *FlyingObject, b int) bool {
	if int(o.CurBlock) != e.CurrentBlock {
		return false
	}
	si := e.rollDice(1, 6, 0)
	for k := 0; k < 6; k++ {
		if e.charStatus(si, 3) && e.charSaveHalf(si, 1) == 0 { // si==6 probes the phantom char (always fails)
			e.damageChar(si, 100)
			e.printMessage(sprintfC(e.X.Str(dsStrDisint), e.Chars[si].NameStr()))
			break
		}
		si++
		if si >= 6 {
			si = 0
		}
	}
	return true
}

// deathSpellHit ports S0ef3_2145.
func (e *Engine) deathSpellHit(o *FlyingObject, b int) bool {
	if int(o.CurBlock) != e.CurrentBlock {
		return false
	}
	e.printMessage(e.X.Str(dsStrDeathSpell))
	n := e.rollDice(1, 4, 0)
	for c := 0; c < 6 && n > 0; c++ {
		if e.charStatus(c, 3) && e.Chars[c].Level[0] < 8 {
			e.damageChar(c, 100)
			n--
		}
	}
	return true
}

// touchAttack ports S0ef3_0e1b (cause wounds).
func (e *Engine) touchAttack(times, pips, mod int) {
	c := int(e.spellCasterChar)
	ch := &e.Chars[c]
	if e.u8(dsTouchCanCast+c) == 0 {
		e.printMessageNow(sprintfC(e.X.Str(dsStrFrontRank), ch.NameStr()))
		return
	}
	blk := e.calcNewBlock(e.CurrentBlock, e.CurrentDirection)
	mi := e.pickMonsterTarget(c, blk)
	if mi == -1 {
		e.printMessageNow(sprintfC(e.X.Str(dsStrMissed), ch.NameStr()))
		return
	}
	if !e.rollToHit(c, mi, 0, true) {
		return
	}
	d := e.rollDice(times, pips, mod)
	if !e.monsterMagicResist(mi) {
		e.damageMonster(&e.Monsters[mi], d)
	}
}

// printMessageNow ports S082c_0b8f: message plus the 0x4f "fail" sound.
func (e *Engine) printMessageNow(s string) {
	e.printMessage(s)
	e.playSound(0x4f)
}

// spArmor ports S0ef3_0f0b.
func (e *Engine) spArmor() {
	ch := &e.Chars[e.spellTarget]
	dex := int(e.X.I16(dsDexAC + int(ch.DexCur)*2))
	if int(ch.AC) > dex+6 {
		ch.EffectsRemainder[0] = byte(e.mageLevel(int(e.spellCasterChar)) + 8)
		e.recalcArmorClass(int(e.spellTarget))
	} else {
		e.printMessageNow(sprintfC(e.X.Str(dsStrHighAC), ch.NameStr()))
	}
}

// spBurningHands ports S0ef3_0f99.
func (e *Engine) spBurningHands() {
	for i := 0; i < 6; i++ {
		k, fl := int(e.X.I8(dsBurnShape+i)), 0
		if k < 0 {
			k, fl = -k, gfx.DrawFlip
		}
		if k >= 1 && k <= len(e.MiscShapes) { // shape ptrs ds:fb34+k*4 = MiscShapes[k-1]
			e.Scr.DrawShape(0, e.MiscShapes[k-1], int(e.X.I16(dsBurnX+i*2)), int(e.X.I8(dsBurnY+i)), 0, fl)
		}
	}
	e.seqDelay(2)
	lvl := e.mageLevel(int(e.spellCasterChar))
	blk := e.calcNewBlock(e.CurrentBlock, e.CurrentDirection)
	e.areaDamageFront(blk, 1, 3, lvl*2, 1)
	e.sceneUpdate = true
}

// areaDamageFront ports S08e6_16d6.
func (e *Engine) areaDamageFront(block, times, pips, mod, save int) {
	l := e.monstersInBlock(block)
	e.batchHits = true
	if l[4] != -1 {
		e.areaDamageMonster(l[4], times, pips, mod, save)
	}
	for i := 0; i < 2; i++ {
		p := e.u8(dsAreaPick + e.CurrentDirection*2 + i)
		if l[p] != -1 {
			e.areaDamageMonster(l[p], times, pips, mod, save)
		}
	}
	e.flushMonsterHits()
}

// areaDamageMonster ports S08e6_1634.
func (e *Engine) areaDamageMonster(mi, times, pips, mod, save int) {
	m := &e.Monsters[mi]
	d := e.rollDice(times, pips, mod)
	// Hit dice byte is signed (0xff = under 1 HD -> level -1).
	if save != 0 && e.rollDice(1, 20, 0) >= e.savingThrowValue(0, int(int8(e.mprop(m, 2))), 4) {
		if save == 1 {
			d >>= 1
		} else {
			d = 0
		}
	}
	if !e.monsterMagicResist(mi) {
		e.damageMonster(m, d)
	}
}

// spDetectMagic ports S0ef3_1069.
func (e *Engine) spDetectMagic() {
	e.Vars.Raw[10] |= 1
	e.refreshCursor()
	e.DrawPlayfield()
	e.setSpellTimer(int(e.curSpell), 0, 0x222, 2, true)
}

// spShield ports S0ef3_1122.
func (e *Engine) spShield() {
	c := int(e.spellCasterChar)
	e.Chars[c].EffectFlags |= 2
	e.recalcArmorClass(c)
	e.setSpellTimer(int(e.curSpell), 0, 0x222, 5, true)
}

// magicHandSlot ports S0ef3_0482 (called with the target char ds:f51c).
func (e *Engine) magicHandSlot() int {
	if e.Chars[e.spellTarget].Inventory[1] == 0 {
		return 1
	}
	return 0
}

// spShockingGrasp ports S0ef3_1197.
func (e *Engine) spShockingGrasp() {
	c := int(e.spellCasterChar)
	h := e.magicHandSlot()
	t := e.allocTempItemType(0, 0, 0, 0xf, 1, 8, e.mageLevel(c), 1)
	it := e.allocTempItem(0x10, 0x52, 0, t)
	e.Chars[e.spellTarget].Inventory[h] = uint16(it)
	e.drawCharPanel(c)
	e.setSpellTimer(int(e.curSpell), 0, 0x222, 1, true)
}

// spVampiricTouch ports S0ef3_1656.
func (e *Engine) spVampiricTouch() {
	c := int(e.spellCasterChar)
	h := e.magicHandSlot()
	t := e.allocTempItemType(0, 0, 0, 0xf, e.mageLevel(c)>>1, 6, 0, 1)
	it := e.allocTempItem(0x18, 0x53, 0, t)
	e.Chars[e.spellTarget].Inventory[h] = uint16(it)
	e.drawCharPanel(c)
	e.setSpellTimer(int(e.curSpell), 0, 0x222, 1, true)
}

// spFlameBlade ports S0ef3_1d38.
func (e *Engine) spFlameBlade() {
	c := int(e.spellCasterChar)
	h := e.magicHandSlot()
	t := e.allocTempItemType(0, 0, 0, 0xf, 1, 4, 4, 1)
	it := e.allocTempItem(0, 0x54, 0, t)
	e.Chars[e.spellTarget].Inventory[h] = uint16(it)
	e.drawCharPanel(c)
	e.setSpellTimer(int(e.curSpell), 4, 0x222, e.clericLevel(c)>>1, true)
}

// removeMagicHandItems ports S0ef3_1233.
func (e *Engine) removeMagicHandItems(c int) {
	ch := &e.Chars[c]
	for i := 0; i < 2; i++ {
		it := int(ch.Inventory[i])
		if it >= 0xb && it <= 0x11 {
			e.freeTempItem(it)
			ch.Inventory[i] = 0
			break
		}
	}
	e.drawCharPanel(int(e.spellTargetFromTimer))
}

// allocTempItemType ports S0ef3_035a: temporary item types 0x33..0x38.
func (e *Engine) allocTempItemType(invFlags, handFlags, ac, allowed, dice, pips, inc, extra int) int {
	si := 0x33
	for ; si < 0x39; si++ {
		if byte(e.ItemTypes[si].ArmorClass) == 0xe2 {
			break
		}
	}
	t := &e.ItemTypes[si%len(e.ItemTypes)]
	t.InvFlags = uint16(invFlags)
	t.RequiredHands = 0 // es:[bx+6]
	t.HandFlags = uint16(handFlags)
	t.ArmorClass = int8(ac)
	t.AllowedClasses = byte(allowed)
	t.DmgDiceS, t.DmgDiceL = byte(dice), byte(dice)
	t.DmgPipsS, t.DmgPipsL = byte(pips), byte(pips)
	t.DmgIncS, t.DmgIncL = int8(inc), int8(inc)
	t.ExtraProps = uint16(extra)
	return si
}

// freeTempItem ports S0ef3_03d8.
func (e *Engine) freeTempItem(it int) {
	e.Items[it].Block = -2
	e.Items[it].Level = 0xff
	e.ItemTypes[e.Items[it].Type].ArmorClass = int8(-30)
}

// allocTempItem ports S0ef3_0418: items 11..16 are reserved for magic.
func (e *Engine) allocTempItem(name, icon, value, typ int) int {
	si := 0xb
	for ; si < 0x11; si++ {
		if e.Items[si].Block == -2 {
			break
		}
	}
	it := &e.Items[si]
	it.Flags = byte(name + 0x20)
	it.Icon = int8(icon)
	it.Type = int8(typ)
	it.Value = int8(value)
	// Original bug: clears byte ds:8dd5 (Items[0].Pos), not this item's Pos.
	e.Items[0].Pos = 0
	it.Block, it.NameUnid, it.NameID, it.Next, it.Prev = 0, 0, 0, 0, 0
	return si
}

// spInvisibility ports S0ef3_127f.
func (e *Engine) spInvisibility() {
	e.Chars[e.spellTarget].EffectFlags |= 4
	e.drawCharPanel(int(e.spellTarget))
}

// spInvis10 ports S0ef3_15d0.
func (e *Engine) spInvis10() {
	for c := 0; c < 6; c++ {
		e.Chars[c].EffectFlags |= 4
	}
	e.DrawPlayfield()
}

// spDispel ports S0ef3_131a.
func (e *Engine) spDispel() {
	c := int(e.spellTarget)
	ch := &e.Chars[c]
	e.printMessageNow(sprintfC(e.X.Str(dsStrDispelled), ch.NameStr()))
	for i := 0; i < 10; i++ {
		ev := int(ch.Events[i])
		if ev < 0 {
			if f, ok := e.spellEnd[e.Spells[-ev].Expire]; ok {
				f(c)
			}
			ch.Events[i] = 0
			ch.Timers[i] = 0
		}
		if ch.EffectFlags&4 != 0 {
			e.spellEnd[cbAddr(0x0ef3, 0x12a1)](c)
		}
		if ch.EffectsRemainder[0] > 0 {
			e.recalcArmorClass(c)
		}
	}
	ch.EffectsRemainder[0] = 0
	ch.EffectFlags = 0
	e.drawCharPanel(c)
}

// spHaste ports S0ef3_1497.
func (e *Engine) spHaste() {
	for c := 0; c < 6; c++ {
		if e.Chars[c].Flags&1 == 0 {
			continue
		}
		e.Chars[c].EffectFlags |= 8
		e.spellTarget = int8(c)
		e.setSpellTimer(int(e.curSpell), 3, 0x222, 1, true)
	}
}

// spFear ports S0ef3_1702.
func (e *Engine) spFear() {
	e.wandSparkle()
	blk := e.calcNewBlock(e.CurrentBlock, e.CurrentDirection)
	for i := range e.Monsters {
		m := &e.Monsters[i]
		if int(m.Block) != blk {
			continue
		}
		if int(e.X.U16(dsMonsterProps+int(m.Type)*monsterPropSize+0xf))&3 == 0 {
			continue
		}
		if e.monsterMagicResist(i) {
			continue
		}
		if e.rollDice(1, 20, 0) < e.savingThrowValue(0, int(int8(e.mprop(m, 2))), 4) {
			e.monsterFlee(m)
		}
	}
}

// monsterFlee ports S08e6_000b.
func (e *Engine) monsterFlee(m *Monster) {
	if m.Mode == 7 || m.Mode == 8 || m.Mode == 10 {
		return
	}
	m.Mode = 0
	m.Dir = int8((e.blockDirection8(int(m.Block), e.CurrentBlock) ^ 4) >> 1)
	m.Flags |= 8
}

// spStoneskin ports S0ef3_1865.
func (e *Engine) spStoneskin() {
	e.Chars[e.spellTarget].EffectsRemainder[1] = byte(e.rollDice(1, 4, 0) + e.mageLevel(int(e.spellCasterChar))>>1)
}

// charProtected ports S0ef3_18a2 (stoneskin absorbs an attack).
func (e *Engine) charProtected(c int) bool {
	ch := &e.Chars[c]
	if ch.EffectsRemainder[1] == 0 {
		return false
	}
	ch.EffectsRemainder[1]--
	if ch.EffectsRemainder[1] == 0 {
		ch.EffectsRemainder[1] = 0
	}
	return true
}

// spConeOfCold ports S0ef3_18d8.
func (e *Engine) spConeOfCold() {
	lvl := e.mageLevel(int(e.spellCasterChar))
	e.Scr.FillRect(scenePage, 0, 0, 0xb0, 0x78, 0)
	e.drawSceneShapes(7)
	e.coneFx(0x96, 0x32, 0xa, 0x64, dsConeColours)
	tab := int(e.X.U16(dsConeBlocks + e.CurrentDirection*4))
	e.batchHits = true
	for i := 0; i < 7; i++ {
		b := (e.CurrentBlock + int(e.X.I8(tab+i))) & 0x3ff
		for _, mi := range e.monsterList(b, 4, e.CurrentDirection, true, true) {
			d := e.rollDice(lvl, 4, lvl)
			if !e.monsterMagicResist(mi) {
				e.damageMonster(&e.Monsters[mi], d)
			}
		}
	}
	e.flushMonsterHits()
}

// spBless ports S0ef3_1ad0.
func (e *Engine) spBless() {
	if e.Vars.Raw[10]&0x10 == 0 {
		e.setSpellTimer(int(e.curSpell), 0, 0x222, 6, true)
		e.Vars.Raw[10] |= 0x10
		e.DrawPlayfield()
	} else {
		e.printMessageNow(e.X.Str(dsStrBlessed))
	}
}

// spProtEvil ports S0ef3_1b5a.
func (e *Engine) spProtEvil() {
	e.Chars[e.spellTarget].EffectFlags |= 0x200
	e.setSpellTimer(int(e.curSpell), 0, 0x222, 3, true)
}

// spAid ports S0ef3_1ba3.
func (e *Engine) spAid() {
	c := int(e.spellTarget)
	if !e.charStatus(c, 3) {
		e.printMessageNow(e.X.Str(dsStrAidFails))
		return
	}
	ch := &e.Chars[c]
	ch.EffectFlags |= 0x40
	v := e.rollDice(1, 8, 0)
	if int(ch.HPCur) < int(int8(byte(int(ch.HPCur)+v))) {
		ch.EffectsRemainder[3] += byte(v)
		ch.HPCur += int8(v)
	}
	e.setSpellTimer(int(e.curSpell), 1, 0x222, 1, true)
}

// spSlowPoison ports S0ef3_1c75.
func (e *Engine) spSlowPoison() {
	c := int(e.spellTarget)
	if e.Chars[c].Flags&2 == 0 {
		e.printMessage(e.X.Str(dsStrNoEffect1))
		return
	}
	e.Chars[c].EffectFlags |= 0x80
	e.setSpellTimer(int(e.curSpell), 1, 0x7ff8, e.clericLevel(int(e.spellCasterChar)), true)
}

// spCreateFood ports S0ef3_1dc0.
func (e *Engine) spCreateFood() {
	for c := 0; c < 6; c++ {
		e.Chars[c].Food = 100
	}
}

// spVestment ports S0ef3_1ddf.
func (e *Engine) spVestment() {
	c := int(e.spellCasterChar)
	e.Chars[e.spellTarget].EffectFlags |= 0x100
	e.recalcArmorClass(c)
	e.setSpellTimer(int(e.curSpell), 0, 0x222, 5, true)
}

// spPrayer ports S0ef3_1e3e.
func (e *Engine) spPrayer() {
	e.Vars.Raw[10] |= 0x20
	e.setSpellTimer(int(e.curSpell), 0, 0x222, 2, true)
	e.DrawPlayfield()
}

// spRemoveParalysis ports S0ef3_1e7e.
func (e *Engine) spRemoveParalysis() {
	n := 4
	for c := 0; c < 6 && n > 0; c++ {
		if e.Chars[c].Flags&4 != 0 {
			e.Chars[c].Flags &^= 4
			n--
		}
	}
	e.DrawPlayfield()
}

// curePoison ports S0ef3_1ce8.
func (e *Engine) curePoison(c int) {
	ch := &e.Chars[c]
	ch.Flags &^= 2
	ch.EffectFlags &^= 0x80
	e.clearCharEvent(c, -34)
	e.drawCharPanel(c)
}

// spNeutralizePoison ports S0ef3_1f0b.
func (e *Engine) spNeutralizePoison() {
	c := int(e.spellTarget)
	if e.Chars[c].Flags != 0 {
		e.curePoison(c)
	} else {
		e.printMessageNow(e.X.Str(dsStrNoEffect2))
	}
}

// spProtEvil10 ports S0ef3_1f33.
func (e *Engine) spProtEvil10() {
	for c := 0; c < 6; c++ {
		e.Chars[c].EffectFlags |= 0x200
	}
	e.setSpellTimer(int(e.curSpell), 0, 0x1554, e.clericLevel(int(e.spellCasterChar)), true)
	e.DrawPlayfield()
}

// spRaiseDead ports S0ef3_2050.
func (e *Engine) spRaiseDead() {
	c := int(e.spellTarget)
	ch := &e.Chars[c]
	if ch.HPCur == -10 && ch.RaceSex>>1 != 1 {
		ch.HPCur = 1
		e.drawCharPanel(c)
	} else {
		e.printMessageNow(e.X.Str(dsStrNoEffect3))
	}
}

// spLayOnHands ports S0ef3_21bc.
func (e *Engine) spLayOnHands() {
	e.healChar(int(e.spellTarget), int(e.Chars[e.spellCasterChar].Level[0])*2)
}

// spellExpires ports S0ef3_0d78.
func (e *Engine) spellExpires(sp, c int) {
	if sp <= 0 || sp >= len(e.Spells) {
		return
	}
	s := &e.Spells[sp]
	e.printMessageNow(sprintfC(e.X.Str(dsStrExpires), e.Chars[c].NameStr(), s.Name))
	e.spellTargetFromTimer = int8(c)
	if f, ok := e.spellEnd[s.Expire]; ok {
		f(c)
	}
	if s.Flags&7 != 0 {
		e.drawCharPanel(c)
	} else if s.Flags&8 != 0 {
		e.DrawPlayfield()
	}
}

func (e *Engine) spellEffectEnds(c int) { e.recalcArmorClass(c) }

// endInvisibility ports S0ef3_12a1 (redraws ds:dcfa, not c, as the original).
func (e *Engine) endInvisibility(c int) {
	e.Chars[c].EffectFlags &^= 4
	e.drawCharPanel(int(e.spellTarget))
}

// refreshCursor is the cursor rebuild in S0ef3_1069 / S0ef3_10ae.
// (S1bf3_00a8(0,0) there only nudges the mouse driver.)
func (e *Engine) refreshCursor() { e.setHandItem(e.heldItem()) }

// recalcArmorClass ports S0de8_0d5e.
func (e *Engine) recalcArmorClass(c int) {
	ch := &e.Chars[c]
	dex := int(e.X.I16(dsDexAC + int(ch.DexCur)*2))
	ac := dex + 10
	for di := 0; di < 4; di++ {
		slot := e.u8(dsACSlots + di)
		it := int(ch.Inventory[slot])
		if it == 0 {
			continue
		}
		if di == 2 && !e.canUseHand(c, 1) {
			continue
		}
		t := int(e.Items[it].Type)
		tt := &e.ItemTypes[t]
		allowed := tt.AllowedClasses & e.X.U8(dsClassMask+int(ch.Class))
		if tt.ExtraProps&0x7f != 0 || allowed == 0 {
			continue
		}
		if di >= 1 && di <= 2 && t != 0x1b {
			continue
		}
		ac += int(tt.ArmorClass)
		ac -= int(e.Items[it].Value)
	}
	if e.Items[ch.Inventory[17]].Value == 0 {
		v1, v2 := 0, 0
		if r := int(ch.Inventory[25]); r != 0 && e.itemType(r).ExtraProps&0x7f == 0 {
			v1 = int(e.Items[r].Value)
		}
		if r := int(ch.Inventory[26]); r != 0 && e.itemType(r).ExtraProps&0x7f == 0 {
			v2 = int(e.Items[r].Value)
		}
		if v2 > v1 {
			v1 = v2
		}
		ac -= v1
	}
	if int8(ch.EffectsRemainder[0]) > 0 {
		if ac > dex+6 {
			ac = dex + 6
		} else {
			ch.EffectsRemainder[0] = 0
		}
	}
	if ch.EffectFlags&2 != 0 && ac > 4 {
		ac = 4
	}
	if ch.EffectFlags&0x100 != 0 {
		cap := 5
		if l := e.clericLevel(c); l > 5 {
			cap += (l - 5) / 3
		}
		if ac > cap {
			ac = cap
		}
	}
	ch.AC = int8(ac)
}
