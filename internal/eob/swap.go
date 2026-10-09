package eob

const dsListSwap = 0x5446 // 7 buttons active while swapping

// clickedCharName ports S0acd_0264: first click selects, second swaps.
func (e *Engine) clickedCharName(c int) bool {
	if !e.charStatus(c, 1) {
		return true
	}
	if e.SpellbookOpen {
		e.closeSpellbook()
	}
	if e.SwapChar == -1 {
		e.SwapChar = int8(c)
		e.drawCharPanel(c)
		e.enableTimer(0, true)
		return true
	}
	src := int(e.SwapChar)
	e.Chars[src], e.Chars[c] = e.Chars[c], e.Chars[src]
	e.enableTimer(0, false)
	e.SwapChar = -1
	e.drawCharPanel(src)
	e.selectChar(c)
	e.drawCharPanel(c)
	e.rebuildCharTimers()
	return true
}

// cancelSwap ports S0acd_0393.
func (e *Engine) cancelSwap() bool {
	e.enableTimer(0, false)
	c := int(e.SwapChar)
	e.SwapChar = -1
	if c >= 0 {
		e.drawCharPanel(c)
	}
	return true
}

// timerSwapBlink ports S1414_0000.
func (e *Engine) timerSwapBlink(int) {
	e.swapBlink = !e.swapBlink
	c := int(e.SwapChar)
	if c < 0 {
		return
	}
	if e.swapBlink {
		e.SwapChar = -1
		e.drawCharPanel(c)
		e.SwapChar = int8(c)
	} else {
		e.drawCharPanel(c)
	}
}

// selectChar ports S0d1c_003f.
func (e *Engine) selectChar(c int) {
	if int(e.SelectedChar) == c {
		return
	}
	old := int(e.SelectedChar)
	e.SelectedChar = int8(c)
	e.drawCharPanel(old)
	e.drawCharPanel(c)
}

// rebuildCharTimers ports S1414_0b2e: re-arm timers 6-11 from each
// character's pending events.
func (e *Engine) rebuildCharTimers() {
	for c := range e.Chars {
		if !e.charStatus(c, 1) {
			continue
		}
		min := uint32(0xffffffff)
		for _, v := range e.Chars[c].Timers {
			if v != 0 && v < min {
				min = v
			}
		}
		t := &e.Timers[6+c]
		if min != 0xffffffff {
			t.Interval = min - e.ticks
			e.enableTimer(6+c, true)
		} else {
			t.Interval = min
			e.enableTimer(6+c, false)
		}
	}
}
