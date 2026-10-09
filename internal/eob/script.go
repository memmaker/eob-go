package eob

// Level script interpreter (port of S122e_1da8 and its 26 opcode handlers).

type scriptState struct {
	block     int
	flags     int
	abort     bool
	stack     []int // subroutine return offsets (max 10)
	evalStack []int
	flagFB76  bool
}

// Level flags live at ds:fbd4+4*level; the global flags word ds:fc04 is the
// same storage as level 12's slot (faithful to the original layout).
func (e *Engine) levelFlag(bit int) bool  { return e.LevelFlags[e.CurrentLevel]&(1<<uint(bit)) != 0 }
func (e *Engine) globalFlag(bit int) bool { return e.LevelFlags[12]&(1<<uint(bit)) != 0 }

// runBlockScript ports S122e_1da8.
func (e *Engine) runBlockScript(block, flags int) {
	b := &e.Blocks[block]
	pos := int(b.Script)
	if pos == 0 {
		return
	}
	mask := int(b.Flags&0xf8)>>3 | 0xe0
	if flags&mask == 0 {
		return
	}
	s := &e.script
	s.flagFB76 = false
	s.block = block
	s.flags = flags
	s.abort = false
	d := e.LevelINF
	for !s.abort && pos < len(d) {
		op := int(int8(d[pos]))
		pos++
		if op >= 0 || op <= -27 {
			continue
		}
		pos = e.scriptOp(op, pos)
	}
}

func (e *Engine) rd8(p *int) int  { v := int(e.LevelINF[*p]); *p++; return v }
func (e *Engine) rds8(p *int) int { v := int(int8(e.LevelINF[*p])); *p++; return v }
func (e *Engine) rd16(p *int) int { v := int(le16(e.LevelINF[*p:])); *p += 2; return v }

func (e *Engine) scriptOp(op, pos int) int {
	s := &e.script
	switch byte(op) {
	case 0xff: // setWallType
		switch e.rds8(&pos) {
		case -9:
			blk, v := e.rd16(&pos), byte(e.rd8(&pos))
			for i := 0; i < 4; i++ {
				e.Blocks[blk].Walls[i] = v
			}
		case -23:
			blk, side, v := e.rd16(&pos), e.rd8(&pos), byte(e.rd8(&pos))
			e.Blocks[blk].Walls[side] = v
		case -19:
			e.CurrentDirection = e.rd8(&pos)
		}
		e.sceneUpdate = true
	case 0xfe: // changeWallType
		switch e.rds8(&pos) {
		case -9:
			blk, a, b := e.rd16(&pos), byte(e.rd8(&pos)), byte(e.rd8(&pos))
			v := a
			if e.Blocks[blk].Walls[0] == a {
				v = b
			}
			for i := 0; i < 4; i++ {
				e.Blocks[blk].Walls[i] = v
			}
		case -22:
			e.toggleDoor(e.rd16(&pos), -1)
		}
		e.sceneUpdate = true
	case 0xfd:
		e.openDoor(e.rd16(&pos))
	case 0xfc:
		e.closeDoor(e.rd16(&pos))
	case 0xfb: // createMonster
		pos++
		a := e.rds8(&pos)
		blk := e.rd16(&pos)
		p, d, t, sh, m, i := e.rds8(&pos), e.rds8(&pos), e.rds8(&pos), e.rds8(&pos), e.rds8(&pos), e.rds8(&pos)
		ri, fi := e.rd16(&pos), e.rd16(&pos)
		e.createMonster(a, blk, p, d, t, sh, m, i, ri, fi)
	case 0xfa: // teleport
		typ := e.rds8(&pos)
		src := 0
		if typ != -15 {
			src = e.rd16(&pos)
		}
		dst := e.rd16(&pos)
		switch typ {
		case -24:
			e.moveParty(dst)
		case -13:
			for i := range e.Monsters {
				if int(e.Monsters[i].Block) == src {
					e.teleportMonster(i, dst, int(e.Monsters[i].Pos))
				}
			}
		case -11:
			for {
				it := e.findBlockItem(src, -1, -1, false)
				if it == 0 {
					break
				}
				pos2 := int(e.Items[it].Pos)
				e.unlinkItem(src, -1, it)
				e.linkItemToBlock(dst, it, pos2)
			}
			for i := range e.FlyingObjects {
				o := &e.FlyingObjects[i]
				if o.Enable == 1 && int(o.CurBlock) == src {
					o.CurBlock = uint16(dst)
				}
			}
		}
		e.sceneUpdate = true
	case 0xf9: // stealSmallItems
		c := e.rds8(&pos)
		blk := e.rd16(&pos)
		ipos := e.rd8(&pos)
		e.scriptStealItem(c, blk, ipos)
	case 0xf8: // message
		start := pos
		for e.LevelINF[pos] != 0 {
			pos++
		}
		text := string(e.LevelINF[start:pos])
		pos++
		fg, bg := e.rd8(&pos), e.rd8(&pos)
		e.printColoredMessage(text, byte(fg), byte(bg))
	case 0xf7: // setFlags
		switch e.rds8(&pos) {
		case -28:
			s.flagFB76 = true
		case -17:
			e.LevelFlags[e.CurrentLevel] |= 1 << uint(e.rd8(&pos))
		case -16:
			e.LevelFlags[12] |= 1 << uint(e.rd8(&pos))
		case -13:
			m := e.rd8(&pos)
			bit := e.rd8(&pos)
			// Unchecked in the original: level 3 names monster 30, whose
			// record would lie past the table in memory nothing else uses.
			if m < len(e.Monsters) {
				e.Monsters[m].Flags |= 1 << uint(bit)
				e.Monsters[m].Mode = 0
			}
		}
	case 0xf6: // playSoundEffect
		id := e.rd8(&pos)
		blk := e.rd16(&pos)
		if blk != 0 {
			e.playSoundAt(id, blk)
		} else {
			e.playSound(id)
		}
	case 0xf5: // removeFlags (always 2 bytes)
		sub, bit := e.rds8(&pos), e.rd8(&pos)
		switch sub {
		case -17:
			e.LevelFlags[e.CurrentLevel] &^= 1 << uint(bit)
		case -16:
			e.LevelFlags[12] &^= 1 << uint(bit)
		}
	case 0xf4: // modifyCharacterHitPoints
		c, v := e.rds8(&pos), e.rds8(&pos)
		if c == -1 {
			for i := 0; i < 6; i++ {
				e.healChar(i, v)
			}
		} else {
			e.healChar(c, v)
		}
	case 0xf3: // calcAndInflictCharacterDamage
		c, a, b, inc := e.rds8(&pos), e.rd8(&pos), e.rd8(&pos), e.rds8(&pos)
		if c == -1 {
			for i := 0; i < 6; i++ {
				d := a
				if b != 0 {
					d = e.rollDice(a, b, inc)
				}
				if e.wearsRing(i, 3) {
					continue
				}
				e.damageChar(i, d)
			}
		} else {
			if b != 0 {
				e.rollDice(a, b, inc) // rolled and discarded
			}
			e.damageChar(c, a) // original passes the unrolled value here
		}
	case 0xf2: // jump
		pos = e.rd16(&pos)
	case 0xf1: // end
		s.abort = true
	case 0xf0: // return
		if n := len(s.stack); n > 0 {
			pos = s.stack[n-1]
			s.stack = s.stack[:n-1]
		}
	case 0xef: // call
		t := e.rd16(&pos)
		if len(s.stack) < 10 {
			s.stack = append(s.stack, pos)
			pos = t
		}
	case 0xee:
		pos = e.scriptEval(pos)
	case 0xed: // deleteItem
		sub := e.rds8(&pos)
		switch sub {
		case -1:
			e.deleteInventoryItem(0, -1)
		case -2:
			e.deleteBlockItems(e.rd16(&pos), -1)
		default:
			e.deleteBlockItems(e.rd16(&pos), sub)
		}
	case 0xec: // loadNewLevel
		pos++
		lvl := e.rd8(&pos)
		blk := e.rd16(&pos)
		dir := e.rds8(&pos)
		e.scriptLoadNewLevel(lvl, blk, dir)
		s.abort = true
	case 0xeb: // increasePartyExperience
		if e.rd8(&pos) == 0xe2 {
			e.increasePartyExperience(e.rd16(&pos))
		}
	case 0xea: // createItem
		tmpl := e.rd16(&pos)
		blk := e.rd16(&pos)
		it := e.duplicateItem(tmpl)
		if it != 0 {
			if blk == 0xffff && e.HandItem == 0 {
				e.setHandItem(it)
			} else {
				e.linkItemToBlock(blk, it, int(e.LevelINF[pos]))
			}
		}
		pos++
	case 0xe9: // launchObject
		isSpell := e.LevelINF[pos] == 0xdf
		pos++
		obj := e.rd8(&pos)
		pos++
		blk := e.rd16(&pos)
		dir := e.rd8(&pos)
		sub := e.rd8(&pos)
		start := e.u8(0x5a40 + dir*2 + sub)
		if !isSpell {
			it := e.duplicateItem(obj)
			if it != 0 && !e.launchItem(-1, it, blk, start, dir, int(e.Items[it].Type)) {
				e.Items[it].Block = -1
			}
		} else {
			e.launchSpellObject(-1, obj, blk, start, dir)
		}
	case 0xe8: // turnParty / turn flying objects
		t, n := e.rds8(&pos), e.rd8(&pos)
		switch t {
		case -15:
			e.CurrentDirection = (e.CurrentDirection + n) & 3
			e.kbdFloorPos = -1
			e.sceneUpdate = true
		case -11:
			for i := range e.FlyingObjects {
				o := &e.FlyingObjects[i]
				if o.Enable != 0 && int(o.CurBlock) == s.block {
					o.Direction = byte((int(o.Direction) + n) & 3)
				}
			}
		}
	case 0xe7: // identifyItems
		blk := e.rd16(&pos)
		if blk == e.CurrentBlock {
			for c := 0; c < 6; c++ {
				ch := &e.Chars[c]
				if ch.Flags&1 == 0 {
					continue
				}
				for i := 0; i < 27; i++ {
					if it := ch.Inventory[i]; it != 0 {
						e.Items[it].Flags |= 0x40
					}
				}
				e.identifyQuiver(c)
			}
		}
		e.identifyBlockItems(blk)
	case 0xe6: // sequence
		e.runSequence(e.rd8(&pos))
	}
	return pos
}

// scriptEval ports S122e_147f.
func (e *Engine) scriptEval(pos int) int {
	st := e.script.evalStack[:0]
	push := func(v int) { st = append(st, v) }
	pop := func() int {
		if len(st) == 0 {
			return 0
		}
		v := st[len(st)-1]
		st = st[:len(st)-1]
		return v
	}
	b2i := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	for {
		op := e.LevelINF[pos]
		pos++
		if op == 0xee {
			break
		}
		switch op {
		case 0xff, 0xfe, 0xfd, 0xfc, 0xfb, 0xfa:
			b := pop()
			a := pop()
			var r bool
			switch op {
			case 0xff:
				r = a == b
			case 0xfe:
				r = a != b
			case 0xfd:
				r = a < b
			case 0xfc:
				r = a <= b
			case 0xfb:
				r = a > b
			case 0xfa:
				r = a >= b
			}
			push(b2i(r))
		case 0xf9:
			b, a := pop(), pop()
			push(b2i(a != 0 && b != 0))
		case 0xf8:
			b, a := pop(), pop()
			push(b2i(a != 0 || b != 0))
		case 0xf7:
			push(int(e.Blocks[e.rd16(&pos)].Walls[0]))
		case 0xf5:
			t := e.rds8(&pos)
			blk := e.rd16(&pos)
			push(e.findBlockItem(blk, -1, t, true))
		case 0xf3:
			push(b2i(e.Blocks[e.rd16(&pos)].Flags&1 != 0))
		case 0xf2:
			it := e.rd16(&pos)
			blk := e.rd16(&pos)
			push(e.findBlockItem(blk, it, -1, false))
		case 0xf1:
			push(b2i(e.CurrentBlock == e.rd16(&pos)))
		case 0xf0:
			push(b2i(e.globalFlag(e.rd8(&pos))))
		case 0xef:
			push(b2i(e.levelFlag(e.rd8(&pos))))
		case 0xed:
			push(e.CurrentDirection)
		case 0xe0:
			push(e.script.flags)
		case 0xe7:
			h := e.HandItem
			sub := e.LevelINF[pos]
			pos++
			switch sub {
			case 0xe1:
				push(int(e.Items[h].Type))
			case 0xf5:
				push(h)
			default:
				push(int(e.Items[h].Value))
			}
		case 0xe9:
			side := e.rd8(&pos)
			push(int(e.Blocks[e.rd16(&pos)].Walls[side]))
		case 0xdd:
			r := e.rd8(&pos)
			f := 0
			for c := 0; c < 6; c++ {
				if e.Chars[c].Flags&1 != 0 && int(e.Chars[c].RaceSex>>1) == r {
					f = 1
					break
				}
			}
			push(f)
		case 0xdc:
			m := e.rd8(&pos)
			f := 0
			for c := 0; c < 6; c++ {
				if e.Chars[c].Flags&1 != 0 && int(e.X.U8(0x1231+int(e.Chars[c].Class)))&m != 0 {
					f = 1
					break
				}
			}
			push(f)
		case 0xdb:
			a, b, c := e.rd8(&pos), e.rd8(&pos), e.rds8(&pos)
			push(e.rollDice(a, b, c))
		case 0xda:
			f := 0
			for c := 0; c < 6; c++ {
				if e.Chars[c].Flags&1 != 0 && e.Chars[c].EffectFlags&4 == 0 {
					f = 1
					break
				}
			}
			push(b2i(f == 0))
		default:
			if op < 0x80 {
				push(int(op))
			}
		}
	}
	e.script.evalStack = st
	if pop() != 0 {
		pos += 2
	} else {
		pos = int(le16(e.LevelINF[pos:]))
	}
	return pos
}
