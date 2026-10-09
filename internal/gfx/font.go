package gfx

import (
	"encoding/binary"
	"errors"
)

// Font is an original Westwood "old DOS" bitmap font (FONT6.FNT/FONT8.FNT).
type Font struct {
	W, H   int
	glyphs [128][]byte
}

// LoadFont parses a .FNT file; ports S1a47_0005 (whose size/header checks
// all pass for the shipped FONT6/FONT8).
func LoadFont(b []byte) (*Font, error) {
	if len(b) < 0x104 {
		return nil, errors.New("font too short")
	}
	f := &Font{W: int(b[0x102]), H: int(b[0x103])}
	for i := 0; i < 128; i++ {
		o := int(binary.LittleEndian.Uint16(b[2+2*i:]))
		if o+f.H <= len(b) {
			f.glyphs[i] = b[o : o+f.H]
		}
	}
	return f, nil
}

// DrawChar draws one character; bg == 0 means transparent background.
func (s *Screen) DrawChar(f *Font, page int, c byte, x, y int, fg, bg byte) {
	g := f.glyphs[c&0x7f]
	p := s.Pages[page]
	for yy := 0; yy < f.H; yy++ {
		row := byte(0)
		if g != nil {
			row = g[yy]
		}
		for xx := 0; xx < f.W; xx++ {
			px, py := x+xx, y+yy
			if px < 0 || px >= 320 || py < 0 || py >= 200 {
				continue
			}
			if row&(0x80>>xx) != 0 {
				p[py*320+px] = fg
			} else if bg != 0 {
				p[py*320+px] = bg
			}
		}
	}
	if page == 0 {
		s.Dirty = true
	}
}

// PrintText draws a string.
func (s *Screen) PrintText(f *Font, page int, str string, x, y int, fg, bg byte) {
	x0 := x
	for i := 0; i < len(str); i++ {
		if str[i] == '\r' { // line break, as the original text printer
			x, y = x0, y+f.H
			continue
		}
		s.DrawChar(f, page, str[i], x, y, fg, bg)
		x += f.W
	}
}
