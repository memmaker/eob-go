package eob

// Minimap (extension, not in the original): Tab toggles a map of the
// blocks the party has seen, drawn in the free playfield area right of the
// compass.
// ponytail: "seen" = the 18 blocks the renderer looked at, which can reveal
// a block behind a wall; not saved with the game.

import "slices"

const (
	miniW    = 27 // cells across, party in the middle
	miniH    = 13 // cells down
	miniCell = 5  // pixels per cell
	miniX    = 318 - miniW*miniCell
	miniY    = 108
)

// markSeen records the blocks drawn by the current scene.
func (e *Engine) markSeen() {
	if e.seen == nil {
		e.seen = map[int]*[1024]bool{}
	}
	s := e.seen[e.CurrentLevel]
	if s == nil {
		s = new([1024]bool)
		e.seen[e.CurrentLevel] = s
	}
	s[e.CurrentBlock] = true
	for _, b := range e.scene.visIndex {
		s[b] = true
	}
}

// drawMiniMap draws onto page p, north up.
func (e *Engine) drawMiniMap(p int) {
	e.mapColors()
	sc := e.Scr
	sc.FillRect(p, miniX-1, miniY-1, miniX+miniW*miniCell, miniY+miniH*miniCell, 0)
	px, py := e.CurrentBlock&31, e.CurrentBlock>>5
	for cy := 0; cy < miniH; cy++ {
		for cx := 0; cx < miniW; cx++ {
			bx, by := px+cx-miniW/2, py+cy-miniH/2
			if bx < 0 || by < 0 || bx > 31 || by > 31 {
				continue
			}
			if c, ok := e.mapCell(by<<5 | bx); ok {
				x, y := miniX+cx*miniCell, miniY+cy*miniCell
				sc.FillRect(p, x, y, x+miniCell-1, y+miniCell-1, c)
			}
		}
	}
	e.drawPartyMark(p, miniX+miniW/2*miniCell+miniCell/2, miniY+miniH/2*miniCell+miniCell/2)
}

// drawPartyMark draws the party at x, y: a dot plus a tick toward the
// facing side (0 N, 1 E, 2 S, 3 W).
func (e *Engine) drawPartyMark(p, x, y int) {
	sc := e.Scr
	dx := [4]int{0, 1, 0, -1}[e.CurrentDirection]
	dy := [4]int{-1, 0, 1, 0}[e.CurrentDirection]
	sc.FillRect(p, x-1, y-1, x+1, y+1, 0xf) // white: yellow marks points of interest
	sc.DrawLine(p, x, y, x+dx*2, y+dy*2, 0xf) // white: yellow marks points of interest
}

// mapCell returns the map colour of block blk on the current level; false
// for a block neither seen nor marked.
func (e *Engine) mapCell(blk int) (byte, bool) {
	if _, ok := e.pois[e.CurrentLevel][blk]; ok {
		return e.miniPOI, true
	}
	if s := e.seen[e.CurrentLevel]; s != nil && s[blk] {
		if c, ok := e.switchColor(blk); ok {
			return c, true
		}
		return e.mapCellColor(blk), true
	}
	return 0, false
}

// mapColors picks the current level's map colours (the upper palette
// changes per level).
func (e *Engine) mapColors() {
	if e.miniLvl != e.CurrentLevel {
		e.miniLvl = e.CurrentLevel
		e.miniFloor, e.miniWall = e.vmpColor(8*22, 330, &e.vcn.ColBg, 55), e.vmpColor(330, len(e.vmp), &e.vcn.ColWall, 130)
		e.miniDoor, e.miniOpenDoor, e.miniPOI = e.nearestColor(24, 36, 63), e.nearestColor(8, 12, 40), e.nearestColor(63, 63, 0)
		e.miniSwitch, e.miniSwitchOn = e.nearestColor(24, 63, 24), e.nearestColor(0, 28, 0)
		e.miniPlate, e.miniPlateOn = e.nearestColor(63, 44, 12), e.nearestColor(40, 20, 0)
	}
}

// mapCellColor is the map colour of a seen block: wall, floor or door.
func (e *Engine) mapCellColor(blk int) byte {
	c := e.miniWall
	for _, w := range e.Blocks[blk].Walls {
		if f := e.WllWallFlags[w]; f&8 != 0 { // door (drawSceneShapes' drawDoor test)
			if f&1 != 0 { // passable: fully open (still moving counts as closed)
				return e.miniOpenDoor
			}
			return e.miniDoor
		}
		if e.WllWallFlags[w]&1 != 0 {
			c = e.miniFloor
		}
	}
	return c
}

// nearestColor returns the palette index closest to 6-bit r, g, b; the
// upper palette changes per level, so this is looked up on every draw.
func (e *Engine) nearestColor(r, g, b int) byte {
	best, bd := 0, 1<<30
	for i := 0; i < 256; i++ {
		p := e.Scr.Pal[i*3:]
		dr, dg, db := int(p[0]&63)-r, int(p[1]&63)-g, int(p[2]&63)-b
		if d := dr*dr + dg*dg + db*db; d < bd {
			best, bd = i, d
		}
	}
	return byte(best)
}

// vmpColor returns the palette index nearest the average colour of the VCN
// blocks in vmp[from:to] (background rows 8+ are floor, 330+ are walls),
// scaled to pct%: the averages are close, so floor goes darker, walls lighter.
func (e *Engine) vmpColor(from, to int, col *[16]byte, pct int) byte {
	var sum [3]int
	n := 0
	for _, v := range e.vmp[from:to] {
		idx := int(v & 0x3fff)
		if v == 0 || idx*32+32 > len(e.vcn.Blocks) {
			continue
		}
		for _, px := range e.vcn.Blocks[idx*32 : idx*32+32] {
			for _, c := range [2]byte{col[px>>4], col[px&15]} {
				if c == 0 {
					continue
				}
				for k := range sum {
					sum[k] += int(e.Scr.Pal[int(c)*3+k] & 63)
				}
				n++
			}
		}
	}
	if n == 0 {
		return 0xd
	}
	for k := range sum {
		sum[k] = min(63, sum[k]*pct/(n*100))
	}
	return e.nearestColor(sum[0], sum[1], sum[2])
}

// plateWall is the pressure plate's wall index; every EOB1 level keeps it
// (26 hole above, 27 pit, 28 plate), only the graphics differ.
const plateWall = 28

// switchUse is what the map knows about a lever, button or plate: whether
// its script ran, and the other blocks that script changed (saved in the
// .map file; the original walls come from LEVELn.MAZ).
type switchUse struct {
	Ran     bool
	Targets []int
}

// isSwitch tells whether block blk holds a pressure plate, lever or button.
func (e *Engine) isSwitch(blk int) bool {
	_, ok := e.switchColor(blk)
	return ok
}

// trackSwitch records a script run of switch block blk; call the returned
// func once the script is done.
func (e *Engine) trackSwitch(blk int) func() {
	var before [1024][4]byte
	for i := range e.Blocks {
		before[i] = e.Blocks[i].Walls
	}
	lvl := e.CurrentLevel
	return func() {
		if e.switches == nil {
			e.switches = map[int]map[int]*switchUse{}
		}
		if e.switches[lvl] == nil {
			e.switches[lvl] = map[int]*switchUse{}
		}
		u := e.switches[lvl][blk]
		if u == nil {
			u = &switchUse{}
			e.switches[lvl][blk] = u
		}
		u.Ran = true
		if e.CurrentLevel != lvl { // the script changed level: block numbers mean something else
			return
		}
		for i := range e.Blocks {
			if i != blk && e.Blocks[i].Walls != before[i] && !slices.Contains(u.Targets, i) {
				u.Targets = append(u.Targets, i)
			}
		}
	}
}

// switchColor colours a block with a lever or button (green) or a pressure
// plate (orange): light while it and every block its script ever changed
// are as the level started, dark otherwise. A button or plate whose script
// changed no block (a message, an item) stays dark once it ran: nothing
// resets it. A door button toggles its own block's door.
func (e *Engine) switchColor(blk int) (byte, bool) {
	cur, orig := e.Blocks[blk].Walls, e.Blocks[blk].Walls
	m := e.mazeWalls[e.CurrentLevel]
	if m != nil {
		orig = m[blk]
	}
	light, dark, lever := e.miniPlate, e.miniPlateOn, false
	if cur[0] != plateWall && orig[0] != plateWall {
		found := false
		for _, w := range cur {
			if t := e.SpecialWallTypes[w]; t >= 1 && t <= 4 { // door button, wall button, lever up / down
				found, lever = true, lever || t >= 3
			}
		}
		if !found {
			return 0, false
		}
		light, dark = e.miniSwitch, e.miniSwitchOn
	}
	used := cur != orig
	if u := e.switches[e.CurrentLevel][blk]; u != nil {
		used = used || u.Ran && len(u.Targets) == 0 && !lever // a lever's own position is its state
		for _, b := range u.Targets {
			used = used || m != nil && e.Blocks[b].Walls != m[b]
		}
	}
	if used {
		return dark, true
	}
	return light, true
}
