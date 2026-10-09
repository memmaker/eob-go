package eob

// Timer is an original 16-byte timer slot (ds:5ba8, 16 entries).
type Timer struct {
	CB       uint32 // original callback seg<<16|off
	Interval uint32 // ticks (18.2 Hz)
	Next     uint32
	Param    uint16
	Extra    int8
	Enabled  bool
}

const (
	dsTimers   = 0x5ba8
	numTimers  = 16
	timerDoor  = 12
	tickMillis = 55
	turnTicks  = 5 // one turn ≈ 275ms, like DeepCrawlerForge's 250ms
)

func (e *Engine) loadTimers() {
	for i := 0; i < numTimers; i++ {
		o := dsTimers + i*16
		x := e.X
		e.Timers[i] = Timer{CB: uint32(x.U16(o+2))<<16 | uint32(x.U16(o)), Interval: uint32(x.U16(o+4)) | uint32(x.U16(o+6))<<16,
			Param: x.U16(o + 0xc), Extra: x.I8(o + 0xe), Enabled: x.U8(o+0xf) != 0}
	}
}

// Ticks returns the game tick counter (ds:8492).
func (e *Engine) Ticks() uint32 { return e.ticks }

// setTimer ports S1414_055b.
func (e *Engine) setTimer(id, param, extra int, enable bool) {
	t := &e.Timers[id]
	t.Enabled = enable
	t.Next = e.ticks + t.Interval
	t.Param = uint16(param)
	t.Extra = int8(extra)
}

// enableTimer ports S1414_05a4.
func (e *Engine) enableTimer(id int, on bool) {
	t := &e.Timers[id]
	t.Enabled = on
	t.Next = e.ticks + t.Interval
}

// processTimers ports S1414_04e7.
func (e *Engine) processTimers() {
	for i := range e.Timers {
		t := &e.Timers[i]
		if !t.Enabled || t.Next >= e.ticks {
			continue
		}
		if h, ok := e.timerHandlers[t.CB]; ok {
			h(int(t.Param))
		}
		if t.Enabled {
			t.Next = e.ticks + t.Interval
		}
	}
	e.updateMonstersPost()
}

func (e *Engine) initTimerHandlers() {
	e.timerHandlers = map[uint32]func(int){
		cbAddr(0x1414, 0x0000): e.timerSwapBlink,
		cbAddr(0x1414, 0x03b4): e.timerDoorStep,
		cbAddr(0x1414, 0x05df): e.updateMonsters, // ports S1414_05df (thunk to S08e6_0cc8)
		cbAddr(0x1414, 0x0285): e.timerFlyingObjects,
		cbAddr(0x1414, 0x07ca): e.timerCharacter,
		cbAddr(0x1414, 0x0ce7): e.monsterIdleJitter,
		cbAddr(0x1414, 0x0c82): e.timerFood,
		cbAddr(0x1414, 0x04b0): e.timerSpecialWall,
	}
}

// timerDoorStep ports S1414_03b4: one door animation step.
func (e *Engine) timerDoorStep(block int) {
	t := &e.Timers[timerDoor]
	dir := int(t.Extra)
	b := &e.Blocks[block]
	di := 1
	if e.WllWallFlags[b.Walls[0]]&8 != 0 {
		di = 0
	}
	f := e.WllWallFlags[b.Walls[di]]
	if (dir == -1 && f&0x20 != 0) || (dir == 1 && f&0x10 != 0) {
		e.enableTimer(timerDoor, false)
		return
	}
	b.Walls[di] = byte(int(b.Walls[di]) + dir)
	b.Walls[di+2] = byte(int(b.Walls[di+2]) + dir)
	snd := 3
	if e.WllWallFlags[b.Walls[di]]&0x20 != 0 {
		snd = 5
	} else if dir == -1 {
		snd = 4
	}
	e.playSoundUpdate(snd)
	e.updateBlockIfVisible(block)
}

func (e *Engine) doorBusy(block int) bool {
	t := &e.Timers[timerDoor]
	return t.Enabled && int(t.Param) == block
}

// openDoor ports S122e_0154.
func (e *Engine) openDoor(block int) {
	t := &e.Timers[timerDoor]
	if t.Enabled {
		if int(t.Param) == block {
			t.Extra = 1
		}
		return
	}
	b := &e.Blocks[block]
	di := 1
	if e.WllWallFlags[b.Walls[0]]&8 != 0 {
		di = 0
	}
	if e.WllWallFlags[b.Walls[di]]&1 != 0 {
		return
	}
	e.setTimer(timerDoor, block, 1, true)
	e.timerDoorStep(block)
	e.setTimer(timerDoor, block, 1, true)
}

// closeDoor ports S122e_01d9.
func (e *Engine) closeDoor(block int) {
	if e.Blocks[block].Flags&7 != 0 {
		return
	}
	t := &e.Timers[timerDoor]
	if t.Enabled {
		if int(t.Param) == block {
			t.Extra = -1
		}
		return
	}
	b := &e.Blocks[block]
	di := 1
	if e.WllWallFlags[b.Walls[0]]&8 != 0 {
		di = 0
	}
	if e.WllWallFlags[b.Walls[di]]&1 == 0 {
		return
	}
	e.setTimer(timerDoor, block, -1, true)
	e.timerDoorStep(block)
	e.setTimer(timerDoor, block, -1, true)
}

// toggleDoor ports S122e_0271 (side == -1: auto).
func (e *Engine) toggleDoor(block, side int) {
	t := &e.Timers[timerDoor]
	if t.Enabled {
		if int(t.Param) == block {
			t.Extra = -t.Extra
		}
		return
	}
	b := &e.Blocks[block]
	if side == -1 {
		side = 1
		if e.WllWallFlags[b.Walls[0]]&8 != 0 {
			side = 0
		}
	}
	dir := 1
	if e.WllWallFlags[b.Walls[side]]&1 != 0 {
		dir = -1
		if b.Flags&1 != 0 {
			return
		}
	}
	e.setTimer(timerDoor, block, dir, true)
	e.timerDoorStep(block)
	e.setTimer(timerDoor, block, dir, true)
}

// updateBlockIfVisible is S0d1c_0006 called for its redraw side effect.
func (e *Engine) updateBlockIfVisible(block int) { e.blockVisible(block) }

// timerFood ports S1414_0c82.
func (e *Engine) timerFood(int) {
	for c := 0; c < 6; c++ {
		ch := &e.Chars[c]
		// original tests wearsRing(c, 2) < 0, which never holds (0/1 result)
		if ch.Food == 0 || ch.Flags&1 == 0 || ch.HPCur <= -10 {
			continue
		}
		ch.Food--
		e.drawFood(c)
	}
}

// timerSpecialWall ports S1414_04b0.
func (e *Engine) timerSpecialWall(int) {
	e.scene.specialWallFlip ^= 1
	for i := 0; i < 18; i++ {
		if e.vis(i).Walls[e.scene.drawVarDown] == 0x34 {
			e.sceneUpdate = true
			return
		}
	}
}
