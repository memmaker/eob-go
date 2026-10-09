package eob

import "testing"

func TestRollStats(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	for race := 0; race < 6; race++ {
		for cls := 0; cls < 15; cls++ {
			ch := &e.Chars[0]
			ch.RaceSex, ch.Class = uint8(race<<1), uint8(cls)
			e.cg.slot = 0
			e.cgRollStats(0)
			v := []int{int(ch.StrCur), int(ch.IntCur), int(ch.WisCur), int(ch.DexCur), int(ch.ConCur), int(ch.ChaCur)}
			for i, s := range v {
				if e.cg.min[i] > e.cg.max[i] {
					continue // illegal race/class combination
				}
				if s < e.cg.min[i] || s > e.cg.max[i] {
					t.Errorf("race %d class %d stat %d = %d not in [%d,%d]", race, cls, i, s, e.cg.min[i], e.cg.max[i])
				}
			}
			if ch.HPMax <= 0 || ch.HPCur != ch.HPMax || ch.Level[0] == 0 {
				t.Errorf("race %d class %d: hp %d/%d level %v", race, cls, ch.HPCur, ch.HPMax, ch.Level)
			}
		}
	}
}

type evSys struct {
	fakeSys
	ev []Event
}

func (s *evSys) PollEvent() (Event, bool) {
	if len(s.ev) == 0 {
		return Event{}, false
	}
	ev := s.ev[0]
	s.ev = s.ev[1:]
	return ev, true
}

func TestInputString(t *testing.T) {
	s := &evSys{}
	e, err := New(testDir, s)
	if err != nil {
		t.Skip(err)
	}
	for _, r := range "gandalfµµx³\x7f" { // µ (0xb5) left, ³ (0xb3) right, 0x7f delete
		s.ev = append(s.ev, Event{Kind: EvKeyDown, Ascii: r})
	}
	s.ev = append(s.ev, Event{Kind: EvKeyDown, Key: KeyBack}, Event{Kind: EvKeyDown, Key: KeyEnter})
	if err := e.InitResources(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 11)
	if n := e.inputString(buf, 0x18, 0x64, 10, 0xf, 0, 8); n != 6 || cstr(buf) != "GANDAX" {
		t.Errorf("got %d %q, want 6 GANDAX", n, cstr(buf))
	}
}
