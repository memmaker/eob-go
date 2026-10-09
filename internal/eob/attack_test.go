package eob

import "testing"

// G throws from each hand holding a throwing weapon in turn, skipping others.
func TestPartyThrowCycle(t *testing.T) {
	e, err := New(testDir, &fakeSys{})
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testDir + "/EOBDATA.SAV"); err != nil {
		t.Skip(err)
	}
	dagger := 0
	for i := 1; i < len(e.Items); i++ {
		if t := e.ItemTypes[e.Items[i].Type]; t.ExtraProps&0x7f == 2 && e.classCanUse(0, i) && e.classCanUse(2, i) {
			dagger = i
			break
		}
	}
	if dagger == 0 {
		t.Skip("no throwing weapon in this save")
	}
	for c := range e.Chars {
		e.Chars[c].Inventory[0], e.Chars[c].Inventory[1], e.Chars[c].DisabledSlots = 0, 0, 0
	}
	e.Chars[0].Inventory[0] = uint16(dagger)
	e.Chars[2].Inventory[0] = uint16(e.duplicateItem(dagger))
	bow, arrow := 0, 0
	for i := 1; i < len(e.Items); i++ {
		switch e.Items[i].Type {
		case 7:
			bow = i
		case 0x10:
			arrow = i
		}
	}
	order := []int{0, 2}
	if bow != 0 && arrow != 0 && e.classCanUse(3, bow) { // char 3 has arrows, char 1 none: skipped
		e.Chars[3].Inventory[0], e.Chars[3].Inventory[16] = uint16(bow), 0
		e.linkItem(&e.Chars[3].Inventory[16], -2, e.duplicateItem(arrow), 0)
		e.Chars[1].Inventory[0], e.Chars[1].Inventory[16] = uint16(e.duplicateItem(bow)), 0
		order = append(order, 3)
	}
	for i, c := range order {
		e.partyThrow()
		if e.Chars[c].DisabledSlots&1 == 0 {
			t.Fatalf("throw %d: char %d did not throw", i+1, c)
		}
	}
	if len(order) < 3 {
		t.Log("no bow / arrow in this save: launcher case not checked")
	}
}

// F cycles primary hands, then secondary hands holding a weapon.
func TestPartyAttackCycle(t *testing.T) {
	e, err := New(testDir, &fakeSys{})
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testDir + "/EOBDATA.SAV"); err != nil {
		t.Skip(err)
	}
	for c := 0; c < 2; c++ {
		e.Chars[c].DisabledSlots = 0
	}
	for i := 1; i < len(e.Items); i++ { // an off-hand weapon (needs no hand, e.g. a dagger) for char 0
		if t := e.ItemTypes[e.Items[i].Type]; t.ExtraProps&0x7f == 1 || t.ExtraProps&0x7f == 2 {
			if t.RequiredHands == 0 && e.classCanUse(0, i) {
				e.Chars[0].Inventory[1] = uint16(i)
				break
			}
		}
	}
	if !e.canUseHand(0, 1) {
		t.Skip("no off-hand weapon in this save")
	}
	e.Chars[1].Inventory[1] = 0
	want := [][2]uint8{{1, 0}, {1, 1}, {3, 1}, {3, 1}} // char 1's empty secondary is skipped
	for i, w := range want {
		e.partyAttack()
		if g := [2]uint8{uint8(e.Chars[0].DisabledSlots & 3), uint8(e.Chars[1].DisabledSlots & 3)}; g != w {
			t.Fatalf("press %d: disabled %v, want %v", i+1, g, w)
		}
	}
}
