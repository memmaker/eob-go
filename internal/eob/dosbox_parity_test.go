package eob

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Differential tests against saves written by the original EOB.EXE under
// DOSBox-X (~/Tools/eob-dosbox, see its README). Each case replays the
// keys that were sent to the original from the title menu, starting from
// the pristine EOBDATA.orig.SAV, and compares the EOBDATA.SAV Go writes.
const dosboxDir = "/Users/felix/Tools/eob-dosbox"

const keyC = 48 // Westwood key code of C (camp button)

func TestDOSBoxSaveParity(t *testing.T) {
	// 0 entries are idle polls standing in for the waits between keys sent
	// to the original (menus draw their highlight before reading input).
	const w = 0
	cases := []struct {
		ref  string
		keys []int
	}{
		// load game, camp, Game Options, Save Game, Yes
		{"orig_roundtrip.SAV", []int{KeyEnter, w, w, keyC, w, w, KeyKP2, KeyKP2, KeyKP2, KeyKP2, KeyKP2, KeyEnter, w, w, KeyKP2, KeyEnter, w, w, KeyEnter, w, w}},
		// load game, then by mouse: camp, Preferences, Bar Graphs, Sounds, Exit,
		// Game Options, Save Game, Yes
		{"orig_prefs_mouse.SAV", []int{KeyEnter, w, w, click(299, 186, 1), w, w, click(80, 95, 1), w, w,
			click(80, 61, 1), w, w, click(80, 44, 1), w, w, click(148, 128, 1), w, w, click(80, 112, 1), w, w,
			click(80, 44, 1), w, w, click(32, 63, 1), w, w}},
	}
	for _, c := range cases {
		t.Run(c.ref, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join(dosboxDir, c.ref))
			if err != nil {
				t.Skip(err)
			}
			got := runForSave(t, c.keys)
			e, _ := New(testDir, nil)
			reportSaveDiff(t, e, got, want)
		})
	}
}

func runForSave(t *testing.T, keys []int) []byte {
	dir := t.TempDir()
	ents, err := os.ReadDir(testDir)
	if err != nil {
		t.Skip(err)
	}
	for _, f := range ents {
		// no INTRO.EXE: the original was started as EOB.EXE directly
		if !f.IsDir() && f.Name() != "INTRO.EXE" {
			os.Symlink(filepath.Join(testDir, f.Name()), filepath.Join(dir, f.Name()))
		}
	}
	os.Remove(filepath.Join(dir, "EOBDATA.SAV"))
	orig, err := os.ReadFile(filepath.Join(dosboxDir, "EOBDATA.orig.SAV"))
	if err != nil {
		t.Skip(err)
	}
	sav := filepath.Join(dir, "EOBDATA.SAV")
	os.WriteFile(sav, orig, 0o644)
	old := time.Now().Add(-time.Hour)
	os.Chtimes(sav, old, old)
	e, err := New(dir, &keySys{keys: keys})
	if err != nil {
		t.Skip(err)
	}
	if err := e.Run(""); err != nil {
		t.Fatal(err)
	}
	if st, err := os.Stat(sav); err != nil || !st.ModTime().After(old) {
		t.Fatal("Go did not write EOBDATA.SAV")
	}
	got, err := os.ReadFile(sav)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// timeMasked reports fields that legitimately differ: the original runs in
// real time while keys are sent (food timer, monster movement on the current
// level), and it saves the raw far pointer to each portrait buffer.
func timeMasked(e *Engine, sec, off int, want []byte) string {
	switch {
	case sec == 0 && off%charSize == 0x23:
		return "food (timer)"
	case sec == 0 && off%charSize >= 0x33 && off%charSize <= 0x36:
		return "portrait far pointer"
	case sec == 3:
		lvl := int(want[e.saveSizes()[0]]) // vars+0: current level
		m := off - (lvl-1)*levelStateSize - 0x4b0
		if m >= 0 && m < 30*28 {
			switch m % 28 {
			case 2, 3, 4, 5, 6, 7, 8, 9, 10, 16, 17, 23, 27: // block, pos, dir, anim, mode, dest, idle
				return "current-level monster movement (real time)"
			}
		}
	}
	return ""
}

// reportSaveDiff names each differing byte by save section and record.
func reportSaveDiff(t *testing.T, e *Engine, got, want []byte) {
	if len(got) != len(want) {
		t.Fatalf("size %d, want %d", len(got), len(want))
	}
	names := []string{"char", "vars", "item", "levels", "opts", "extra"}
	recs := []int{charSize, 64, 14, 0, 0, 0}
	n := 0
	base := 0
	masked := map[string]int{}
	defer func() { t.Logf("masked differences: %v", masked) }()
	for s, sz := range e.saveSizes() {
		for i := base; i < base+sz; i++ {
			if got[i] == want[i] {
				continue
			}
			if why := timeMasked(e, s, i-base, want); why != "" {
				masked[why]++
				continue
			}
			if n++; n <= 40 {
				where := fmt.Sprintf("%s+%#x", names[s], i-base)
				if recs[s] > 0 {
					where = fmt.Sprintf("%s[%d]+%#x", names[s], (i-base)/recs[s], (i-base)%recs[s])
				}
				t.Errorf("%-18s file %#05x: go %02x, original %02x", where, i, got[i], want[i])
			}
		}
		base += sz
	}
	if n > 40 {
		t.Errorf("... %d differing bytes in total", n)
	}
}
