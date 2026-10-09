package eob

import "github.com/felix/goeob/internal/gfx"

// InitResources loads the resident graphics, fonts and palette (game start).
func (e *Engine) InitResources() error {
	pal, err := e.FS.Read("EOBPAL.COL")
	if err != nil {
		return err
	}
	e.Scr.SetPalette(pal)
	fb, err := e.FS.Read("FONT6.FNT")
	if err != nil {
		return err
	}
	if e.font6, err = gfx.LoadFont(fb); err != nil {
		return err
	}
	if fb, err = e.FS.Read(e.X.Str(0x3067)); err != nil {
		return err
	}
	if e.font8, err = gfx.LoadFont(fb); err != nil {
		return err
	}
	e.font = e.font8 // S07c8_043d
	e.initGuiColors()
	// ports O2106_0496: no swap, cleared shape pointers (Go zero values),
	// character slot ids; the 0x514-byte per-character buffers are Go slices.
	e.SwapChar = -1
	for i := range e.Chars {
		e.Chars[i].ID = byte(i)
	}
	if err := e.loadItems(); err != nil {
		return err
	}
	if err := e.loadItemTypes(); err != nil {
		return err
	}
	e.loadDecorateShapes()
	return nil
}

// loadPlayfield loads PLAYFLD to page 0 and INVENT to page 2 (startGame).
func (e *Engine) loadPlayfield() {
	e.mustCPS(e.X.Str(0x37a4), 0)
	e.mustCPS(e.X.Str(0x37ac), 2)
}

// DebugEnter loads graphics and a level and places the party (test helper).
func (e *Engine) DebugEnter(level, block, dir int) error {
	if err := e.InitResources(); err != nil {
		return err
	}
	e.loadPlayfield()
	if err := e.loadLevel(level); err != nil {
		return err
	}
	e.CurrentBlock, e.CurrentDirection = block, dir
	return nil
}

// DebugLoadSave loads EOBDATA.SAV from the game directory.
func (e *Engine) DebugLoadSave(path string) error {
	if err := e.InitResources(); err != nil {
		return err
	}
	e.loadPlayfield()
	return e.LoadOriginalSave(path)
}
