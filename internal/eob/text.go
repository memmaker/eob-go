package eob

// Text output into the current screen dim (bottom message window = dim 7).
// S1de7_00c9/025e (dim frame from tiles) and S1de7_05df (print number) have
// no callers in EOB.EXE and are not ported.

// setDim ports S1de7_0009: saves colours/cursor into the old dim, loads the new one.
func (e *Engine) setDim(dim int) {
	if e.dimInit {
		d := &e.Scr.Dims[e.txtDim]
		d.Unk8, d.UnkA, d.UnkC, d.UnkE = e.txtFg, e.txtBg, e.txtCol, e.txtLine
	} else {
		e.dimInit = true
	}
	e.txtDim = dim
	d := e.Scr.Dims[dim]
	e.txtFg, e.txtBg, e.txtCol, e.txtLine = d.Unk8, d.UnkA, d.UnkC, d.UnkE
}

// setDimClear ports S1de7_00b6.
func (e *Engine) setDimClear(dim int) {
	e.setDim(dim)
	e.clearDim()
}

// clearDim ports S1de7_0344: fills the current dim with the background colour.
func (e *Engine) clearDim() {
	d := e.Scr.Dims[e.txtDim]
	e.Scr.FillRect(e.drawPage, d.X*8, d.Y, (d.X+d.W)*8-1, d.Y+d.H-1, byte(e.txtBg))
	e.txtCol, e.txtLine = 0, 0
}

// printLine ports S1de7_03b2: prints buf at the cursor (scrolling the dim up
// when it is full), advances the cursor and empties buf.
func (e *Engine) printLine(buf *[]byte, nl bool) {
	d := e.Scr.Dims[e.txtDim]
	cw, lh := e.font.W, e.font.H
	y := lh * e.txtLine
	if y > d.H-lh && e.seqMode {
		e.seqMore() // overflow callback [0x8117] = O211e_025f
		y = lh * e.txtLine
	}
	s := string(*buf)
	if d.H-lh >= y {
		e.printText(s, d.X*8+cw*e.txtCol, d.Y+y, byte(e.txtFg), byte(e.txtBg))
		if !nl {
			e.txtCol += len(s)
			if d.W*8-cw >= cw*e.txtCol {
				*buf = (*buf)[:0]
				return
			}
		}
	} else {
		y = (d.H/lh - 1) * lh
		e.Scr.CopyRegion(d.X*8, d.Y+lh, d.X*8, d.Y, d.W*8, y, e.drawPage, e.drawPage)
		e.Scr.FillRect(e.drawPage, d.X*8, d.Y+y, (d.X+d.W)*8-1, d.Y+y+lh-1, byte(e.txtBg))
		e.printText(s, d.X*8, d.Y+y, byte(e.txtFg), byte(e.txtBg))
		if !nl {
			e.txtLine = y / lh
			e.txtCol += len(s)
			if d.W*8-cw >= cw*e.txtCol {
				*buf = (*buf)[:0]
				return
			}
		}
	}
	e.txtCol = 0
	e.txtLine++
	*buf = (*buf)[:0]
}

// printMessage ports S1de7_060a: word-wrapped output with control codes
// 0x0d newline, 0x02 bg colour, 0x06 fg colour, 0x09 column, 0x13 pad to
// column; a char with bit 7 set is the last one printed.
func (e *Engine) printMessage(s string) {
	cw, w := e.font.W, e.Scr.Dims[e.txtDim].W*8
	var line, word []byte
	col := e.txtCol // di: column where line starts + len(line)
	// flush emits line+word, keeping them on one line when they fit.
	flush := func(nl bool) {
		if (len(word)+col)*cw <= w {
			line = append(line, word...)
			e.printLine(&line, nl)
		} else {
			e.printLine(&line, true)
			e.printLine(&word, nl)
		}
		word = word[:0]
	}
	arg := func(i *int) int {
		if *i >= len(s) {
			return 0
		}
		*i++
		return int(s[*i-1])
	}
	for i := 0; i < len(s) && s[i] != 0; {
		c := s[i]
		i++
		last := c&0x80 != 0
		c &= 0x7f
		switch c {
		case 0x0d:
			flush(true)
			col = 0
		case 0x02:
			flush(false)
			col = e.txtCol
			e.txtBg = arg(&i)
		case 0x06:
			flush(false)
			col = e.txtCol
			e.txtFg = arg(&i)
		case 0x09:
			flush(false)
			col = arg(&i)
			e.txtCol = col
			if (col+1)*cw >= w {
				col, e.txtCol = 0, 0
			}
		case 0x13:
			n := arg(&i)
			if e.txtCol >= n {
				break
			}
			for ; e.txtCol < n; n-- {
				word = append(word, ' ')
			}
			line = append(line, word...)
			col += len(word)
			word = word[:0]
		case ' ':
			if (len(word)+col)*cw <= w {
				if (len(word)+col+1)*cw <= w && len(word)+col != 0 {
					word = append(word, ' ')
				}
				line = append(line, word...)
			} else {
				if (len(word)+1)*cw <= w && len(word) != 0 {
					word = append(word, ' ')
				}
				e.printLine(&line, true)
				col = 0
				line = append(line[:0], word...)
			}
			col += len(word)
			word = word[:0]
		default:
			if cw*len(word) >= w {
				if len(line) != 0 {
					e.printLine(&line, true)
				}
				line = append(line, word...)
				e.printLine(&line, true)
				word, col = word[:0], 0
			}
			word = append(word, c)
		}
		if last {
			break
		}
	}
	if len(word) != 0 || len(line) != 0 {
		if w-cw >= (len(word)+col)*cw {
			line = append(line, word...)
			e.printLine(&line, false)
		} else {
			e.printLine(&line, true)
			e.printLine(&word, false)
		}
	}
}
