package opl

import "testing"

func TestNote(t *testing.T) {
	c := New(44100)
	for _, rv := range [][2]byte{{0x01, 0x20}, {0x20, 0x01}, {0x23, 0x01}, {0x40, 0x10}, {0x43, 0x00},
		{0x60, 0xf0}, {0x63, 0xf0}, {0x80, 0x77}, {0x83, 0x77}, {0xa0, 0x98}, {0xb0, 0x31}} {
		c.Write(rv[0], rv[1])
	}
	buf := make([]int16, 4410)
	c.Generate(buf)
	peak := 0
	for _, s := range buf {
		peak = max(peak, abs(int(s)))
	}
	if peak < 1000 {
		t.Fatalf("peak %d, want audible note", peak)
	}
	c.Write(0xb0, 0x11) // key off; release rate 7 decays within ~1s
	c.Generate(make([]int16, 44100))
	c.Generate(buf)
	for _, s := range buf {
		if abs(int(s)) > 10 {
			t.Fatalf("still sounding after release: %d", s)
		}
	}
}

func abs(x int) int { return max(x, -x) }
