// Package adlib ports the Westwood AdLib sound driver (version 1, as used by
// Eye of the Beholder) that plays ADLIB.DAT bytecode on an OPL2 chip.
//
// ADLIB.DAT layout: 120-byte sound-id -> program table, then the driver data:
// 150 u16 program offsets, 150 u16 instrument offsets (relative to the data),
// programs and 11-byte instruments. Absolute pointers inside v1 bytecode are
// biased by 191 (the driver's load offset).
package adlib

import (
	"bytes"
	"encoding/binary"
	"errors"
	"sync"

	"github.com/memmaker/opl2"
)

const (
	numPrograms  = 150
	ticksPerSec  = 72 // driver callback rate
	pointerBias  = 191
	volumeModMax = 0xff
)

var regOffset = [9]int{0x00, 0x01, 0x02, 0x08, 0x09, 0x0a, 0x10, 0x11, 0x12}

type effect int

const (
	effNone effect = iota
	effSlide
	effVibrato
)

type channel struct {
	pc                                                              int // -1 = idle
	duration, repeatCounter                                         uint8
	baseOctave                                                      int8
	priority                                                        uint8
	stack                                                           []int
	baseNote                                                        int8
	slideTempo, slideTimer                                          uint8
	slideStep                                                       int16
	vibratoStep                                                     int16
	vibratoStepRange, vibratoStepsCountdown, vibratoNumSteps        uint8
	vibratoDelay, vibratoTempo, vibratoTimer, vibratoDelayCountdown uint8
	opExtraLevel1, opExtraLevel2, opExtraLevel3                     uint8
	spacing1, spacing2, fractionalSpacing                           uint8
	baseFreq, tempo, position, regAx, regBx                         uint8
	primary                                                         effect
	secondary                                                       bool
	opLevel1, opLevel2, twoChan                                     uint8
	durationRandomness                                              uint8
	secTempo, secTimer, secRegbase                                  uint8
	secSize, secPos                                                 int8
	secData                                                         int
	tempoReset                                                      bool
	rawNote                                                         uint8
	pitchBend                                                       int8
}

// Player runs the driver and renders PCM. It is safe for concurrent use.
type Player struct {
	mu        sync.Mutex
	chip      *opl2.Resampler
	ids       []byte // sound id -> program
	data      []byte
	ch        [10]channel
	cur       int
	tempo     uint8
	rnd       uint16
	ampm      byte
	startWait int
	rate      int
	acc       int
	pcm       []int16
	cs        []byte // driver code segment image (frequency/pitch-bend tables)
	freqAt    int    // cs offset of the 12-entry note frequency table
	bendAt    int    // cs offset of the pitch-bend row pointers

	hold             int // 0000:3b36 in driver ticks (11 interrupts < 1 tick)
	lastProg         int // ds:5e26: program whose header bytes are altered
	lastLvl, lastPri uint8
}

// New parses ADLIB.DAT contents and returns a player rendering at rate Hz.
func New(file []byte, rate int) (*Player, error) {
	if len(file) < 120+4*numPrograms {
		return nil, errors.New("adlib: data too short")
	}
	p := &Player{chip: opl2.NewResampler(opl2.New(), rate), ids: file[:120], data: file[120:], rate: rate, rnd: 0x1234,
		lastLvl: 0xff, lastPri: 0xff} // ds:5e28/5e2a start as 0xffff (5e26 = 0)
	p.reset()
	return p, nil
}

// LoadDriver finds the Westwood driver embedded in an executable (EOB.EXE
// or INTRO.EXE) and keeps its code segment, from which the note frequency
// table and pitch-bend tables are read. It reports whether it was found.
func (p *Player) LoadDriver(exe []byte) bool {
	// setupNote: shl bl,1; mov ax,[bx+freq]; sub dx,dx
	fi := bytes.Index(exe, []byte{0xd0, 0xe3, 0x8b, 0x87})
	// pitch bend: inc bl; inc bl; shl bx,1; push si; mov si,[bx+tab]
	bi := bytes.Index(exe, []byte{0xfe, 0xc3, 0xfe, 0xc3, 0xd1, 0xe3, 0x56, 0x8b, 0xb7})
	if fi < 0 || bi < 0 || fi+6 > len(exe) || bi+11 > len(exe) {
		return false
	}
	freq := int(binary.LittleEndian.Uint16(exe[fi+4:]))
	tab := int(binary.LittleEndian.Uint16(exe[bi+9:]))
	u16 := func(i int) int {
		if i < 0 || i+2 > len(exe) {
			return -1
		}
		return int(binary.LittleEndian.Uint16(exe[i:]))
	}
	// The segment base is where both tables have their known shape:
	// 12 rising note frequencies and 14 bend rows 32 bytes apart, each
	// starting at 0 and ending higher than it starts.
	for base := fi - 0x10000; base < len(exe); base++ {
		ok := true
		for n := 0; n < 12 && ok; n++ {
			f := u16(base + freq + 2*n)
			ok = f >= 0x100 && f < 0x300 && (n == 0 || f > u16(base+freq+2*n-2))
		}
		for r := 1; r < 14 && ok; r++ {
			ok = u16(base+tab+2*r)-u16(base+tab+2*r-2) == 32
		}
		for r := 0; r < 14 && ok; r++ {
			row := base + u16(base+tab+2*r)
			ok = row >= 0 && row+32 <= len(exe) && exe[row] == 0 && exe[row+1] <= 2 && exe[row+31] > 0x1f
		}
		if !ok {
			continue
		}
		cs := make([]byte, 0x10000)
		if base < 0 {
			copy(cs[-base:], exe)
		} else {
			copy(cs, exe[base:])
		}
		p.mu.Lock()
		p.cs, p.freqAt, p.bendAt = cs, freq, tab
		p.mu.Unlock()
		return true
	}
	return false
}

func (p *Player) w(r, v int) { p.chip.Chip.Write(r, byte(v)) }

func (p *Player) u8(i int) uint8 {
	if i < 0 || i >= len(p.data) {
		return 0x88 // stopChannel on out-of-range reads
	}
	return p.data[i]
}

func (p *Player) u16(i int) int { return int(p.u8(i)) | int(p.u8(i+1))<<8 }

func (p *Player) program(id int) int { return p.u16(2 * id) }

func (p *Player) reset() {
	p.w(0x01, 0x20)
	p.w(0x08, 0)
	p.w(0xbd, 0)
	for i := 9; i >= 0; i-- {
		if i != 9 {
			p.w(0x40+regOffset[i], 0x3f)
			p.w(0x43+regOffset[i], 0x3f)
		}
		p.initChannel(&p.ch[i])
	}
}

// Play starts sound id (index into the 120-byte table, as the game's
// playSound S177d_0070 does). Ids mapping to >= 150 are silent.
func (p *Player) Play(id int) {
	if id < 0 || id >= len(p.ids) || p.ids[id] >= numPrograms {
		return
	}
	p.PlayProgram(int(p.ids[id]))
}

// PlayAt ports the AdLib branch of S177d_0106 for one queued sound: id at
// dist blocks from the party. Sounds 16+ blocks away are dropped. The
// previous sound's header bytes (+3 first level, +1 priority) are restored
// once it has started, then this one's are saved and, when dist > 0, its
// level is attenuated toward 0x3f and its priority lowered to 0x20-dist.
func (p *Player) PlayAt(id, dist int) {
	if id < 0 || id >= len(p.ids) || p.ids[id] == 0 || dist >= 16 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// The original busy-waits for the driver interrupt to count 3b36 down;
	// run that tick now instead of sleeping until Ebiten's next buffered
	// Read (up to 50ms of frozen game and input).
	if p.hold > 0 {
		p.tick()
		p.acc += p.rate
	}
	n := int(p.ids[id]) // ds:5e22 (> 0 tested above)
	p.setHeader(p.lastProg, 3, p.lastLvl)
	p.setHeader(p.lastProg, 1, p.lastPri)
	p.lastProg = n
	p.lastLvl, p.lastPri = p.header(n, 3), p.header(n, 1)
	if dist > 0 {
		l := int(p.lastLvl)
		p.setHeader(n, 3, uint8(l+(0x3f-l)*(dist-1)>>4))
		p.setHeader(n, 1, uint8(0x20-dist))
	}
	p.startProgram(n)
	p.hold = 1
}

// header / setHeader port S0000_03ca / S0000_03b0: byte off of program n.
func (p *Player) header(n, off int) uint8 {
	if n < 0 || n >= numPrograms {
		return 0
	}
	return p.u8(p.program(n) + off)
}

func (p *Player) setHeader(n, off int, v uint8) {
	if n < 0 || n >= numPrograms {
		return
	}
	if i := p.program(n) + off; i >= 0 && i < len(p.data) {
		p.data[i] = v
	}
}

// PlayProgram starts driver program n directly.
func (p *Player) PlayProgram(n int) {
	p.mu.Lock()
	p.startProgram(n)
	p.mu.Unlock()
}

func (p *Player) startProgram(n int) {
	if n < 0 || n >= numPrograms {
		return
	}
	ptr := p.program(n)
	chn := int(p.u8(ptr))
	prio := p.u8(ptr + 1)
	if chn > 9 {
		return
	}
	c := &p.ch[chn]
	if prio >= c.priority {
		p.startWait = 2
		p.initChannel(c)
		c.priority, c.pc, c.tempo, c.position, c.duration = prio, ptr+2, 0xff, 0xff, 1
		p.initAdlibChannel(chn)
	}
}

func (p *Player) initChannel(c *channel) {
	el2 := c.opExtraLevel2
	*c = channel{pc: -1, opExtraLevel2: el2, tempo: 0xff, spacing1: 1}
}

func (p *Player) initAdlibChannel(n int) {
	if n >= 9 {
		return
	}
	o := regOffset[n]
	p.w(0x60+o, 0xff)
	p.w(0x63+o, 0xff)
	p.w(0x80+o, 0xff)
	p.w(0x83+o, 0xff)
	p.w(0xb0+n, 0)
	p.w(0xb0+n, 0x20)
}

func (p *Player) random() uint16 {
	p.rnd += 0x9248
	lo := p.rnd & 7
	p.rnd = p.rnd>>3 | lo<<13
	return p.rnd
}

// tick is one driver callback (1/72 s).
func (p *Player) tick() {
	if p.hold > 0 { // S0000_3eaf
		p.hold--
	}
	if p.startWait > 0 {
		p.startWait--
	}
	for p.cur = 9; p.cur >= 0; p.cur-- {
		c := &p.ch[p.cur]
		if c.pc < 0 {
			continue
		}
		if c.tempoReset {
			c.tempo = p.tempo
		}
		result := 1
		old := c.position
		c.position += c.tempo
		if c.position < old {
			c.duration--
			if c.duration != 0 {
				if c.duration == c.spacing2 || c.duration == c.spacing1 && p.cur != 9 {
					p.noteOff(c)
				}
			} else {
				result = p.run(c)
			}
		}
		if result == 1 && c.pc >= 0 {
			switch c.primary {
			case effSlide:
				p.slide(c)
			case effVibrato:
				p.vibrato(c)
			}
			if c.secondary {
				p.secondaryEffect(c)
			}
		}
	}
}

// run interprets bytecode until a note/rest with duration or a stop.
func (p *Player) run(c *channel) int {
	for steps := 0; c.pc >= 0 && steps < 1000; steps++ {
		op, a := p.u8(c.pc), p.u8(c.pc+1)
		if op < 0x80 {
			c.pc += 2
			p.setupNote(op, c, false)
			p.noteOn(c)
			p.setupDuration(a, c)
			if a != 0 {
				return 1
			}
			continue
		}
		start := c.pc
		n := opArgs(op)
		c.pc += 1 + n
		if r := p.exec(c, op, start+1, a); r != 0 {
			return r
		}
	}
	return 2
}

var argCount = map[uint8]int{
	0x80: 1, 0x81: 2, 0x82: 1, 0x83: 1, 0x84: 2, 0x85: 2, 0x87: 1, 0x89: 1, 0x8a: 2, 0x8b: 2,
	0x8c: 1, 0x8d: 5, 0x8e: 1, 0x8f: 1, 0x90: 1, 0x91: 3, 0x93: 1, 0x95: 4, 0x9a: 1,
	0x9c: 1, 0x9d: 1, 0x9e: 1, 0xa0: 1, 0xa1: 1, 0xa4: 1, 0xa6: 1, 0xa9: 1,
	0xab: 1, 0xac: 2, 0xad: 2, 0xae: 1, 0xaf: 1, 0xb0: 1, 0xb3: 1, 0xb5: 2, 0xb9: 1, 0xbc: 1,
	0xbd: 1, 0xbf: 2, 0xc1: 9, 0xc2: 1, 0xc4: 2, 0xc5: 2, 0xc6: 2, 0xc7: 1, 0xc8: 1, 0xc9: 2,
}

func opArgs(op uint8) int { return argCount[op] }

// exec runs opcode op with args at data[at:]; c.pc already points past them.
// Returns 0 to continue, 1 to stop parsing and run effects, 2 to stop without.
func (p *Player) exec(c *channel, op uint8, at int, a uint8) int {
	b := p.u8(at + 1)
	switch op {
	case 0x80:
		c.repeatCounter = a
	case 0x81:
		c.repeatCounter--
		if c.repeatCounter != 0 {
			c.pc += int(int16(p.u16(at)))
		}
	case 0x82:
		if a != 0xff {
			p.startProgram(int(a))
		}
	case 0x83:
		c.spacing1 = a
	case 0x84:
		c.pc = p.u16(at) - pointerBias
	case 0x85:
		c.stack = append(c.stack, c.pc)
		c.pc = p.u16(at) - pointerBias
	case 0x86:
		if n := len(c.stack); n > 0 {
			c.pc, c.stack = c.stack[n-1], c.stack[:n-1]
		} else {
			return p.stop(c)
		}
	case 0x87:
		c.baseOctave = int8(a)
	case 0x89:
		p.setupDuration(a, c)
		p.noteOff(c)
		if a != 0 {
			return 1
		}
	case 0x8a:
		p.w(int(a), int(b))
	case 0x8b:
		p.setupNote(a, c, false)
		p.setupDuration(b, c)
		if b != 0 {
			return 1
		}
	case 0x8c:
		c.baseNote = int8(a)
	case 0x8d:
		c.secTimer, c.secTempo = a, a
		c.secSize, c.secPos = int8(b), int8(b)
		c.secRegbase = p.u8(at + 2)
		c.secData = p.u16(at+3) - pointerBias
		c.secondary = c.secData >= 0 && c.secData < len(p.data)
	case 0x8e:
		if a <= 9 {
			o := &p.ch[a]
			o.duration, o.priority, o.pc = 0, 0, -1
		}
	case 0x8f:
		if q := int(p.u8(p.program(int(a)))); q <= 9 && p.ch[q].pc >= 0 {
			c.pc = at - 1
			return 2
		}
	case 0x90:
		p.setupInstrument(c, p.program(numPrograms+int(a)))
	case 0x91:
		c.slideTempo, c.slideTimer = a, 0xff
		c.slideStep = int16(uint16(b)<<8 | uint16(p.u8(at+2)))
		c.primary = effSlide
	case 0x92:
		c.primary, c.slideStep = effNone, 0
	case 0x93:
		c.baseFreq = a
	case 0x95:
		c.vibratoTempo = a
		c.vibratoStepRange = b
		c.vibratoNumSteps = p.u8(at + 2)
		c.vibratoStepsCountdown = c.vibratoNumSteps >> 1
		c.vibratoDelay = p.u8(at + 3)
		c.vibratoDelayCountdown = c.vibratoDelay
		c.primary = effVibrato
	case 0x9a:
		c.priority = a
	case 0x9b:
		c.primary = effNone
	case 0x9c, 0x9d: // beat divider / wait for beat: unused by the effects
	case 0x9e:
		c.opExtraLevel1 = a
		p.adjustVolume(c)
	case 0xa0:
		p.setupDuration(a, c)
		if a != 0 {
			return 1
		}
	case 0xa1:
		p.setupDuration(a, c)
		p.noteOn(c)
		if a != 0 {
			return 1
		}
	case 0xa4:
		c.fractionalSpacing = a & 7
	case 0xa6:
		p.tempo = a
	case 0xa7:
		c.secondary = false
	case 0xa9:
		c.tempo = a
	case 0xab:
		c.opExtraLevel3 = a
	case 0xac, 0xad:
		if a <= 9 {
			save := p.cur
			p.cur = int(a)
			o := &p.ch[a]
			if op == 0xac {
				o.opExtraLevel2 = b
			} else {
				o.opExtraLevel2 += b
			}
			p.adjustVolume(o)
			p.cur = save
		}
	case 0xae:
		p.ampm = p.ampm&0x7f | (a&1)<<7
		p.w(0xbd, int(p.ampm))
	case 0xaf:
		p.ampm = p.ampm&0xbf | (a&1)<<6
		p.w(0xbd, int(p.ampm))
	case 0xb0:
		c.opExtraLevel1 += a
		p.adjustVolume(c)
	case 0xb3:
		if a <= 9 {
			o := &p.ch[a]
			o.duration, o.priority, o.pc, o.opExtraLevel2 = 0, 0, -1, 0
			if a != 9 {
				r := regOffset[a]
				for _, rv := range [][2]int{{0xe0, 0}, {0xe3, 0}, {0x40, 0x3f}, {0x43, 0x3f}, {0x60, 0xff}, {0x63, 0xff}, {0x80, 0xff}, {0x83, 0xff}} {
					p.w(rv[0]+r, rv[1])
				}
				p.w(0xb0+int(a), 0)
			}
		}
	case 0xb5:
		if p.cur < 9 {
			u := (int(a)<<8 | int(b)) & int(p.random())
			f := (int(c.regBx)&0x1f)<<8 | int(c.regAx)
			f = (f + u) | (int(c.regBx)&0x20)<<8
			p.w(0xa0+p.cur, f&0xff)
			p.w(0xb0+p.cur, f>>8&0xff)
		}
	case 0xb6:
		c.primary = effNone
	case 0xb9:
		c.pitchBend = int8(a)
		p.setupNote(c.rawNote, c, true)
	case 0xba:
		c.tempo = p.tempo
	case 0xbc:
		c.durationRandomness = a
	case 0xbd:
		c.tempo = uint8(max(1, min(255, int(c.tempo)+int(int8(a)))))
	case 0xc8:
		c.tempoReset = a != 0
	case 0xc3:
		// cs:4838 clears the rhythm section: register 0xbd keeps only the
		// AM/vibrato depth bits.
		p.w(0xbd, int(p.ampm&0xc0))
	case 0xbb, 0xbf, 0xc0, 0xc1, 0xc2, 0xc4, 0xc5, 0xc6, 0xc7, 0xc9:
		// nop / rhythm section setup and levels / sound trigger. Neither
		// ADLIB.DAT nor SOUND.DAT uses these (rhythm mode is never enabled
		// by 0xc1), so they only consume their arguments.
	default: // 0x88 and all unassigned opcodes stop the channel
		return p.stop(c)
	}
	return 0
}

func (p *Player) stop(c *channel) int {
	c.priority = 0
	if p.cur != 9 {
		p.noteOff(c)
	}
	c.pc = -1
	return 2
}

func (p *Player) noteOff(c *channel) {
	if p.cur >= 9 {
		return
	}
	c.regBx &= 0xdf
	p.w(0xb0+p.cur, int(c.regBx))
}

func (p *Player) noteOn(c *channel) {
	if p.cur >= 9 {
		return
	}
	c.regBx |= 0x20
	p.w(0xb0+p.cur, int(c.regBx))
	shift := 9 - min(int(c.vibratoStepRange), 9)
	f := (int(c.regBx)<<8 | int(c.regAx)) & 0x3ff
	c.vibratoStep = int16(f >> shift & 0xff)
	c.vibratoDelayCountdown = c.vibratoDelay
}

func (p *Player) setupNote(raw uint8, c *channel, bend bool) {
	if p.cur >= 9 {
		return
	}
	c.rawNote = raw
	ch := raw&0xf0 + uint8(c.baseOctave)
	note := int8(raw&15) + c.baseNote
	if note >= 12 {
		note -= 12
		ch += 0x10
	} else if note < 0 {
		note += 12
		ch -= 0x10
	}
	f := p.freq(int(note)) + uint16(c.baseFreq)
	cx := uint16(ch>>2&0x1c|uint8(f>>8))<<8 | f&0xff
	// The set-pitch-bend opcode (bend) also applies table entry 0.
	if c.pitchBend > 0 || (bend && c.pitchBend == 0) {
		cx += uint16(p.bendTable(int(raw&15)+2, uint8(c.pitchBend)))
	} else if c.pitchBend < 0 {
		cx -= uint16(p.bendTable(int(raw&15), uint8(-c.pitchBend)))
	}
	c.regAx = uint8(cx)
	c.regBx = c.regBx&0x20 | uint8(cx>>8)
	p.w(0xa0+p.cur, int(c.regAx))
	p.w(0xb0+p.cur, int(c.regBx))
}

func (p *Player) freq(note int) uint16 {
	if p.cs == nil {
		return 0
	}
	return binary.LittleEndian.Uint16(p.cs[p.freqAt+2*note:])
}

// bendTable reads byte i of pitch-bend table row through the driver's
// pointer table (cs:395a in EOB.EXE), exactly as the original indexes it.
func (p *Player) bendTable(row int, i uint8) uint8 {
	if p.cs == nil {
		return 0
	}
	at := p.bendAt + 2*row
	ptr := int(p.cs[at]) | int(p.cs[at+1])<<8
	return p.cs[(ptr+int(i))&0xffff]
}

func (p *Player) setupDuration(d uint8, c *channel) {
	if c.durationRandomness != 0 {
		c.duration = d + uint8(p.random())&c.durationRandomness
		return
	}
	if c.fractionalSpacing != 0 {
		c.spacing2 = (d >> 3) * c.fractionalSpacing
	}
	c.duration = d
}

func (p *Player) setupInstrument(c *channel, at int) {
	if p.cur >= 9 {
		return
	}
	r := regOffset[p.cur]
	d := func(i int) int { return int(p.u8(at + i)) }
	p.w(0x20+r, d(0))
	p.w(0x23+r, d(1))
	p.w(0xc0+p.cur, d(2))
	c.twoChan = uint8(d(2) & 1)
	p.w(0xe0+r, d(3))
	p.w(0xe3+r, d(4))
	c.opLevel1, c.opLevel2 = uint8(d(5)), uint8(d(6))
	p.w(0x40+r, p.opLevel(c, c.opLevel1, c.twoChan != 0))
	p.w(0x43+r, p.opLevel(c, c.opLevel2, true))
	p.w(0x60+r, d(7))
	p.w(0x63+r, d(8))
	p.w(0x80+r, d(9))
	p.w(0x83+r, d(10))
}

func (p *Player) opLevel(c *channel, lvl uint8, extra bool) int {
	v := int(lvl & 0x3f)
	if extra {
		v += int(c.opExtraLevel1) + int(c.opExtraLevel2)
		l3 := (int(c.opExtraLevel3) ^ 0x3f) * volumeModMax
		if l3 != 0 {
			l3 = (l3 + 0x3f) >> 8
		}
		v += l3 ^ 0x3f
	}
	return min(max(v, 0), 0x3f) | int(lvl&0xc0)
}

func (p *Player) adjustVolume(c *channel) {
	if p.cur >= 9 {
		return
	}
	r := regOffset[p.cur]
	p.w(0x43+r, p.opLevel(c, c.opLevel2, true))
	if c.twoChan != 0 {
		p.w(0x40+r, p.opLevel(c, c.opLevel1, true))
	}
}

func (p *Player) writeFreq(c *channel) {
	p.w(0xa0+p.cur, int(c.regAx))
	p.w(0xb0+p.cur, int(c.regBx))
}

func (p *Player) slide(c *channel) {
	if p.cur >= 9 {
		return
	}
	old := c.slideTimer
	c.slideTimer += c.slideTempo
	if c.slideTimer >= old {
		return
	}
	f := int(c.regBx&3)<<8 | int(c.regAx)
	oct := int(c.regBx & 0x1c)
	on := c.regBx & 0x20
	f += max(-0x3ff, min(0x3ff, int(c.slideStep)))
	if c.slideStep >= 0 && f >= 734 {
		f >>= 1
		if f&0x3ff == 0 {
			f++
		}
		oct += 4
	} else if c.slideStep < 0 && f < 388 {
		f <<= 1
		if f&0x3ff == 0 {
			f--
		}
		oct -= 4
	}
	c.regAx = uint8(f)
	c.regBx = on | uint8(oct&0x1c) | uint8(f>>8&3)
	p.writeFreq(c)
}

func (p *Player) vibrato(c *channel) {
	if p.cur >= 9 {
		return
	}
	if c.vibratoDelayCountdown != 0 {
		c.vibratoDelayCountdown--
		return
	}
	old := c.vibratoTimer
	c.vibratoTimer += c.vibratoTempo
	if c.vibratoTimer >= old {
		return
	}
	c.vibratoStepsCountdown--
	if c.vibratoStepsCountdown == 0 {
		c.vibratoStep = -c.vibratoStep
		c.vibratoStepsCountdown = c.vibratoNumSteps
	}
	f := (int(c.regBx)<<8 | int(c.regAx)) & 0x3ff
	f = (f + int(c.vibratoStep)) & 0x3ff
	c.regAx = uint8(f)
	c.regBx = c.regBx&0xfc | uint8(f>>8)
	p.writeFreq(c)
}

func (p *Player) secondaryEffect(c *channel) {
	if p.cur >= 9 {
		return
	}
	old := c.secTimer
	c.secTimer += c.secTempo
	if c.secTimer < old {
		c.secPos--
		if c.secPos < 0 {
			c.secPos = c.secSize
		}
		p.w(int(c.secRegbase)+regOffset[p.cur], int(p.u8(c.secData+int(c.secPos))))
	}
}

// Busy reports whether any channel is still running a program.
func (p *Player) Busy() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.ch {
		if c.pc >= 0 {
			return true
		}
	}
	return false
}

// Read renders 16-bit signed little-endian stereo PCM (Ebiten's format).
func (p *Player) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(b) / 4
	if cap(p.pcm) < n {
		p.pcm = make([]int16, n)
	}
	pcm := p.pcm[:n]
	for i := 0; i < n; {
		if p.acc <= 0 {
			p.tick()
			p.acc += p.rate
		}
		k := min(n-i, (p.acc+ticksPerSec-1)/ticksPerSec)
		p.chip.Generate(pcm[i : i+k])
		p.acc -= k * ticksPerSec
		i += k
	}
	for i, s := range pcm {
		binary.LittleEndian.PutUint16(b[4*i:], uint16(s))
		binary.LittleEndian.PutUint16(b[4*i+2:], uint16(s))
	}
	return n * 4, nil
}
