package pcspk

import (
	"os"
	"testing"

	"github.com/felix/goeob/internal/pak"
)

const testDir = "/Users/felix/Projects/EOB/Eye of the Beholder.app/Contents/Resources/game"

func load(t *testing.T, voices int) *Player {
	fs, err := pak.Open(testDir)
	if err != nil {
		t.Skip(err)
	}
	d, err := fs.Read("PCSOUND.DAT")
	if err != nil {
		t.Skip(err)
	}
	exe, err := os.ReadFile(testDir + "/EOB.EXE")
	if err != nil {
		t.Skip(err)
	}
	p, err := New(d, exe, 22050, voices)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// Every mapped sound gates the speaker on and finishes within 30 s (program 11, id 2, is a 19 s tune). PCSOUND.DAT maps 13 ids to its programs 1-13 (program 0 is silent).
func TestPlayAll(t *testing.T) {
	for _, voices := range []int{1, 3} {
		p := load(t, voices)
		if p.div[1] != 65023 || p.div[95] != 285 {
			t.Fatalf("divisors %d..%d", p.div[1], p.div[95])
		}
		buf := make([]byte, 4*2205)
		played := 0
		for id := range 120 {
			if p.ids[id] >= numPrograms {
				continue
			}
			p.Play(id)
			sound := false
			for i := 0; i < 300 && p.Busy(); i++ {
				p.Read(buf)
				for j := 0; j < len(buf) && !sound; j += 4 {
					sound = buf[j] != 0 || buf[j+1] != 0
				}
			}
			if p.Busy() {
				t.Errorf("voices %d id %d (program %d) still playing after 30 s", voices, id, p.ids[id])
			}
			if !sound && p.ids[id] != 0 {
				t.Logf("silent: id %d program %d", id, p.ids[id])
			}
			if sound {
				played++
			}
		}
		if played != 13 {
			t.Errorf("voices %d: %d sounds made noise, want 13", voices, played)
		}
		t.Logf("voices %d: %d sounds", voices, played)
	}
}
