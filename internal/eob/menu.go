package eob

import (
	"errors"
	"path/filepath"
)

// Generic text menus (segment S14ef) and the main menu (O2106).
// Menu m uses screen dim 14+m: X, Y (relative to the current text dim),
// W (columns), H = item count, Unk8 = selected row, UnkA/UnkC = colours.

const (
	dsMainMenuStrs = 0x65de
	dsStrLoadFail  = 0x67de
	dsStrIntro     = 0x67a1
	mainMenuDim    = 6
	quickRows      = 8 // one 8px menu row
	menuDimBase    = 14
	menuCustom     = -1
)

// errQuit unwinds modal front-end loops when the window is closed.
var errQuit = errors.New("quit")

// menuIndex ports S14ef_0007: maps visible row i to a string index through
// an item mask (all bits set = identity).
func menuIndex(i int, mask uint32, bit int) int {
	if mask == 0xffffffff {
		return i
	}
	k := 0
	for ; i != 0; k++ {
		if mask&(1<<uint(k+bit)) != 0 {
			i--
		}
	}
	for mask&(1<<uint(k+bit)) == 0 {
		k++
	}
	return k
}

// menuCount ports S14ef_0107: number of set bits among the low n.
func menuCount(n int, mask uint32, bit int) int {
	c := 0
	for i := 0; i < n; i++ {
		if mask&(1<<uint(i+bit)) != 0 {
			c++
		}
	}
	return c
}

// menuStr returns item idx of far string table strs, or of e.menuItems
// when strs is menuCustom (extension menus).
func (e *Engine) menuStr(strs, idx int) string {
	if strs == menuCustom {
		return e.menuItems[idx]
	}
	return e.farStr(strs + idx*4)
}

func inBox(x, y, x1, y1, x2, y2 int) bool { return x >= x1 && x <= x2 && y >= y1 && y <= y2 } // ports S14ef_00e1

// menuOrigin returns the screen position of row 0 and the row height.
func (e *Engine) menuOrigin(m int) (x, y, rowH int) {
	d, t := e.Scr.Dims[menuDimBase+m], e.Scr.Dims[e.txtDim]
	return (t.X + d.X) * 8, t.Y + d.Y, e.font.H + e.menuGap
}

// drawMenu ports S14ef_0154.
func (e *Engine) drawMenu(m, strs int, mask uint32, bit, gap int) {
	e.menuGap = gap
	d := e.Scr.Dims[menuDimBase+m]
	x, y, rowH := e.menuOrigin(m)
	sel := menuIndex(d.Unk8, mask, bit)
	for i := 0; i < d.H; i++ {
		idx := menuIndex(i, mask, bit)
		s := e.menuStr(strs, idx)
		e.printShadow(s, x, y+i*rowH, byte(d.UnkA), 0)
		if idx == sel {
			e.printText(s, x, y+i*rowH, byte(d.UnkC), 0)
		}
	}
	e.present()
	e.flushInput() // S1aad_0480
}

// pollMenu ports S14ef_0274: handles one input event and returns the chosen
// string index or -1. Unhandled keys / clicks are left in e.menuKey.
func (e *Engine) pollMenu(m, strs int, mask uint32, bit int) int {
	d := &e.Scr.Dims[menuDimBase+m]
	last := d.H - 1
	old := d.Unk8 % d.H
	cur := old
	x, y0, rowH := e.menuOrigin(m)
	y1 := y0 - e.menuGap/2
	x2, y2 := x+d.W*e.font.W-1, y1+d.H*rowH-1
	res := -1
	e.menuKey = 0
	e.present()
	e.Sys.Sleep(16)
	mx, my := e.mouse()
	if !e.KbdMode && inBox(mx, my, x, y1, x2, y2) {
		cur = (my - y1) / rowH
	}
	if ev, ok := e.poll(); ok {
		switch ev.Kind {
		case EvQuit:
			panic(errQuit)
		case EvKeyDown:
			switch ev.Key {
			case KeyKP8:
				if cur--; cur < 0 {
					cur = last
				}
			case KeyKP2:
				if cur++; cur > last {
					cur = 0
				}
			case KeyKP7, KeyKP9:
				cur = 0
			case KeyKP1, KeyKP3:
				cur = last
			case KeyEnter, KeySpace, KeyKP5:
				res = cur
			default:
				e.menuKey = ev.Key
			}
		case EvMouseDown:
			e.mouseX, e.mouseY = ev.X, ev.Y
			if inBox(ev.X, ev.Y, x, y1, x2, y2) {
				cur = (ev.Y - y1) / rowH
				res = cur
			} else {
				e.menuKey = 0x40 + ev.Button // 0x41 / 0x42 mouse button codes
			}
		}
	}
	if cur != old {
		e.printText(e.menuStr(strs, menuIndex(old, mask, bit)), x, y0+old*rowH, byte(d.UnkA), 0)
		e.printText(e.menuStr(strs, menuIndex(cur, mask, bit)), x, y0+cur*rowH, byte(d.UnkC), 0)
	}
	if res != -1 {
		res = menuIndex(cur, mask, bit)
		e.flashMenuItem(e.menuStr(strs, res), x, y0+cur*rowH, byte(d.UnkA), byte(d.UnkC))
	}
	d.Unk8 = cur
	return res
}

// flashMenuItem ports S14ef_0078.
func (e *Engine) flashMenuItem(s string, x, y int, col, hi byte) {
	for i := 0; i < 3; i++ {
		e.printText(s, x, y, hi, 0)
		e.present()
		e.waitTicks(2)
		e.printText(s, x, y, col, 0)
		e.present()
		e.waitTicks(2)
	}
}

// showTitle ports O2106_078d: INTRO.CPS with the menu box.
func (e *Engine) showTitle() {
	e.mustCPS(e.X.Str(dsStrIntro), 2)
	e.Scr.FillRect(2, 0, 0x9f, 0x13f, 0xc7, e.Scr.Pages[2][0])
	e.Scr.DrawBox3D(2, 0x4d, 0xa5-quickRows, 0xf9, 0xc1, 0xd, 0xe, 0xc) // taller by one row for QUICK START
	e.Scr.DrawBox3D(2, 0x4c, 0xa4-quickRows, 0xfa, 0xc2, 0xd, 0xe, -1)
	e.Scr.CopyPage(2, 0)
}

// mainMenu ports O2106_0846. It returns true when a new party was created
// (false: the saved game was loaded).
func (e *Engine) mainMenu() (newGame bool, err error) {
	// QUICK START (extension) goes between LOAD and START A NEW PARTY.
	e.menuItems = []string{e.farStr(dsMainMenuStrs), "QUICK START\r", e.farStr(dsMainMenuStrs + 4), e.farStr(dsMainMenuStrs + 8)}
	e.Scr.Dims[mainMenuDim].Y -= quickRows
	e.Scr.Dims[mainMenuDim].H += quickRows
	e.Scr.Dims[menuDimBase+mainMenuDim].H = 4
	e.showTitle()
	for {
		e.setDim(mainMenuDim)
		e.clearDim()
		e.drawMenu(mainMenuDim, menuCustom, 0xffffffff, 0, 0)
		r := -1
		for r == -1 {
			r = e.pollMenu(mainMenuDim, menuCustom, 0xffffffff, 0)
		}
		switch r {
		case 0:
			if e.loadSave(e.savePath, true) == nil { // ds:a998 = 0xff
				return false, nil
			}
			e.clearDim()
			e.printMessage(e.X.Str(dsStrLoadFail))
			e.waitInput()
		case 1:
			e.quickStart()
			return true, nil
		case 2:
			e.chargen()
			return true, nil
		case 3:
			panic(errQuit)
		}
	}
}

// waitInput ports S1aad_053a: waits for a key or mouse click.
func (e *Engine) waitInput() {
	for {
		e.present()
		e.Sys.Sleep(16)
		ev, ok := e.poll()
		if !ok {
			continue
		}
		if ev.Kind == EvQuit {
			panic(errQuit)
		}
		if ev.Kind == EvKeyDown || ev.Kind == EvMouseDown {
			return
		}
	}
}

// frontEnd ports O2106_09cc (init via InitResources, then the main menu).
// It then starts the game as S0acd_0f72 does. That routine also calls
// O2116_2039, whose body is empty (push si / xor si,si / inc si / pop si);
// this ports O2116_2039 as nothing.
func (e *Engine) frontEnd() error {
	e.savePath = filepath.Join(e.dir, "EOBDATA.SAV")
	pal := e.Scr.Pal // INTRO.EXE is a separate process: EOB.EXE keeps its own palette
	e.playIntro()    // INTRO.EXE runs before EOB.EXE
	e.Scr.SetPalette(pal[:])
	if e.quit {
		panic(errQuit)
	}
	newGame, err := e.mainMenu()
	if err != nil {
		return err
	}
	e.setFont(e.font6) // S07a9_00f1
	if newGame {       // ports S0acd_1033 (level 1, block 0x1ea, north)
		if err := e.loadLevel(1); err != nil {
			return err
		}
		e.CurrentBlock, e.CurrentDirection = 0x1ea, 0
		if !e.KbdMode {
			e.setHandItem(0)
		}
	} else { // ports S0acd_0f72 (ds:a998 != 0: game loaded from the menu)
		if !e.KbdMode {
			e.setHandItem(e.HandItem)
		}
		if err := e.loadLevel(e.CurrentLevel); err != nil {
			return err
		}
	}
	// ports S0acd_0f72 tail (Run adds DrawPlayfield/DrawScene/setDim(7));
	// ds:dcaa=1 is not modelled (see kbdCommand).
	e.SpellbookOpen = false          // ds:f3b9
	e.SelectedChar, e.InvChar = 0, 0 // ds:1f3a
	e.loadPlayfield()
	return nil
}
