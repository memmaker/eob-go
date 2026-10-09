package eob

import (
	"path/filepath"
	"testing"
)

func TestMapNotesRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "EOBDATA.SAV")
	e := &Engine{seen: map[int]*[1024]bool{1: {5: true}}, pois: map[int]map[int]string{1: {7: "STAIRS"}},
		switches: map[int]map[int]*switchUse{1: {9: {Fired: true, Targets: []int{10}}}}}
	if err := e.saveMapNotes(p); err != nil {
		t.Fatal(err)
	}
	f := &Engine{}
	f.loadMapNotes(p)
	if u := f.switches[1][9]; !f.seen[1][5] || f.pois[1][7] != "STAIRS" || u == nil || !u.Fired || len(u.Targets) != 1 || u.Targets[0] != 10 {
		t.Fatalf("got seen %v pois %v switch %+v", f.seen[1][5], f.pois, u)
	}
}

// Closed doors, switches, plates, walls, floor and POIs need distinct map colours
// on every level's palette.
func TestMapColorsDistinct(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	for l := 1; l <= 12; l++ {
		e.loadLevel(l)
		e.Scr.SetPalette(e.scene.levelPal)
		e.CurrentLevel = l
		e.mapColors()
		cs := []byte{e.miniDoor, e.miniPOI, e.miniWall, e.miniFloor, 0xf, e.miniSwitch, e.miniSwitchOn, e.miniPlate, e.miniPlateOn}
		for i := range cs {
			for j := i + 1; j < len(cs); j++ {
				if cs[i] == cs[j] {
					t.Errorf("level %d: colours %d and %d share index %#x (%v)", l, i, j, cs[i], cs)
				}
			}
		}
	}
}
