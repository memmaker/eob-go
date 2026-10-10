package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/mewkiz/flac"
)

// The Sega CD release (T-75014) has Yuzo Koshiro's soundtrack as CD audio,
// tracks 2-12. levelTrack is the Sega CD's own level table (sub-CPU program
// SPROG, loaded at $8000: words at $4059e, read before the call to its
// looping play routine at $814e): one track per dungeon area.
var levelTrack = [13]int{0, 7, 7, 7, 6, 6, 6, 4, 4, 4, 5, 5, 10}

const cdDir = "CD" // under -data: TRACKnn.FLA, the tracks as FLAC (lossless, about 2/3 of the PCM)

// importCD (-import-cd) finds the Sega CD's .cue anywhere under src and
// copies the music tracks into dst/CD.
func importCD(src, dst string) error {
	var found error = errors.New("no Eye of the Beholder Sega CD (.cue/.bin) found")
	filepath.WalkDir(src, func(p string, d fs.DirEntry, _ error) error {
		if d == nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".cue") {
			return nil
		}
		ts, err := parseCue(p)
		if err != nil || !isEOBDisc(ts) {
			return nil
		}
		found = copyTracks(ts, filepath.Join(dst, cdDir))
		return fs.SkipAll
	})
	return found
}

type cueTrack struct {
	n          int
	audio      bool
	file       string
	start, end int64 // bytes in file; end -1 = to the end
}

// parseCue reads the tracks of a .cue sheet (single or one-file-per-track bins).
func parseCue(path string) ([]cueTrack, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var ts []cueTrack
	var file string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		w := strings.Fields(sc.Text())
		switch {
		case len(w) >= 3 && strings.EqualFold(w[0], "FILE"):
			l := sc.Text()
			file = filepath.Join(filepath.Dir(path), strings.Trim(l[strings.Index(l, w[1]):strings.LastIndex(l, " ")], `"`))
		case len(w) >= 3 && strings.EqualFold(w[0], "TRACK"):
			n, _ := strconv.Atoi(w[1])
			ts = append(ts, cueTrack{n: n, audio: strings.EqualFold(w[2], "AUDIO"), file: file, end: -1})
		case len(w) >= 3 && strings.EqualFold(w[0], "INDEX") && len(ts) > 0:
			var m, s, fr int
			fmt.Sscanf(w[2], "%d:%d:%d", &m, &s, &fr)
			at := int64((m*60+s)*75+fr) * 2352
			t := &ts[len(ts)-1]
			if len(ts) > 1 && ts[len(ts)-2].file == file && ts[len(ts)-2].end < 0 {
				ts[len(ts)-2].end = at // the previous track ends at this one's first index
			}
			if w[1] == "01" {
				t.start = at
			}
		}
	}
	return ts, sc.Err()
}

// isEOBDisc checks the Sega CD header of the first (data) track.
func isEOBDisc(ts []cueTrack) bool {
	if len(ts) < 12 || ts[0].audio {
		return false
	}
	f, err := os.Open(ts[0].file)
	if err != nil {
		return false
	}
	defer f.Close()
	b := make([]byte, 0x400)
	f.ReadAt(b, ts[0].start)
	return bytes.Contains(b, []byte("SEGADISCSYSTEM")) && bytes.Contains(b, []byte("T-75014"))
}

func copyTracks(ts []cueTrack, dir string) error {
	os.RemoveAll(dir) // an earlier import's tracks
	os.MkdirAll(dir, 0o755)
	for _, t := range ts {
		if !t.audio || t.n < 2 || t.n > 12 {
			continue
		}
		b, err := os.ReadFile(t.file)
		if err != nil {
			return err
		}
		end := int64(len(b))
		if t.end >= 0 && t.end < end {
			end = t.end
		}
		if t.start > end {
			return fmt.Errorf("track %d: bad cue offsets", t.n)
		}
		var out bytes.Buffer
		if err := encodeFLAC(&out, b[t.start:end]); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("TRACK%02d.FLA", t.n)), out.Bytes(), 0o644); err != nil {
			return err
		}
		time.Sleep(time.Millisecond) // in the browser: let the page breathe between tracks
	}
	return nil
}

// encodeFLAC writes CD audio (44.1 kHz 16-bit stereo little-endian) as FLAC:
// per frame the best fixed predictor (order 0-4) per channel, left/right or
// mid/side, one Rice partition. The stream's MD5 is left unset (allowed).
// ponytail: no LPC, about 5% bigger than flac -5; mewkiz/flac's encoder is 3x slower (MD5 per sample)
func encodeFLAC(w io.Writer, pcm []byte) error {
	const block = 4096
	n := len(pcm) / 4
	var b bitWriter
	b.bytes([]byte("fLaC"))
	b.bits(1<<7|0, 8) // last metadata block, STREAMINFO
	b.bits(34, 24)
	b.bits(block, 16)
	b.bits(block, 16)
	b.bits(0, 24) // frame sizes unknown
	b.bits(0, 24)
	b.bits(44100, 20)
	b.bits(2-1, 3)
	b.bits(16-1, 5)
	b.bits(uint64(n), 36)
	b.bytes(make([]byte, 16)) // MD5 unset
	ch := [4][]int32{make([]int32, block), make([]int32, block), make([]int32, block), make([]int32, block)}
	for at, num := 0, uint64(0); at < n; at, num = at+block, num+1 {
		k := min(block, n-at)
		l, r, m, sd := ch[0][:k], ch[1][:k], ch[2][:k], ch[3][:k]
		p := pcm[4*at:]
		for i := range k {
			l[i] = int32(int16(uint16(p[4*i]) | uint16(p[4*i+1])<<8))
			r[i] = int32(int16(uint16(p[4*i+2]) | uint16(p[4*i+3])<<8))
			m[i], sd[i] = (l[i]+r[i])>>1, l[i]-r[i]
		}
		var best [4]subframePlan
		for c := range best {
			best[c] = planSubframe(ch[c][:k])
		}
		// channel assignment: 1 L/R, 8 L/side, 9 side/R, 10 mid/side
		asg, x, y := 1, 0, 1
		for _, o := range [][3]int{{8, 0, 3}, {9, 3, 1}, {10, 2, 3}} {
			if best[o[1]].bits+best[o[2]].bits < best[x].bits+best[y].bits {
				asg, x, y = o[0], o[1], o[2]
			}
		}
		start := len(b.buf)
		b.bits(0x3ffe<<2, 16)    // sync, fixed block size
		bsCode, bsTail := 7, k-1 // block size in the next 16 bits
		if k == block {
			bsCode, bsTail = 12, -1 // 4096
		}
		b.bits(uint64(bsCode), 4)
		b.bits(9, 4) // 44.1 kHz
		b.bits(uint64(asg), 4)
		b.bits(4<<1, 4) // 16 bits per sample
		b.utf8(num)
		if bsTail >= 0 {
			b.bits(uint64(bsTail), 16)
		}
		b.bits(uint64(crc8(b.buf[start:])), 8)
		for _, idx := range []int{x, y} {
			bps := 16
			if idx == 3 { // the side channel needs a 17th bit
				bps = 17
			}
			best[idx].write(&b, ch[idx][:k], bps)
		}
		b.align()
		b.bits(uint64(crc16(b.buf[start:])), 16)
		if len(b.buf) > 1<<20 {
			if _, err := w.Write(b.buf); err != nil {
				return err
			}
			b.buf = b.buf[:0]
		}
	}
	_, err := w.Write(b.buf)
	return err
}

type subframePlan struct{ order, rice, bits int }

// planSubframe picks the fixed predictor and Rice parameter with the fewest bits.
func planSubframe(s []int32) subframePlan {
	best := subframePlan{bits: 1 << 62}
	for order := 0; order <= 4 && order < len(s); order++ {
		var sum uint64
		for i := order; i < len(s); i++ {
			sum += uint64(zigzag(residual(s, i, order)))
		}
		cnt := uint64(len(s) - order)
		rice := 0
		for rice < 14 && cnt<<(rice+1) < sum {
			rice++
		}
		bits := int(cnt*uint64(rice+1)+sum>>rice) + order*17 + 12
		if bits < best.bits {
			best = subframePlan{order, rice, bits}
		}
	}
	return best
}

func residual(s []int32, i, order int) int32 {
	switch order {
	case 1:
		return s[i] - s[i-1]
	case 2:
		return s[i] - 2*s[i-1] + s[i-2]
	case 3:
		return s[i] - 3*s[i-1] + 3*s[i-2] - s[i-3]
	case 4:
		return s[i] - 4*s[i-1] + 6*s[i-2] - 4*s[i-3] + s[i-4]
	}
	return s[i]
}

func zigzag(v int32) uint32 { return uint32(v<<1) ^ uint32(v>>31) }

func (p subframePlan) write(b *bitWriter, s []int32, bps int) {
	b.bits(uint64(8|p.order)<<1, 8) // padding bit, SUBFRAME_FIXED, no wasted bits
	for i := range p.order {
		b.bits(uint64(uint32(s[i]))&(1<<bps-1), bps)
	}
	b.bits(0, 2) // Rice, 4-bit parameters
	b.bits(0, 4) // one partition
	b.bits(uint64(p.rice), 4)
	for i := p.order; i < len(s); i++ {
		u := zigzag(residual(s, i, p.order))
		for q := u >> p.rice; q > 0; q -= min(q, 32) {
			b.bits(0, int(min(q, 32)))
		}
		b.bits(1, 1)
		b.bits(uint64(u)&(1<<p.rice-1), p.rice)
	}
}

type bitWriter struct {
	buf []byte
	acc uint64
	n   int
}

func (b *bitWriter) bits(v uint64, n int) {
	for n > 0 {
		k := min(n, 56-b.n)
		b.acc = b.acc<<k | (v>>(n-k))&(1<<k-1)
		b.n += k
		n -= k
		for b.n >= 8 {
			b.n -= 8
			b.buf = append(b.buf, byte(b.acc>>b.n))
		}
	}
}

func (b *bitWriter) bytes(p []byte) {
	for _, c := range p {
		b.bits(uint64(c), 8)
	}
}

func (b *bitWriter) align() {
	if b.n > 0 {
		b.bits(0, 8-b.n)
	}
}

// utf8 writes a frame number the way FLAC does (UTF-8 style, up to 36 bits).
func (b *bitWriter) utf8(v uint64) {
	if v < 0x80 {
		b.bits(v, 8)
		return
	}
	n := 2
	for v >= 1<<(5*n+1) {
		n++
	}
	b.bits(uint64(0xff00>>n)&0xff|v>>(6*(n-1)), 8)
	for i := n - 2; i >= 0; i-- {
		b.bits(0x80|v>>(6*i)&0x3f, 8)
	}
}

func crc8(p []byte) byte {
	var c byte
	for _, x := range p {
		c ^= x
		for range 8 {
			c = c<<1 ^ byte(int8(c)>>7)&0x07
		}
	}
	return c
}

func crc16(p []byte) uint16 {
	var c uint16
	for _, x := range p {
		c ^= uint16(x) << 8
		for range 8 {
			c = c<<1 ^ uint16(int16(c)>>15)&0x8005
		}
	}
	return c
}

// flacPCM decodes a FLAC file to the PCM the audio context plays, frame by
// frame. ponytail: a seek back restarts and decodes forward; fine for looping music.
type flacPCM struct {
	f   *os.File
	s   *flac.Stream
	buf []byte
	pos int64
}

func openFLAC(path string) (*flacPCM, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	r := &flacPCM{f: f}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, 0, err
	}
	return r, int64(r.s.Info.NSamples) * 4, nil
}

func (r *flacPCM) Read(p []byte) (int, error) {
	for len(r.buf) == 0 {
		fr, err := r.s.ParseNext()
		if err != nil {
			return 0, err
		}
		for i := range fr.Subframes[0].NSamples {
			r.buf = binary.LittleEndian.AppendUint16(r.buf, uint16(fr.Subframes[0].Samples[i]))
			r.buf = binary.LittleEndian.AppendUint16(r.buf, uint16(fr.Subframes[1].Samples[i]))
		}
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	r.pos += int64(n)
	return n, nil
}

func (r *flacPCM) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case io.SeekCurrent:
		off += r.pos
	case io.SeekEnd:
		off += int64(r.s.Info.NSamples) * 4
	}
	if off < 0 {
		return r.pos, errors.New("flacPCM: seek before the start")
	}
	if off < r.pos || r.s == nil {
		if _, err := r.f.Seek(0, io.SeekStart); err != nil {
			return r.pos, err
		}
		s, err := flac.New(bufio.NewReaderSize(r.f, 64<<10))
		if err != nil {
			return r.pos, err
		}
		r.s, r.buf, r.pos = s, nil, 0
	}
	if _, err := io.CopyN(io.Discard, r, off-r.pos); err != nil && err != io.EOF {
		return r.pos, err
	}
	return r.pos, nil
}

func (r *flacPCM) Close() error { return r.f.Close() }

// cdMusic loops the level's CD track when the soundtrack was imported.
type cdMusic struct {
	ctx   *audio.Context
	dir   string
	mu    sync.Mutex
	track int
	pl    *audio.Player
	f     *flacPCM
}

func (c *cdMusic) PlayLevelMusic(level int) {
	if level < 1 || level >= len(levelTrack) {
		return
	}
	c.play(levelTrack[level])
}

func (c *cdMusic) play(track int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if track == c.track {
		return
	}
	c.stopLocked()
	c.track = track
	if track == 0 {
		return
	}
	f, size, err := openFLAC(filepath.Join(c.dir, cdDir, fmt.Sprintf("TRACK%02d.FLA", track)))
	if err != nil {
		return
	}
	pl, err := c.ctx.NewPlayer(audio.NewInfiniteLoop(f, size))
	if err != nil {
		f.Close()
		return
	}
	pl.SetVolume(0.6)
	pl.Play()
	c.pl, c.f = pl, f
}

func (c *cdMusic) stopLocked() {
	if c.pl != nil {
		c.pl.Close()
		c.f.Close()
		c.pl, c.f = nil, nil
	}
}

// hasCD reports whether the soundtrack was imported into dir.
func hasCD(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, cdDir, "TRACK02.FLA"))
	return err == nil
}
