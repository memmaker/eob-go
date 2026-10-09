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

// A map click walks the party to a seen block.
func TestAutoWalk(t *testing.T) {
	e, err := New(testDir, &fakeSys{})
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testDir + "/EOBDATA.SAV"); err != nil {
		t.Skip(err)
	}
	e.seen = map[int]*[1024]bool{e.CurrentLevel: new([1024]bool)}
	// explore a few blocks reachable from the start, remember the farthest
	dst, q := e.CurrentBlock, []int{e.CurrentBlock}
	e.seen[e.CurrentLevel][dst] = true
	for i := 0; i < 6 && len(q) > 0; i++ {
		for d := 0; d < 4; d++ {
			if nb := e.checkMove(q[0], d); nb >= 0 && !e.seen[e.CurrentLevel][nb] {
				e.seen[e.CurrentLevel][nb] = true
				q, dst = append(q, nb), nb
			}
		}
		q = q[1:]
	}
	if e.autoPath = e.findPath(dst); e.autoPath == nil {
		t.Fatal("no path")
	}
	for i := 0; i < 50 && len(e.autoPath) > 0; i++ {
		e.autoNext = 0
		e.autoWalk()
	}
	if e.CurrentBlock != dst {
		t.Fatalf("at %d, want %d", e.CurrentBlock, dst)
	}
}

// Map colours: levers / buttons green, pressure plates orange, dark once
// changed from the level's original state.
func TestSwitchColor(t *testing.T) {
	e, err := New(testDir, &fakeSys{})
	if err != nil {
		t.Skip(err)
	}
	if err := e.InitResources(); err != nil {
		t.Skip(err)
	}
	if err := e.loadLevel(2); err != nil { // fixed level, not the live savegame
		t.Fatal(err)
	}
	e.Scr.SetPalette(e.scene.levelPal)
	e.mapColors()
	find := func(ok func(w [4]byte) bool) int {
		for b := range e.Blocks {
			if ok(e.Blocks[b].Walls) && e.Blocks[b].Walls == e.mazeWalls[e.CurrentLevel][b] {
				return b
			}
		}
		t.Skip("not on this level")
		return 0
	}
	plate := find(func(w [4]byte) bool { return w[0] == plateWall })
	side := func(w [4]byte, types ...byte) int {
		for s, x := range w {
			for _, ty := range types {
				if e.SpecialWallTypes[x] == ty {
					return s
				}
			}
		}
		return -1
	}
	lever := find(func(w [4]byte) bool { return side(w, 3, 4) >= 0 })
	button := 0
	for b := range e.Blocks {
		if w := e.Blocks[b].Walls; side(w, 2) >= 0 && side(w, 3, 4) < 0 && w == e.mazeWalls[e.CurrentLevel][b] {
			if _, ok := e.switchColor(b); !ok != !e.scriptChanges(int(e.Blocks[b].Script)) {
				t.Fatalf("decoration block %d: marked %v", b, ok)
			} else if ok && button == 0 {
				button = b
			}
		}
	}
	for b := range e.Blocks { // door buttons keep the door colours
		if side(e.Blocks[b].Walls, 1) >= 0 && side(e.Blocks[b].Walls, 2, 3, 4) < 0 && e.Blocks[b].Walls[0] != plateWall {
			if _, ok := e.switchColor(b); ok {
				t.Fatalf("door button block %d marked", b)
			}
		}
	}
	check := func(blk int, want byte) {
		t.Helper()
		if c, ok := e.switchColor(blk); !ok || c != want {
			t.Fatalf("block %d: colour %d %v, want %d", blk, c, ok, want)
		}
	}
	check(plate, e.miniPlate)
	check(lever, e.miniSwitch)
	check(button, e.miniSwitch)
	e.Blocks[plate].Walls = [4]byte{}
	e.Blocks[lever].Walls[side(e.Blocks[lever].Walls, 3, 4)]++
	e.trackSwitch(button)() // pressed, its script only showed a message
	check(button, e.miniSwitch)
	done := e.trackSwitch(button) // pressed again, now its script sets a flag
	e.LevelFlags[e.CurrentLevel] ^= 1
	done()
	check(plate, e.miniPlateOn)
	check(lever, e.miniSwitchOn)
	check(button, e.miniSwitchOn)
	door := (lever + 1) & 0x3ff // a block the lever's script changed
	e.switches[e.CurrentLevel][lever] = &switchUse{Fired: true, Targets: []int{door}}
	e.Blocks[door].Walls[0]++
	e.Blocks[lever].Walls[side(e.Blocks[lever].Walls, 3, 4)]-- // lever back, door not
	check(lever, e.miniSwitchOn)
	e.Blocks[door].Walls[0]-- // door back too
	check(lever, e.miniSwitch)
	if _, ok := e.switchColor(0); ok {
		t.Fatal("block 0 marked")
	}
}

// An auto-walk stops when a monster or item comes into view, not for one
// already in view when it started, nor for one behind a wall.
func TestAutoWatch(t *testing.T) {
	e, err := New(testDir, &fakeSys{})
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testDir + "/EOBDATA.SAV"); err != nil {
		t.Skip(err)
	}
	e.assignVisibleBlocks(e.CurrentBlock, e.CurrentDirection)
	vis := e.visibleSlots()
	var seen, hidden []int
	for i, b := range e.scene.visIndex {
		e.Blocks[b].ItemHead, e.Blocks[b].Flags = 0, e.Blocks[b].Flags&^7
		if i == 16 {
			continue
		}
		if vis[i] {
			seen = append(seen, b)
		} else {
			hidden = append(hidden, b)
		}
	}
	if len(seen) < 2 || len(hidden) == 0 {
		t.Skipf("start view: %d visible, %d hidden", len(seen), len(hidden))
	}
	e.Blocks[seen[0]].ItemHead = 7 // in view from the start
	e.autoPath, e.autoSpots = []int{0, 0}, e.visibleSpots()
	e.Blocks[hidden[0]].Flags |= 1 // a monster behind a wall
	e.autoWatch()
	if e.autoPath == nil {
		t.Fatal("stopped for something already in view or hidden")
	}
	e.Blocks[seen[1]].Flags |= 1 // a monster steps into view
	e.autoWatch()
	if e.autoPath != nil {
		t.Fatal("kept walking with a new monster in view")
	}
}
