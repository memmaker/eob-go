package eob

import "github.com/felix/goeob/internal/gfx"

// loadItemGraphics ports O2106_0171.
func (e *Engine) loadItemGraphics() {
	x := e.X
	pg := func() []byte { return e.Scr.Pages[5] }
	e.mustCPS(x.Str(0x6768), 5)
	for si := 0; si < 14; si++ {
		for di := 0; di < 4; di++ {
			e.ItemShapes[si][di] = gfx.Encode4(pg(), e.i16(dsItemL1Enc+si*4), e.i16(dsItemL1Enc+si*4+2)+e.u8(dsItemScaleYOfs+di),
				e.u8(dsItemLDims+di*2), e.u8(dsItemLDims+di*2+1))
		}
	}
	e.mustCPS(x.Str(0x676f), 5)
	for si := 0; si < 23; si++ {
		for di := 0; di < 4; di++ {
			e.ItemShapes[15+si][di] = gfx.Encode4(pg(), e.i16(dsItemS1Enc+si*4), e.i16(dsItemS1Enc+si*4+2)+e.u8(dsItemScaleYOfs+di),
				e.u8(dsItemSDims+di*2), e.u8(dsItemSDims+di*2+1))
		}
	}
	e.mustCPS(x.Str(0x6776), 5)
	for si := 0; si < 12; si++ {
		for di := 0; di < 4; di++ {
			e.ThrownShapes[si][di] = gfx.Encode4(pg(), e.i16(dsItemS1Enc+si*4), e.i16(dsItemS1Enc+si*4+2)+e.u8(dsItemScaleYOfs+di),
				e.u8(dsItemSDims+di*2), e.u8(dsItemSDims+di*2+1))
		}
	}
	e.MiscShapes[0] = gfx.Encode4(pg(), 0x10, 0, 4, 0x18)
	e.MiscShapes[1] = gfx.Encode4(pg(), 0x10, 0x18, 4, 0x18)
	e.MiscShapes[2] = gfx.Encode4(pg(), 0x10, 0x30, 3, 0x18)
	e.MiscShapes[3] = gfx.Encode4(pg(), 0x10, 0x90, 5, 0x18)
	e.MiscShapes[4] = gfx.Encode4(pg(), 0x10, 0xa8, 5, 0x10)
	e.mustCPS(x.Str(0x677d), 5)
	for si := 0; si < 0x59; si++ {
		e.ItemIcons[si] = gfx.Encode4(pg(), (si%20)*2, (si/20)*16, 2, 16)
	}
	e.ItemRemap, _ = e.ReadFile("ITEMRMP.VGA") // ds:6785 "ITEMRMP.%s"
}

// loadDecorateShapes ports O2106_0577.
func (e *Engine) loadDecorateShapes() {
	e.loadItemGraphics()
	e.mustCPS(e.X.Str(0x6798), 5)
	pg := e.Scr.Pages[5]
	for i := 0; i < 6; i++ {
		o := dsDecorateEnc + i*4
		e.SpecialShapes[i] = gfx.Encode4(pg, e.u8(o), e.u8(o+1), e.u8(o+2), e.u8(o+3))
	}
	e.GuiShapes[0] = gfx.Encode4(pg, 0x1d, 0, 2, 0x10)
	e.GuiShapes[1] = gfx.Encode4(pg, 0x1f, 0, 2, 0x10)
	e.GuiShapes[2] = gfx.Encode4(pg, 0x21, 0, 2, 0x10)
	e.GuiShapes[3] = gfx.Encode4(pg, 0, 0x58, 4, 0x20)
	e.GuiShapes[4] = gfx.Encode4(pg, 4, 0x58, 4, 0x20)
	e.GuiShapes[5] = gfx.Encode4(pg, 8, 0x58, 4, 0x10)
	e.GuiShapes[6] = gfx.Encode4(pg, 9, 0x58, 2, 8)
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			e.CompassShapes[i][j] = gfx.Encode4(pg, j*3, e.u8(dsCompassYPos+i), 3, e.u8(dsCompassH+i))
		}
	}
}
