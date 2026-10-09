// Package pcspk ports EOB.EXE's PC speaker sound driver (segment S1948) that
// plays PCSOUND.DAT for the "Built in speaker" (1 voice) and "Tandy speaker"
// (3 voices) sound options. The driver's Tandy flag (driver ds:1c7) is 0 in
// the shipped image and never written, so both options drive the PC speaker:
// the 3-voice mode plays the same stream three times, voices 1 and 2 starting
// 0x23 and 0x46 ticks late, all writing the one speaker.
//
// PCSOUND.DAT has ADLIB.DAT's layout: a 120-byte sound-id -> program table,
// then u16 program offsets relative to the data that follows the table.
package pcspk

import (
	"encoding/binary"
	"errors"
	"sync"
)

const (
	pitHz       = 1193182
	ticksPerSec = pitHz / 0x1000 // S1948_0377 programs PIT channel 0 with 0x1000
	numPrograms = 150
	amp         = 6000
)

type voice struct {
	pc    int   // driver ds:e[v]: offset into the playing program
	fetch bool  // ds:1a4
	stop  bool  // ds:1a0
	ended bool  // ds:1a8
	dur   int16 // ds:1ac
	mult  uint8 // ds:1b8
	loop1 uint8 // ds:1bc
	loop2 uint8 // ds:1c0
}

// Player runs the driver and renders PCM. It is safe for concurrent use.
type Player struct {
	mu      sync.Mutex
	ids     []byte
	data    []byte
	div     []uint16 // driver ds:1a: PIT divisor per note
	nvoices int      // ds:19b
	v       [3]voice
	prog    int  // ds:16: program start (offset into data)
	start   bool // ds:19e
	playing bool // ds:19d
	halt    bool // ds:19f
	delay   [3]uint8
	gate    bool
	pitDiv  int
	rate    int
	acc     int
	phase   int
	tandy   bool
	started bool // ds:8a67
}

// New ports S1948_0377 (PIT at 0x1000, one voice) and S177d_000a (load
// PCSOUND.DAT); it reads the note divisor table from exe (EOB.EXE).
// voices is 1 ("Built in speaker") or 3 ("Tandy speaker").
func New(file, exe []byte, rate, voices int) (*Player, error) {
	if len(file) < 120+2*numPrograms {
		return nil, errors.New("pcspk: data too short")
	}
	div := findDivisors(exe)
	if div == nil {
		return nil, errors.New("pcspk: driver note table not found")
	}
	p := &Player{ids: file[:120], data: file[120:], div: div, rate: rate, nvoices: 1}
	if voices == 3 { // S07c8_055e: install, ds:8a67 = 1, ds:9e = 1, S1948_0458
		p.tandy, p.started = true, true
		p.silence()
	}
	return p, nil
}

// findDivisors locates the 96-entry note table: 0, then 95 falling divisors.
func findDivisors(exe []byte) []uint16 {
	u16 := func(i int) uint16 { return binary.LittleEndian.Uint16(exe[i:]) }
	for i := 0; i+192 <= len(exe); i += 2 {
		if u16(i) != 0 || u16(i+2) < 60000 || u16(i+190) > 400 {
			continue
		}
		ok := true
		for n := 2; n < 96 && ok; n++ {
			ok = u16(i+2*n) < u16(i+2*n-2)
		}
		if ok {
			t := make([]uint16, 96)
			for n := range t {
				t[n] = u16(i + 2*n)
			}
			return t
		}
	}
	return nil
}

func (p *Player) u8(i int) uint8 {
	if i < 0 || i >= len(p.data) {
		return 0x60 // end of voice
	}
	return p.data[i]
}

func (p *Player) u16(i int) int { return int(p.u8(i)) | int(p.u8(i+1))<<8 }

// Play ports the speaker branch of S177d_0070 (sound id -> program, ids
// mapping to >= 150 are silent).
func (p *Player) Play(id int) {
	if id < 0 || id >= len(p.ids) || p.ids[id] >= numPrograms {
		return
	}
	p.PlayProgram(int(p.ids[id]))
}

// PlayAt ports the speaker branch of S177d_0106: distance is ignored and the
// program is not range-checked, but program 0 (ds:5e22 > 0 test) is skipped.
func (p *Player) PlayAt(id, dist int) {
	if id < 0 || id >= len(p.ids) || p.ids[id] == 0 {
		return
	}
	p.PlayProgram(int(p.ids[id]))
}

// PlayProgram ports S0000_03e3 + S1948_0418 (start program n&0x7f on the
// next tick) after the shared ds:9e/ds:8a67 voice reset.
func (p *Player) PlayProgram(n int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.tandy && n > 0 && !p.started {
		p.started = true
		p.silence()
	}
	p.prog = p.u16(2 * (n & 0x7f))
	p.start, p.playing = true, true
}

// silence ports S1948_0458 / S1948_034a: 3 voices, all ended, speaker off.
func (p *Player) silence() {
	p.nvoices = 3
	for i := range p.v {
		p.v[i].ended = true
	}
	p.gate = false
}

// tick ports S1948_0050, one PIT interrupt.
func (p *Player) tick() {
	if p.start {
		p.playing, p.start, p.halt = true, false, false
		for i := range p.v { // loop counters (ds:1bc/1c0) are kept
			v := &p.v[i]
			v.pc, v.fetch, v.stop, v.ended, v.dur, v.mult = 0, true, false, false, 0, 1
		}
		p.delay[1], p.delay[2] = 0x23, 0x46
	}
	if !p.playing {
		return
	}
	for si := 0; si < p.nvoices; si++ {
		v := &p.v[si]
		if si > 0 && p.delay[si] > 0 {
			p.delay[si]--
			continue
		}
		if v.fetch {
			p.fetch(v)
			continue
		}
		if p.halt {
			p.endVoice(v)
			continue
		}
		if !v.ended {
			v.dur--
			if v.dur < 0 && v.dur != -1 {
				v.fetch = true
			}
		}
		if v.stop {
			p.endVoice(v)
		}
	}
	ended := 0
	for si := p.nvoices - 1; si >= 0; si-- {
		if p.v[si].ended {
			ended++
		}
	}
	if ended == p.nvoices {
		p.halt, p.playing = false, false
	}
}

func (p *Player) endVoice(v *voice) {
	v.stop, v.ended = false, true
	p.gate = false
}

// fetch interprets bytecode until a note (S1948_0104..0255).
func (p *Player) fetch(v *voice) {
	for guard := 0; guard < 4096; guard++ {
		at := p.prog + v.pc
		op := p.u8(at)
		switch {
		case op <= 0x5f:
			p.note(v, int(p.div[op]), at)
			return
		case op < 0x80:
			p.endVoice(v)
			return
		case op == 0xce || op == 0xd2:
			cnt := &v.loop1
			if op == 0xd2 {
				cnt = &v.loop2
			}
			if *cnt--; *cnt == 0 {
				v.pc += 3
			} else {
				v.pc = v.pc + 1 - p.u16(at+1)
			}
		case op == 0xcd || op == 0xd1:
			c := p.u8(at + 1)
			if c == 0 {
				c = 1
			}
			if op == 0xcd {
				v.loop1 = c
			} else {
				v.loop2 = c
			}
			v.pc += 2
		case op == 0xe6: // raw frequency: divisor = 0x1234dc / f
			f := p.u16(at + 1)
			d := 0
			if f != 0 {
				d = 0x1234dc / f & 0xffff
			}
			v.pc += 2 // op, word, then the note tail's length
			p.note(v, d, at+2)
			return
		case op == 0xe2:
			m := p.u8(at + 1)
			if m == 0 {
				m = 1
			}
			v.mult = m
			v.pc += 2
		case op == 0xe8:
			v.mult -= p.u8(at + 1)
			v.pc += 2
		case op == 0xe9:
			s := int(int8(v.mult)) + int(int8(p.u8(at+1)))
			v.mult += p.u8(at + 1)
			if s > 127 || s < -128 {
				v.mult = 0xff
			}
			v.pc += 2
		default:
			v.fetch = false
			p.endVoice(v)
			return
		}
	}
	p.endVoice(v)
}

// note starts divisor d (0: speaker off) for (len+1)*mult-1 ticks; at is
// the byte before the length.
func (p *Player) note(v *voice, d, at int) {
	if d != 0 {
		p.pitDiv, p.gate = d, true // S1948_0316, S1948_02c7
	} else {
		p.gate = false // S1948_02f1
	}
	v.dur = int16((int(p.u8(at+1))+1)*int(v.mult) - 1)
	v.pc += 2
	v.fetch = false
}

// Busy reports whether a sound is playing.
func (p *Player) Busy() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.playing || p.start
}

// Read renders 16-bit signed little-endian stereo PCM (Ebiten's format):
// a square wave at pitHz/divisor while the speaker gate is on.
func (p *Player) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(b) / 4
	for i := 0; i < n; i++ {
		if p.acc <= 0 {
			p.tick()
			p.acc += p.rate
		}
		p.acc -= ticksPerSec
		var s int16
		if p.gate && p.pitDiv > 0 {
			p.phase += pitHz
			period := p.pitDiv * p.rate
			p.phase %= period
			if p.phase < period/2 {
				s = amp
			} else {
				s = -amp
			}
		}
		binary.LittleEndian.PutUint16(b[4*i:], uint16(s))
		binary.LittleEndian.PutUint16(b[4*i+2:], uint16(s))
	}
	return n * 4, nil
}
