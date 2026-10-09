package eob

// Minimap (extension, not in the original): Tab toggles a map of the
// blocks the party has seen, drawn in the free playfield area right of the
// compass.
// ponytail: "seen" = the 18 blocks the renderer looked at, which can reveal
// a block behind a wall; not saved with the game.

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
		e.miniDoor, e.miniOpenDoor, e.miniPOI = e.nearestColor(8, 16, 63), e.nearestColor(40, 63, 63), e.nearestColor(63, 63, 0)
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
