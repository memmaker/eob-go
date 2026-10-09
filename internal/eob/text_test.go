package eob

import "testing"

func TestPrintMessageWrap(t *testing.T) {
	e, err := New(testDir, nil)
	if err != nil {
		t.Skip(err)
	}
	e.DebugLoadSave(testSave)
	e.setDim(7)
	e.clearDim()
	d := e.Scr.Dims[7]
	cols := d.W * 8 / e.font.W
	e.printMessage("The quick brown fox jumps over the lazy dog and keeps running far beyond the edge of the window.\r")
	if e.txtLine < 2 || e.txtCol != 0 {
		t.Fatalf("line %d col %d (cols %d)", e.txtLine, e.txtCol, cols)
	}
	e.printMessage("A\x06\x04red\x06\x0f ok")
	if e.txtCol != 7 || e.txtFg != 0xf {
		t.Fatalf("col %d fg %d", e.txtCol, e.txtFg)
	}
}
