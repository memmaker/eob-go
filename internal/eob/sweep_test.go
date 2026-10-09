package eob

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
)

// sweepSys drives Run through every level: for each block that can be
// entered from a neighbour it teleports next to it, walks in with the
// forward button (so the block's script, pits and teleporters fire), fights
// anything in the way and checks engine invariants after each step.
type sweepSys struct {
	t        *testing.T
	e        *Engine
	level    int
	now      uint32
	queue    []Event
	wait     int
	steps    []int // pending (block<<2 | dir) moves
	step     int   // current move, -1 none
	phase    int
	fights   int
	stats    sweepStats
	started  bool
	tries    int
	safe     int          // a script-free standable block of s.level
	scripted map[int]bool // blocks of s.level with scripts
}

type sweepStats struct {
	moves, entered, scripted, levelChanges, fights, frames int
}

func (s *sweepSys) Present(pix, pal []byte) { s.stats.frames++ }
func (s *sweepSys) Mouse() (int, int)       { return 0, 0 }
func (s *sweepSys) Millis() uint32          { return s.now }
func (s *sweepSys) Sleep(ms int)            { s.now += uint32(ms) }

func (s *sweepSys) PollEvent() (Event, bool) {
	if len(s.queue) > 0 {
		ev := s.queue[0]
		s.queue = s.queue[1:]
		return ev, true
	}
	if s.wait > 0 {
		s.wait--
		return Event{}, false
	}
	return s.next()
}

func (s *sweepSys) press(x, y, btn int) {
	s.queue = append(s.queue, Event{Kind: EvMouseDown, Button: btn, X: x, Y: y}, Event{Kind: EvMouseUp, Button: btn, X: x, Y: y})
}

func (s *sweepSys) clickCB(cb uint32) {
	for _, k := range s.e.activeButtons() {
		if b := s.e.ButtonDefs[k]; b.CB == cb {
			s.press(b.X+b.W/2, b.Y+b.H/2, 1)
			return
		}
	}
}

// enterable reports whether moving from b in direction d can enter the
// neighbouring block (the wall it shows toward b is passable or a door).
func (s *sweepSys) enterable(b, d int) bool {
	e := s.e
	w := e.Blocks[e.calcNewBlock(b, d)].Walls[e.u8(dsOppositeSide+d)]
	return e.WllWallFlags[w]&1 != 0
}

func (s *sweepSys) plan() {
	e := s.e
	seen := map[int]bool{}
	s.steps = s.steps[:0]
	s.scripted = map[int]bool{}
	for b := range e.Blocks {
		if e.Blocks[b].Script != 0 {
			s.scripted[b] = true
		} else if s.safe == 0 && s.standable(b) {
			s.safe = b
		}
	}
	for b := range e.Blocks {
		for d := 0; d < 4; d++ {
			nb := e.calcNewBlock(b, d)
			if seen[nb] || !s.enterable(b, d) || !s.standable(b) {
				continue
			}
			seen[nb] = true
			s.steps = append(s.steps, b<<2|d)
		}
	}
}

// standable: some neighbour can step onto b.
func (s *sweepSys) standable(b int) bool {
	for d := 0; d < 4; d++ {
		if s.enterable(s.e.calcNewBlock(b, d), d^2) {
			return true
		}
	}
	return false
}

func (s *sweepSys) heal() {
	e := s.e
	for c := range e.Chars {
		ch := &e.Chars[c]
		if ch.Flags&1 == 0 {
			continue
		}
		ch.HPCur = ch.HPMax
		ch.Food = 100
	}
}

func (s *sweepSys) next() (Event, bool) {
	e := s.e
	s.heal()
	s.check()
	if !s.started {
		s.started = true
		e.scriptLoadNewLevel(s.level, e.CurrentBlock, e.CurrentDirection)
		s.plan()
		s.wait = 4
		return Event{}, false
	}
	if e.GuiMode != 0 {
		s.queue = append(s.queue, Event{Kind: EvKeyDown, Key: KeyEsc})
		s.wait = 2
		return Event{}, false
	}
	if e.CurrentLevel != s.level {
		s.stats.levelChanges++
		back := e.CurrentBlock
		if s.step >= 0 {
			back = s.step >> 2
		}
		if s.scripted[back] { // e.Blocks is the other level now: use the plan-time map
			back = s.safe
		}
		e.scriptLoadNewLevel(s.level, back, 0)
		s.phase, s.wait = 0, 4
		return Event{}, false
	}
	switch s.phase {
	case 0: // teleport next to the target
		if s.step >= 0 && e.CurrentBlock == e.calcNewBlock(s.step>>2, s.step&3) {
			s.stats.entered++
		} else if s.step >= 0 && os.Getenv("EOB_SWEEPDBG") != "" {
			t := e.calcNewBlock(s.step>>2, s.step&3)
			s.t.Logf("miss %#x->%#x d%d now %#x dir%d wall %#x flags %x", s.step>>2, t, s.step&3, e.CurrentBlock, e.CurrentDirection, e.Blocks[t].Walls[e.u8(dsOppositeSide+(s.step&3))], e.Blocks[t].Flags)
		}
		if len(s.steps) == 0 {
			return Event{Kind: EvQuit}, true
		}
		s.step, s.steps = s.steps[0], s.steps[1:]
		s.phase, s.fights, s.wait, s.tries = 2, 0, 3, 0
		return Event{Kind: EvDebugGoto, X: s.step >> 2, Y: s.step & 3}, true
	case 2: // a modal sequence (NPC dialogue) swallows the goto: answer it and retry
		if e.CurrentBlock != s.step>>2 && s.tries < 10 {
			s.tries++
			s.queue = append(s.queue, Event{Kind: EvKeyDown, Key: KeyEnter}, Event{Kind: EvDebugGoto, X: s.step >> 2, Y: s.step & 3})
			s.wait = 3
			return Event{}, false
		}
		s.phase = 1
		return Event{}, false
	case 1: // walk in, fighting whatever blocks the way
		front := e.calcNewBlock(e.CurrentBlock, e.CurrentDirection)
		if e.Blocks[front].Flags&7 != 0 && s.fights < 20 {
			s.fights++
			s.stats.fights++
			for _, k := range e.activeButtons() {
				if b := e.ButtonDefs[k]; b.CB == cbAddr(0x0acd, 0x0415) {
					s.press(b.X+b.W/2, b.Y+1, 2)
				}
			}
			s.wait = 15
			return Event{}, false
		}
		s.stats.moves++
		if e.Blocks[front].Script != 0 {
			s.stats.scripted++
		}
		if s.phase = 0; s.stats.moves%7 == 0 { // also poke the wall ahead
			s.press(88, 60, 1)
		}
		s.clickCB(cbAddr(0x0acd, 0x04f8))
		s.wait = 12
		return Event{}, false
	}
	return Event{}, false
}

// check verifies item lists and monster bookkeeping.
func (s *sweepSys) check() {
	e := s.e
	if e.CurrentBlock < 0 || e.CurrentBlock >= 1024 {
		s.t.Fatalf("level %d: party block %d out of range", s.level, e.CurrentBlock)
	}
	for b := range e.Blocks {
		for d, w := range e.Blocks[b].Walls {
			if int(w) >= numWallTypes {
				s.t.Fatalf("level %d: block (%d,%d) side %d has wall %d after step %#x", s.level, b&31, b>>5, d, w, s.step)
			}
		}
		h := int(e.Blocks[b].ItemHead)
		for i, n := h, 0; h != 0; n++ {
			if i <= 0 || i >= len(e.Items) || n > len(e.Items) {
				s.t.Fatalf("level %d block %d: broken item list at %d", s.level, b, i)
			}
			if int(e.Items[i].Block) != b {
				s.t.Fatalf("level %d: item %d in list of block %d says block %d", s.level, i, b, e.Items[i].Block)
			}
			if i = int(e.Items[i].Next); i == h {
				break
			}
		}
	}
	for i := range e.Monsters {
		if m := &e.Monsters[i]; m.HPCur > 0 && m.Block >= 1024 {
			s.t.Fatalf("level %d: monster %d at block %d", s.level, i, m.Block)
		}
	}
}

// TestLevelSweep enters every enterable block of every level (or the
// levels in EOB_SWEEP, e.g. "3,7") through the real game loop.
func TestLevelSweep(t *testing.T) {
	levels := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
	if v := os.Getenv("EOB_SWEEP"); v != "" {
		levels = levels[:0]
		for _, f := range strings.Split(v, ",") {
			n, _ := strconv.Atoi(f)
			levels = append(levels, n)
		}
	} else if testing.Short() {
		t.Skip("long test")
	}
	for _, lv := range levels {
		t.Run(fmt.Sprint("level", lv), func(t *testing.T) {
			s := &sweepSys{t: t, level: lv, step: -1}
			e, err := New(testDir, s)
			if err != nil {
				t.Skip(err)
			}
			s.e = e
			if err := e.Run(testDir + "/EOBDATA.SAV"); err != nil {
				t.Fatal(err)
			}
			t.Logf("level %d: %+v", lv, s.stats)
			if s.stats.entered < s.stats.moves/2 {
				t.Fatalf("only %d of %d moves entered their block", s.stats.entered, s.stats.moves)
			}
		})
	}
}
