package eob

import "testing"

// turnSys sends Space, idles, presses turn-right, idles, then quits,
// sampling game time between.
type turnSys struct {
	botSys
	e     *Engine
	n     int
	ticks []uint32
}

func (s *turnSys) PollEvent() (Event, bool) {
	s.n++
	switch s.n {
	case 1:
		return Event{Kind: EvKeyDown, Key: KeySpace}, true
	case 2, 400, 800:
		s.ticks = append(s.ticks, s.e.ticks)
		if s.n == 400 {
			return Event{Kind: EvKeyDown, Key: KeyKP9}, true
		}
		if s.n == 800 {
			return Event{Kind: EvQuit}, true
		}
	}
	s.now += 20
	return Event{}, false
}

func TestTurnBasedMode(t *testing.T) {
	s := &turnSys{}
	e, err := New(testDir, s)
	if err != nil {
		t.Skip(err)
	}
	s.e = e
	if err := e.Run(testSave); err != nil {
		t.Fatal(err)
	}
	if s.ticks[1] != s.ticks[0] || s.ticks[2]-s.ticks[1] != turnTicks {
		t.Fatalf("ticks %v: want frozen, then +%d after one action", s.ticks, turnTicks)
	}
}
