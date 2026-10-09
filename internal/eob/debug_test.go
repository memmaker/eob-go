package eob

import (
	"fmt"
	"testing"
)

const testDir = "/Users/felix/Projects/EOB/Eye of the Beholder.app/Contents/Resources/game"

// testSave is a frozen copy of a savegame, so tests do not follow the live one.
const testSave = "testdata/EOBDATA.SAV"

func TestIconStrings(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	for _, o := range []int{0x6768, 0x676f, 0x6776, 0x677d, 0x6798} {
		fmt.Printf("%x %q\n", o, e.X.Str(o))
	}
	e.InitResources()
	s := e.ItemIcons[19]
	for y := 0; y < s.H; y++ {
		for x := 0; x < s.W(); x++ {
			if s.At(x, y) != 0 {
				fmt.Print("#")
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println()
	}
}
