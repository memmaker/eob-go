// Package opl emulates the OPL2 (YM3812) FM chip at the integer level: the
// chip's log-sin and exponent ROMs, 9-bit envelope generator with its global
// rate counter, 19-bit phase accumulators, tremolo/vibrato LFOs, the four OPL2
// waveforms and feedback. It follows the hardware behaviour documented by the
// Nuked-OPL3 research (OPL2 subset). Rhythm mode is not emulated: the Westwood
// driver data shipped with Eye of the Beholder never enables it.
//
// The chip runs at its native rate (3.579545 MHz / 72) and is linearly
// resampled to the requested output rate.
package opl

import "math"

const nativeRate = 49716

// Hardware ROMs (derived from their defining formulas; the endpoints match
// the chip dumps: logsin[0]=0x859, exp[0]=0x7fa, exp[255]=0x400).
var logsinROM, expROM [256]uint16

func init() {
	for i := range 256 {
		logsinROM[i] = uint16(math.Round(-math.Log2(math.Sin((float64(i)+0.5)*math.Pi/512)) * 256))
		expROM[i] = uint16(math.Round(math.Exp2(float64(255-i)/256) * 1024))
	}
}

var (
	multX2    = [16]uint32{1, 2, 4, 6, 8, 10, 12, 14, 16, 18, 20, 20, 24, 24, 30, 30}
	kslROM    = [16]int{0, 32, 40, 45, 48, 51, 53, 55, 56, 58, 59, 60, 61, 62, 63, 64}
	kslShift  = [4]uint{8, 1, 2, 0}
	egIncStep = [4][4]uint8{{0, 0, 0, 0}, {1, 0, 0, 0}, {1, 0, 1, 0}, {1, 1, 1, 0}}
)

const (
	egAttack = iota
	egDecay
	egSustain
	egRelease
)

type slot struct {
	ch                 *channel
	am, vib, egt, ksr  bool
	mult, ksl, tl      uint8
	ar, dr, sl, rr, wf uint8
	key                bool
	egRout, egOut      uint16
	egGen              int
	egKSL              uint16
	pgPhase            uint32
	pgOut              uint16
	pgReset            bool
	out, prout, fbmod  int16
	modIn              *int16
}

type channel struct {
	fnum     uint16
	block    uint8
	ksv      uint8
	fb, con  uint8
	mod, car *slot
}

// Chip is one OPL2.
type Chip struct {
	sl  [18]slot // register slot order: slot s belongs to channel s%3+s/6*3
	ch  [9]channel
	wse bool
	nts bool

	timer               uint32
	egTimer             uint64
	egTimerLo, egAdd    uint8
	egState, egTimerRem bool
	tremPos, trem       uint8
	tremShift, vibShift uint
	vibPos              uint8
	zero                int16

	queue [][2]byte

	rate      int
	frac      int // resampler position, in units of 1/rate native samples
	prev, cur int16
}

// New returns a chip producing samples at rate Hz.
func New(rate int) *Chip {
	c := &Chip{rate: rate, tremShift: 4, vibShift: 1}
	for s := range c.sl {
		o := &c.sl[s]
		ch := &c.ch[s%3+s/6*3]
		o.ch = ch
		if s/3%2 == 0 {
			ch.mod = o
		} else {
			ch.car = o
		}
		o.egRout, o.egOut, o.egGen = 0x1ff, 0x1ff, egRelease
	}
	for i := range c.ch {
		c.ch[i].car.modIn = &c.ch[i].mod.out
		c.ch[i].mod.modIn = &c.ch[i].mod.fbmod
	}
	return c
}

// slotIndex maps a register's low 5 bits to a slot, or -1.
func slotIndex(r int) int {
	if r > 0x15 || r&7 > 5 {
		return -1
	}
	return r>>3*6 + r&7
}

func (c *Chip) updateKSL(o *slot) {
	ksl := kslROM[o.ch.fnum>>6]<<2 - (8-int(o.ch.block))<<5
	o.egKSL = uint16(max(ksl, 0))
}

func (c *Chip) updateChannelFreq(ch *channel) {
	ch.ksv = ch.block<<1 | uint8(ch.fnum>>(9-b2u(c.nts))&1)
	c.updateKSL(ch.mod)
	c.updateKSL(ch.car)
}

func b2u(b bool) uint {
	if b {
		return 1
	}
	return 0
}

// Write queues a write of val to register reg. The chip applies at most one
// queued write per native sample: the AdLib needs ~23us after each data
// write (longer than one 20us sample), so back-to-back writes such as a
// key-off followed by a key-on are seen by the chip in separate samples.
func (c *Chip) Write(reg, val byte) { c.queue = append(c.queue, [2]byte{reg, val}) }

func (c *Chip) write(reg, val byte) {
	r, v := int(reg), val
	switch {
	case r == 0x01:
		c.wse = v&0x20 != 0
	case r == 0x08:
		c.nts = v&0x40 != 0
	case r == 0xbd:
		c.tremShift = (b2u(v&0x80 == 0) << 1) + 2
		c.vibShift = b2u(v&0x40 == 0)
	case r >= 0x20 && r < 0xa0 || r >= 0xe0 && r < 0xf6:
		s := slotIndex(r & 0x1f)
		if s < 0 {
			return
		}
		o := &c.sl[s]
		switch r & 0xe0 {
		case 0x20:
			o.am, o.vib, o.egt, o.ksr, o.mult = v&0x80 != 0, v&0x40 != 0, v&0x20 != 0, v&0x10 != 0, v&15
		case 0x40:
			o.ksl, o.tl = v>>6, v&0x3f
			c.updateKSL(o)
		case 0x60:
			o.ar, o.dr = v>>4, v&15
		case 0x80:
			o.sl, o.rr = v>>4, v&15
			if o.sl == 15 {
				o.sl = 0x1f
			}
		case 0xe0:
			o.wf = v & 3
		}
	case r >= 0xa0 && r <= 0xa8:
		ch := &c.ch[r-0xa0]
		ch.fnum = ch.fnum&0x300 | uint16(v)
		c.updateChannelFreq(ch)
	case r >= 0xb0 && r <= 0xb8:
		ch := &c.ch[r-0xb0]
		ch.fnum = ch.fnum&0xff | uint16(v&3)<<8
		ch.block = v >> 2 & 7
		c.updateChannelFreq(ch)
		key := v&0x20 != 0
		ch.mod.key, ch.car.key = key, key
	case r >= 0xc0 && r <= 0xc8:
		ch := &c.ch[r-0xc0]
		ch.fb, ch.con = v>>1&7, v&1
		if ch.con == 0 {
			ch.car.modIn = &ch.mod.out
		} else {
			ch.car.modIn = &c.zero
		}
	}
}

func (c *Chip) envelope(o *slot) {
	var tr uint16
	if o.am {
		tr = uint16(c.trem)
	}
	o.egOut = min(o.egRout+uint16(o.tl)<<2+o.egKSL>>kslShift[o.ksl]+tr, 0x1ff)
	reset := false
	var regRate uint8
	if o.key && o.egGen == egRelease {
		reset, regRate = true, o.ar
	} else {
		switch o.egGen {
		case egAttack:
			regRate = o.ar
		case egDecay:
			regRate = o.dr
		case egSustain:
			if !o.egt {
				regRate = o.rr
			}
		case egRelease:
			regRate = o.rr
		}
	}
	o.pgReset = reset
	ks := o.ch.ksv >> ((b2u(o.ksr) ^ 1) << 1)
	rate := ks + regRate<<2
	rateHi, rateLo := rate>>2, rate&3
	if rateHi&0x10 != 0 {
		rateHi = 0x0f
	}
	egShift := rateHi + c.egAdd
	var shift uint8
	if regRate != 0 {
		if rateHi < 12 {
			if c.egState {
				switch egShift {
				case 12:
					shift = 1
				case 13:
					shift = rateLo >> 1 & 1
				case 14:
					shift = rateLo & 1
				}
			}
		} else {
			shift = rateHi&3 + egIncStep[rateLo][c.egTimerLo]
			if shift&4 != 0 {
				shift = 3
			}
			if shift == 0 && c.egState {
				shift = 1
			}
		}
	}
	rout := o.egRout
	var inc uint16
	off := o.egRout&0x1f8 == 0x1f8
	if reset && rateHi == 0x0f {
		rout = 0
	}
	if o.egGen != egAttack && !reset && off {
		rout = 0x1ff
	}
	switch o.egGen {
	case egAttack:
		if o.egRout == 0 {
			o.egGen = egDecay
		} else if o.key && shift > 0 && rateHi != 0x0f {
			inc = uint16(int16(^o.egRout) >> (4 - shift))
		}
	case egDecay:
		if o.egRout>>4 == uint16(o.sl) {
			o.egGen = egSustain
		} else if !off && !reset && shift > 0 {
			inc = 1 << (shift - 1)
		}
	default:
		if !off && !reset && shift > 0 {
			inc = 1 << (shift - 1)
		}
	}
	o.egRout = (rout + inc) & 0x1ff
	if reset {
		o.egGen = egAttack
	}
	if !o.key {
		o.egGen = egRelease
	}
}

func (c *Chip) phase(o *slot) {
	f := o.ch.fnum
	if o.vib {
		r := f >> 7 & 7
		if c.vibPos&3 == 0 {
			r = 0
		} else if c.vibPos&1 != 0 {
			r >>= 1
		}
		r >>= c.vibShift
		if c.vibPos&4 != 0 {
			f -= r
		} else {
			f += r
		}
	}
	base := uint32(f) << o.ch.block >> 1
	ph := uint16(o.pgPhase >> 9)
	if o.pgReset {
		o.pgPhase = 0
	}
	o.pgPhase += base * multX2[o.mult] >> 1
	o.pgOut = ph
}

func calcExp(level uint32) int16 {
	level = min(level, 0x1fff)
	return int16(uint32(expROM[level&0xff]) << 1 >> (level >> 8))
}

func (c *Chip) generate(o *slot) {
	ph := (o.pgOut + uint16(*o.modIn)) & 0x3ff
	env := uint32(o.egOut) << 3
	wf := o.wf
	if !c.wse {
		wf = 0
	}
	var lvl uint32
	neg := false
	switch wf {
	case 0, 2:
		if ph&0x100 != 0 {
			lvl = uint32(logsinROM[ph&0xff^0xff])
		} else {
			lvl = uint32(logsinROM[ph&0xff])
		}
		neg = wf == 0 && ph&0x200 != 0
	case 1:
		if ph&0x200 != 0 {
			lvl = 0x1000
		} else if ph&0x100 != 0 {
			lvl = uint32(logsinROM[ph&0xff^0xff])
		} else {
			lvl = uint32(logsinROM[ph&0xff])
		}
	case 3:
		if ph&0x100 != 0 {
			lvl = 0x1000
		} else {
			lvl = uint32(logsinROM[ph&0xff])
		}
	}
	out := calcExp(lvl + env)
	if neg {
		out = ^out // the chip's one's-complement negation
	}
	o.out = out
}

// tick advances the chip by one native sample and returns its output.
func (c *Chip) tick() int16 {
	if len(c.queue) > 0 {
		c.write(c.queue[0][0], c.queue[0][1])
		c.queue = c.queue[1:]
		if len(c.queue) == 0 {
			c.queue = c.queue[:0:0]
		}
	}
	acc := 0
	for i := range c.ch {
		ch := &c.ch[i]
		for _, o := range [2]*slot{ch.mod, ch.car} {
			if o == ch.mod {
				if ch.fb != 0 {
					o.fbmod = (o.prout + o.out) >> (9 - ch.fb)
				} else {
					o.fbmod = 0
				}
				o.prout = o.out
			}
			c.envelope(o)
			c.phase(o)
			c.generate(o)
		}
		acc += int(ch.car.out)
		if ch.con == 1 {
			acc += int(ch.mod.out)
		}
	}
	if c.timer&0x3f == 0x3f {
		c.tremPos = (c.tremPos + 1) % 210
	}
	if c.tremPos < 105 {
		c.trem = c.tremPos >> c.tremShift
	} else {
		c.trem = (210 - c.tremPos) >> c.tremShift
	}
	if c.timer&0x3ff == 0x3ff {
		c.vibPos = (c.vibPos + 1) & 7
	}
	c.timer++
	if c.egState {
		shift := uint8(0)
		for shift < 36 && c.egTimer>>shift&1 == 0 {
			shift++
		}
		if shift > 12 {
			c.egAdd = 0
		} else {
			c.egAdd = shift + 1
		}
		c.egTimerLo = uint8(c.egTimer & 3)
	}
	if c.egTimerRem || c.egState {
		if c.egTimer == 0xfffffffff {
			c.egTimer, c.egTimerRem = 0, true
		} else {
			c.egTimer, c.egTimerRem = c.egTimer+1, false
		}
	}
	c.egState = !c.egState
	return int16(max(-32768, min(32767, acc)))
}

// Generate fills out with mono samples at the chip's output rate.
func (c *Chip) Generate(out []int16) {
	for i := range out {
		for c.frac >= c.rate {
			c.frac -= c.rate
			c.prev, c.cur = c.cur, c.tick()
		}
		out[i] = int16((int(c.prev)*(c.rate-c.frac) + int(c.cur)*c.frac) / c.rate)
		c.frac += nativeRate
	}
}
