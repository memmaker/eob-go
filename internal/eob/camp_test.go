package eob

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestSaveRoundTrip(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testSave); err != nil {
		t.Skip(err)
	}
	e.camp.count[0] = 2 // memorize two of spell 1, none ready yet
	e.camp.spellLevel[0] = 1
	e.memorizeApply(0, false)
	if e.Chars[0].MageSpells[0] != -1 || e.Chars[0].MageSpells[1] != -1 || !e.camp.needRest {
		t.Fatalf("memorize: %v", e.Chars[0].MageSpells[:6])
	}
	a, b := filepath.Join(t.TempDir(), "a.sav"), filepath.Join(t.TempDir(), "b.sav")
	if err := e.SaveGame(a); err != nil {
		t.Fatal(err)
	}
	if err := e.LoadOriginalSave(a); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveGame(b); err != nil {
		t.Fatal(err)
	}
	x, _ := os.ReadFile(a)
	y, _ := os.ReadFile(b)
	if !bytes.Equal(x, y) {
		t.Fatal("save/load/save differs")
	}
}

type fakeSys struct{ ms uint32 }

func (s *fakeSys) Present(pix, pal []byte)  {}
func (s *fakeSys) PollEvent() (Event, bool) { return Event{}, false }
func (s *fakeSys) Mouse() (int, int)        { return 0, 0 }
func (s *fakeSys) Millis() uint32           { s.ms += 1000; return s.ms }
func (s *fakeSys) Sleep(ms int)             {}

func TestRestLearnsSpells(t *testing.T) {
	e, err := New(testDir, &fakeSys{})
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testSave); err != nil {
		t.Skip(err)
	}
	e.loadSpells()
	c := -1
	for i := range e.Chars {
		ch := &e.Chars[i]
		ch.HPCur, ch.Flags = ch.HPMax, ch.Flags&^2
		if c == -1 && e.charStatus(i, 5) && e.classPartIndex(int(ch.Class), 2) != -1 {
			c = i
		}
	}
	if c == -1 {
		t.Skip("no cleric")
	}
	for i := range e.Monsters {
		e.Monsters[i].Block = uint16(e.CurrentBlock ^ 0x210)
	}
	e.Chars[c].Food = 100
	e.Chars[c].ClericSpells[0] = -1
	t0 := e.ticks
	if e.restParty() {
		t.Fatal("interrupted")
	}
	if e.Chars[c].ClericSpells[0] != 1 || e.ticks == t0 {
		t.Fatalf("spell %d ticks %d", e.Chars[c].ClericSpells[0], e.ticks-t0)
	}
}

func TestAmbientSound(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.Opt80e6, e.CurrentLevel, e.ticks, e.ambientNext = 1, 2, 100, 50
	e.ambientSound()
	if e.pendingSound != 0x1e || e.ambientNext < 100+4*18 || e.ambientNext > 100+13*18 {
		t.Fatalf("sound %x next %d", e.pendingSound, e.ambientNext)
	}
	e.pendingSound, e.CurrentLevel, e.ambientNext = 0, 4, 0
	e.ambientSound()
	if e.pendingSound != 0 {
		t.Fatal("ambient on level 4")
	}
}
