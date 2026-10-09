package adlib

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/felix/goeob/internal/pak"
)

const testDir = "/Users/felix/Projects/EOB/Eye of the Beholder.app/Contents/Resources/game"

func TestPlayAllSounds(t *testing.T) { testPlayAll(t, "ADLIB.DAT", "EOB.EXE") }

// TestPlayAllMusic: SOUND.DAT (INTRO.EXE music) shares ADLIB.DAT's format.
func TestPlayAllMusic(t *testing.T) { testPlayAll(t, "SOUND.DAT", "INTRO.EXE") }

func TestLoadDriver(t *testing.T) {
	for _, f := range []string{"EOB.EXE", "INTRO.EXE"} {
		exe, err := os.ReadFile(testDir + "/" + f)
		if err != nil {
			t.Skip(err)
		}
		p := &Player{}
		if !p.LoadDriver(exe) {
			t.Fatalf("%s: driver not found", f)
		}
		want := []uint16{0x134, 0x147, 0x15a, 0x16f, 0x184, 0x19c, 0x1b4, 0x1ce, 0x1e9, 0x207, 0x225, 0x246}
		for n, w := range want {
			if p.freq(n) != w {
				t.Fatalf("%s: freq[%d]=%#x want %#x", f, n, p.freq(n), w)
			}
		}
		// Row 2 (note 0, upward bend) starts 0,1,2,3...
		if p.bendTable(2, 0) != 0 || p.bendTable(2, 3) != 3 {
			t.Fatalf("%s: bend table row 2 = %d %d", f, p.bendTable(2, 0), p.bendTable(2, 3))
		}
	}
}

func testPlayAll(t *testing.T, name, exeName string) {
	fs, err := pak.Open(testDir)
	if err != nil {
		t.Skip(err)
	}
	d, err := fs.Read(name)
	if err != nil {
		t.Skip(err)
	}
	p, err := New(d, 22050)
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.ReadFile(testDir + "/" + exeName)
	if err != nil || !p.LoadDriver(exe) {
		t.Fatal("driver not found in", exeName)
	}
	buf := make([]byte, 4*22050/4)
	silent := 0
	for id := range 120 {
		if d[id] >= numPrograms {
			continue
		}
		p.Play(id)
		peak := 0
		for range 8 { // 2s
			p.Read(buf)
			for i := 0; i < len(buf); i += 4 {
				s := int(int16(binary.LittleEndian.Uint16(buf[i:])))
				peak = max(peak, s, -s)
			}
		}
		if peak < 500 {
			silent++
			t.Logf("sound %#x (program %d) peak %d", id, d[id], peak)
		}
	}
	if silent > 10 {
		t.Fatalf("%d sounds silent", silent)
	}
}
