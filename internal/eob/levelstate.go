package eob

import (
	"encoding/binary"

	"github.com/felix/goeob/internal/res"
)

// Per-level persistent state (original LEVELS.TMP, 12 x 2040 bytes):
// [0,0x4b0) LCW-compressed 1024x4 wall bytes, [0x4b0,0x7f8) 30 monster records.
const levelStateSize = 0x7f8

func (m *Monster) unmarshal(b []byte) {
	m.Type, m.Unit = b[0], b[1]
	m.Block = binary.LittleEndian.Uint16(b[2:])
	m.Pos, m.Dir, m.AnimStep, m.ShpIndex = b[4], int8(b[5]), b[6], b[7]
	m.Mode, m.F9, m.CurAttackFrame, m.SpellStatusLeft = int8(b[8]), int8(b[9]), int8(b[10]), int8(b[11])
	m.HPMax = int16(binary.LittleEndian.Uint16(b[12:]))
	m.HPCur = int16(binary.LittleEndian.Uint16(b[14:]))
	m.Dest = binary.LittleEndian.Uint16(b[16:])
	m.RandItem = binary.LittleEndian.Uint16(b[18:])
	m.FixedItem = binary.LittleEndian.Uint16(b[20:])
	m.Flags, m.IdleAnimState, m.CurRemoteWeapon, m.NumRemoteAttacks = b[22], b[23], b[24], b[25]
	m.Palette, m.DirectionChanged = int8(b[26]), b[27]
}

func (m *Monster) marshal(b []byte) {
	b[0], b[1] = m.Type, m.Unit
	binary.LittleEndian.PutUint16(b[2:], m.Block)
	b[4], b[5], b[6], b[7] = m.Pos, byte(m.Dir), m.AnimStep, m.ShpIndex
	b[8], b[9], b[10], b[11] = byte(m.Mode), byte(m.F9), byte(m.CurAttackFrame), byte(m.SpellStatusLeft)
	binary.LittleEndian.PutUint16(b[12:], uint16(m.HPMax))
	binary.LittleEndian.PutUint16(b[14:], uint16(m.HPCur))
	binary.LittleEndian.PutUint16(b[16:], m.Dest)
	binary.LittleEndian.PutUint16(b[18:], m.RandItem)
	binary.LittleEndian.PutUint16(b[20:], m.FixedItem)
	b[22], b[23], b[24], b[25] = m.Flags, m.IdleAnimState, m.CurRemoteWeapon, m.NumRemoteAttacks
	b[26], b[27] = byte(m.Palette), m.DirectionChanged
}

func (e *Engine) levelRecord(level int) []byte {
	if len(e.LevelsTmp) < 12*levelStateSize {
		e.LevelsTmp = append(e.LevelsTmp, make([]byte, 12*levelStateSize-len(e.LevelsTmp))...)
	}
	return e.LevelsTmp[(level-1)*levelStateSize : level*levelStateSize]
}

// restoreLevelState ports O210c_1147 / O210c_109d.
func (e *Engine) restoreLevelState(level int) {
	r := e.levelRecord(level)
	walls := res.LCW(r[:0x4b0], 0x1000)
	for i := 0; i < 1024; i++ {
		copy(e.Blocks[i].Walls[:], walls[i*4:])
	}
	for i := 0; i < 30; i++ {
		e.Monsters[i].unmarshal(r[0x4b0+i*28:])
	}
	for i := range e.Monsters {
		e.Blocks[e.Monsters[i].Block].Flags++
	}
}

// saveLevelState ports O210c_11af / O210c_0fd1.
func (e *Engine) saveLevelState() {
	if e.CurrentLevel < 1 {
		return
	}
	r := e.levelRecord(e.CurrentLevel)
	walls := make([]byte, 0x1000)
	for i := 0; i < 1024; i++ {
		copy(walls[i*4:], e.Blocks[i].Walls[:])
	}
	c := res.EncodeLCW(walls)
	for i := range r[:0x4b0] {
		r[i] = 0
	}
	copy(r, c)
	for i := 0; i < 30; i++ {
		e.Monsters[i].marshal(r[0x4b0+i*28:])
	}
	e.VisitedLevels |= 1 << (e.CurrentLevel - 1)
}
