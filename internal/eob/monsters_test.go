package eob

import "testing"

// S08e6_0309 sign-extends the wall byte before indexing wllWallFlags.
func TestWallFlagsSigned(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.WllShapeMap[0xc], e.SpecialWallTypes[0x45] = 0x55, 0x66
	for w, want := range map[byte]byte{5: e.WllWallFlags[5], 0x80: 0x55, 0xbb: e.SpecialWallTypes[1], 0xff: 0x66} {
		if got := e.wallFlagsSigned(w); got != want {
			t.Errorf("wall %#x: %#x, want %#x", w, got, want)
		}
	}
}

// O20f6_26c6: a class item entry of -1 reads ds:60d1 (item 511, slot 0); it
// is unreachable because every class fills slot 0 before its -1 entry.
func TestCgClassItemTail(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	if e.i16(dsCgItems-4) != 511 || e.i16(dsCgItems-2) != 0 {
		t.Fatal("ds:60d1 is not item 511 / slot 0")
	}
	for c := 0; c < 15; c++ {
		filled := false
		for i := 0; i < 4; i++ {
			k := int(e.X.I8(dsCgClsItem + c*4 + i))
			if k == -1 && !filled {
				t.Errorf("class %d reaches its -1 entry with slot 0 empty", c)
			}
			if k >= 0 && e.i16(dsCgItems+k*4) != -1 && e.i16(dsCgItems+k*4+2) <= 0 {
				filled = true // slot 0, or -1 = first free hand (slot 0 first)
			}
		}
	}
}
