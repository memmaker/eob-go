package eob

import "fmt"

// loadItems ports loadItemDefs (O2106_005e): ITEM.DAT items and names.
func (e *Engine) loadItems() error {
	b, err := e.FS.Read(e.X.Str(0x6752))
	if err != nil {
		b, err = e.FS.Read("ITEM.DAT")
		if err != nil {
			return err
		}
	}
	n := int(le16(b))
	e.Items = make([]Item, 500)
	for i := range e.Items {
		e.Items[i].Block = -1 // unused slots are free for duplicateItem
	}
	p := 2
	for i := 0; i < n && i < 500; i++ {
		r := b[p:]
		e.Items[i] = Item{NameUnid: r[0], NameID: r[1], Flags: r[2], Icon: int8(r[3]), Type: int8(r[4]), Pos: int8(r[5]),
			Block: int16(le16(r[6:])), Next: int16(le16(r[8:])), Prev: int16(le16(r[10:])), Level: r[12], Value: int8(r[13])}
		p += 14
	}
	m := int(le16(b[p:]))
	p += 2
	e.ItemNames = make([]string, m)
	for i := 0; i < m; i++ {
		e.ItemNames[i] = cstr(b[p : p+35])
		p += 35
	}
	return nil
}

// ItemType is an original 16-byte ITEMTYPE.DAT record.
type ItemType struct {
	InvFlags       uint16
	HandFlags      uint16
	ArmorClass     int8
	AllowedClasses byte
	RequiredHands  int8
	DmgDiceS       byte
	DmgPipsS       byte
	DmgIncS        int8
	DmgDiceL       byte
	DmgPipsL       byte
	DmgIncL        int8
	Unk1           byte
	ExtraProps     uint16
}

// loadItemTypes ports O2106_0110.
func (e *Engine) loadItemTypes() error {
	b, err := e.FS.Read("ITEMTYPE.DAT")
	if err != nil {
		return err
	}
	n := int(le16(b))
	e.ItemTypes = make([]ItemType, n)
	for i := 0; i < n; i++ {
		r := b[2+i*16:]
		e.ItemTypes[i] = ItemType{InvFlags: le16(r), HandFlags: le16(r[2:]), ArmorClass: int8(r[4]), AllowedClasses: r[5],
			RequiredHands: int8(r[6]), DmgDiceS: r[7], DmgPipsS: r[8], DmgIncS: int8(r[9]), DmgDiceL: r[10], DmgPipsL: r[11],
			DmgIncL: int8(r[12]), Unk1: r[13], ExtraProps: le16(r[14:])}
	}
	return nil
}

func (e *Engine) itemType(it int) *ItemType {
	t := int(e.Items[it].Type)
	if t < 0 || t >= len(e.ItemTypes) {
		return &ItemType{}
	}
	return &e.ItemTypes[t]
}

const (
	dsSlotMask       = 0x1c8d // u16 per inventory slot
	dsFloorBlockOffs = 0x36bc // int8 per direction
	dsFloorItemPos   = 0x36c0 // [dir*4 + area]
	dsHandSplitY     = 0x36b0 // u16 per character
)

// clickedFloor ports S0acd_0644 (areas 0..3 at the bottom of the 3D view).
// Extension: a left click with an empty hand stows the item in the backpack
// of the character with the most free space (falls back to the hand).
func (e *Engine) clickedFloor(area, button int) {
	blk := e.CurrentBlock
	if area >= 2 {
		blk = (e.CurrentBlock + int(e.X.I8(dsFloorBlockOffs+e.CurrentDirection))) & 0x3ff
		w := e.Blocks[blk].Walls[e.scene.drawVarDown]
		if e.WllWallFlags[w]&0xb == 0 {
			return
		}
	}
	pos := e.u8(dsFloorItemPos + e.CurrentDirection*4 + area)
	if e.HandItem == 0 {
		it := e.unlinkItem(blk, pos, -1)
		if it == 0 {
			return
		}
		e.runBlockScript(blk, 8)
		if !(button == 1 && e.stowItem(it)) {
			e.setHandItem(it)
		}
	} else {
		e.linkItemToBlock(blk, e.HandItem, pos)
		e.runBlockScript(blk, 4)
		e.setHandItem(0)
	}
	e.sceneUpdate = true
	e.spendTurn()
}

// stowItem puts item it into the first free backpack slot (2..15) of the
// character with the most free slots; false if every backpack is full.
func (e *Engine) stowItem(it int) bool {
	best, bestFree, slot := -1, 0, 0
	for c := range e.Chars {
		if !e.charStatus(c, 1) {
			continue
		}
		free, first := 0, 0
		for s := 15; s >= 2; s-- {
			if e.Chars[c].Inventory[s] == 0 {
				free, first = free+1, s
			}
		}
		if free > bestFree {
			best, bestFree, slot = c, free, first
		}
	}
	if best < 0 {
		return false
	}
	e.Chars[best].Inventory[slot] = uint16(it)
	e.printMessage(fmt.Sprintf("%s takes %s.\r", e.Chars[best].NameStr(), e.itemName(it)))
	e.drawCharPanel(best)
	return true
}

// clickedHandSlot ports S0bd2_0c8e: swap mouse item with a hand slot.
func (e *Engine) clickedHandSlot(c, slot int) {
	ch := &e.Chars[c]
	cur := int(ch.Inventory[slot])
	if e.Items[cur].Flags&0x20 != 0 && cur != 0 {
		return
	}
	h := e.HandItem
	mask := uint16(0xff)
	if h != 0 {
		mask = uint16(byte(e.itemType(h).InvFlags))
	}
	if mask&e.X.U16(dsSlotMask+slot*2) != 0 {
		e.setHandItem(cur)
		ch.Inventory[slot] = uint16(h)
		e.drawCharPanel(c)
		e.spendTurn()
	}
	e.recalcArmorClass(c)
}

// clickedCharHand ports S0acd_0415.
func (e *Engine) clickedCharHand(c int, ev Event) {
	if !e.charStatus(c, 1) {
		return
	}
	hand := 0
	if e.u8(dsHandSplitY+c*2) <= ev.Y {
		hand = 1
	}
	if ev.Button == 1 {
		e.clickedHandSlot(c, hand)
	} else {
		e.useItem(c, hand)
	}
}

// linkItemTop ports S0bd2_0458: links item into block and makes the item
// after it the block's top item.
func (e *Engine) linkItemTop(block, item, pos int) {
	e.linkItemToBlock(block, item, pos)
	if nx := int(e.Items[item].Next); nx != item {
		e.Blocks[block].ItemHead = uint16(nx)
	}
}

// kbdCommand ports S0acd_0d9d (keyboard-mode keys left over by the buttons).
// ds:dcaa==5 guards here and in S0acd_0a34/0a61 are dropped: ds:dcaa is only
// ever set to 1 or 2 (S0acd_0753, S0acd_0fb3), so they never fire.
func (e *Engine) kbdCommand(key int) {
	if !e.KbdMode {
		return
	}
	si := -1
	switch key {
	case 0x34:
		e.kbdClickWall()
	case 0x17:
		if e.GuiMode != 2 {
			e.useItem(e.InvChar, e.kbdSlot)
		}
	case 0x21:
		if e.GuiMode != 2 {
			e.kbdThrow(0)
		}
	case 0x15:
		if e.GuiMode != 2 {
			e.kbdThrow(1)
		}
	case 0x23:
		if e.GuiMode != 2 {
			e.kbdFloorItem()
		}
	case 0x12:
		si = 0
	case 0x20:
		si = 1
	case 0x2e:
		si = 2
	case 0x1f:
		si = 3
	}
	if si != -1 && e.GuiMode != 2 {
		e.kbdMoveCursor(si)
	}
}

// kbdClickWall ports S0acd_08e7.
func (e *Engine) kbdClickWall() {
	blk := e.calcNewBlock(e.CurrentBlock, e.CurrentDirection)
	e.clickFlags = 0xff // ds:efd2
	e.clickedWall(blk, e.CurrentDirection)
	e.recalcArmorClass(e.InvChar)
}

// kbdMoveCursor ports S0acd_0a34.
func (e *Engine) kbdMoveCursor(d int) {
	switch e.GuiMode {
	case 0:
		e.kbdMoveHand(d)
	case 1:
		e.kbdMoveSlot(d)
	}
}

// kbdMoveHand ports S0acd_091c: moves the hand cursor between portraits.
func (e *Engine) kbdMoveHand(d int) {
	old := e.InvChar
	switch d {
	case 0:
		if e.kbdSlot != 0 {
			e.kbdSlot = 0
			break
		}
		for {
			if e.InvChar -= 2; e.InvChar < 0 {
				e.InvChar += 6
			}
			if e.charStatus(e.InvChar, 1) {
				break
			}
		}
		e.kbdSlot = 1
	case 1, 3:
		if e.charStatus(e.InvChar, 1) {
			e.InvChar ^= 1
		}
	case 2:
		if e.kbdSlot == 0 {
			e.kbdSlot = 1
			break
		}
		for {
			if e.InvChar += 2; e.InvChar > 5 {
				e.InvChar -= 6
			}
			if e.charStatus(e.InvChar, 1) {
				break
			}
		}
		e.kbdSlot = 0
	}
	e.drawCharPanel(old)
	if e.InvChar != old {
		e.drawCharPanel(e.InvChar)
	}
}

// kbdMoveSlot ports S0acd_09ef: moves the inventory slot cursor.
func (e *Engine) kbdMoveSlot(d int) {
	old := e.kbdSlot
	e.kbdSlot = e.u8(0x36d0 + e.kbdSlot*4 + d)
	e.drawInvSlot(old, true, 0)
	e.drawInvSlot(e.kbdSlot, true, 0)
}

// kbdThrow ports S0acd_0a61: drops (throw==0) or throws the selected item.
func (e *Engine) kbdThrow(throw int) {
	e.spendTurn()
	c := e.InvChar
	var it int
	if e.kbdSlot != 0x10 {
		it = e.heldItem()
		if e.Items[it].Flags&0x20 != 0 {
			return
		}
	} else {
		it = e.unlinkListItem(&e.Chars[c].Inventory[0x10], 0, -1)
	}
	if it == 0 {
		return
	}
	pos := e.u8(0x373c + e.CurrentDirection*2 + e.u8(0x3744+c))
	ok := true
	if throw == 0 {
		e.linkItemToBlock(e.CurrentBlock, it, pos)
		e.runBlockScript(e.CurrentBlock, 4)
	} else {
		ok = e.launchItem(c, it, e.CurrentBlock, pos, e.CurrentDirection, int(e.Items[it].Type))
	}
	switch {
	case ok:
		if !e.KbdMode || e.kbdSlot != 0x10 {
			e.setHandItem(0)
		} else {
			e.drawCharPanel(c)
		}
		e.sceneUpdate = true // ds:df4c
		e.kbdFloorPos = -1
	case e.kbdSlot == 0x10:
		e.linkItem(&e.Chars[c].Inventory[0x10], -2, it, 0)
	}
	e.recalcArmorClass(c)
}

// kbdFloorItem ports S0acd_0ba6: cycles the floor position ds:efb2 and
// swaps the next floor item with the keyboard-selected slot.
func (e *Engine) kbdFloorItem() {
	dir := e.CurrentDirection
	if e.Blocks[e.CurrentBlock].ItemHead == 0 && e.Blocks[e.calcNewBlock(e.CurrentBlock, dir)].ItemHead == 0 {
		e.recalcArmorClass(e.InvChar)
		return
	}
	for tries := 0; ; {
		if e.kbdFloorPos == -1 {
			e.kbdFloorFirst = -1
		}
		if e.kbdFloorFirst == -1 {
			e.kbdFloorPos = (e.kbdFloorPos + 1) & 3
			tries++
		}
		blk := e.CurrentBlock
		if e.X.U16(0x374a+e.kbdFloorPos*2) != 0 {
			blk = e.calcNewBlock(blk, dir)
		}
		pos := e.u8(0x1a1b + dir*4 + e.kbdFloorPos)
		it := e.unlinkItem(blk, pos, -1)
		if it == 0 || it == e.kbdFloorFirst {
			if it != 0 {
				e.linkItemToBlock(blk, it, pos)
				e.runBlockScript(blk, 4)
			}
			e.kbdFloorFirst = -1
			if tries >= 5 {
				break
			}
			continue
		}
		if e.kbdFloorFirst == -1 {
			e.kbdFloorFirst = it
		}
		switch {
		case !e.canEquip(it, e.InvChar, e.kbdSlot):
			e.linkItemTop(blk, it, pos)
			e.runBlockScript(blk, 4)
		case e.kbdSlot == 0x10:
			e.linkItem(&e.Chars[e.InvChar].Inventory[0x10], -2, it, 0)
			e.drawCharPanel(e.InvChar)
		default:
			if h := e.heldItem(); h != 0 {
				e.linkItemTop(blk, h, pos)
				e.runBlockScript(blk, 4)
			}
			e.setHandItem(it)
		}
		e.sceneUpdate = true // ds:df4c
		break
	}
	e.recalcArmorClass(e.InvChar)
}
