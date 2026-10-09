package eob

import "testing"

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
