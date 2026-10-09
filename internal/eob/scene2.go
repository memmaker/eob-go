package eob

import "github.com/felix/goeob/internal/gfx"

// door scene tables (DS offsets)
const (
	dsDoorShapeBase  = 0x716e // [wall] base shape index
	dsDoorClosedWall = 0x714e // [wall] fully closed wall value for this door
	dsDoorY1         = 0x70f0 // [dm]
	dsDoorY1L12      = 0x7102
	dsDoorY2         = 0x70f4
	dsDoorY3         = 0x70f8
	dsDoorY4a        = 0x70fc
	dsDoorY4b        = 0x70ff
	dsDoorStep1      = 0x718e // [dm] pixels per step (vertical)
	dsDoorOpen1      = 0x7192 // [dm]
	dsDoorStep2      = 0x7196
	dsDoorOpen2      = 0x719a
	dsDoorStep3      = 0x719e
	dsDoorOpen3      = 0x71a2
	dsDoorClipX      = 0x7106 // [index*4] i16 x1, x2 (pixels)
	dsDoorDimLvlA    = 0x6f6d // [level]
	dsDoorDimLvlB    = 0x6f79 // [level]
	dsDoorDimY       = 0x6f4e // [a + dm]

	dsFlyObjSlot    = 0x7246 // [dir*5 + pos&3]
	dsFlyObjX       = 0x71a6 // [index*4 + subpos]
	dsFlyObjY       = 0x71ee // [dm*4 + subpos]
	dsFlyObjScale   = 0x71fe // [index*4 + subpos]
	dsThrownMap     = 0x1d07 // [rel*45 + itemShape]
	dsFlyObjFlip    = 0x7262 // [objDir*4 + curPos]
	dsFlyObjAdjW    = 0x725a // [scale]
	dsFlyObjAdjX    = 0x725e // [scale]
	dsSpecialWallX  = 0x7272
	dsSpecialWallY  = 0x7275
	dsHitRemapVGA   = 0x1cd3
	dsMonsterProps  = 0x1a3b
	monsterPropSize = 27
)

// FlyingObject is an original 18-byte thrown item / spell missile.
type FlyingObject struct {
	Enable     byte // 1 item, 2 spell
	U1         byte
	U2         uint16 // 1 until the object leaves its start block
	CurBlock   uint16
	CurPos     byte
	Direction  byte
	Item       uint16 // item index or spell id
	ShapeIndex byte   // item type (items) / thrown shape (spells)
	U11        byte   // attacker
	Flags      byte
	Range      byte
	Callback   uint32
}

// MonsterProps is the original 27-byte monster type record (partially named).
type MonsterProps struct {
	Raw   [27]byte
	Flags byte // +0x0e
	U15   byte // +0x15
}

func (e *Engine) monsterProps(t int) *MonsterProps {
	o := dsMonsterProps + t*monsterPropSize
	p := &MonsterProps{}
	copy(p.Raw[:], e.X.Bytes(o, monsterPropSize))
	p.Flags = p.Raw[0x0e]
	p.U15 = p.Raw[0x15]
	return p
}

func (e *Engine) hitRemap() []byte {
	var r [16]byte
	copy(r[:], e.X.Bytes(dsHitRemapVGA, 16))
	e.Scr.Remap16 = r
	return nil
}

func (e *Engine) u8(o int) int  { return int(e.X.U8(o)) }
func (e *Engine) i16(o int) int { return int(e.X.I16(o)) }

// setDoorDim ports O2116_021e: vertical clip for doors.
func (e *Engine) setDoorDim(index, dim int) {
	a := e.u8(dsDoorDimLvlA + e.CurrentLevel)
	b := e.u8(dsDoorDimLvlB + e.CurrentLevel)
	dm := int(e.T.DimMap[index])
	y1 := e.u8(dsDoorDimY + a + dm)
	y2 := e.u8(dsDoorDimY + b + dm)
	d := e.Scr.Dims[dim]
	e.setScreenDim(dim, d.X, y1, d.W, y2-y1)
}

// drawDoor ports O2116_1411.
func (e *Engine) drawDoor(index int) {
	dm := int(e.T.DimMap[index])
	w := int(e.vis(index).Walls[e.scene.drawVarDown])
	t := e.u8(dsDoorShapeBase+w) + 2 - dm
	if t < 0 || t >= 6 {
		return
	}
	shp := e.DoorShapes[t]
	raw := int(e.T.ShapeCoords[(index*5+4)*2])
	x := raw + 88
	lvl := e.CurrentLevel
	if lvl < 7 || lvl > 9 {
		e.setDoorDim(index, 5)
	}
	steps := func(stepTab, openTab int) int {
		if w < 0x1e {
			return (w - e.u8(dsDoorClosedWall+w)) * e.u8(stepTab+dm)
		}
		return e.u8(openTab + dm)
	}
	drawSwitch := func(xofs int) {
		if e.WllShapeMap[w] == 0xff {
			sw := e.DoorSwitches[t]
			e.drawShp(sw.Shp, sw.X+raw-xofs, sw.Y, 5, false)
		}
	}
	if shp == nil {
		goto done
	}
	switch lvl {
	case 1, 2, 3, 12:
		var y int
		if lvl == 12 {
			y = e.u8(dsDoorY1L12 + dm)
		} else {
			y = e.u8(dsDoorY1 + dm)
		}
		y -= shp.H
		x -= shp.W() / 2
		y -= steps(dsDoorStep1, dsDoorOpen1)
		e.drawShp(shp, x, y, 5, false)
		drawSwitch(0)
	case 4, 5, 6:
		y := e.u8(dsDoorY2+dm) - shp.H
		a := e.i16(dsDoorClipX+index*4) >> 3
		b := e.i16(dsDoorClipX+index*4+2) >> 3
		if e.scene.shpDmX1 > a {
			a = e.scene.shpDmX1
		}
		if e.scene.shpDmX2 < b {
			b = e.scene.shpDmX2
		}
		d := e.Scr.Dims[5]
		e.setScreenDim(5, a, d.Y, b-a, d.H)
		off := -steps(dsDoorStep2, dsDoorOpen2) - shp.W()
		e.drawShp(shp, x+off, y, 5, false)
		off += shp.W()
		e.drawShp(shp, x-off, y, 5, true)
		drawSwitch(off)
	case 7, 8, 9:
		frame := e.DoorShapes[(t+3)%6]
		y := e.u8(dsDoorY3 + dm)
		if frame != nil {
			e.drawShp(frame, x-frame.W()/2, y-frame.H, 5, false)
		}
		x -= shp.W() / 2
		e.setDoorDim(index, 5)
		y -= steps(dsDoorStep1, dsDoorOpen1)
		e.drawShp(shp, x, y, 5, false)
		drawSwitch(0)
	case 10, 11:
		off := -steps(dsDoorStep3, dsDoorOpen3)
		x -= shp.W() / 2
		e.drawShp(shp, x, e.u8(dsDoorY4a+dm)+off, 5, false)
		off = off>>2 + off>>3
		if s2 := e.DoorShapes[(t+3)%6]; s2 != nil {
			e.drawShp(s2, x, e.u8(dsDoorY4b+dm)-off, 5, false)
		}
		drawSwitch(0)
	}
done:
	e.setSceneDimBlocks(5, e.scene.shpDmX1, 0, e.scene.shpDmX2, 15)
}

// drawFlyingObjects ports O2116_19c4.
func (e *Engine) drawFlyingObjects(index int) {
	blk := e.scene.visIndex[index]
	w := e.Blocks[blk].Walls[e.scene.drawVarDown]
	if e.WllVmpMap[w] != 0 && e.WllWallFlags[w]&0x80 == 0 {
		return
	}
	var slots [5]*FlyingObject
	for i := range e.FlyingObjects {
		o := &e.FlyingObjects[i]
		if o.Enable != 0 && int(o.CurBlock) == blk {
			slots[e.u8(dsFlyObjSlot+e.CurrentDirection*5+int(o.CurPos&3))] = o
		}
	}
	dm := int(e.T.DimMap[index])
	for sl := 0; sl < 5; sl++ {
		o := slots[sl]
		if o == nil {
			continue
		}
		sp := int(e.T.ItemPosIndex[e.CurrentDirection*4+int(o.CurPos&3)])
		x := e.u8(dsFlyObjX + index*4 + sp)
		y := e.u8(dsFlyObjY + dm*4 + sp)
		scale := int(e.X.I8(dsFlyObjScale + index*4 + sp))
		if scale < 0 {
			continue
		}
		var shp *gfx.Shape
		flip := false
		if o.Enable == 1 {
			s := int(e.T.ItemShapeMap[byte(e.Items[o.Item].Icon)])
			rel := -1
			if int(o.Direction) == e.CurrentDirection {
				rel = 0
			} else if int(o.Direction) == e.CurrentDirection^2 {
				rel = 1
			}
			if rel != -1 && e.X.U8(dsThrownMap+rel*45+s) != 0xff {
				shp = e.ThrownShapes[e.X.U8(dsThrownMap+rel*45+s)][scale]
				flip = e.X.U8(dsFlyObjFlip+int(o.Direction)*4+int(int8(o.CurPos))) != 0
			} else {
				shp = e.ItemShapes[s][scale]
				flip = int(o.Direction) == (e.CurrentDirection+1)&3
			}
			if shp != nil && shp.W8 == e.u8(dsFlyObjAdjW+scale) {
				x += e.u8(dsFlyObjAdjX + scale)
			}
		} else {
			shp = e.ThrownShapes[o.ShapeIndex][scale]
			flip = e.X.U8(dsFlyObjFlip+int(o.Direction)*4+int(int8(o.CurPos))) != 0
			if shp != nil && o.Flags&0x40 != 0 {
				x = int(e.T.ShapeCoords[(index*5+4)*2]) + 88 - shp.W()/2
				y = 44 - shp.H/2
			}
		}
		if shp != nil {
			e.drawShp(shp, x, y, 5, flip)
		}
	}
}

// drawSpecialWall34 ports O2116_1d20.
func (e *Engine) drawSpecialWall34(index int) {
	si := 2 - int(e.T.DimMap[index])
	if si < 0 {
		return
	}
	bx := int(e.T.ShapeX[index]) - e.u8(dsSpecialWallX+si)
	by := e.u8(dsSpecialWallY + si)
	tab := e.X.SegOffset(0x20ab, 0)
	for k := 0; k < 2; k++ {
		idx := si*2 + k
		dx, dy := 0, 0
		if idx == 0 {
			dx, dy = -4, -4
		}
		shp := e.SpecialShapes[(idx^e.scene.specialWallFlip)%6]
		for di := 0; di < 13; di++ {
			px := int(e.X.FileU8(tab + idx*26 + di*2))
			py := int(e.X.FileU8(tab + idx*26 + di*2 + 1))
			e.drawShp(shp, dx+bx+px, dy+by+py, 5, false)
		}
	}
}
