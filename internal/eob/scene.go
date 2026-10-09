package eob

import "github.com/felix/goeob/internal/gfx"

const scenePage = 2

type sceneState struct {
	drawVarDown, drawVarRight, drawVarLeft int
	visIndex                               [18]int
	lvlShapeLR                             [36]int
	blockBuf                               [660]uint16 // walls layer + background layer
	wllProcessFlag                         bool
	shpDmX1, shpDmX2                       int
	lastCompassDir                         int
	levelPal                               []byte // ds:fe68
	palettePending                         bool   // ds:fb45
	specialWallFlip                        int    // ds:fb78
}

func (e *Engine) vis(i int) *Block { return &e.Blocks[e.scene.visIndex[i]] }

// hasWall ports O2116_042c.
func (e *Engine) hasWall(w byte) bool { return w != 0 && e.WllWallFlags[w]&8 == 0 }

// assignVisibleBlocks ports O2116_0000.
func (e *Engine) assignVisibleBlocks(block, dir int) {
	for i := 0; i < 18; i++ {
		e.scene.visIndex[i] = (block + int(e.T.BlockIndex[dir*18+i])) & 0x3ff
	}
	for i := range e.scene.lvlShapeLR {
		e.scene.lvlShapeLR[i] = -1
	}
}

// generateVmpTileData ports O2116_02a5 / O2116_036a.
func (e *Engine) generateVmpTileData(sx, sy int, wall byte, vmpOfs, nx, ny int, flip bool) {
	if e.WllVmpMap[wall] == 0 {
		return
	}
	base := (int(e.WllVmpMap[wall])-1)*431 + vmpOfs + 330
	for i := 0; i < ny; i++ {
		for ii := 0; ii < nx; ii++ {
			x := sx + ii
			if x < 0 || x >= 22 {
				continue
			}
			var v uint16
			if flip {
				v = e.vmp[base+i*nx+(nx-1-ii)]
				if v == 0 {
					continue
				}
				v ^= 0x4000
			} else {
				v = e.vmp[base+i*nx+ii]
				if v == 0 {
					continue
				}
			}
			e.scene.blockBuf[(sy+i)*22+x] = v
		}
	}
}

// generateBlockDrawingBuffer ports O2116_044b.
func (e *Engine) generateBlockDrawingBuffer(block, dir int) {
	s := &e.scene
	s.drawVarDown = int(e.T.BlockMap[dir])
	s.drawVarRight = int(e.T.BlockMap[dir+4])
	s.drawVarLeft = int(e.T.BlockMap[dir+8])
	for i := 0; i < 330; i++ {
		s.blockBuf[i] = 0
	}
	s.wllProcessFlag = ((block>>5)+(block&0x1f)+dir)&1 != 0
	e.generateVmpTileData(0, 15, 1, -330, 22, 15, s.wllProcessFlag)
	e.assignVisibleBlocks(block, dir)
	D := func(i int) byte { return e.vis(i).Walls[s.drawVarDown] }
	R := func(i int) byte { return e.vis(i).Walls[s.drawVarRight] }
	L := func(i int) byte { return e.vis(i).Walls[s.drawVarLeft] }
	g := e.generateVmpTileData
	if t := R(0); t != 0 {
		g(-2, 3, t, 102, 3, 5, false)
	}
	if t := L(6); t != 0 {
		g(21, 3, t, 102, 3, 5, true)
	}
	t, t2 := R(1), D(2)
	if e.hasWall(t) && e.WllWallFlags[t2]&8 == 0 {
		g(2, 3, t, 102, 3, 5, false)
	} else if t != 0 && e.WllWallFlags[t2]&8 != 0 {
		g(2, 3, t2, 102, 3, 5, false)
	}
	t, t2 = L(5), D(4)
	if e.hasWall(t) && e.WllWallFlags[t2]&8 == 0 {
		g(17, 3, t, 102, 3, 5, true)
	} else if t != 0 && e.WllWallFlags[t2]&8 != 0 {
		g(17, 3, t2, 102, 3, 5, true)
	}
	if t := R(2); t != 0 {
		g(8, 3, t, 97, 1, 5, false)
	}
	if t := L(4); t != 0 {
		g(13, 3, t, 97, 1, 5, true)
	}
	if t := D(1); e.hasWall(t) {
		g(-4, 3, t, 129, 6, 5, false)
	}
	if t := D(5); e.hasWall(t) {
		g(20, 3, t, 129, 6, 5, false)
	}
	if t := D(2); e.hasWall(t) {
		g(2, 3, t, 129, 6, 5, false)
	}
	if t := D(4); e.hasWall(t) {
		g(14, 3, t, 129, 6, 5, false)
	}
	if t := D(3); t != 0 {
		g(8, 3, t, 129, 6, 5, false)
	}
	if t := R(7); t != 0 {
		g(0, 3, t, 117, 2, 6, false)
	}
	if t := L(11); t != 0 {
		g(20, 3, t, 117, 2, 6, true)
	}
	if t := R(8); t != 0 {
		g(6, 2, t, 81, 2, 8, false)
	}
	if t := L(10); t != 0 {
		g(14, 2, t, 81, 2, 8, true)
	}
	if t := D(8); e.hasWall(t) {
		g(-4, 2, t, 159, 10, 8, false)
	}
	if t := D(10); e.hasWall(t) {
		g(16, 2, t, 159, 10, 8, false)
	}
	if t := D(9); t != 0 {
		g(6, 2, t, 159, 10, 8, false)
	}
	if t := R(12); t != 0 {
		g(3, 1, t, 45, 3, 12, false)
	}
	if t := L(14); t != 0 {
		g(16, 1, t, 45, 3, 12, true)
	}
	if t := D(12); e.WllWallFlags[t]&8 == 0 {
		g(-13, 1, t, 239, 16, 12, false)
	}
	if t := D(14); e.WllWallFlags[t]&8 == 0 {
		g(19, 1, t, 239, 16, 12, false)
	}
	if t := D(13); t != 0 {
		g(3, 1, t, 239, 16, 12, false)
	}
	r, l := R(15), L(17)
	if lv := e.CurrentLevel; lv > 6 && lv < 10 { // 0a4a: wall types 0x17/0x18 hidden at the far edges
		if l == 0x17 || l == 0x18 {
			l = 0
		}
	} else if lv > 3 {
		if r == 0x18 {
			r = 0
		}
		if l == 0x17 {
			l = 0
		}
	}
	if r != 0 {
		g(0, 0, r, 0, 3, 15, false)
	}
	if l != 0 {
		g(19, 0, l, 0, 3, 15, true)
	}
}

// drawVcnBlocks ports the VGA path of S17a6_03e7 onto page 2.
func (e *Engine) drawVcnBlocks() {
	dst := e.Scr.Pages[scenePage]
	v := e.vcn
	for by := 0; by < 15; by++ {
		for bx := 0; bx < 22; bx++ {
			i := by*22 + bx
			ax := e.scene.blockBuf[i]
			over := uint16(0)
			if ax >= 0x8000 {
				over = ax - 0x8000
				ax = 0
			}
			flip := false
			if ax&0x4000 != 0 {
				flip = true
				ax &= 0x3fff
			}
			bg := false
			if ax == 0 {
				bg = true
				ax = e.scene.blockBuf[i+330]
				if ax&0x4000 != 0 {
					flip = true
					ax &= 0x3fff
				}
			}
			col := &v.ColWall
			if bg {
				col = &v.ColBg
			}
			e.blitVcn(dst, bx, by, int(ax), flip, false, col)
			if over != 0 {
				f2 := false
				if over&0x4000 != 0 {
					f2 = true
					over &= 0x3fff
				}
				e.blitVcn(dst, bx, by, int(over), f2, true, &v.ColWall)
			}
		}
	}
}

func (e *Engine) blitVcn(dst []byte, bx, by, idx int, flip, transp bool, col *[16]byte) {
	b := e.vcn.Blocks
	if idx*32+32 > len(b) {
		return
	}
	src := b[idx*32:]
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			sx := x
			if flip {
				sx = 7 - x
			}
			v := src[y*4+sx/2]
			n := v >> 4
			if sx&1 == 1 {
				n = v & 15
			}
			c := col[n]
			if transp && c == 0 {
				continue
			}
			dst[(by*8+y)*320+bx*8+x] = c
		}
	}
}

// setLevelShapesDim ports O2116_0097.
func (e *Engine) setLevelShapesDim(index int, dim int) {
	s := &e.scene
	var x1, x2 int
	if s.lvlShapeLR[index*2] != -1 {
		x1, x2 = s.lvlShapeLR[index*2], s.lvlShapeLR[index*2+1]
	} else {
		x1, x2 = 0, 22
		m := index * 18
		for i := 0; i < 18; i++ {
			d := e.vis(i).Walls[s.drawVarDown]
			if e.WllWallFlags[d]&8 != 0 {
				if t := int(e.T.Dim2[(m+i)*2]); t > x1 {
					x1 = t
				}
				if t := int(e.T.Dim2[(m+i)*2+1]); t < x2 {
					x2 = t
				}
			} else {
				t := int(e.T.Dim1[m+i])
				if e.WllVmpMap[d] == 0 || t == -40 {
					continue
				}
				if t == -41 {
					x1, x2 = 22, 0
					break
				}
				if t > 0 && x2 > t {
					x2 = t
				}
				if t < 0 && x1 < -t {
					x1 = -t
				}
			}
			if x2 < x1 {
				break
			}
		}
		s.lvlShapeLR[index*2], s.lvlShapeLR[index*2+1] = x1, x2
	}
	s.shpDmX1, s.shpDmX2 = x1, x2
	e.setSceneDimBlocks(dim, x1, 0, x2, 15)
}

// setSceneDimBlocks ports O2116_0066.
func (e *Engine) setSceneDimBlocks(dim, x1, y1, x2, y2 int) {
	e.setScreenDim(dim, x1, y1*8, x2-x1, (y2-y1)*8)
}

// setScreenDim ports S1de7_02fe.
func (e *Engine) setScreenDim(dim, x, y, w, h int) {
	d := &e.Scr.Dims[dim]
	d.X, d.Y, d.W, d.H = x, y, w, h
}

func (e *Engine) drawShp(shp *gfx.Shape, x, y, dim int, flip bool) {
	f := 0
	if flip {
		f = gfx.DrawFlip
	}
	e.Scr.DrawShape(scenePage, shp, x, y, dim, f)
}

// drawDecorations ports O2116_1091.
func (e *Engine) drawDecorations(index int) {
	s := &e.scene
	vars := [3]int{s.drawVarDown, s.drawVarRight, s.drawVarLeft}
	for i := 1; i >= 0; i-- {
		si := index*2 + i
		wm := e.T.WallMapping[si]
		if wm < 0 {
			continue
		}
		w := e.vis(index).Walls[vars[wm]]
		l := int(e.WllShapeMap[w])
		for l > 0 {
			l--
			if l >= len(e.DecoProps) {
				break
			}
			dp := &e.DecoProps[l]
			ix := int(e.T.ShapeIndex[si])
			if i == 0 && (dp.Flags&1 != 0 || (dp.Flags&2 != 0 && s.wllProcessFlag)) {
				ix = -ix
			}
			a := ix
			if a < 0 {
				a = -a
			}
			slot := a - 1
			if slot < 0 {
				l = int(dp.Next)
				continue
			}
			shpIdx := dp.ShapeIndex[slot]
			if shpIdx != 0xff {
				shp := e.DecoShapes[shpIdx]
				if shp != nil {
					var x int
					if i == 1 {
						if ix >= 0 {
							x = int(dp.ShapeX[slot])
						} else {
							x = 176 - int(dp.ShapeX[slot]) - shp.W()
						}
					} else {
						var base int
						if dp.Flags&4 != 0 {
							base = int(e.T.ShapeCoords[(index*5+4)*2])
						} else {
							base = int(e.T.ShapeX2[index])
						}
						if ix >= 0 {
							x = int(dp.ShapeX[slot]) + base
						} else {
							x = 176 - int(dp.ShapeX[slot]) + base - shp.W()
						}
					}
					e.drawShp(shp, x, int(dp.ShapeY[slot]), 5, ix < 0)
				}
			}
			l = int(dp.Next)
		}
	}
}

// drawBlockItems ports O2116_0ad0.
func (e *Engine) drawBlockItems(index int, head int) {
	s := &e.scene
	w := e.Blocks[s.visIndex[index]].Walls[s.drawVarDown]
	var flg int
	if index == 16 {
		flg = 0x80
	} else {
		flg = int(e.WllWallFlags[w])
	}
	if e.WllVmpMap[w] != 0 && flg&0x80 == 0 {
		return
	}
	cur := int(e.Items[head].Next)
	first := cur
	firstPass := true
	for cur != first || firstPass {
		it := &e.Items[cur]
		onWall := it.Pos == 8
		dimSet := -1
		if onWall || it.Pos < 4 {
			p := int(e.T.ItemPosIndex[e.CurrentDirection*4+int(it.Pos)&3])
			if onWall {
				p = 4
			}
			ci := (index*5 + p) * 2
			x := int(e.T.ShapeCoords[ci]) + 88
			var y int
			dm := int(e.T.DimMap[index])
			if onWall {
				x = int(e.T.ShapeX[index])
				// word ds:6fa8[dm]; for the farthest row (dm 3) the
				// original reads past the 3-entry table into ds:6fae.
				y = int(e.X.I16(dsDscItemWallY + 2*dm))
				p = 0
			} else {
				y = int(e.T.ShapeCoords[ci+1]) + 124
			}
			scale := int(e.T.ItemScale[dm*4+p])
			if flg&8 != 0 && p < 2 && scale != 0 {
				dimSet = int(e.T.ItemDim[index])
				if dimSet != -1 {
					e.setLevelShapesDim(dimSet, 5)
				}
				y -= 4
			}
			if scale >= 0 {
				shp := e.ItemShapes[e.T.ItemShapeMap[byte(it.Icon)]][scale]
				if shp != nil {
					x -= shp.W() / 2
					x += int(e.T.ItemJitter[cur&7]) * 2
					y -= shp.H
					y += int(e.T.ItemJitter[(cur>>1)&7])
					e.drawShp(shp, x, y, 5, false)
				}
			}
		}
		cur = int(it.Next)
		firstPass = false
		if dimSet != -1 {
			e.setLevelShapesDim(index, 5)
		}
	}
}

// drawMonsters ports O2116_0d1b.
func (e *Engine) drawMonsters(index int) {
	dm := int(e.T.DimMap[index])
	steps := int(e.T.MonScale[dm])
	blk := (e.CurrentBlock + int(e.T.BlockIndex[e.CurrentDirection*18+index])) & 0x3ff
	var slots [5]*Monster
	for i := range e.Monsters {
		m := &e.Monsters[i]
		if int(m.Block) == blk {
			slots[e.T.MonSubPos[e.CurrentDirection*5+int(m.Pos)]] = m
		}
	}
	for sl := 0; sl < 5; sl++ {
		m := slots[sl]
		if m == nil {
			continue
		}
		props := e.monsterProps(int(m.Type))
		k := int(m.AnimStep)*16 + e.CurrentDirection*4 + int(m.Dir)
		var frm int
		if props.Flags&2 != 0 {
			frm = int(e.T.MonFrm1[k])
		} else {
			frm = int(e.T.MonFrm2[k])
		}
		if dm == 0 {
			switch byte(m.CurAttackFrame) {
			case 0xfe:
				frm = 5
			case 0xff:
				frm = 6
			}
		}
		a := frm
		if a < 0 {
			a = -a
		}
		off := 0
		if m.ShpIndex != 0 {
			off = 18
		}
		if a+off-1 < 0 || a+off-1 >= 36 {
			continue
		}
		shp := e.MonsterShapes[a+off-1]
		if shp == nil {
			continue
		}
		for n := steps; n > 0; n-- {
			shp = shp.Scale23()
		}
		sp := 4
		if m.Pos != 4 {
			sp = int(e.T.ItemPosIndex[e.CurrentDirection*4+int(m.Pos)])
		}
		ci := (index*5 + sp) * 2
		x := int(e.T.ShapeCoords[ci]) + 88
		y := int(e.T.ShapeCoords[ci+1]) + 127
		if props.U15 == 1 && (a == 1 || a > 3) {
			y += int(e.T.MonYOffs[dm])
		}
		x -= shp.W() / 2
		y -= shp.H
		x += int(m.IdleAnimState&0xf0) >> 4
		y += int(m.IdleAnimState & 0xf)
		f := 0
		if frm < 0 {
			f |= gfx.DrawFlip
		}
		if m.Flags&2 != 0 {
			e.Scr.Remap = e.hitRemap()
			f |= gfx.DrawRemap
		}
		e.Scr.DrawShape(scenePage, shp, x, y, 5, f)
	}
}

// drawSceneShapes ports O2116_1e18.
func (e *Engine) drawSceneShapes(start int) {
	for di := start; di < 18; di++ {
		si := int(e.T.TileOrder[di])
		e.setLevelShapesDim(si, 5)
		if e.scene.shpDmX2 < e.scene.shpDmX1 {
			continue
		}
		e.drawDecorations(si)
		b := e.vis(si)
		w := b.Walls[e.scene.drawVarDown]
		if b.ItemHead != 0 {
			e.drawBlockItems(si, int(b.ItemHead))
		}
		if si < 15 {
			if e.WllWallFlags[w]&8 != 0 {
				e.drawDoor(si)
			}
			if b.Flags&7 != 0 {
				e.drawMonsters(si)
			}
		}
		e.drawFlyingObjects(si)
		if w == 0x34 {
			e.drawSpecialWall34(si)
		}
	}
}

// DrawScene ports O2116_1f51: renders the 3D view and copies it to page 0.
func (e *Engine) DrawScene(refresh bool) {
	e.generateBlockDrawingBuffer(e.CurrentBlock, e.CurrentDirection)
	e.drawVcnBlocks()
	e.drawSceneShapes(0)
	e.markSeen()
	if e.scene.palettePending { // 1f79: blit first, then switch the palette
		if refresh {
			e.Scr.CopyRegion(0, 0, 0, 0, 176, 120, scenePage, 0)
		}
		e.Scr.SetPalette(e.scene.levelPal)
		e.scene.palettePending = false
	}
	if refresh && !e.noMonsterSound {
		e.Scr.CopyRegion(0, 0, 0, 0, 176, 120, scenePage, 0)
	}
	if !e.SpellbookOpen { // ds:f3b9: the spell menu covers the compass
		e.drawCompass()
	}
}

// drawCompass ports O2116_1ede.
func (e *Engine) drawCompass() {
	if e.scene.lastCompassDir == e.CurrentDirection+1 {
		return
	}
	for i := 0; i < 3; i++ {
		e.Scr.DrawShape(0, e.CompassShapes[i][e.CurrentDirection], int(e.T.CompassX[i]), int(e.T.CompassY[i]), 0, 0)
	}
	e.scene.lastCompassDir = e.CurrentDirection + 1
}
