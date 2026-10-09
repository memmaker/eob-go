package eob

import "testing"

// Detect Magic: a magic item's icon is drawn through its ITEMRMP.VGA table.
func TestDetectMagicRemap(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.InitResources()
	if len(e.ItemRemap) != 0x640 {
		t.Fatalf("ITEMRMP.VGA: %d bytes", len(e.ItemRemap))
	}
	it := 1
	for int(byte(e.Items[it].Icon)) != 3 { // a sword: its colours change
		it++
	}
	e.Items[it].Flags |= 0x80
	icon := 3
	draw := func() []byte {
		pg := e.Scr.Pages[2]
		clear(pg)
		e.drawItemIcon(2, it, 0, 0)
		return append([]byte(nil), pg[:320*16]...)
	}
	plain := draw()
	e.Vars.Raw[10] |= 1
	glow := draw()
	s := e.ItemIcons[icon]
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			n := s.Pix[y*16+x]
			want := e.ItemRemap[icon*16+int(n)]
			if s.At(x, y) == 0 && n == 0 {
				want = 0
			}
			if glow[y*320+x] != want {
				t.Fatalf("(%d,%d) nib %d: got %d want %d (plain %d)", x, y, n, glow[y*320+x], want, plain[y*320+x])
			}
		}
	}
	if string(plain) == string(glow) {
		t.Fatal("no remap applied")
	}
}
