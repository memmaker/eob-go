package eob

import (
	"path/filepath"
	"testing"
)

func TestMapNotesRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "EOBDATA.SAV")
	e := &Engine{seen: map[int]*[1024]bool{1: {5: true}}, pois: map[int]map[int]string{1: {7: "STAIRS"}}}
	if err := e.saveMapNotes(p); err != nil {
		t.Fatal(err)
	}
	f := &Engine{}
	f.loadMapNotes(p)
	if !f.seen[1][5] || f.pois[1][7] != "STAIRS" {
		t.Fatalf("got seen %v pois %v", f.seen[1][5], f.pois)
	}
}

// Closed and open doors, walls, floor and POIs need distinct map colours
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
		cs := []byte{e.miniDoor, e.miniOpenDoor, e.miniPOI, e.miniWall, e.miniFloor, 0xf}
		for i := range cs {
			for j := i + 1; j < len(cs); j++ {
				if cs[i] == cs[j] {
					t.Errorf("level %d: colours %d and %d share index %#x (%v)", l, i, j, cs[i], cs)
				}
			}
		}
	}
}
