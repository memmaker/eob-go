package eob

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// botSys drives Run like a player: BFS to a goal block, turn, walk, open
// doors by clicking the scene centre, and attack with every hand button.
type botSys struct {
	t        *testing.T
	e        *Engine
	goal     int
	now      uint32
	queue    []Event
	wait     int
	polls    int
	tries    int
	last     int
	log      []string
	frames   int
	pix, pal []byte
	started  bool
	final    int // real goal while a lever subgoal is active
	face     int // direction to face at the lever subgoal (-1 none)
	used     map[int]bool
	blocked  map[int]bool // locked doors with no reachable trigger
	drop     bool         // drop a hand item at the subgoal (pressure plate)
}

func (s *botSys) Present(pix, pal []byte) {
	s.frames++
	s.pix, s.pal = append(s.pix[:0], pix...), append(s.pal[:0], pal...)
}

// savePNG writes the last frame (6-bit VGA palette).
func (s *botSys) savePNG(path string) error {
	img := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for i, c := range s.pix {
		p := s.pal[int(c)*3:]
		img.Set(i%320, i/320, color.RGBA{p[0] << 2, p[1] << 2, p[2] << 2, 255})
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
func (s *botSys) Mouse() (int, int) { return 0, 0 }
func (s *botSys) Millis() uint32    { return s.now }
func (s *botSys) Sleep(ms int)      { s.now += uint32(ms) }

func (s *botSys) PollEvent() (Event, bool) {
	if len(s.queue) > 0 {
		ev := s.queue[0]
		s.queue = s.queue[1:]
		return ev, true
	}
	if s.wait > 0 {
		s.wait--
		return Event{}, false
	}
	if !s.started {
		s.started = true
		if s.e.CurrentLevel != 1 {
			s.e.scriptLoadNewLevel(1, 0x1ea, 0)
		}
	}
	s.polls++
	if s.polls > 4000 || s.e.CurrentLevel != 1 || s.partyDead() {
		return Event{Kind: EvQuit}, true
	}
	s.decide()
	return Event{}, false
}

func (s *botSys) partyDead() bool {
	for c := range s.e.Chars {
		if s.e.charStatus(c, 5) {
			return false
		}
	}
	return true
}

// click queues a press/release on the first active button with this callback.
func (s *botSys) click(cb uint32, arg int) bool {
	for _, k := range s.e.activeButtons() {
		b := s.e.ButtonDefs[k]
		if b.CB != cb || (arg >= 0 && b.Arg != arg) {
			continue
		}
		btn := 1
		if b.Flags&0x900 == 0 {
			btn = 2
		}
		x, y := b.X+b.W/2, b.Y+b.H/2
		s.queue = append(s.queue, Event{Kind: EvMouseDown, Button: btn, X: x, Y: y}, Event{Kind: EvMouseUp, Button: btn, X: x, Y: y})
		return true
	}
	return false
}

// firstStep returns the direction of the first step from cur toward goal.
func (s *botSys) firstStep(cur int) int { return s.route(cur, s.goal, false) }

// route BFSes from cur to goal; strict only crosses walls that are open now.
func (s *botSys) route(cur, goal int, strict bool) int {
	e := s.e
	pass := func(w byte) bool { return e.WllWallFlags[w]&1 != 0 || (!strict && w >= 3 && w <= 22) }
	first := map[int]int{cur: -1}
	q := []int{cur}
	for len(q) > 0 {
		b := q[0]
		q = q[1:]
		if b == goal {
			return first[b]
		}
		for d := 0; d < 4; d++ {
			nb := e.calcNewBlock(b, d)
			if _, seen := first[nb]; seen || s.blocked[nb] || !pass(e.Blocks[nb].Walls[e.u8(dsOppositeSide+d)]) {
				continue
			}
			first[nb] = first[b]
			if b == cur {
				first[nb] = d
			}
			q = append(q, nb)
		}
	}
	return -1
}

// clickWall left-clicks the centre of the wall's first decoration shape,
// where decorationHit (S122e_0055) looks for the mouse.
func (s *botSys) clickWall(blk, side int) {
	e := s.e
	x, y := 88, 60
	if e.SpecialWallTypes[e.Blocks[blk].Walls[side]] == 1 { // door button, as clickedDoorSwitch
		sw := e.DoorSwitches[e.u8(0x59f8+int(e.vis(13).Walls[e.scene.drawVarDown]))%6]
		x, y = sw.X+int(e.X.I16(0x1e75)), sw.Y
		if sw.Shp != nil {
			x, y = x+sw.Shp.W()/2, y+sw.Shp.H/2
		}
	} else if l := int(e.WllShapeMap[e.Blocks[blk].Walls[side]]); l > 0 {
		d := &e.DecoProps[l-1]
		x, y = int(d.ShapeX[1]), int(d.ShapeY[1])
		if sh := e.DecoShapes[d.ShapeIndex[1]]; sh != nil {
			x, y = x+sh.W()/2, y+sh.H/2
		}
		if d.Flags&1 != 0 {
			x = 0xb0 - x
		}
	}
	s.queue = append(s.queue, Event{Kind: EvMouseDown, Button: 1, X: x, Y: y}, Event{Kind: EvMouseUp, Button: 1, X: x, Y: y})
}

// plateFor finds a reachable block whose script opens blk (opcode fd blk).
func (s *botSys) plateFor(blk int) bool {
	e := s.e
	d := e.LevelINF
	for i := 0; i+2 < len(d); i++ {
		if (d[i] != 0xfd && d[i] != 0xfc) || int(le16(d[i+1:])) != blk {
			continue
		}
		best, owner := -1, -1
		for b := range e.Blocks {
			if o := int(e.Blocks[b].Script); o != 0 && o <= i && o > best {
				best, owner = o, b
			}
		}
		if owner < 0 || s.used[owner<<2] || (owner != e.CurrentBlock && s.route(e.CurrentBlock, owner, true) < 0) {
			continue
		}
		s.used[owner<<2] = true
		for k := range s.used { // levers may be needed again once the plate holds
			if k&3 != 0 || k>>2 != owner {
				delete(s.used, k)
			}
		}
		s.final, s.goal, s.drop = s.goal, owner, true
		return true
	}
	return false
}

// dropItem picks up the first hand item with a left click and drops it on
// the floor in front of the party (clickedFloor).
func (s *botSys) dropItem() {
	e := s.e
	for _, k := range e.activeButtons() {
		b := e.ButtonDefs[k]
		if b.CB != cbAddr(0x0acd, 0x0415) || e.Chars[b.Arg].Inventory[0] == 0 {
			continue
		}
		s.queue = append(s.queue, Event{Kind: EvMouseDown, Button: 1, X: b.X + 2, Y: b.Y + 1}, Event{Kind: EvMouseUp, Button: 1, X: b.X + 2, Y: b.Y + 1})
		break
	}
	for _, k := range e.activeButtons() {
		if b := e.ButtonDefs[k]; b.CB == cbAddr(0x0acd, 0x0644) {
			s.queue = append(s.queue, Event{Kind: EvMouseDown, Button: 1, X: b.X + b.W/2, Y: b.Y + b.H/2}, Event{Kind: EvMouseUp, Button: 1, X: b.X + b.W/2, Y: b.Y + b.H/2})
			return
		}
	}
}

func (s *botSys) turnTo(d int) {
	cb := cbAddr(0x0acd, 0x0593)
	if (s.e.CurrentDirection+3)&3 == d {
		cb = cbAddr(0x0acd, 0x056c)
	}
	s.click(cb, -1)
}

// leverFor finds an unused clickable wall whose block script toggles the
// door at blk (opcode fe ea blk) and makes standing in front of it the goal.
func (s *botSys) leverFor(blk int) bool {
	e := s.e
	d := e.LevelINF
	for i := 0; i+3 < len(d); i++ {
		toggle := d[i] == 0xfe && d[i+1] == 0xea && int(le16(d[i+2:])) == blk
		if !toggle && (d[i] != 0xfd || int(le16(d[i+1:])) != blk) {
			continue
		}
		best, owner := -1, -1
		for b := range e.Blocks {
			if o := int(e.Blocks[b].Script); o != 0 && o <= i && o > best {
				best, owner = o, b
			}
		}
		for side := 0; side < 4 && owner >= 0; side++ {
			stand := e.calcNewBlock(owner, side)
			if e.SpecialWallTypes[e.Blocks[owner].Walls[side]] == 0 || s.used[stand<<2|side] ||
				(stand != e.CurrentBlock && s.route(e.CurrentBlock, stand, true) < 0) {
				continue
			}
			s.used[stand<<2|side] = true
			s.final, s.goal, s.face = s.goal, stand, side^2
			return true
		}
	}
	return false
}

func (s *botSys) decide() {
	e := s.e
	s.wait = 8
	if e.GuiMode != 0 {
		s.queue = append(s.queue, Event{Kind: EvKeyDown, Key: KeyEsc})
		return
	}
	cur := e.CurrentBlock
	if s.drop && cur == s.goal {
		s.dropItem()
		s.log = append(s.log, fmt.Sprintf("plate@(%d,%d)", cur&31, cur>>5))
		s.goal, s.drop, s.wait = s.final, false, 30
		return
	}
	if s.face >= 0 && cur == s.goal {
		if e.CurrentDirection != s.face {
			s.turnTo(s.face)
			return
		}
		s.log = append(s.log, fmt.Sprintf("lever@(%d,%d)", cur&31, cur>>5))
		s.clickWall(e.calcNewBlock(cur, s.face), s.face^2)
		s.goal, s.face, s.wait = s.final, -1, 40
		return
	}
	d := s.firstStep(cur)
	if d < 0 {
		s.t.Fatalf("no route from (%d,%d), log %v", cur&31, cur>>5, s.log)
	}
	if e.CurrentDirection != d {
		s.turnTo(d)
		return
	}
	front := e.calcNewBlock(cur, d)
	if e.Blocks[front].Flags&7 != 0 {
		hp := ""
		for i := range e.Monsters {
			if int(e.Monsters[i].Block) == front {
				hp += fmt.Sprint(e.Monsters[i].HPCur, " ")
			}
		}
		for c := 0; c < 6; c++ {
			hp += fmt.Sprint("c", e.Chars[c].HPCur, " ")
		}
		s.log = append(s.log, "fight "+hp)
		for _, k := range e.activeButtons() { // right-click both hands: attack
			if b := e.ButtonDefs[k]; b.CB == cbAddr(0x0acd, 0x0415) {
				for _, y := range []int{b.Y + 1, b.Y + b.H - 2} {
					s.queue = append(s.queue, Event{Kind: EvMouseDown, Button: 2, X: b.X + b.W/2, Y: y}, Event{Kind: EvMouseUp, Button: 2, X: b.X + b.W/2, Y: y})
				}
			}
		}
		s.wait = 30
		return
	}
	if cur == s.last {
		s.tries++
	} else {
		s.tries, s.last = 0, cur
		s.log = append(s.log, fmt.Sprintf("(%d,%d)", cur&31, cur>>5))
	}
	if w := e.Blocks[front].Walls[e.u8(dsOppositeSide+d)]; e.SpecialWallTypes[w] == 6 {
		if !s.leverFor(front) && !s.plateFor(front) {
			s.blocked[front] = true
			s.log = append(s.log, fmt.Sprintf("blocked(%d,%d)", front&31, front>>5))
		}
		return
	}
	if e.Timers[timerDoor].Enabled { // door still moving
		s.tries--
		return
	}
	if s.tries > 0 && s.tries%2 == 1 && e.WllWallFlags[e.Blocks[front].Walls[e.u8(dsOppositeSide+d)]]&1 == 0 {
		s.clickWall(front, d^2) // door button / lever in front
		s.wait = 40
		return
	}
	if s.tries > 12 {
		s.t.Fatalf("stuck at (%d,%d) dir %d, front wall %#x, log %v", cur&31, cur>>5, d, e.Blocks[front].Walls[e.u8(dsOppositeSide+d)], s.log)
	}
	s.click(cbAddr(0x0acd, 0x04f8), -1)
}

func TestPlaythroughLevel1(t *testing.T) {
	s := &botSys{t: t, goal: 0x2f2, last: -1, face: -1, used: map[int]bool{}, blocked: map[int]bool{}}
	e, err := New(testDir, s)
	if err != nil {
		t.Skip(err)
	}
	s.e = e
	if err := e.Run(testSave); err != nil {
		t.Fatal(err)
	}
	t.Logf("route %v", s.log)
	if p := os.Getenv("EOB_SHOT"); p != "" {
		s.savePNG(p)
	}
	if e.CurrentLevel != 2 {
		t.Fatalf("level %d at (%d,%d) after %d decisions, dead=%v", e.CurrentLevel, e.CurrentBlock&31, e.CurrentBlock>>5, s.polls, s.partyDead())
	}
}
