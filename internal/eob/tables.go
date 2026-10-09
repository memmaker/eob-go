package eob

import "github.com/felix/goeob/internal/exe"

// DS offsets of original data tables inside EOB.EXE (GOG/DOS English v1.0).
const (
	dsWllVmpMap       = 0x1879
	dsWllShapeMap     = 0x18bf
	dsSpecialWallType = 0x1905
	dsWllWallFlags    = 0x194b
	numWallTypes      = 70

	dsDscBlockMap      = 0x1991
	dsDscBlockIndex    = 0x199d
	dsDscShapeX        = 0x19e5
	dsDscDimMap        = 0x1a09
	dsDscItemPosIndex  = 0x1a2b
	dsDscShapeCoords   = 0x1d61
	dsItemShapeMap     = 0x181f
	dsDscDim1          = 0x6b82
	dsDscDim2          = 0x6cc6
	dsDscItemDim       = 0x6f86
	dsDscItemScale     = 0x6f98
	dsDscItemWallY     = 0x6fa8
	dsDscItemJitter    = 0x6fae
	dsDscMonScaleSteps = 0x6fbe
	dsDscMonFrm1       = 0x6fc1
	dsDscMonFrm2       = 0x6fe1
	dsDscMonSubPos     = 0x7001
	dsDscMonYOffs      = 0x7015
	dsDscWallMapping   = 0x7018
	dsDscShapeIndex    = 0x70a8
	dsDscShapeX2       = 0x70cc
	dsDscTileOrder     = 0x7278
	dsCompassX         = 0x728a
	dsCompassY         = 0x7290
	dsScreenDims       = 0x009f

	// graphics encode tables
	dsDoorEnc       = 0x6840 // [type*12 + i*4] x8,y,w8,h
	dsDoorSwitchEnc = 0x68a0
	dsDoorSwitchPos = 0x6900 // [type*6 + i*2] x,y
	dsMonEnc0       = 0x6930 // 6 x {x8,y,w8,h} words
	dsMonEnc1       = 0x69c0
	dsMonEnc2       = 0x6a50
	dsItemL1Enc     = 0x64f8 // 14 x {x8,y} words
	dsItemS1Enc     = 0x6534 // 23 (and thrown 12) x {x8,y} words
	dsItemScaleYOfs = 0x65ac // 4 bytes
	dsItemLDims     = 0x65b0 // 4 x {w8,h}
	dsItemSDims     = 0x65b8 // 4 x {w8,h}
	dsCompassH      = 0x65c0 // 3
	dsCompassYPos   = 0x65c3 // 3
	dsDecorateEnc   = 0x65c6 // 6 x {x8,y,w8,h} bytes
)

// Tables holds the static tables read from EOB.EXE.
type Tables struct {
	X *exe.EXE

	DefVmpMap, DefShapeMap, DefSpecial, DefFlags []byte

	BlockMap     []byte
	BlockIndex   []int8
	ShapeX       []int16
	DimMap       []byte
	ItemPosIndex []byte
	ShapeCoords  []int16
	ItemShapeMap []byte
	Dim1         []int8
	Dim2         []int8
	ItemDim      []int8
	ItemScale    []int8
	ItemWallY    []int16
	ItemJitter   []int16
	MonScale     []byte
	MonFrm1      []int8
	MonFrm2      []int8
	MonSubPos    []byte
	MonYOffs     []byte
	WallMapping  [36]int // -1 none, 0 down, 1 right, 2 left
	ShapeIndex   []int8
	ShapeX2      []int16
	TileOrder    []byte
	CompassX     []int16
	CompassY     []int16
}

func loadTables(x *exe.EXE) *Tables {
	t := &Tables{X: x}
	t.DefVmpMap = x.Bytes(dsWllVmpMap, numWallTypes)
	t.DefShapeMap = x.Bytes(dsWllShapeMap, numWallTypes)
	t.DefSpecial = x.Bytes(dsSpecialWallType, numWallTypes)
	t.DefFlags = x.Bytes(dsWllWallFlags, numWallTypes)
	t.BlockMap = x.Bytes(dsDscBlockMap, 12)
	t.BlockIndex = x.I8s(dsDscBlockIndex, 72)
	t.ShapeX = x.I16s(dsDscShapeX, 18)
	t.DimMap = x.Bytes(dsDscDimMap, 18)
	t.ItemPosIndex = x.Bytes(dsDscItemPosIndex, 16)
	t.ShapeCoords = x.I16s(dsDscShapeCoords, 180)
	t.ItemShapeMap = x.Bytes(dsItemShapeMap, 256)
	t.Dim1 = x.I8s(dsDscDim1, 324)
	t.Dim2 = x.I8s(dsDscDim2, 648)
	t.ItemDim = x.I8s(dsDscItemDim, 18)
	t.ItemScale = x.I8s(dsDscItemScale, 16)
	t.ItemWallY = x.I16s(dsDscItemWallY, 3)
	t.ItemJitter = x.I16s(dsDscItemJitter, 8)
	t.MonScale = x.Bytes(dsDscMonScaleSteps, 3)
	t.MonFrm1 = x.I8s(dsDscMonFrm1, 32)
	t.MonFrm2 = x.I8s(dsDscMonFrm2, 32)
	t.MonSubPos = x.Bytes(dsDscMonSubPos, 20)
	t.MonYOffs = x.Bytes(dsDscMonYOffs, 3)
	for i := 0; i < 36; i++ {
		switch x.U16(dsDscWallMapping + i*4) {
		case 0xa9c6:
			t.WallMapping[i] = 0
		case 0xefb9:
			t.WallMapping[i] = 1
		case 0xefd0:
			t.WallMapping[i] = 2
		default:
			t.WallMapping[i] = -1
		}
	}
	t.ShapeIndex = x.I8s(dsDscShapeIndex, 36)
	t.ShapeX2 = x.I16s(dsDscShapeX2, 18)
	t.TileOrder = x.Bytes(dsDscTileOrder, 18)
	t.CompassX = x.I16s(dsCompassX, 3)
	t.CompassY = x.I16s(dsCompassY, 3)
	return t
}
