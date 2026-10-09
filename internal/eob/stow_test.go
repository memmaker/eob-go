package eob

import "testing"

// Floor pickups go to the character who suits the item.
func TestStowItemPreferences(t *testing.T) {
	e, err := New(testDir, &fakeSys{})
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testDir + "/EOBDATA.SAV"); err != nil {
		t.Skip(err)
	}
	used := map[int]bool{}
	item := func(typ int) int {
		for i := 1; i < len(e.Items); i++ {
			if int(e.Items[i].Type) == typ && !used[i] {
				used[i] = true
				return i
			}
		}
		t.Fatalf("no item of type %x", typ)
		return 0
	}
	for c := range e.Chars {
		e.Chars[c].Inventory = [len(e.Chars[c].Inventory)]uint16{}
		e.Chars[c].Flags &^= 1
	}
	for c, class := range []byte{5, 3, 0, 3} { // thief, mage, fighter, mage
		e.Chars[c].Flags |= 1
		e.Chars[c].HPCur = 10
		e.Chars[c].Class = class
	}
	e.Chars[2].Inventory[0] = uint16(item(7))    // bow
	e.Chars[3].Inventory[5] = uint16(item(5))    // dagger
	e.Chars[0].Inventory[2] = uint16(item(0x1f)) // rations
	e.Chars[2].Inventory[2] = uint16(item(0x1f))
	e.Chars[3].Inventory[2] = uint16(item(0x1f))
	for _, c := range []struct {
		typ, char, slot int
	}{
		{0x10, 2, 16}, // arrow: bow wielder's quiver
		{5, 3, 0x16},  // dagger: dagger carrier's belt
		{0x1f, 1, 2},  // rations: the one without any
		{0x1c, 0, 3},  // lock picks: the thief
		{0x18, 2, 3},  // plate mail: the fighter
	} {
		it := item(c.typ)
		if !e.stowItem(it) {
			t.Fatalf("type %x not stowed", c.typ)
		}
		if got := e.Chars[c.char].Inventory[c.slot]; got == 0 {
			t.Fatalf("type %x: char %d slot %x empty", c.typ, c.char, c.slot)
		}
	}
}

// A right click on a ration in the inventory eats it.
func TestEatFromSlot(t *testing.T) {
	e, err := New(testDir, &fakeSys{})
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testDir + "/EOBDATA.SAV"); err != nil {
		t.Skip(err)
	}
	ration := 0
	for i := 1; i < len(e.Items); i++ {
		if e.Items[i].Type == 0x1f {
			ration = i
			break
		}
	}
	e.loadButtons()
	e.GuiMode, e.InvChar = 1, 0
	ch := &e.Chars[0]
	ch.Food, ch.Inventory[2] = 10, uint16(ration)
	found := false
	for _, k := range e.activeButtons() {
		if b := e.ButtonDefs[k]; b.CB == cbAddr(0x0acd, 0x01c0) && b.Arg == 2 {
			found = e.eatFromSlot(Event{Kind: EvMouseDown, Button: 2, X: b.X, Y: b.Y})
		}
	}
	if !found {
		t.Fatal("not eaten")
	}
	if ch.Inventory[2] != 0 || ch.Food <= 10 {
		t.Fatalf("slot %d food %d", ch.Inventory[2], ch.Food)
	}
}
