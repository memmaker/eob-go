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
		if it == 0 && area < 2 {
			// Extension: also take from the own-block quadrant behind the camera, same side.
			for p := 0; p < 4 && it == 0; p++ {
				if int(e.T.ItemPosIndex[e.CurrentDirection*4+p]) == area+2 {
					it = e.unlinkItem(blk, p, -1)
				}
			}
		}
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

// stowItem gives item it to the character who suits it best, false if
// nobody has room. Preferred, in order: for ammo, a character wielding the
// matching launcher (arrows go into the quiver); for throwing weapons, one
// already carrying that type (onto the belt when it has room); for rations,
// the one carrying fewest; for lock picks and class-restricted weapons /
// armour, one who can use it. Ties go to the most free backpack slots.
func (e *Engine) stowItem(it int) bool {
	typ := int(e.Items[it].Type)
	t := &e.ItemTypes[typ]
	fits := func(s int) bool { return e.X.U16(dsSlotMask+s*2)&t.InvFlags != 0 }
	best, bestPref, bestFree, slot := -1, 0, 0, 0
	for c := range e.Chars {
		if !e.charStatus(c, 1) {
			continue
		}
		ch := &e.Chars[c]
		free, have, launcher, to := 0, 0, false, -1
		for s, x := range ch.Inventory {
			switch {
			case x == 0 && s >= 2 && s <= 15:
				if free++; to < 0 {
					to = s
				}
			case x != 0 && int(e.Items[x].Type) == typ:
				have++
			case x != 0 && s < 2 && e.ItemTypes[e.Items[x].Type].ExtraProps&0x7f == 3:
				launcher = launcher || e.u8(dsAmmoType+int(e.Items[x].Type)) == typ
			}
		}
		pref := 0
		switch {
		case typ == 0x1f: // rations
			pref = -have
		case launcher:
			pref = 3
			if fits(16) {
				to = 16
			}
		case t.ExtraProps&0x7f == 2 && have > 0:
			pref = 2
			for s := 0x16; s <= 0x18; s++ {
				if ch.Inventory[s] == 0 && fits(s) {
					to = s
					break
				}
			}
		case t.AllowedClasses != 0xf && e.classCanUse(c, it):
			pref = 1
		}
		if to >= 0 && (best < 0 || pref > bestPref || pref == bestPref && free > bestFree) {
			best, bestPref, bestFree, slot = c, pref, free, to
		}
	}
	if best < 0 {
		return false
	}
	if ch := &e.Chars[best]; slot == 16 {
		e.linkItem(&ch.Inventory[16], -2, it, 0)
	} else {
		ch.Inventory[slot] = uint16(it)
	}
	e.printMessage(fmt.Sprintf("%s takes %s.\r", e.Chars[best].NameStr(), e.itemName(it)))
	e.drawCharPanel(best)
	return true
}

// clickedHandSlot ports S0bd2_0c8e: swap mouse item with a hand slot.
func (e *Engine) clickedHandSlot(c, slot int) {
	ch := &e.Chars[c]
	cur := int(ch.Inventory[slot])
	if e.Items[cur].Flags&0x20 != 0 {
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

// kbdClickWall ports S0acd_08e7: R uses the wall in front with the mouse item.
func (e *Engine) kbdClickWall() {
	blk := e.calcNewBlock(e.CurrentBlock, e.CurrentDirection)
	e.clickFlags = 0xff // ds:efd2
	e.clickedWall(blk, e.CurrentDirection)
}
