package eob

import (
	"bytes"
	"testing"

	"github.com/felix/goeob/internal/res"
)

// TestLevelStateEncoderMatchesOriginal re-encodes the wall blocks the
// original game stored in EOBDATA.SAV and expects byte-identical output.
func TestLevelStateEncoderMatchesOriginal(t *testing.T) {
	e, err := New(testDir, &sweepSys{})
	if err != nil {
		t.Skip(err)
	}
	if err := e.DebugLoadSave(testSave); err != nil {
		t.Skip(err)
	}
	checked := 0
	for lv := 1; lv <= 12; lv++ {
		if e.VisitedLevels&(1<<(lv-1)) == 0 {
			continue
		}
		c := e.levelRecord(lv)[:0x4b0]
		walls := res.LCW(c, 0x1000)
		enc := res.EncodeLCW(walls)
		if !bytes.Equal(enc, c[:len(enc)]) {
			i := 0
			for i < len(enc) && enc[i] == c[i] {
				i++
			}
			t.Fatalf("level %d: differs at byte %d of %d: got % x want % x", lv, i, len(enc), enc[i:min(i+8, len(enc))], c[i:min(i+8, len(c))])
		}
		checked++
	}
	if checked == 0 {
		t.Skip("save has no visited levels")
	}
	t.Logf("%d level records byte-identical", checked)
}
