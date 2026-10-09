package eob

import "testing"

func TestLevelRoundTrip(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.DebugLoadSave(testDir + "/EOBDATA.SAV")
	lvl, blk := e.CurrentLevel, e.CurrentBlock
	w := e.Blocks[blk].Walls
	e.scriptLoadNewLevel(lvl%12+1, blk, -1)
	if e.CurrentLevel != lvl%12+1 {
		t.Fatalf("level %d", e.CurrentLevel)
	}
	e.scriptLoadNewLevel(lvl, blk, -1)
	if e.CurrentLevel != lvl || e.Blocks[blk].Walls != w {
		t.Fatalf("not restored: level %d walls %v want %v", e.CurrentLevel, e.Blocks[blk].Walls, w)
	}
}

func TestItemNames(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.DebugLoadSave(testDir + "/EOBDATA.SAV")
	seen := map[string]bool{}
	for i := 1; i < len(e.Items); i++ {
		if e.Items[i].Type < 0 || e.Items[i].NameUnid == 0 {
			continue
		}
		e.Items[i].Flags |= 0x40
		seen[e.itemName(i)] = true
	}
	for _, n := range []string{"Potion of Giant Strength", "Mage Scroll of Fireball", "Cursed Axe -3", "Drow Shield +3", "Ring of Wizardry"} {
		if !seen[n] {
			t.Errorf("missing %q", n)
		}
	}
}

func TestSwap(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.InitResources()
	e.loadPlayfield()
	e.loadButtons()
	e.loadTimers()
	e.DebugLoadSave(testDir + "/EOBDATA.SAV")
	a, b := e.Chars[0].Name, e.Chars[1].Name
	e.clickedCharName(0)
	if e.SwapChar != 0 || len(e.activeButtons()) != 7 {
		t.Fatal("swap not armed", e.SwapChar, len(e.activeButtons()), e.Chars[0].Flags, e.ButtonDefs != nil)
	}
	e.clickedCharName(1)
	if e.Chars[0].Name != b || e.Chars[1].Name != a || e.SwapChar != -1 {
		t.Fatal("not swapped")
	}
}

func TestDrinkVitality(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.InitResources()
	e.loadPlayfield()
	e.loadTimers()
	e.DebugLoadSave(testDir + "/EOBDATA.SAV")
	for i := 1; i < len(e.Items); i++ {
		it := &e.Items[i]
		if it.Type >= 0 && e.ItemTypes[it.Type].ExtraProps&0x7f == 0xe && it.Value == 5 {
			e.Chars[0].Food = 3
			e.Chars[0].Inventory[2] = uint16(i)
			e.drinkPotion(0, 2)
			if e.Chars[0].Food != 100 || e.Chars[0].Inventory[2] != 0 {
				t.Fatalf("food %d slot %d", e.Chars[0].Food, e.Chars[0].Inventory[2])
			}
			return
		}
	}
	t.Skip("no vitality potion in save")
}

func TestEquippedItemsUsable(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.DebugLoadSave(testDir + "/EOBDATA.SAV")
	for c := range e.Chars {
		if !e.charStatus(c, 1) {
			continue
		}
		for s := 0; s < 27; s++ {
			if it := int(e.Chars[c].Inventory[s]); it != 0 && s < 2 && !e.classCanUse(c, it) {
				t.Errorf("%s cannot use %s in hand %d", e.Chars[c].NameStr(), e.itemName(it), s)
			}
		}
	}
}

func TestLevelPaletteAndIdentify(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugEnter(1, 0x1ea, 0); err != nil {
		t.Fatal(err)
	}
	if !e.scene.palettePending || len(e.scene.levelPal) < 0x300 {
		t.Fatalf("level palette not loaded: %d bytes", len(e.scene.levelPal))
	}
	for b := range e.Blocks {
		if h := e.Blocks[b].ItemHead; h != 0 {
			e.identifyBlockItems(b)
			if e.Items[h].Flags&0x40 == 0 || e.Items[e.Items[h].Prev].Flags&0x40 == 0 {
				t.Fatal("block items not identified")
			}
			return
		}
	}
	t.Fatal("no items on level 1")
}

func TestTurnMonster(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.DebugLoadSave(testDir + "/EOBDATA.SAV")
	m := &e.Monsters[0]
	need := 0
	for m.Type = 0; m.Type < 32; m.Type++ {
		if need = e.turnTable(0, e.mprop(m, 0x19)); need > 0 && need < 99 {
			break
		}
	}
	if e.turnMonster(m, need-1, 0) || !e.turnMonster(m, need, 0) || m.Flags&8 == 0 {
		t.Fatalf("need %d, flags %#x", need, m.Flags)
	}
}
