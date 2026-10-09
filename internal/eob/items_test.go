package eob

import "testing"

func TestLinkItemTop(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testDir + "/EOBDATA.SAV"); err != nil {
		t.Skip(err)
	}
	const blk, a, b = 5, 10, 11
	e.Blocks[blk].ItemHead = 0
	e.linkItemTop(blk, a, 2)
	e.linkItemTop(blk, b, 2)
	if h := e.Blocks[blk].ItemHead; h != a || e.Items[a].Next != b {
		t.Fatalf("head %d", h)
	}
}

func TestKbdMoveSlot(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.kbdSlot = 0
	e.kbdMoveSlot(2)
	if want := int(e.X.U8(0x36d0 + 2)); e.kbdSlot != want {
		t.Fatalf("slot %d want %d", e.kbdSlot, want)
	}
}

// Floor sprites: ItemShapeMap indexes ds:ec9a, 14 large then 23 small from
// slot 15 (O2106_0171); every mapped icon must land on a loaded shape.
func TestItemFloorShapes(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.InitResources()
	e.loadItemGraphics()
	for i := 1; i < len(e.Items); i++ {
		m := e.T.ItemShapeMap[byte(e.Items[i].Icon)]
		if int(m) >= len(e.ItemShapes) || e.ItemShapes[m][0] == nil {
			t.Fatalf("item %d icon %d -> shape %d not loaded", i, e.Items[i].Icon, m)
		}
	}
}

func TestStowItem(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testDir + "/EOBDATA.SAV"); err != nil {
		t.Skip(err)
	}
	for c := range e.Chars { // char 2 gets the emptiest backpack
		for s := 2; s <= 15; s++ {
			e.Chars[c].Inventory[s] = 0
			if c != 2 && s < 5 {
				e.Chars[c].Inventory[s] = 1
			}
		}
	}
	if !e.stowItem(7) || e.Chars[2].Inventory[2] != 7 {
		t.Fatalf("item not stowed with char 2: %v", e.Chars[2].Inventory[:16])
	}
}
