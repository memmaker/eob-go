package eob

import "fmt"

// rand ports the Borland C runtime LCG used by the game.
func (e *Engine) rand() int {
	e.seed = e.seed*22695477 + 1
	return int(e.seed>>16) & 0x7fff
}

// rollDice ports S082c_0458.
func (e *Engine) rollDice(times, pips, inc int) int {
	r := 0
	if pips > 0 && times > 0 {
		for i := 0; i < times; i++ {
			v := e.rand()*pips/0x7fff + 1
			if v > pips {
				v -= pips
			}
			r += v
		}
	}
	return r + inc
}

// findBlockItem ports S0bd2_0385 for a block's item list.
func (e *Engine) findBlockItem(block, item, typ int, count bool) int {
	return e.findListItem(e.Blocks[block].ItemHead, item, typ, count)
}

// findListItem ports S0bd2_0385: walks an item list (via prev).
func (e *Engine) findListItem(h uint16, item, typ int, count bool) int {
	head := int(h)
	if head == 0 {
		return 0
	}
	n := 0
	si := head
	first := true
	for si != head || first {
		first = false
		it := &e.Items[si]
		if (item == -1 && typ == -1) || (item == -1 && int(it.Type) == typ) || (typ == -1 && si == item) {
			if !count {
				return si
			}
			n++
		}
		si = int(it.Prev)
	}
	if count {
		return n
	}
	return 0
}

// unlinkItem ports S0bd2_02b9 for a block list.
func (e *Engine) unlinkItem(block, pos, item int) int {
	return e.unlinkListItem(&e.Blocks[block].ItemHead, pos, item)
}

// unlinkListItem ports S0bd2_02b9: removes item (or first item at pos if item==-1).
func (e *Engine) unlinkListItem(hp *uint16, pos, item int) int {
	head := int(*hp)
	if head == 0 {
		return 0
	}
	si := head
	first := true
	for si != head || first {
		first = false
		it := &e.Items[si]
		if (item == -1 && int(it.Pos) == pos) || si == item {
			nx, pv := int(it.Next), int(it.Prev)
			e.Items[nx].Prev = int16(pv)
			e.Items[pv].Next = int16(nx)
			it.Prev, it.Next, it.Block, it.Level = 0, 0, 0, 0
			if int(*hp) == si {
				*hp = uint16(pv)
			}
			if int(*hp) == si {
				*hp = 0
			}
			return si
		}
		si = int(it.Prev)
	}
	return 0
}

// linkItemToBlock ports S0bd2_01d0 for a block list.
func (e *Engine) linkItemToBlock(block, item, pos int) {
	e.linkItem(&e.Blocks[block&0x3ff].ItemHead, block, item, pos)
}

// linkItem ports S0bd2_01d0.
func (e *Engine) linkItem(hp *uint16, block, item, pos int) {
	if item == 0 {
		return
	}
	it := &e.Items[item]
	it.Block = int16(block)
	it.Pos = int8(pos)
	if block >= 0 {
		it.Level = byte(e.CurrentLevel)
	} else {
		it.Level = 0xff
	}
	if *hp == 0 {
		*hp = uint16(item)
		it.Next, it.Prev = int16(item), int16(item)
		return
	}
	h := int(*hp)
	di := int(e.Items[h].Next)
	it.Prev = e.Items[di].Prev
	it.Next = int16(di)
	e.Items[di].Prev = int16(item)
	e.Items[h].Next = int16(item)
	*hp = uint16(item)
}

// deleteBlockItems ports S0bd2_04aa.
func (e *Engine) deleteBlockItems(block, typ int) {
	b := &e.Blocks[block]
	si := int(b.ItemHead)
	if si == 0 {
		return
	}
	b.ItemHead = 0
	start := si
	first := true
	for si != start || first {
		first = false
		it := &e.Items[si]
		di := si
		si = int(it.Prev)
		e.Items[di].Prev, e.Items[di].Next = 0, 0
		if int(it.Type) == typ || typ == -1 {
			it.Block = -1
			it.Level = 0
		} else {
			e.linkItemToBlock(block, di, int(it.Pos))
		}
	}
}

// duplicateItem ports S0bd2_089e: copies a template item into a free slot.
func (e *Engine) duplicateItem(tmpl int) int {
	if e.Items[tmpl].Block == -1 {
		return 0
	}
	for i := 1; i < 500; i++ {
		if e.Items[i].Block == -1 {
			e.Items[i] = e.Items[tmpl]
			return i
		}
	}
	return 0
}

// deleteInventoryItem ports S0bd2_0106 (slot -1: item in hand / mouse).
func (e *Engine) deleteInventoryItem(c, slot int) int {
	var it int
	if slot == -1 {
		it = e.heldItem()
	} else {
		it = int(e.Chars[c].Inventory[slot])
	}
	e.Items[it].Block = -1
	if slot == -1 {
		e.setHandItem(0)
	} else {
		e.Chars[c].Inventory[slot] = 0
		if e.GuiMode == 1 {
			e.drawInvSlot(slot, true, 0)
		} else if e.GuiMode == 0 {
			e.drawCharPanel(c)
		}
	}
	return int(e.Items[it].Value)
}

// setHandItem ports S07a9_010c (mouse cursor item).
func (e *Engine) setHandItem(it int) {
	e.setHeldItem(it)
	if it != 0 && e.txtDim == 7 {
		e.printMessage(e.itemName(it))
		e.printMessage(e.X.Str(0x94)) // " taken.\r"
	}
}

// heldItem ports S0d1c_02dc: the mouse item, or the keyboard-selected slot.
func (e *Engine) heldItem() int {
	if !e.KbdMode {
		return e.HandItem
	}
	return int(e.Chars[e.InvChar].Inventory[e.kbdSlot])
}

// setHeldItem ports S0d1c_0304.
func (e *Engine) setHeldItem(it int) {
	if !e.KbdMode {
		e.HandItem = it
		return
	}
	e.Chars[e.InvChar].Inventory[e.kbdSlot] = uint16(it)
	if e.GuiMode == 0 {
		e.drawCharPanel(e.InvChar)
	} else {
		e.drawInvSlot(e.kbdSlot, true, 0)
	}
}

// itemName ports S0bd2_05b5 (which prints the result).
func (e *Engine) itemName(it int) string {
	t := &e.Items[it]
	unid := e.ItemNames[t.NameUnid]
	if t.Flags&0x40 == 0 {
		return unid
	}
	if id := e.ItemNames[t.NameID]; id != "" {
		return id
	}
	v := int(t.Value)
	var pre, suf string
	switch e.ItemTypes[t.Type].ExtraProps & 0x7f {
	case 0, 1, 2, 3:
		switch {
		case v < 0:
			return fmt.Sprintf("Cursed %s %d", unid, v)
		case v > 0:
			return fmt.Sprintf("%s +%d", unid, v)
		}
		return unid
	case 9:
		pre, suf = e.X.Str(0x393a), e.farStr(dsMageNames+v*4)
	case 10:
		pre, suf = e.X.Str(0x3946), e.farStr(dsClericNames+v*4)
	case 16:
		pre, suf = e.X.Str(0x3954), e.nthStr(0x37b4, v)
	case 14:
		pre, suf = e.X.Str(0x3959), e.nthStr(0x37e1, v)
	case 18:
		pre, suf = e.X.Str(0x3960), e.nthStr(0x3837, v)
	default:
		return unid
	}
	return fmt.Sprintf("%s of %s", pre, suf)
}

// nthStr ports S0d1c_07a7: the n-th string of a NUL-separated DS list.
func (e *Engine) nthStr(o, n int) string {
	for ; n > 0; n-- {
		o += len(e.X.Str(o)) + 1
	}
	return e.X.Str(o)
}

// printColoredMessage ports S122e_0fdc's output: the message wrapped in
// the colour-code prefix ds:5a48, then the old colours and ds:5b9f.
func (e *Engine) printColoredMessage(s string, fg, bg byte) {
	b := []byte(e.X.Str(0x5a48))
	of, ob := byte(e.txtFg), byte(e.txtBg)
	b[1], b[3] = fg, bg
	e.printMessage(string(b))
	e.printMessage(s)
	b[1], b[3] = of, ob
	e.printMessage(string(b))
	e.printMessage(e.X.Str(0x5b9f))
}

// playSoundAt ports S177d_0106: plays the previously queued sound, then
// queues id with its distance from the party. Only the non-AdLib (digital)
// path scales volume by distance; the AdLib path, ported here, ignores it.
func (e *Engine) playSoundAt(id, block int) {
	if e.Opt80e6 == 0 {
		return
	}
	if e.pendingSound > 0 {
		if s, ok := e.Sys.(SoundPlayer); ok {
			s.PlaySound(e.pendingSound)
		}
	}
	e.pendingDist = 0
	if block != 0 {
		e.pendingDist = blockDistance(e.CurrentBlock, block)
	}
	e.pendingSound = id
}

// playSoundUpdate ports S177d_00f3.
func (e *Engine) playSoundUpdate(id int) { e.playSoundAt(id, e.CurrentBlock) }

// identifyQuiver identifies the item list in inventory slot 16 (S122e_1cad).
func (e *Engine) identifyQuiver(c int) { e.identifyList(&e.Chars[c].Inventory[16]) }

// identifyBlockItems identifies a block's item list (S122e_1cad).
func (e *Engine) identifyBlockItems(b int) { e.identifyList(&e.Blocks[b].ItemHead) }

// identifyList ports S0bd2_040e: sets flag 0x40 on every item of a list.
func (e *Engine) identifyList(hp *uint16) {
	head := int(*hp)
	if head == 0 {
		return
	}
	for si, first := head, true; si != head || first; si = int(e.Items[si].Prev) {
		first = false
		e.Items[si].Flags |= 0x40
	}
}

// scriptLoadNewLevel ports S122e_1aab.
func (e *Engine) scriptLoadNewLevel(level, block, dir int) {
	if e.GuiMode != 0 {
		e.closeInventory()
	}
	if e.SpellbookOpen {
		e.closeSpellbook()
	}
	e.CurrentBlock = block
	if dir != -1 {
		e.CurrentDirection = dir
	}
	e.leaveLevel()
	e.saveLevelState()
	e.loadLevel(level)
	e.mustCPS(e.X.Str(0x5ba1), 2)
	e.moveParty(block)
}

// leaveLevel ports O210c_19f7: drop in-flight items, finish a running door
// animation instantly.
func (e *Engine) leaveLevel() {
	for i := range e.Monsters {
		e.Monsters[i].CurAttackFrame = 0
	}
	for i := range e.FlyingObjects {
		o := &e.FlyingObjects[i]
		if o.Enable == 1 {
			e.Items[o.Item].Pos &= 3
		}
		o.Enable = 0
	}
	t := &e.Timers[timerDoor]
	if !t.Enabled {
		return
	}
	b := &e.Blocks[t.Param]
	di := 1
	if e.WllWallFlags[b.Walls[0]]&8 != 0 {
		di = 0
	}
	for e.WllWallFlags[b.Walls[di]]&0x30 == 0 {
		b.Walls[di] = byte(int(b.Walls[di]) + int(t.Extra))
		b.Walls[di+2] = byte(int(b.Walls[di+2]) + int(t.Extra))
	}
	t.Enabled = false
}
