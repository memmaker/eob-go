package eob

import (
	"os"
	"testing"
)

// cursorSys is a System that shows the pointer itself, or lets the engine draw it.
type cursorSys struct {
	hw        bool
	frame     []byte
	pix       []byte
	mask      []bool
	w, h      int
	hotX, hot int
}

func (s *cursorSys) Present(pix, pal []byte)  { s.frame = append(s.frame[:0], pix...) }
func (s *cursorSys) PollEvent() (Event, bool) { return Event{}, false }
func (s *cursorSys) Mouse() (int, int)        { return 100, 80 }
func (s *cursorSys) Millis() uint32           { return 0 }
func (s *cursorSys) Sleep(int)                {}
func (s *cursorSys) HardwareCursor() bool     { return s.hw }
func (s *cursorSys) ShowCursor(pix []byte, mask []bool, w, h, hotX, hotY int, pal []byte) {
	s.pix, s.mask, s.w, s.h, s.hotX, s.hot = pix, mask, w, h, hotX, hotY
}

// The pointer handed to a CursorShower is what the engine draws without one: the arrow
// and an item in hand.
func TestHardwareCursor(t *testing.T) {
	dir := os.Getenv("EOB_DATA")
	if dir == "" {
		t.Skip("EOB_DATA not set")
	}
	sys := &cursorSys{}
	e, err := New(dir, sys)
	if err != nil {
		t.Fatal(err)
	}
	e.InitResources()
	for _, item := range []int{0, 1} {
		e.HandItem = item
		sys.hw = false
		e.present()
		drawn := append([]byte(nil), sys.frame...)
		sys.hw = true
		e.present()
		if sys.pix == nil {
			t.Fatalf("item %d: no hardware cursor", item)
		}
		opaque := 0
		for y := range sys.h {
			for x := range sys.w {
				i := y*sys.w + x
				o := (80-sys.hot+y)*320 + 100 - sys.hotX + x
				if sys.mask[i] {
					opaque++
					if drawn[o] != sys.pix[i] {
						t.Fatalf("item %d (%d, %d): %d, drawn %d", item, x, y, sys.pix[i], drawn[o])
					}
				} else if drawn[o] != sys.frame[o] {
					t.Fatalf("item %d (%d, %d): clear, but drawn %d", item, x, y, drawn[o])
				}
			}
		}
		t.Logf("item %d: %dx%d, hot spot %d,%d, %d opaque", item, sys.w, sys.h, sys.hotX, sys.hot, opaque)
	}
}
