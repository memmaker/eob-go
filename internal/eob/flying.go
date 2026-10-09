package eob

// Flying objects: thrown items, missiles and spell projectiles
// (ports of S0bd2_07de, S1414_0285, S1414_003e, S1414_0172, S1414_020f,
// S0de8_02d4 and S0de8_01ff).

const dsFlyStep = 0x5cab // [dir*4 + pos]: bit7 = enter next block, bits0-1 new pos

// launchItem ports S0bd2_07de.
func (e *Engine) launchItem(c, item, block, pos, dir, typ int) bool {
	slot := -1
	for i := range e.FlyingObjects {
		if e.FlyingObjects[i].Enable == 0 {
			slot = i
			break
		}
	}
	if slot < 0 {
		return false
	}
	e.linkItemToBlock(block, item, pos|4)
	o := &e.FlyingObjects[slot]
	*o = FlyingObject{Enable: 1, U2: 1, CurBlock: uint16(block), CurPos: byte(pos), Direction: byte(dir),
		Item: uint16(item), ShapeIndex: byte(typ), U11: byte(c), Range: 0xc}
	if typ != 7 {
		e.playSound(0xb)
	} else {
		e.playSound(0x1a)
	}
	return true
}

// moveFlyingObject ports S1414_003e.
func (e *Engine) moveFlyingObject(o *FlyingObject, nb, sp int) bool {
	w := e.Blocks[nb].Walls[o.Direction^2]
	fl := e.WllWallFlags[w]
	if o.Enable == 1 {
		it := int(o.Item)
		small := e.T.ItemShapeMap[byte(e.Items[it].Icon)] >= 0xf
		if fl&1 != 0 || o.U2 != 0 || (fl&2 != 0 && small) {
			e.unlinkItem(int(o.CurBlock), 0, it)
			e.linkItemToBlock(nb, it, sp|4)
			o.CurBlock = uint16(nb)
			o.CurPos = byte(sp)
			o.Range--
			return true
		}
		e.clickFlags = 0x10
	} else {
		if fl&1 != 0 || int(o.CurBlock) == nb {
			o.CurBlock = uint16(nb)
			o.CurPos = byte(sp)
			if o.Range != 0xff {
				o.Range--
			}
			return true
		}
		e.clickFlags = 1
	}
	e.clickedWall(nb, int(o.Direction))
	return false
}

// flyingObjectHit ports S1414_0172.
func (e *Engine) flyingObjectHit(o *FlyingObject, nb, sp int) bool {
	if o.U2 != 0 {
		return false
	}
	if o.Enable == 2 {
		return e.spellObjectHit(o, nb, sp)
	}
	if e.Blocks[nb].Flags&7 != 0 {
		for i := range e.Monsters {
			m := &e.Monsters[i]
			if int(m.Block) == nb && (int(m.Pos) == sp || m.Pos == 4) { // ports S08e6_189d inline
				if e.thrownHitsMonster(o, i) {
					return true
				}
			}
		}
		return false
	}
	if nb == e.CurrentBlock {
		return e.thrownHitsParty(o)
	}
	return false
}

// isThrowOrMissile ports S0de8_01d7.
func (e *Engine) isThrowOrMissile(typ int) bool {
	v := e.ItemTypes[typ].ExtraProps & 0x7f
	return v == 2 || v == 3
}

// thrownHitsMonster ports S0de8_02d4.
func (e *Engine) thrownHitsMonster(o *FlyingObject, mon int) bool {
	it := int(o.Item)
	typ := int(o.ShapeIndex)
	c := int(int8(o.U11))
	if c != -1 && !e.rollToHit(c, mon, it, false) {
		return false
	}
	d := 1
	if e.isThrowOrMissile(typ) {
		d = e.calcDamage(c, mon, it, typ, false)
	}
	e.damageMonster(&e.Monsters[mon], d)
	return true
}

// thrownHitsParty ports S0de8_01ff.
func (e *Engine) thrownHitsParty(o *FlyingObject) bool {
	it := int(o.Item)
	typ := int(o.ShapeIndex)
	si := e.u8(dsDscItemPosIndex + e.CurrentDirection*4 + int(e.Items[it].Pos&3))
	d := 1
	if e.isThrowOrMissile(typ) {
		d = e.calcDamage(-1, -1, it, typ, false)
	}
	di := si
	// original also tests charStatus(6, 1): char 6 is past the array
	// (ds:70d3, never written, always 0), so it is always false.
	if e.charStatus(5, 1) {
		if e.rollDice(1, 2, -1) != 0 {
			di = si + 2
		}
	}
	if di < 6 && e.charStatus(di, 1) {
		e.damageChar(di, d)
		return true
	}
	return false
}

// endFlyingObject ports S1414_020f.
func (e *Engine) endFlyingObject(o *FlyingObject) {
	if o.Enable == 1 {
		e.Items[o.Item].Pos &= 3
		e.runBlockScript(int(o.CurBlock), 4)
		e.playSoundUpdate(0x12)
	}
	*o = FlyingObject{}
}

// timerFlyingObjects ports S1414_0285.
func (e *Engine) timerFlyingObjects(int) {
	for i := range e.FlyingObjects {
		o := &e.FlyingObjects[i]
		if o.Enable == 0 {
			continue
		}
		done := o.Range == 0
		v := e.u8(dsFlyStep + int(o.Direction)*4 + int(o.CurPos&3))
		cross := v&0x80 != 0
		sp := v & 3
		nb := int(o.CurBlock)
		if cross {
			nb = e.calcNewBlock(int(o.CurBlock), int(o.Direction))
			o.U2 = 0
		}
		if e.moveFlyingObject(o, nb, sp) {
			if cross {
				e.runBlockScript(nb, 0x10)
			}
			if e.flyingObjectHit(o, nb, sp) {
				done = true
			}
		} else {
			if o.Flags&0x20 != 0 && !e.flyingObjectHit(o, int(o.CurBlock), int(o.CurPos)) {
				e.spellObjectExplode(o)
			}
			done = true
		}
		if done {
			e.endFlyingObject(o)
		}
		e.sceneUpdate = true
	}
}

// throwFromMouse ports S0acd_0494: throw the item held by the mouse.
func (e *Engine) throwFromMouse(side int) {
	e.spendTurn()
	it := e.HandItem
	if it == 0 {
		return
	}
	pos := e.u8(0x1a1b + e.CurrentDirection*4 + side)
	if e.launchItem(int(e.SelectedChar), it, e.CurrentBlock, pos, e.CurrentDirection, int(e.Items[it].Type)) {
		e.setHandItem(0)
		e.sceneUpdate = true
	}
}
