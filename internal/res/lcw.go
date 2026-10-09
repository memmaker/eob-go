// Package res decodes Westwood/EOB resource formats.
package res

import (
	"encoding/binary"
	"fmt"
)

// LCW decompresses Westwood "format 80" data into a buffer of size n.
// LCW ports S1d9a_0013 (CPS compression type 4).
func LCW(src []byte, n int) []byte {
	dst := make([]byte, n)
	s, d := 0, 0
	rd := func() byte {
		if s >= len(src) {
			return 0x80
		}
		b := src[s]
		s++
		return b
	}
	rd16 := func() int { a := int(rd()); return a | int(rd())<<8 }
	put := func(b byte) {
		if d < n {
			dst[d] = b
		}
		d++
	}
	for d < n && s < len(src) {
		c := rd()
		switch {
		case c&0x80 == 0:
			cnt := int(c>>4) + 3
			off := int(c&0x0f)<<8 | int(rd())
			for i := 0; i < cnt; i++ {
				if d-off >= 0 && d-off < n {
					put(dst[d-off])
				} else {
					put(0)
				}
			}
		case c == 0x80:
			return dst
		case c&0x40 == 0:
			cnt := int(c & 0x3f)
			for i := 0; i < cnt; i++ {
				put(rd())
			}
		case c == 0xFE:
			cnt := rd16()
			v := rd()
			for i := 0; i < cnt; i++ {
				put(v)
			}
		default:
			var cnt int
			if c == 0xFF {
				cnt = rd16()
			} else {
				cnt = int(c&0x3f) + 3
			}
			off := rd16()
			for i := 0; i < cnt; i++ {
				if off+i < n {
					put(dst[off+i])
				} else {
					put(0)
				}
			}
		}
	}
	return dst
}

// CPS is a decoded CPS/CMP image file.
type CPS struct {
	Data    []byte
	Palette []byte // 768 bytes 6-bit VGA, or nil
}

// DecodeCPS decodes a CPS-container file (also used for CMP, VCN, VMP, INF...).
// DecodeCPS ports S1b82_00c7 for types 0 (copy) and 4 (LCW); types 1-3
// (RLE/LZW12/LZW14) occur in no shipped file and are rejected.
func DecodeCPS(b []byte) (*CPS, error) {
	if len(b) < 10 {
		return nil, fmt.Errorf("cps too short")
	}
	comp := binary.LittleEndian.Uint16(b[2:])
	size := int(binary.LittleEndian.Uint32(b[4:]))
	pal := int(binary.LittleEndian.Uint16(b[8:]))
	c := &CPS{}
	p := 10
	if pal > 0 {
		c.Palette = b[p : p+pal]
		p += pal
	}
	switch comp {
	case 0:
		c.Data = append([]byte(nil), b[p:p+size]...)
	case 4:
		c.Data = LCW(b[p:], size)
	default:
		return nil, fmt.Errorf("unsupported compression %d", comp)
	}
	return c, nil
}

// EncodeLCW ports the game's LCW compressor (S19fc_0008, Westwood's
// LCW_Comp) instruction for instruction, so its output matches what the
// original writes to LEVELS.TMP. Matches are searched from the start of the
// input; quirks kept: a fill or match reaching the very end of the input is
// one byte shorter than it could be.
func EncodeLCW(src []byte) []byte {
	n := len(src)
	if n == 0 {
		return []byte{0x80}
	}
	at := func(i int) int { // bytes past the end never equal a real byte
		if i < n {
			return int(src[i])
		}
		return -1
	}
	out := []byte{0x81, src[0]}
	lenPtr, lastLit := 0, true
	si := 1
	for si < n {
		bestLen, bestPos := 1, 0
		di := 0 // search pointer
	search:
		for {
			al := src[si]
			if at(si+0x40) == int(al) {
				end := si
				for end < n && src[end] == al {
					end++
				}
				if end == n {
					end-- // repe scasb ran out: dec di lands on the last byte
				}
				if run := end - si; run >= 0x41 {
					lastLit = false
					si = end
					out = append(out, 0xfe, byte(run), byte(run>>8), al)
					if si >= n {
						break search
					}
					continue // cs:00b2 jumps back to 006f: keep bestLen and di
				}
			}
			for {
				// repne scasb for al in src[di:si]
				for di < si && src[di] != al {
					di++
				}
				if di >= si {
					break search
				}
				di++ // one past the found byte
				if at(si+bestLen-1) != at(di+bestLen-2) {
					continue
				}
				found := di - 1
				k := 0
				for si+k < n && src[found+k] == src[si+k] {
					k++
				}
				if si+k == n {
					k-- // repe cmpsb ran out of count
				}
				if k >= bestLen {
					bestLen, bestPos = k, found
				}
				continue search
			}
		}
		if si >= n {
			break
		}
		switch d := si - bestPos; {
		case bestLen <= 2:
			if !lastLit || out[lenPtr] == 0xbf {
				lenPtr = len(out)
				out = append(out, 0x80)
			}
			out[lenPtr]++
			out = append(out, src[si])
			si++
			lastLit = true
			continue
		case bestLen <= 10 && d <= 0xfff:
			out = append(out, byte(bestLen-3)<<4|byte(d>>8), byte(d))
		case bestLen <= 0x40:
			out = append(out, byte(bestLen-3)|0xc0, byte(bestPos), byte(bestPos>>8))
		default:
			out = append(out, 0xff, byte(bestLen), byte(bestLen>>8), byte(bestPos), byte(bestPos>>8))
		}
		si += bestLen
		lastLit = false
	}
	return append(out, 0x80)
}
