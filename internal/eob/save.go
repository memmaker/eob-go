package eob

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/felix/goeob/internal/gfx"
)

// save chunk sizes are read from the EXE (ds:6a8c, 6 words)
const dsSaveChunkSizes = 0x6a8c

// GameVars mirrors the 64-byte variable block saved from ds:fbc8.
type GameVars struct {
	Raw [64]byte
}

func (e *Engine) saveSizes() [6]int {
	var s [6]int
	for i := range s {
		s[i] = int(e.X.U16(dsSaveChunkSizes + 2*i))
	}
	return s
}

// LoadOriginalSave ports O210c_15b9: loads EOBDATA.SAV.
func (e *Engine) LoadOriginalSave(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	sz := e.saveSizes()
	total := 0
	for _, s := range sz {
		total += s
	}
	if len(b) != total {
		return fmt.Errorf("%s: unexpected size %d (want %d)", filepath.Base(path), len(b), total)
	}
	e.leaveLevel()
	p := 0
	for i := 0; i < 6; i++ {
		e.Chars[i].unmarshal(b[p+i*charSize:])
	}
	e.charTimersToAbsolute()
	p += sz[0]
	copy(e.Vars.Raw[:], b[p:p+sz[1]])
	e.CurrentLevel = int(binary.LittleEndian.Uint16(e.Vars.Raw[0:]))
	e.CurrentBlock = int(binary.LittleEndian.Uint16(e.Vars.Raw[2:]))
	e.CurrentDirection = int(binary.LittleEndian.Uint16(e.Vars.Raw[4:]))
	e.HandItem = int(int16(binary.LittleEndian.Uint16(e.Vars.Raw[6:])))
	e.VisitedLevels = binary.LittleEndian.Uint16(e.Vars.Raw[8:])
	for l := 0; l < 13; l++ {
		e.LevelFlags[l] = binary.LittleEndian.Uint32(e.Vars.Raw[12+4*l:])
	}
	p += sz[1]
	e.Items = make([]Item, 500)
	for i := 0; i < 500; i++ {
		r := b[p+i*14:]
		e.Items[i] = Item{NameUnid: r[0], NameID: r[1], Flags: r[2], Icon: int8(r[3]), Type: int8(r[4]), Pos: int8(r[5]),
			Block: int16(le16(r[6:])), Next: int16(le16(r[8:])), Prev: int16(le16(r[10:])), Level: r[12], Value: int8(r[13])}
	}
	p += sz[2]
	e.LevelsTmp = append([]byte(nil), b[p:p+sz[3]]...)
	p += sz[3]
	e.Opt80e8, e.Opt80e6, e.OptF43c = b[p], b[p+1], b[p+2]
	e.BarGraphs = e.OptF43c != 0
	p += sz[4]
	copy(e.SaveExtra[:], b[p:p+sz[5]])
	e.setFont(e.font6)
	e.loadPortraits()
	if err := e.loadLevel(e.CurrentLevel); err != nil {
		return err
	}
	if !e.KbdMode {
		e.setHandItem(e.HandItem)
	} else if e.HandItem != 0 {
		e.linkItemToBlock(e.CurrentBlock, e.HandItem, e.u8(0x1a1b+e.CurrentDirection*4+e.rollDice(1, 2, -1)))
	}
	return nil
}

// SaveGame ports O210c_1368: writes EOBDATA.SAV in the layout read by
// LoadOriginalSave.
func (e *Engine) SaveGame(path string) error {
	e.leaveLevel()
	e.saveLevelState()
	sz := e.saveSizes()
	var b []byte
	ch := make([]byte, sz[0])
	e.charTimersToRelative()
	for i := range e.Chars {
		e.Chars[i].marshal(ch[i*charSize:])
	}
	e.charTimersToAbsolute()
	b = append(b, ch...)
	v := e.Vars.Raw
	binary.LittleEndian.PutUint16(v[0:], uint16(e.CurrentLevel))
	binary.LittleEndian.PutUint16(v[2:], uint16(e.CurrentBlock))
	binary.LittleEndian.PutUint16(v[4:], uint16(e.CurrentDirection))
	binary.LittleEndian.PutUint16(v[6:], uint16(e.HandItem))
	binary.LittleEndian.PutUint16(v[8:], e.VisitedLevels)
	for l := 0; l < 13; l++ {
		binary.LittleEndian.PutUint32(v[12+4*l:], e.LevelFlags[l])
	}
	b = append(b, v[:sz[1]]...)
	it := make([]byte, sz[2])
	for i, x := range e.Items {
		r := it[i*14:]
		r[0], r[1], r[2], r[3], r[4], r[5] = x.NameUnid, x.NameID, x.Flags, byte(x.Icon), byte(x.Type), byte(x.Pos)
		binary.LittleEndian.PutUint16(r[6:], uint16(x.Block))
		binary.LittleEndian.PutUint16(r[8:], uint16(x.Next))
		binary.LittleEndian.PutUint16(r[10:], uint16(x.Prev))
		r[12], r[13] = x.Level, byte(x.Value)
	}
	b = append(b, it...)
	b = append(b, e.LevelsTmp[:sz[3]]...)
	b = append(b, e.Opt80e8, e.Opt80e6, e.OptF43c)
	b = append(b, e.SaveExtra[:sz[5]]...)
	return os.WriteFile(path, b, 0o644)
}

// charTimersToRelative ports O210c_123d: active characters' pending
// timers become ticks-from-now (an already due timer becomes 1).
func (e *Engine) charTimersToRelative() {
	for c := 0; c < 6; c++ {
		if !e.charStatus(c, 1) {
			continue
		}
		t := &e.Chars[c].Timers
		for i := range t {
			if t[i] > e.ticks {
				t[i] -= e.ticks
			} else if t[i] != 0 {
				t[i] = 1
			}
		}
	}
}

// charTimersToAbsolute ports O210c_12f1: the inverse of
// charTimersToRelative, then S1414_0b2e re-arms the timers.
func (e *Engine) charTimersToAbsolute() {
	for c := 0; c < 6; c++ {
		if !e.charStatus(c, 1) {
			continue
		}
		t := &e.Chars[c].Timers
		for i := range t {
			if t[i] != 0 {
				t[i] += e.ticks
			}
		}
	}
	e.rebuildCharTimers()
}

// loadPortraits ports the face loading part of O210c_15b9.
func (e *Engine) loadPortraits() {
	e.mustCPS(e.X.Str(0x6b61), 5)
	for i := range e.Chars {
		c := &e.Chars[i]
		if c.Flags == 0 || c.Portrait < 0 {
			continue
		}
		p := int(c.Portrait)
		c.Face = gfx.Encode8(e.Scr.Pages[5], (p%10)*4, (p/10)*32, 4, 32)
	}
	e.mustCPS(e.X.Str(0x6b6a), 5)
	for i := range e.Chars {
		c := &e.Chars[i]
		if c.Flags == 0 || c.Portrait >= 0 {
			continue
		}
		c.Face = gfx.Encode8(e.Scr.Pages[5], (-(int(c.Portrait) + 1))*4, 0xa0, 4, 32)
	}
}
