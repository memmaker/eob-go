package eob

// Monster AI (port of segment S08e6 and helpers).

const (
	dsDir8          = 0x7fbc
	dsMonFreePos    = 0x31f6
	dsMonSubCenter  = 0x31f2
	dsMonPath1      = 0x31fe
	dsMonPath2      = 0x320e
	dsMonModeTurnB  = 0x3234
	dsMonModeTurnA  = 0x3237
	dsMonCanAttack  = 0x323b
	dsMonAtkRow     = 0x324b
	dsMonTargets    = 0x325b
	dsMonSide       = 0x328b
	dsMonPickTarget = 0x329b
	dsStrPoisoned2  = 0x3366
	dsStrParalyzed2 = 0x336f
)

func (e *Engine) mprop(m *Monster, off int) int {
	return int(e.X.U8(dsMonsterProps + int(m.Type)*monsterPropSize + off))
}

// blockDistance ports S17a6_0f61.
func blockDistance(a, b int) int {
	dx := (b & 0x1f) - (a & 0x1f)
	dy := (b >> 5 & 0x1f) - (a >> 5 & 0x1f)
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx < dy {
		dx, dy = dy, dx
	}
	return dx + dy/2
}

// blockDirection8 ports S17a6_0314: octant direction 0..7 from a to b (-1 same).
func (e *Engine) blockDirection8(a, b int) int {
	ay, ax := a/32, a%32
	by, bx := b/32, b%32
	si := 0
	dy := ay - by // positive if b is north
	ady := dy
	if ady < 0 {
		ady = -ady
	}
	dx := bx - ax
	adx := dx
	if adx < 0 {
		adx = -adx
	}
	if dy*2 >= adx {
		si |= 8
	}
	if -dy*2 >= adx {
		si |= 4
	}
	if dx*2 >= ady {
		si |= 2
	}
	if -dx*2 >= ady {
		si |= 1
	}
	return int(e.X.I8(dsDir8 + si))
}

// placeMonster ports S08e6_0297.
func (e *Engine) placeMonster(m *Monster, block, dir int) {
	if block != -1 {
		e.updateBlockIfVisible(int(m.Block))
		if e.Blocks[m.Block].Flags&7 != 0 {
			e.Blocks[m.Block].Flags--
		}
		m.Block = uint16(block)
		e.Blocks[block].Flags++
	}
	if dir != -1 {
		m.Dir = int8(dir)
		block = int(m.Block)
	}
	e.updateBlockIfVisible(block)
}

// initMonster ports S08e6_131d.
func (e *Engine) initMonster(index, unit, block, pos, dir, typ, shp, mode, spell, randItem, fixedItem int) {
	if index < 0 || index >= 30 {
		return
	}
	m := &e.Monsters[index]
	*m = Monster{}
	if block == 0 {
		return
	}
	unit = unit*2 + index&1
	m.Type = byte(typ)
	m.NumRemoteAttacks = byte(e.mprop(m, 0x18))
	m.Unit = byte(unit)
	m.Pos = byte(pos)
	m.ShpIndex = byte(shp)
	m.Mode = int8(mode)
	m.SpellStatusLeft = int8(spell)
	m.Dir = int8(dir)
	var hp int
	if hd := e.mprop(m, 2); hd != 0xff {
		hp = e.rollDice(hd, 8, 0)
	} else {
		hp = e.rollDice(1, 4, 0)
	}
	m.HPMax, m.HPCur = int16(hp), int16(hp)
	m.RandItem, m.FixedItem = uint16(randItem), uint16(fixedItem)
	e.placeMonster(m, block, dir)
}

// createMonster ports S08e6_1505.
func (e *Engine) createMonster(a, blk, p, d, t, sh, mode, spell, ri, fi int) {
	if e.Blocks[blk].Flags&7 != 0 {
		return
	}
	for i := range e.Monsters {
		if e.Monsters[i].HPCur <= 0 {
			e.initMonster(i, a, blk, p, d, t, sh, mode, spell, ri, fi)
			return
		}
	}
}

// teleportMonster ports the monster branch of opcode 0xfa. The original
// passes the monster's sub-position (byte +4) as placeMonster's direction.
func (e *Engine) teleportMonster(i, blk, pos int) {
	e.placeMonster(&e.Monsters[i], blk, pos)
}

// freeSubPositions ports S08e6_017e: bitmask of occupied sub positions
// (-2 if the block is full, -1 if a big monster occupies it).
func (e *Engine) freeSubPositions(block int) int {
	n := int(e.Blocks[block].Flags & 7)
	if n == 4 {
		return -2
	}
	mask := 0
	for i := range e.Monsters {
		m := &e.Monsters[i]
		if int(m.Block) != block {
			continue
		}
		if e.mprop(m, 0x15) != 0 {
			return -1
		}
		if m.Pos == 4 {
			m.Pos = byte(e.u8(dsMonSubCenter + int(m.Dir)))
		}
		mask |= 1 << m.Pos
		n--
		if n == 0 {
			break
		}
	}
	return mask
}

// findFreeSubPos ports S08e6_0218.
func (e *Engine) findFreeSubPos(m *Monster, block int) int {
	if e.mprop(m, 0x15) != 0 {
		return -1
	}
	occ := e.freeSubPositions(block)
	if occ < 0 {
		return -1
	}
	base := dsMonFreePos + (int(m.Dir)&1)*4
	for i := 0; i < 4; i++ {
		p := int(m.Dir) ^ e.u8(base+i)
		if occ&(1<<p) == 0 {
			return p
		}
	}
	return -1
}

// moveMonster ports S08e6_0309.
func (e *Engine) moveMonster(m *Monster, block, dir int) bool {
	old := int(m.Block)
	if int(m.Block) == block || block == -1 {
		goto turn
	}
	if dir == -1 {
		dir = int(m.Dir)
	}
	if e.WllWallFlags[e.Blocks[block].Walls[dir^2]]&4 == 0 {
		return false
	}
	if block == e.CurrentBlock {
		return false
	}
	if e.Blocks[block].Flags&7 != 0 && block != 0 {
		p := e.findFreeSubPos(m, block)
		if p == -1 {
			return false
		}
		m.Pos = byte(p)
	}
	e.placeMonster(m, block, dir)
	dir = -1
turn:
	if dir != -1 {
		m.Dir = int8(dir)
	}
	e.updateBlockIfVisible(old)
	if !e.noMonsterSound {
		e.playSoundAt(int(int8(e.mprop(m, 0x17))), int(m.Block))
	}
	return true
}

// updateMonsterDest ports S08e6_00c3: notice the party.
func (e *Engine) updateMonsterDest(m *Monster) {
	if m.Mode >= 7 && m.Mode <= 10 || m.HPCur <= 0 {
		return
	}
	d := blockDistance(int(m.Block), e.CurrentBlock)
	if d >= 4 {
		return
	}
	s := e.blockDirection8(int(m.Block), e.CurrentBlock) - int(m.Dir)*2 - 3
	if s < 0 {
		s += 8
	}
	if s <= 2 && d >= 2 {
		return
	}
	m.Mode = 0
	m.Dest = uint16(e.CurrentBlock)
}

// updateAllMonsterDests ports S08e6_0150.
func (e *Engine) updateAllMonsterDests() {
	for i := range e.Monsters {
		e.updateMonsterDest(&e.Monsters[i])
	}
}

// monsterAggro ports S08e6_03fe.
func (e *Engine) monsterAggro(m *Monster) {
	if m.Flags&1 == 0 || m.Mode == 10 {
		return
	}
	if m.Mode == 8 {
		e.releaseMode8Monsters()
		return
	}
	m.Mode = 0
	m.Dest = uint16(e.CurrentBlock)
}

// releaseMode8Monsters ports S08e6_0055.
func (e *Engine) releaseMode8Monsters() {
	var last *Monster
	for i := range e.Monsters {
		m := &e.Monsters[i]
		if m.Mode == 8 {
			m.Mode = 0
			m.Dest = uint16(e.CurrentBlock)
			last = m
		}
	}
	if last == nil {
		return
	}
	switch last.Type {
	case 7:
		e.LevelFlags[12] |= 1 << 18
	case 12:
		e.LevelFlags[12] |= 1 << 27
	}
}

// walkMonster ports S08e6_0430: move one step towards dest.
func (e *Engine) walkMonster(m *Monster, dest int) {
	if m.F9 > 6 {
		m.F9 = 0
		e.pathToggle ^= 1
	}
	tab := dsMonPath1
	if e.pathToggle != 0 {
		tab = dsMonPath2
	}
	si := int(m.Dir) * 2
	blk := int(m.Block)
	d := e.blockDirection8(blk, dest)
	if d == -1 {
		return
	}
	if m.Flags&8 != 0 {
		d ^= 4
	}
	di := (d - si) & 7
	if d&1 == 0 && (blk+int(e.X.I8(dsDirBlockOffs+d>>1))) == e.CurrentBlock {
		if di != 0 && di < 5 {
			si = int(m.Dir) + 1
		} else if di != 0 {
			si = int(m.Dir) - 1
		} // di == 0 keeps si = dir*2 (original: 04e5 and si,3)
		e.moveMonster(m, -1, si&3)
		return
	}
	if di != 0 {
		if d&1 != 0 {
			if di < 5 {
				si++
			} else {
				si--
			}
		} else if di < 5 {
			si += 2
		} else {
			si -= 2
		}
		si &= 7
	}
	for k := 7; k > -1; k-- {
		si = (si + int(e.X.I16(tab+k*2))) & 7
		nb := 0
		if si&1 == 0 {
			nb = e.calcNewBlock(blk, si>>1)
		}
		if nb != 0 && e.moveMonster(m, nb, si>>1) {
			return
		}
	}
}

// monsterCanAttack ports S08e6_103d.
func (e *Engine) monsterCanAttack(m *Monster) bool {
	if m.Pos == 4 {
		return true
	}
	return e.u8(dsMonCanAttack+int(m.Dir)*4+int(m.Pos)) != 0
}

// monsterRollHit ports S08e6_106b.
func (e *Engine) monsterRollHit(m *Monster, c int) bool {
	r := e.rollDice(1, 20, 0)
	if r != 20 {
		if e.Chars[c].EffectFlags&0x200 != 0 {
			r -= 2
		}
		if e.Vars.Raw[10]&0x20 != 0 {
			r--
		}
	}
	t := e.thac0Target(e.mprop(m, 1), int(e.Chars[c].AC))
	return r >= t || r == 20
}

// monsterMeleeAttack ports S08e6_10f3.
func (e *Engine) monsterMeleeAttack(m *Monster) {
	row := e.u8(dsMonAtkRow + e.CurrentDirection*4 + int(m.Dir))
	di := row * 12
	var side int
	if m.Pos != 4 {
		side = e.u8(dsMonSide + int(m.Dir)*4 + int(m.Pos))
	} else {
		side = e.rollDice(1, 2, -1)
	}
	if side != 0 {
		di += 6
	}
	target := -1
	for si := di; si < di+6; si++ {
		c := e.u8(dsMonTargets + si)
		ch := &e.Chars[c]
		if ch.Flags&1 == 0 || ch.HPCur <= -10 {
			continue
		}
		if ch.EffectFlags&4 != 0 && e.rollDice(1, 20, 0) >= 5 {
			continue
		}
		target = c
		break
	}
	if target < 0 {
		return
	}
	dmg := 0
	n := e.mprop(m, 3)
	for i := 0; i < n; i++ {
		if !e.monsterRollHit(m, target) {
			continue
		}
		dmg += e.rollDice(e.mprop(m, 4+i*3), e.mprop(m, 5+i*3), int(int8(e.mprop(m, 6+i*3))))
		if e.charProtected(target) {
			dmg = 0
		}
	}
	if dmg <= 0 {
		return
	}
	fl := e.mprop(m, 0x0e)
	if fl&0x80 != 0 {
		e.monsterDestroyItem(target)
	}
	e.damageChar(target, dmg)
	if fl&0x10 != 0 {
		e.savingThrowEffect(target, 2, e.X.Str(dsStrPoisoned2), 0, 1, 8, 1)
		e.Chars[target].EffectFlags &^= 0x80
	}
	if fl&0x20 != 0 {
		e.savingThrowEffect(target, 4, e.X.Str(dsStrParalyzed2), 2, 5, 9, 1)
	}
}

// monsterAttackParty ports S08e6_05ea.
func (e *Engine) monsterAttackParty(m *Monster, blk int) bool {
	if blk == -1 {
		blk = e.calcNewBlock(int(m.Block), int(m.Dir))
	}
	if blk != e.CurrentBlock {
		return false
	}
	if !e.monsterCanAttack(m) {
		e.monsterAdjustPos(m)
		return true
	}
	m.Flags ^= 4
	if m.Flags&4 == 0 {
		return true
	}
	vis := e.blockVisible(int(m.Block))
	snd := int(int8(e.mprop(m, 0x16)))
	if vis {
		if m.Type == 4 {
			e.playSoundUpdate(snd)
		}
		m.CurAttackFrame = -2
		e.redrawSceneNow()
		m.CurAttackFrame = -1
		if m.Type != 4 {
			e.playSoundUpdate(snd)
		}
		e.redrawSceneNow()
	} else {
		e.playSoundUpdate(snd)
	}
	e.monsterMeleeAttack(m)
	if vis {
		m.CurAttackFrame = 0
		m.AnimStep ^= 1
		e.sceneUpdate = true
	}
	return true
}

// monsterAdjustPos ports S08e6_058a.
func (e *Engine) monsterAdjustPos(m *Monster) {
	b := int(m.Block)
	if e.Blocks[b].Flags&7 == 1 {
		m.Pos = 4
	} else if p := e.findFreeSubPos(m, b); p >= 0 {
		m.Pos = byte(p)
	} else {
		return
	}
	e.updateBlockIfVisible(b)
}

// monsterChase ports S08e6_0ab8 (mode 0).
func (e *Engine) monsterChase(m *Monster) {
	nb := e.calcNewBlock(int(m.Block), int(m.Dir))
	if e.monsterRangedAttack(m) {
		return
	}
	if e.monsterAttackParty(m, nb) {
		return
	}
	m.CurAttackFrame = 0
	e.walkMonster(m, int(m.Dest))
	if e.mprop(m, 0x0e)&8 != 0 {
		e.monsterAttackParty(m, -1)
	}
}

// monsterWalkTurn ports S08e6_0b34 (modes 1-3).
func (e *Engine) monsterWalkTurn(m *Monster, turn int) {
	nb := e.calcNewBlock(int(m.Block), int(m.Dir))
	if !e.moveMonster(m, nb, -1) {
		m.Dir = int8((int(m.Dir) + turn) & 3)
		e.moveMonster(m, -1, int(m.Dir))
	}
}

// monsterFollowWall ports S08e6_0b94 (modes 5,6).
func (e *Engine) monsterFollowWall(m *Monster, turn int) {
	if m.F9 >= 0 {
		if m.F9 == 0 {
			e.monsterWalkTurn(m, -turn)
		}
		si := (int(m.Dir) + turn) & 3
		nb := e.calcNewBlock(int(m.Block), si)
		w := e.Blocks[nb].Walls[e.T.BlockMap[si]]
		di := e.WllWallFlags[w] & 4
		if m.F9 == 0 {
			if di == 0 {
				m.F9 = -1
			}
			return
		}
		if di != 0 {
			e.moveMonster(m, -1, si)
			m.F9 = -1
			return
		}
	}
	nb := e.calcNewBlock(int(m.Block), int(m.Dir))
	if !e.moveMonster(m, nb, -1) {
		e.moveMonster(m, -1, (int(m.Dir)-turn)&3)
		m.F9 = 0
	} else {
		m.F9 = 1
	}
}

// updateMonsters ports S08e6_0cc8 (monster group timer).
func (e *Engine) updateMonsters(unit int) {
	for i := range e.Monsters {
		m := &e.Monsters[i]
		if int(m.Unit) != unit || m.HPCur <= 0 {
			continue
		}
		e.updateMonsterDest(m)
		if m.Mode > 0 {
			e.monsterAggro(m)
		}
		switch m.Mode {
		case 0:
			e.monsterChase(m)
		case 1, 2, 3:
			e.monsterWalkTurn(m, int(e.X.I8(dsMonModeTurnA+int(m.Mode))))
		case 5, 6:
			e.monsterFollowWall(m, int(e.X.I8(dsMonModeTurnB+int(m.Mode))))
		case 7, 10: // ports S08e6_0ca6 (spell status countdown)
			if m.SpellStatusLeft != 0 {
				m.SpellStatusLeft--
				if m.SpellStatusLeft == 0 {
					m.Mode = 0
				}
			}
		}
		if m.Mode != 4 && m.Mode != 7 {
			m.AnimStep ^= 1
		}
	}
	e.flyingObjectsHitParty()
}

// damageMonster ports S08e6_0fbe.
func (e *Engine) damageMonster(m *Monster, dmg int) {
	e.monsterWasHit = true
	m.HPCur -= int16(dmg)
	m.Flags &^= 8
	vis := false
	if !e.batchHits {
		e.monsterHitFlash(m)
	} else if vis = e.blockVisible(int(m.Block)); vis {
		m.Flags |= 2
	}
	m.Flags |= 1
	if m.HPCur <= 0 && (!e.batchHits || !vis) {
		e.killMonster(m)
	}
}

// monsterHitFlash ports S08e6_0dca.
func (e *Engine) monsterHitFlash(m *Monster) {
	e.blockVisible(int(m.Block))
	if !e.sceneUpdate { // original tests ds:df4c, which may already be set
		return
	}
	m.Flags |= 2
	e.redrawSceneNow() // no delay in the original
	m.Flags &^= 2
	e.sceneUpdate = true
}

// flushMonsterHits ports S08e6_0e48 (after area attacks).
func (e *Engine) flushMonsterHits() {
	e.redrawSceneNow()
	any := false
	for i := range e.Monsters {
		m := &e.Monsters[i]
		if m.Flags&2 == 0 {
			continue
		}
		m.Flags &^= 2
		any = true
		if m.HPCur <= 0 {
			e.killMonster(m)
		}
	}
	e.sceneUpdate = any // original assigns, so no hit clears it
	e.monsterWasHit = false
	e.batchHits = false
}

// killMonster ports S08e6_0ec3.
func (e *Engine) killMonster(m *Monster) {
	pos := int(m.Pos)
	if pos == 4 {
		pos = e.rollDice(1, 4, -1)
	}
	if m.RandItem != 0 && e.rollDice(1, 2, 0) == 1 {
		if it := e.duplicateItem(int(m.RandItem)); it != 0 {
			e.linkItemToBlock(int(m.Block), it, pos)
		}
	}
	if m.FixedItem != 0 {
		if it := e.duplicateItem(int(m.FixedItem)); it != 0 {
			e.linkItemToBlock(int(m.Block), it, pos)
		}
	}
	e.increasePartyExperience(int(e.X.U16(dsMonsterProps + int(m.Type)*monsterPropSize + 0x13)))
	e.placeMonster(m, 0, -1)
	if m.Type == 0x15 {
		e.runFinale()
	}
	if m.Mode == 8 {
		e.releaseMode8Monsters()
	}
}

// monstersInBlock ports S08e6_155f (by sub position, -1 = none).
func (e *Engine) monstersInBlock(block int) [6]int {
	var r [6]int
	for i := range r {
		r[i] = -1
	}
	for i := range e.Monsters {
		if int(e.Monsters[i].Block) == block {
			r[e.Monsters[i].Pos] = i
		}
	}
	return r
}

// pickMonsterTarget ports S08e6_15b4.
func (e *Engine) pickMonsterTarget(c, block int) int {
	l := e.monstersInBlock(block)
	if l[4] != -1 {
		return l[4]
	}
	o := dsMonPickTarget + e.CurrentDirection*8 + (c&1)*4
	for i := 0; i < 4; i++ {
		if v := l[e.u8(o+i)]; v != -1 {
			return v
		}
	}
	return -1
}

// scriptTimersAndRespawn ports S08e6_1410 (called after the timer loop).
func (e *Engine) updateMonstersPost() {
	mode := e.ScriptTimersMode
	stepDue := e.StepCounter >= int(int16(e.StepsUntilScript))
	fire := false
	switch mode {
	case 1:
		fire = e.scriptTimerNext < e.ticks
	case 2:
		fire = stepDue
	case 3:
		// Original bug (S08e6_1454): mode 3 fires while the timer is NOT yet due.
		fire = stepDue || e.scriptTimerNext >= e.ticks
	}
	if !fire {
		return
	}
	e.runBlockScript(0, 0x20)
	dead := 0
	for i := 1; i < 30; i++ {
		if e.Monsters[i].HPCur <= 0 {
			dead++
		}
	}
	if dead == 0 {
		for i := 1; i < 30; i++ {
			if blockDistance(int(e.Monsters[i].Block), e.CurrentBlock) > 3 {
				e.killMonster(&e.Monsters[i])
				break
			}
		}
	}
	e.scriptTimerNext = e.ticks + uint32(int32(int16(e.ScriptTimerTicks)))
	e.StepCounter = 0
}

// monsterIdleJitter ports S1414_0ce7.
func (e *Engine) monsterIdleJitter(int) {
	for i := range e.Monsters {
		m := &e.Monsters[i]
		if e.rollDice(1, 2, 0) != 1 {
			continue
		}
		v := byte(e.rollDice(1, 2, 0)) << 4
		v += byte(e.rollDice(1, 2, 0))
		m.IdleAnimState = v
		e.updateBlockIfVisible(int(m.Block))
	}
}

// blockVisible ports S0d1c_0006: block among the first 15 view blocks
// (ds:a9c8, the party's own row 15-17 excluded); requests a redraw if so.
func (e *Engine) blockVisible(block int) bool {
	for i := 0; i < 15; i++ {
		if (e.CurrentBlock+int(e.T.BlockIndex[e.CurrentDirection*18+i]))&0x3ff == block {
			e.sceneUpdate = true
			return true
		}
	}
	return false
}

// redrawSceneNow draws and presents immediately (used by attack animations).
func (e *Engine) redrawSceneNow() {
	e.DrawScene(true)
	e.present()
}

// waitTicks delays while keeping the display alive.
func (e *Engine) waitTicks(n int) {
	e.Sys.Sleep(n * tickMillis)
}

// monsterRangedAttack ports S08e6_0765: missiles, thrown items and spells.
// CurRemoteWeapon (+0x18) is the cooldown, NumRemoteAttacks (+0x19) the
// remaining attacks (0xff = unlimited).
func (e *Engine) monsterRangedAttack(m *Monster) bool {
	if e.mprop(m, 0xe)&0x40 == 0 || m.NumRemoteAttacks == 0 {
		return false
	}
	if m.CurRemoteWeapon != 0 {
		m.CurRemoteWeapon--
		return false
	}
	mb := int(m.Block)
	if blockDistance(mb, e.CurrentBlock) > 3 || e.blockDirection8(mb, e.CurrentBlock) != int(m.Dir)*2 {
		return false
	}
	d := int(m.Dir)
	for b := e.calcNewBlock(mb, d); b != e.CurrentBlock; b = e.calcNewBlock(b, d) {
		if e.WllWallFlags[e.Blocks[b].Walls[d^2]]&3 == 0 || e.Blocks[b].Flags&7 != 0 {
			return false
		}
	}
	missile := func(sp int) { e.launchSpellObject(-1, sp, mb, int(m.Pos), d) } // ports S08e6_073f
	switch m.Type {
	case 4:
		missile(9)
		e.playSoundAt(0x1f, mb)
	case 14:
		n := int(m.NumRemoteAttacks)
		e.playSoundAt(int(e.u8(0x322f+n)), mb)
		missile(int(e.u8(0x3226 + n)))
	case 15:
		it := e.duplicateItem(0x3c)
		if it != 0 && !e.launchItem(-1, it, mb, int(m.Pos), d, int(e.Items[it].Type)) {
			e.Items[it].Block = -1
		}
	case 16:
		e.launchSpellObject(-1, 0, mb, int(m.Pos), d)
		e.playSoundAt(0x55, mb)
	case 17:
		e.playSoundAt(0x53, mb)
		e.printMessage(e.X.Str(0x332f))
		for c := range e.Chars {
			if e.charStatus(c, 3) {
				e.savingThrowEffect(c, 4, e.X.Str(0x335c), 1, 5, 9, 1)
				e.drawCharPanel(c)
			}
		}
	case 21:
		r := e.rollDice(1, 4, -1)
		if r < 3 {
			missile(int(e.u8(0x3220 + r)))
			e.playSoundAt(int(e.u8(0x3223+r)), mb)
			break
		}
		for c := range e.Chars {
			if e.charStatus(c, 3) {
				e.printMessage(sprintfC(e.X.Str(0x3303), e.Chars[c].NameStr()))
				e.damageChar(c, e.rollDice(2, 8, 1))
				break
			}
		}
		e.playSoundAt(0x6c, mb)
	}
	e.DrawScene(true)
	e.sceneUpdate = false
	if m.NumRemoteAttacks != 0xff {
		m.NumRemoteAttacks--
	}
	m.CurRemoteWeapon = 5
	return true
}

// monsterDestroyItem ports S0bd2_000e: destroys one random fragile item.
func (e *Engine) monsterDestroyItem(c int) {
	ch := &e.Chars[c]
	slot := e.rollDice(1, 27, -1)
	for n := 0; n < 27; n++ {
		it := int(ch.Inventory[slot])
		if it != 0 && e.ItemTypes[e.Items[it].Type].ExtraProps&0x80 != 0 {
			ch.Inventory[slot] = 0
			f := 0x3912
			if ch.RaceSex&1 != 0 { // female
				f = 0x3901
			}
			e.printMessage(sprintfC(e.X.Str(f), ch.NameStr()))
			e.printMessage(e.itemName(it))
			e.printMessage(e.X.Str(0x3923))
			break
		}
		if slot++; slot >= 27 {
			slot = 0
		}
	}
	e.drawCharPanel(c)
}
