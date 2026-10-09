// Package eob is a reimplementation of the Eye of the Beholder engine. Game
// data (graphics, levels, tables, text) is read from the original files.
package eob

import (
	"fmt"
	"strings"

	"github.com/felix/goeob/internal/exe"
	"github.com/felix/goeob/internal/gfx"
	"github.com/felix/goeob/internal/pak"
	"github.com/felix/goeob/internal/res"
)

// Block is one map square (original 9-byte level block property).
type Block struct {
	Walls    [4]byte
	Flags    byte   // bits 0-2: monsters present
	ItemHead uint16 // index of last item in circular list (0 = none)
	Script   uint16 // offset into level script
}

// Item is an original 14-byte item record (ITEM.DAT).
type Item struct {
	NameUnid, NameID, Flags byte
	Icon                    int8
	Type                    int8
	Pos                     int8
	Block                   int16
	Next, Prev              int16
	Level                   byte
	Value                   int8
}

// Monster is an active monster (original 28-byte record).
type Monster struct {
	Type, Unit       byte
	Block            uint16
	Pos              byte
	Dir              int8
	AnimStep         byte
	ShpIndex         byte
	Mode             int8
	F9               int8
	CurAttackFrame   int8
	SpellStatusLeft  int8
	HPMax, HPCur     int16
	Dest             uint16
	RandItem         uint16
	FixedItem        uint16
	Flags            byte
	IdleAnimState    byte
	CurRemoteWeapon  byte
	NumRemoteAttacks byte
	Palette          int8
	DirectionChanged byte
	StepsTillRemote  byte
	Sub              byte
}

// DecoProp is an original 52-byte decoration record (.DAT).
type DecoProp struct {
	ShapeIndex [10]byte
	Next       byte
	Flags      byte
	ShapeX     [10]int16
	ShapeY     [10]int16
}

type doorSwitch struct {
	Shp  *gfx.Shape
	X, Y int
}

// Engine is the game.
type Engine struct {
	nullChar Character // see charRec
	FS       *pak.FS
	X        *exe.EXE
	T        *Tables
	Scr      *gfx.Screen
	Sys      System

	CurrentLevel     int
	CurrentBlock     int
	CurrentDirection int

	Chars                     [6]Character
	Vars                      GameVars
	HandItem                  int
	LevelsTmp                 []byte
	SaveExtra                 [96]byte
	Opt80e8, Opt80e6, OptF43c byte
	pendingSound, pendingDist int // ds:5e22/5e24, queued by playSoundAt

	Blocks    [1024]Block
	Items     []Item
	ItemNames []string
	ItemTypes []ItemType
	Monsters  [30]Monster

	WllVmpMap, WllShapeMap, SpecialWallTypes, WllWallFlags [numWallTypes]byte

	DecoProps        []DecoProp // mapped decorations
	decoData         []DecoProp // current .DAT records
	decoRects        [][4]int   // current .DAT shape rects
	DecoShapes       [200]*gfx.Shape
	DoorShapes       [6]*gfx.Shape
	DoorSwitches     [6]doorSwitch
	MonsterShapes    [36]*gfx.Shape
	ItemShapes       [38][4]*gfx.Shape // ds:ec9a: 14 large, slot 14 unused, 23 small from 15 (ds:ed8a)
	ThrownShapes     [12][4]*gfx.Shape
	ItemIcons        [89]*gfx.Shape
	ItemRemap        []byte // ds:f534: ITEMRMP.VGA, 16-colour remap per icon (Detect Magic)
	CompassShapes    [3][4]*gfx.Shape
	SpecialShapes    [6]*gfx.Shape
	MiscShapes       [5]*gfx.Shape // fb38,fb3c,fb40,efb4,efbe
	GuiShapes        [7]*gfx.Shape // dd1c,dd20,dd24,f2f4,dc94,8a7e,dd28
	VisitedLevels    uint16
	ScriptTimersMode byte
	ScriptTimerTicks uint16
	StepsUntilScript uint16
	LevelINF         []byte

	vcn *res.VCN
	vmp []uint16

	scene                sceneState
	col                  guiColors
	ButtonDefs           []Button
	handlers             map[uint32]func(arg int, ev Event) bool
	txtCol, txtLine      int  // ds:fe52, fe54
	txtFg, txtBg         int  // ds:811d, 811b
	txtDim               int  // ds:811f
	kbdSlot              int  // ds:f51a, keyboard-selected inventory slot
	kbdFloorPos          int  // ds:efb2, keyboard floor-item sub-position (-1 none)
	kbdFloorFirst        int  // ds:f3ba, first item seen this cycle (-1 none)
	dimInit              bool // ds:fec0
	sceneUpdate          bool
	StepCounter          int
	clickFlags           int
	book                 spellbook
	Spells               []Spell
	spellCast            map[uint32]func()
	spellHit             map[uint32]func(o *FlyingObject, blk int) bool
	spellEnd             map[uint32]func(c int)
	castFromItemFlag     bool
	castItemSlot         int
	savedCaster          int
	savedCleric          bool
	KbdMode              bool   // ds:814a, mouse disabled in preferences
	hl, hlCol            int    // ds:54f2 / ds:54f4 keyboard highlight rect, colour index
	hlNext               uint32 // ds:fc5e
	camp                 campState
	savePath             string
	tickOfs              uint32   // game time skipped by resting
	menuItems            []string // menuCustom items
	MiniMap              bool     // Tab: minimap overlay (extension)
	attackNext           int      // front-row character the F key attacks with next (extension)
	miniLvl              int      // level the minimap colours below were taken from
	miniFloor, miniWall  byte     // average floor / wall texture colours
	seen                 map[int]*[1024]bool
	inGame               bool
	TurnBased            bool // Space: game time runs only for turnTicks after each action
	turnLeft             int
	spellCaster          int8
	spellCasterChar      int8
	spellTarget          int8
	spellTargetFromTimer int8
	spellStartPos        int8
	curSpell             int8
	SelectedChar         int8
	attackTarget         int
	gameOver             bool
	pathToggle           int
	noMonsterSound       bool // ds:f442: monsters move during rest, silently and undrawn
	monsterWasHit        bool
	batchHits            bool
	scriptTimerNext      uint32
	ambientNext          uint32 // ds:fc5a
	SpellbookOpen        bool
	mouseX, mouseY       int
	Timers               [numTimers]Timer
	timerHandlers        map[uint32]func(int)
	ticks                uint32
	seed                 uint32
	fxRnd                []byte // ds:8484 byte RNG state (S1d8c_0000)
	script               scriptState
	LevelFlags           [13]uint32
	GlobalFlags          uint32
	quit                 bool
	dir                  string
	menuKey              int // ds:fc7a, last key not handled by a menu
	menuGap              int // ds:fc78
	cg                   chargen
	font                 *gfx.Font
	font6, font8         *gfx.Font // ds:a9c0 / ds:fe5a
	drawPage             int
	GuiMode              int
	InvChar              int
	SwapChar             int8
	swapBlink            bool
	BarGraphs            bool
	FlyingObjects        [10]FlyingObject
	seqMode              bool   // ds:8110: MORE prompt instead of scrolling
	textItemMode         bool   // ds:fe42
	seqNPC               int    // ds:efc6
	textDat              []byte // TEXT.DAT (ds:df4e)
}

// New creates an engine reading data from the given game directory.
func New(dir string, sys System) (*Engine, error) {
	fs, err := pak.Open(dir)
	if err != nil {
		return nil, err
	}
	x, err := exe.Load(dir + "/EOB.EXE")
	if err != nil {
		return nil, err
	}
	e := &Engine{FS: fs, X: x, T: loadTables(x), Scr: gfx.New(), Sys: sys, dir: dir}
	for i := 0; i < 22; i++ {
		o := dsScreenDims + i*16
		e.Scr.Dims = append(e.Scr.Dims, gfx.Dim{X: int(x.I16(o)), Y: int(x.I16(o + 2)), W: int(x.I16(o + 4)), H: int(x.I16(o + 6)),
			Unk8: int(x.I16(o + 8)), UnkA: int(x.I16(o + 10)), UnkC: int(x.I16(o + 12)), UnkE: int(x.I16(o + 14))})
	}
	e.hl = int(x.I16(dsHlIdx))
	copy(e.WllVmpMap[:], e.T.DefVmpMap)
	copy(e.WllShapeMap[:], e.T.DefShapeMap)
	copy(e.SpecialWallTypes[:], e.T.DefSpecial)
	copy(e.WllWallFlags[:], e.T.DefFlags)
	return e, nil
}

// ReadFile reads a game file (loose or from PAKs).
// ReadFile ports S1b82_0009 (open, read whole file into buffer, close).
func (e *Engine) ReadFile(name string) ([]byte, error) { return e.FS.Read(name) }

// LoadCPS decodes a CPS/CMP file into a page (and returns its palette, if any).
// LoadCPS ports S082c_051b S082c_05ec (VGA path: "%s.CPS"; EGA ext unused).
// LoadCPS ports S1b82_0041: read header + data, then S1b82_00c7 decompress.
func (e *Engine) LoadCPS(name string, page int) ([]byte, error) {
	if !strings.Contains(name, ".") {
		name += ".CPS"
	}
	b, err := e.FS.Read(name)
	if err != nil {
		return nil, err
	}
	c, err := res.DecodeCPS(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	copy(e.Scr.Pages[page], c.Data)
	return c.Palette, nil
}

func (e *Engine) mustCPS(name string, page int) {
	if _, err := e.LoadCPS(name, page); err != nil {
		panic(err)
	}
}
