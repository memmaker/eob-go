package eob

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/felix/goeob/internal/gfx"
	"github.com/felix/goeob/internal/res"
)

func cstr(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}

func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }

// loadLevel ports O210c_0a7c.
// loadLevel ports S082c_0ae8 ("%s%d.%s" filename sprintf, inlined).
func (e *Engine) loadLevel(level int) error {
	e.CurrentLevel = level
	raw, err := e.FS.Read(fmt.Sprintf("LEVEL%d.INF", level))
	if err != nil {
		return err
	}
	c, err := res.DecodeCPS(raw)
	if err != nil {
		return err
	}
	inf := c.Data
	e.LevelINF = inf
	p := 2
	if err := e.loadMaze(cstr(inf[p : p+12])); err != nil {
		return err
	}
	p += 12
	vmpName := strings.ToUpper(cstr(inf[p : p+12]))
	vb, err := e.FS.Read(vmpName + ".VMP")
	if err != nil {
		return err
	}
	e.vmp = res.DecodeU16(vb)[1:]
	if pal, err := e.FS.Read(sprintfC(e.X.Str(0x6ac7), cstr(inf[p:p+12]))); err == nil {
		e.scene.levelPal, e.scene.palettePending = pal, true // ds:fe68, ds:fb45
	}
	vcnName := strings.ToUpper(cstr(inf[0x1a : 0x1a+12]))
	p += 0x18

	e.freeDoorShapes()
	for i := 0; i < 36; i++ { // ports O210c_05d2(0, 0x24)
		e.MonsterShapes[i] = nil
	}
	e.freeDecorations()

	e.loadDoorShapes(int(int8(inf[p])), int(inf[p+1]), int(int8(inf[p+2])), int(inf[p+3]))
	p += 4
	e.ScriptTimersMode = inf[p]
	p++
	e.ScriptTimerTicks = le16(inf[p:])
	p += 2
	e.StepsUntilScript = le16(inf[p:])
	p += 2
	e.scriptTimerNext = e.ticks + uint32(e.ScriptTimerTicks)
	e.StepCounter = 0

	for i := 0; i < 2; i++ {
		enc := int(int8(inf[p]))
		p++
		if enc != -1 {
			e.loadMonsterShapes(cstr(inf[p:p+12]), enc, i*18)
		}
		p += 12
	}
	for {
		idx := inf[p]
		p++
		if idx == 0xff {
			break
		}
		v := uint32(int8(inf[p]))
		p++
		e.Timers[2+int(idx)*2].Interval = v
		e.Timers[3+int(idx)*2].Interval = v
	}
	for i := 2; i < 6; i++ {
		t := &e.Timers[i]
		if i&1 == 1 {
			t.Next = e.ticks + t.Interval/2
		} else {
			t.Next = e.ticks + t.Interval
		}
	}
	if e.VisitedLevels&(1<<(level-1)) == 0 {
		e.Monsters = [30]Monster{}
		for i := 0; i < 30; i++ {
			r := inf[p : p+14]
			e.initMonster(int(r[0]), int(r[1]), int(le16(r[2:])), int(r[4]), int(int8(r[5])), int(r[6]), int(r[7]), int(r[8]), int(r[9]), int(le16(r[10:])), int(le16(r[12:])))
			p += 14
		}
	} else {
		p += 420
	}
	n := int(le16(inf[p:]))
	p += 2
	for i := 0; i < n; i++ {
		tag := inf[p]
		p++
		if tag == 0xEC {
			e.loadDecorations(cstr(inf[p:p+12]), cstr(inf[p+12:p+24]))
			p += 24
		} else {
			e.assignWallsAndDecorations(int(inf[p]), inf[p+1], int(int8(inf[p+2])), inf[p+3], inf[p+4])
			p += 5
		}
	}
	// block flags / scripts
	bp := int(le16(inf))
	cnt := int(le16(inf[bp:]))
	bp += 2
	for i := range e.Blocks {
		e.Blocks[i].ItemHead = 0
	}
	e.linkLevelItems()
	for i := 0; i < cnt; i++ {
		b := int(le16(inf[bp:]))
		e.Blocks[b&0x3ff].Flags |= inf[bp+2]
		e.Blocks[b&0x3ff].Script = le16(inf[bp+3:])
		bp += 5
	}
	return e.loadVcn(vcnName)
}

// loadMaze ports O210c_09aa.
func (e *Engine) loadMaze(name string) error {
	e.Blocks = [1024]Block{}
	b, err := e.FS.Read(name)
	if err != nil {
		return err
	}
	if e.VisitedLevels&(1<<(e.CurrentLevel-1)) != 0 {
		e.restoreLevelState(e.CurrentLevel)
		return nil
	}
	for i := 0; i < 1024; i++ {
		copy(e.Blocks[i].Walls[:], b[6+i*4:])
	}
	return nil
}

// loadVcn ports O210c_07a5 (VGA "%s.VCN"; the mode 2/0 branches are
// nonvga). A NULL name in the original reloads the last VCN after a
// sequence clobbered the shared buffer; e.vcn is never clobbered here.
func (e *Engine) loadVcn(name string) error {
	b, err := e.FS.Read(name + ".VCN")
	if err != nil {
		return err
	}
	c, err := res.DecodeCPS(b)
	if err != nil {
		return err
	}
	e.vcn = res.DecodeVCN(c.Data)
	return nil
}

// freeDoorShapes ports O210c_031a (frees door shapes and switch shapes).
func (e *Engine) freeDoorShapes() {
	for i := range e.DoorShapes {
		e.DoorShapes[i] = nil
		e.DoorSwitches[i] = doorSwitch{}
	}
}

// freeDecorations ports O210c_0000 (frees decoration shapes, resets count).
func (e *Engine) freeDecorations() {
	for i := range e.DecoShapes {
		e.DecoShapes[i] = nil
	}
	e.DecoProps = e.DecoProps[:0]
}

// loadDoorShapes ports O210c_03a2.
func (e *Engine) loadDoorShapes(type1, shp1, type2, shp2 int) {
	e.mustCPS(e.X.Str(0x6a98), 5)
	pg := e.Scr.Pages[5]
	load := func(t, base int) {
		for i := 0; i < 3; i++ {
			o := t*12 + i*4
			x := e.X
			e.DoorShapes[base+i] = gfx.Encode4(pg, int(x.U8(dsDoorEnc+o)), int(x.U8(dsDoorEnc+o+1)), int(x.U8(dsDoorEnc+o+2)), int(x.U8(dsDoorEnc+o+3)))
			sw := doorSwitch{Shp: gfx.Encode4(pg, int(x.U8(dsDoorSwitchEnc+o)), int(x.U8(dsDoorSwitchEnc+o+1)), int(x.U8(dsDoorSwitchEnc+o+2)), int(x.U8(dsDoorSwitchEnc+o+3)))}
			// the original reads the switch position with type1 for both doors
			sw.X = int(x.U8(dsDoorSwitchPos + type1*6 + i*2))
			sw.Y = int(x.U8(dsDoorSwitchPos + type1*6 + i*2 + 1))
			e.DoorSwitches[base+i] = sw
		}
	}
	if type1 != -1 {
		load(type1, shp1)
	}
	if type2 != -1 {
		load(type2, shp2)
	}
}

// loadMonsterShapes ports the three loaders O210c_0624, O210c_069e, O210c_0718
// (called through the far-pointer table ds:6a80).
func (e *Engine) loadMonsterShapes(name string, enc, base int) {
	name = strings.ToUpper(name)
	tab := []int{dsMonEnc0, dsMonEnc1, dsMonEnc2}[enc]
	if enc == 2 {
		// third variant formats the file name with an EXE format string
		name = strings.ToUpper(sprintfC(e.X.Str(0x6a9d), name))
	}
	if _, err := e.LoadCPS(name, 5); err != nil {
		return
	}
	pg := e.Scr.Pages[5]
	for i := 0; i < 6; i++ {
		o := tab + i*8
		x := e.X
		e.MonsterShapes[base+i] = gfx.Encode4(pg, int(x.I16(o)), int(x.I16(o+2)), int(x.I16(o+4)), int(x.I16(o+6)))
	}
}

// loadDecorations ports O210c_004f.
func (e *Engine) loadDecorations(cps, dat string) {
	e.mustCPS(strings.ToUpper(cps), 5)
	b, err := e.FS.Read(strings.ToUpper(dat))
	if err != nil {
		return
	}
	n := int(le16(b))
	p := 2
	e.decoData = make([]DecoProp, n)
	for i := 0; i < n; i++ {
		d := &e.decoData[i]
		copy(d.ShapeIndex[:], b[p:])
		d.Next = b[p+10]
		d.Flags = b[p+11]
		for k := 0; k < 10; k++ {
			d.ShapeX[k] = int16(le16(b[p+12+2*k:]))
			d.ShapeY[k] = int16(le16(b[p+32+2*k:]))
		}
		p += 52
	}
	m := int(le16(b[p:]))
	p += 2
	e.decoRects = make([][4]int, m)
	for i := 0; i < m; i++ {
		for k := 0; k < 4; k++ {
			e.decoRects[i][k] = int(le16(b[p+2*k:]))
		}
		p += 8
	}
}

// assignWallsAndDecorations ports O210c_01c8.
func (e *Engine) assignWallsAndDecorations(wall int, vmp byte, dec int, special, flags byte) {
	e.WllVmpMap[wall] = vmp
	e.WllShapeMap[wall] = byte(len(e.DecoProps) + 1)
	e.SpecialWallTypes[wall] = special
	e.WllWallFlags[wall] = flags ^ 4
	if dec == -1 {
		e.WllShapeMap[wall] = 0
		return
	}
	for dec != -1 {
		d := e.decoData[dec]
		for k := 0; k < 10; k++ {
			s := int(d.ShapeIndex[k])
			if s == 0xff || e.DecoShapes[s] != nil {
				continue
			}
			if s < len(e.decoRects) {
				r := e.decoRects[s]
				e.DecoShapes[s] = gfx.Encode4(e.Scr.Pages[5], r[0], r[1], r[2], r[3])
			}
		}
		e.DecoProps = append(e.DecoProps, d)
		cur := len(e.DecoProps) - 1
		if d.Next != 0 {
			dec = int(d.Next)
			e.DecoProps[cur].Next = byte(len(e.DecoProps) + 1)
		} else {
			dec = -1
		}
	}
}

// linkLevelItems ports O210c_0951.
func (e *Engine) linkLevelItems() {
	for i := range e.Items {
		it := &e.Items[i]
		if int(it.Level) == e.CurrentLevel && it.Block > 0 {
			e.linkItemToBlock(int(it.Block), i, int(it.Pos))
		}
	}
}
