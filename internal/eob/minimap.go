package eob

// Minimap (extension, not in the original): Tab toggles a map of the
// blocks the party has seen, drawn in the free playfield area right of the
// compass.
// "Seen" = the view slots visibleSlots finds unobstructed.

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
	vis := e.visibleSlots()
	for i, b := range e.scene.visIndex {
		if vis[i] {
			s[b] = true
		}
	}
}

// visibleSlots tells which of the 18 view slots (visIndex) the party can
// actually see. Sight spreads from the party's block forward and sideways,
// into a neighbour only through an open (passable) face of the block it
// leaves: walls and closed doors are seen, what lies behind them is not.
// ponytail: a grid flood, not a perspective test; it can see round a
// corner the 3D view would hide at the far edge of a row.
func (e *Engine) visibleSlots() [18]bool {
	var vis [18]bool
	slot := map[[2]int]int{} // forward, sideways (right +) -> slot
	for i := 0; i < 18; i++ {
		off := int(e.T.BlockIndex[i]) // direction 0: north is -32
		dx := ((off+16)%32+32)%32 - 16
		slot[[2]int{-(off - dx) / 32, dx}] = i
	}
	d := e.CurrentDirection
	steps := []struct{ df, dl, dir int }{{1, 0, d}, {0, 1, (d + 1) & 3}, {0, -1, (d + 3) & 3}}
	start := slot[[2]int{0, 0}]
	vis[start] = true
	for q := [][2]int{{0, 0}}; len(q) > 0; q = q[1:] {
		c := q[0]
		blk := e.scene.visIndex[slot[c]]
		for _, st := range steps {
			n := [2]int{c[0] + st.df, c[1] + st.dl}
			i, ok := slot[n]
			if !ok || vis[i] || e.WllWallFlags[e.Blocks[blk].Walls[st.dir]]&1 == 0 {
				continue
			}
			vis[i] = true
			q = append(q, n)
		}
	}
	return vis
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
	sc.FillRect(p, x-1, y-1, x+1, y+1, 0xf)   // white: yellow marks points of interest
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
		e.miniDoor, e.miniPOI = e.nearestColor(40, 40, 40), e.nearestColor(63, 63, 0)
		e.miniSwitch, e.miniSwitchOn = e.nearestColor(24, 40, 63), e.nearestColor(6, 10, 40)
		e.miniPlate, e.miniPlateOn = e.nearestColor(42, 0, 42), e.nearestColor(20, 2, 27)
	}
}

// mapCellColor is the map colour of a seen block: wall, floor or door.
func (e *Engine) mapCellColor(blk int) byte {
	c := e.miniWall
	for _, w := range e.Blocks[blk].Walls {
		if f := e.WllWallFlags[w]; f&8 != 0 { // door (drawSceneShapes' drawDoor test)
			if f&1 != 0 { // passable: fully open (still moving counts as closed)
				return e.miniFloor // open doors read as floor
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
// its script changed lasting state other than walls (flags, items,
// monsters), and the other blocks whose walls it changed (saved in the .map
// file; the original walls come from LEVELn.MAZ). A script that only shows
// a message, plays a sound or moves the party changes neither.
type switchUse struct {
	Fired   bool
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
	lvl, flags, items, monsters, hand := e.CurrentLevel, e.LevelFlags, slices.Clone(e.Items), e.Monsters, e.HandItem
	return func() {
		if e.CurrentLevel != lvl { // stairs / level change: block numbers mean something else
			return
		}
		var targets []int
		for i := range e.Blocks {
			if i != blk && e.Blocks[i].Walls != before[i] {
				targets = append(targets, i)
			}
		}
		fired := flags != e.LevelFlags || !slices.Equal(items, e.Items) || monsters != e.Monsters || hand != e.HandItem
		if !fired && targets == nil {
			return
		}
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
		u.Fired = u.Fired || fired
		for _, i := range targets {
			if !slices.Contains(u.Targets, i) {
				u.Targets = append(u.Targets, i)
			}
		}
	}
}

// switchColor colours a block with a lever or button (blue) or a pressure
// plate (purple): light while it and every block its script ever changed
// are as the level started, dark otherwise. A button or plate whose script
// changed no walls but other lasting state (an item, a flag) stays dark:
// that cannot be checked for a reset. A door button toggles its own block's door.
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
			// lever up / down; a clickable decoration only if its block script
			// can change something (door buttons just follow the door colours)
			if t := e.SpecialWallTypes[w]; t == 3 || t == 4 || t == 2 && e.scriptChanges(int(e.Blocks[blk].Script)) {
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
		used = used || u.Fired && len(u.Targets) == 0 && !lever // a lever's own position is its state
		for _, b := range u.Targets {
			used = used || m != nil && e.Blocks[b].Walls != m[b]
		}
	}
	if used {
		return dark, true
	}
	return light, true
}

// scriptChanges reports whether the level script starting at pos can, on
// any path, change walls, flags, items or monsters, or change level (not
// just print or play).
func (e *Engine) scriptChanges(pos int) bool {
	d := e.LevelINF
	seen := map[int]bool{}
	todo := []int{pos}
	for len(todo) > 0 {
		p := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		for p > 0 && p < len(d) && !seen[p] {
			seen[p] = true
			op := d[p]
			p++
			arg := func(i int) int8 { // i-th operand byte, 0 past the end
				if p+i < len(d) {
					return int8(d[p+i])
				}
				return 0
			}
			switch op {
			case 0xff: // setWallType; -19 only turns the party
				if arg(0) != -19 {
					return true
				}
				p += 2
			case 0xfe, 0xfd, 0xfc, 0xfb, 0xf9, 0xf5, 0xed, 0xea, 0xec: // walls, doors, monsters, flags, items, level
				return true
			case 0xf7: // setFlags; -28 is script-internal
				if arg(0) != -28 {
					return true
				}
				p++
			case 0xfa: // teleport: moving monsters or items counts, the party not
				if t := arg(0); t == -13 || t == -11 {
					return true
				} else if t == -15 {
					p += 3
				} else {
					p += 5
				}
			case 0xf8: // message
				for p < len(d) && d[p] != 0 {
					p++
				}
				p += 3
			case 0xf6:
				p += 3
			case 0xf4:
				p += 2
			case 0xf3:
				p += 4
			case 0xf2: // jump
				p = int(le16(d[p:]))
			case 0xf1, 0xf0: // end, return
				p = 0
			case 0xef: // call
				todo = append(todo, int(le16(d[p:])))
				p += 2
			case 0xee: // condition: both branches
				p = e.skipCond(p)
				if p+2 <= len(d) {
					todo = append(todo, int(le16(d[p:])))
				}
				p += 2
			case 0xeb:
				p += 3
			case 0xe9:
				p += 7
			case 0xe8:
				p += 2
			case 0xe7:
				p += 2
			case 0xe6:
				p++
			}
		}
	}
	return false
}

// skipCond returns the position of the jump target after the expression at p
// (operand sizes as in scriptEval).
func (e *Engine) skipCond(p int) int {
	d := e.LevelINF
	for p < len(d) {
		op := d[p]
		p++
		switch op {
		case 0xee:
			return p
		case 0xf7, 0xf3, 0xf1:
			p += 2
		case 0xf5, 0xe9, 0xdb:
			p += 3
		case 0xf2:
			p += 4
		case 0xf0, 0xef, 0xe7, 0xdd, 0xdc:
			p++
		}
	}
	return p
}
