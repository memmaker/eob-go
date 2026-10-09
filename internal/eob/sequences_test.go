package eob

import "testing"

type keySys struct{ keys []int }

func (s *keySys) Present(pix, pal []byte) {}
func (s *keySys) PollEvent() (Event, bool) {
	if len(s.keys) == 0 {
		return Event{Kind: EvQuit}, true
	}
	k := s.keys[0]
	s.keys = s.keys[1:]
	return Event{Kind: EvKeyDown, Key: k}, true
}
func (s *keySys) Mouse() (int, int) { return 0, 0 }
func (s *keySys) Millis() uint32    { return 0 }
func (s *keySys) Sleep(ms int)      {}

// The dwarf (NPC 6) accepted with "yes" joins in the first free slot.
func TestNPCJoin(t *testing.T) {
	e, err := New(testDir, &keySys{keys: []int{KeyEnter, KeyEnter}})
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testDir + "/EOBDATA.SAV"); err != nil {
		t.Skip(err)
	}
	free := -1
	for c := range e.Chars {
		if !e.Chars[c].Active() {
			free = c
			break
		}
	}
	if free < 0 {
		t.Skip("party full")
	}
	e.mustCPS(e.X.Str(0x7f07), seqGfxPage)
	e.loadTextDat()
	e.setDim(9)
	e.npcJoin(6, 0xc, 0x17, 2)
	ch := &e.Chars[free]
	if !ch.Active() || ch.Face == nil || ch.NameStr() != cstr(e.X.Bytes(dsNPCChars+6*charSize+2, 11)) {
		t.Fatalf("slot %d: active=%v name=%q", free, ch.Active(), ch.NameStr())
	}
}
