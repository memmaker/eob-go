package eob

// Full-screen map (extension, not in the original): M or a click on the
// minimap opens the current level's 32x32 grid.
// Left click a block to add or edit a point of interest (an empty label
// removes it), right click one to remove it; hovering shows its label.
// Game time stands still while the map is open.

const (
	bigCell = 6
	bigX    = 4
	bigY    = 4
	bigMax  = 13 // label length (font8 columns 26-38 of the side panel)
)

func (e *Engine) bigMap() {
	sc := e.Scr
	sc.CopyPage(0, 14)
	mm := e.MiniMap
	e.MiniMap = false // present() would draw it over the map
	defer e.setFont(e.setFont(e.font8))
	defer func() {
		sc.CopyPage(14, 0)
		e.MiniMap = mm
		e.tickOfs = e.ticks - e.Sys.Millis()/tickMillis // no time passed
	}()
	cellAt := func(x, y int) int {
		cx, cy := (x-bigX)/bigCell, (y-bigY)/bigCell
		if x < bigX || y < bigY || cx > 31 || cy > 31 {
			return -1
		}
		return cy<<5 | cx
	}
	for {
		e.mapColors()
		sc.FillRect(0, 0, 0, 319, 199, 0)
		for blk := 0; blk < 1024; blk++ {
			if c, ok := e.mapCell(blk); ok {
				x, y := bigX+(blk&31)*bigCell, bigY+(blk>>5)*bigCell
				sc.FillRect(0, x, y, x+bigCell-2, y+bigCell-2, c)
			}
		}
		e.drawPartyMark(0, bigX+(e.CurrentBlock&31)*bigCell+bigCell/2-1, bigY+(e.CurrentBlock>>5)*bigCell+bigCell/2-1)
		e.printShadow("LEVEL "+itoa(e.CurrentLevel), 208, 8, 0xf, 0)
		for i, l := range []string{"LEFT: ADD/EDIT", "RIGHT: REMOVE", "M/ESC: CLOSE"} {
			e.printShadow(l, 208, 160+i*10, 0xf, 0)
		}
		mx, my := e.Sys.Mouse()
		if l, ok := e.pois[e.CurrentLevel][cellAt(mx, my)]; ok {
			e.printShadow(l, 208, 40, e.miniPOI, 0)
		}
		e.present()
		e.Sys.Sleep(16)
		for {
			ev, ok := e.poll()
			if !ok {
				break
			}
			switch {
			case ev.Kind == EvQuit:
				panic(errQuit)
			case ev.Kind == EvKeyDown && (ev.Key == KeyEsc || ev.Key == KeyMap):
				return
			case ev.Kind != EvMouseDown:
			case cellAt(ev.X, ev.Y) < 0:
				if ev.Button == 2 {
					return
				}
			case ev.Button == 2:
				delete(e.pois[e.CurrentLevel], cellAt(ev.X, ev.Y))
			default:
				e.editPOI(cellAt(ev.X, ev.Y))
			}
		}
	}
}

// editPOI edits block blk's label on the current level with the original
// line editor; an empty label removes the point, Esc leaves it as it was.
func (e *Engine) editPOI(blk int) {
	buf := make([]byte, bigMax+1)
	copy(buf, e.pois[e.CurrentLevel][blk])
	e.printShadow("LABEL:", 208, 40, 0xf, 0)
	n := e.inputString(buf, 26, 52, bigMax, 0xf, 0, 0xc)
	switch {
	case n < 0:
	case n == 0:
		delete(e.pois[e.CurrentLevel], blk)
	default:
		if e.pois == nil {
			e.pois = map[int]map[int]string{}
		}
		if e.pois[e.CurrentLevel] == nil {
			e.pois[e.CurrentLevel] = map[int]string{}
		}
		e.pois[e.CurrentLevel][blk] = cstr(buf)
	}
}
