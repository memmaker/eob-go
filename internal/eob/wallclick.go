package eob

// clickedSceneCenter ports S122e_0933: click on the wall/door in front.
func (e *Engine) clickedSceneCenter() {
	e.clickFlags = 0x40
	blk := e.calcNewBlock(e.CurrentBlock, e.CurrentDirection)
	e.clickedWall(blk, e.CurrentDirection)
}

// clickedWall ports S122e_0896.
func (e *Engine) clickedWall(block, dir int) bool {
	e.spendTurn()
	side := dir ^ 2
	w := e.Blocks[block].Walls[side]
	t := e.SpecialWallTypes[w]
	if t == 0 {
		return false
	}
	mask := int(e.Blocks[block].Flags&0xf8)>>3 | 0xe0
	if e.clickFlags&mask == 0 {
		return false
	}
	var r bool
	switch t {
	case 1:
		r = e.clickedDoorSwitch(block, side)
	case 2:
		r = e.clickedWallShape(block, side)
	case 3:
		r = e.clickedLever(block, side, 1)
	case 4:
		r = e.clickedLever(block, side, -1)
	case 5:
		r = e.clickedForceDoor(block, side)
	case 6:
		r = e.clickedDoorPry(block, side)
	case 10:
		r = e.clickedNiche(block, side)
	}
	e.clickFlags = 0
	e.sceneUpdate = true
	return r
}

// mouseIn ports S082c_0a5d.
func (e *Engine) mouseIn(x1, y1, x2, y2 int) bool {
	mx, my := e.mouseX, e.mouseY
	return mx >= x1 && mx <= x2 && my >= y1 && my <= y2
}

// clickedDoorSwitch ports S122e_04ca.
func (e *Engine) clickedDoorSwitch(block, side int) bool {
	w := int(e.vis(13).Walls[e.scene.drawVarDown])
	si := e.u8(0x59f8 + w)
	sw := e.DoorSwitches[si%6]
	x := sw.X + int(e.X.I16(0x1e75)) - 4
	y := sw.Y - 4
	var w2, h2 int
	if sw.Shp != nil {
		w2, h2 = sw.Shp.W()+8, sw.Shp.H+8
	}
	if e.CurrentLevel >= 4 && e.CurrentLevel <= 6 {
		if w < 0x1e {
			x += (w - e.u8(0x59d8+w)) * 9
		} else {
			x += 4
		}
	}
	if e.mouseIn(x, y, x+w2, y+h2) || e.clickFlags != 0x40 {
		e.toggleDoor(block, side)
		e.playSound(6)
		return true
	}
	return e.clickedDoorPry(block, side)
}

// clickedWallShape ports S122e_05df.
func (e *Engine) clickedWallShape(block, side int) bool {
	if !e.decorationHit(int(e.WllShapeMap[e.Blocks[block].Walls[side]])) {
		return false
	}
	e.runBlockScript(block, e.clickFlags)
	return true
}

// clickedLever ports S122e_0620 / S122e_0678.
func (e *Engine) clickedLever(block, side, d int) bool {
	if !e.decorationHit(int(e.WllShapeMap[e.Blocks[block].Walls[side]])) {
		return false
	}
	e.Blocks[block].Walls[side] = byte(int(e.Blocks[block].Walls[side]) + d)
	e.sceneUpdate = true
	e.runBlockScript(block, e.clickFlags)
	return true
}

// clickedDoorPry ports S122e_0847.
func (e *Engine) clickedDoorPry(block, side int) bool {
	if e.mouseIn(0x28, 0x10, 0x88, 0x58) && e.WllWallFlags[e.Blocks[block].Walls[side]]&0x20 != 0 {
		e.printMessage(e.X.Str(0x5b78))
		return true
	}
	return false
}

// decorationHit ports S122e_0055.
func (e *Engine) decorationHit(l int) bool {
	if e.clickFlags != 0x40 {
		return true
	}
	hit := false
	for l > 0 && !hit {
		l--
		if l >= len(e.DecoProps) {
			break
		}
		d := &e.DecoProps[l]
		x := int(d.ShapeX[1]) - 3
		y := int(d.ShapeY[1]) - 3
		s := e.DecoShapes[d.ShapeIndex[1]]
		w, h := 6, 6
		if s != nil {
			w, h = s.W()+6, s.H+6
		}
		if d.Flags&1 != 0 {
			x = 0xb0 - x - w
		}
		hit = e.mouseIn(x, y, x+w, y+h)
		l = int(d.Next)
	}
	return hit
}

// clickedForceDoor ports S122e_0325. The strongest conscious character
// tries; the original sprintf()s "%s forces the door!" / "Nobody is able
// to force the door." but never prints them, so neither do we.
func (e *Engine) clickedForceDoor(block, side int) bool {
	if !e.mouseIn(0x28, 0x10, 0x88, 0x58) && e.clickFlags == 0x40 {
		return false
	}
	si := -1
	for c := range e.Chars {
		ch := &e.Chars[c]
		if ch.Flags&1 == 0 || ch.HPCur <= 0 {
			continue
		}
		// original compares str[si]+ext[si] against str[c]+ext[si]
		if si < 0 || ch.StrCur >= e.Chars[si].StrCur {
			si = c
		}
	}
	if si == -1 {
		return true
	}
	if e.rollDice(1, 20, 0) > int(int16(e.X.U16(0x59b0+int(e.Chars[si].StrCur)*2))) {
		e.printMessage(e.X.Str(0x5b0b))
		return true
	}
	e.printMessage(e.X.Str(0x5ada))
	w := byte(0x12)
	if e.Blocks[block].Walls[side] == 0x1e {
		w = 8
	}
	e.Blocks[block].Walls[side] = w
	e.Blocks[block].Walls[side^2] = w
	e.openDoor(block)
	return true
}

// clickedNiche ports S122e_06d0 (mouse path; the keyboard-mode quiver
// branch is not ported).
func (e *Engine) clickedNiche(block, side int) bool {
	if !e.decorationHit(int(e.WllShapeMap[e.Blocks[block].Walls[side]])) {
		return false
	}
	b := &e.Blocks[block]
	if e.HandItem == 0 {
		if it := e.unlinkListItem(&b.ItemHead, 8, -1); it != 0 {
			e.runBlockScript(block, 8)
			e.setHandItem(it)
			e.sceneUpdate = true
		}
		return true
	}
	if e.T.ItemShapeMap[byte(e.Items[e.HandItem].Icon)] <= 0xe {
		e.printMessage(e.X.Str(0x5b59))
		return true
	}
	e.linkItem(&b.ItemHead, block, e.HandItem, 8)
	e.runBlockScript(block, 4)
	e.setHandItem(0)
	e.sceneUpdate = true
	return true
}
