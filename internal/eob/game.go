package eob

const (
	dsDirBlockOffs = 0x1790 // int8[4] block offset per direction
	dsOppositeSide = 0x547e // byte[4] wall side to test when entering
	dsStrCantGo    = 0x3752
)

func cbAddr(seg, off int) uint32 { return uint32(seg)<<16 | uint32(off) }

func (e *Engine) initHandlers() {
	e.handlers = map[uint32]func(int, Event) bool{
		cbAddr(0x0acd, 0x04f8): func(int, Event) bool { e.moveRel(0); return true },
		cbAddr(0x0acd, 0x0529): func(int, Event) bool { e.moveRel(2); return true },
		cbAddr(0x0acd, 0x05ba): func(int, Event) bool { e.moveRel(3); return true },
		cbAddr(0x0acd, 0x05ff): func(int, Event) bool { e.moveRel(1); return true },
		cbAddr(0x0acd, 0x056c): func(int, Event) bool { e.turn(-1); return true },
		cbAddr(0x0acd, 0x0593): func(int, Event) bool { e.turn(1); return true },
		cbAddr(0x122e, 0x0933): func(int, Event) bool { e.clickedSceneCenter(); return true },
		cbAddr(0x0acd, 0x0644): func(a int, ev Event) bool { e.clickedFloor(a, ev.Button); return true },
		cbAddr(0x0acd, 0x0494): func(a int, _ Event) bool { e.throwFromMouse(a); return true },
		cbAddr(0x0acd, 0x0034): func(a int, _ Event) bool { e.openInventory(a); return true },
		cbAddr(0x0acd, 0x01c0): func(a int, _ Event) bool { e.clickedInvSlot(a); return true }, // ports S0acd_01c0 (mouse hide/show is host-side)
		cbAddr(0x0acd, 0x01e3): func(int, Event) bool { e.closeInventory(); return true },
		cbAddr(0x0acd, 0x018e): func(int, Event) bool { e.toggleCharInfo(); return true },
		cbAddr(0x0acd, 0x008c): func(int, Event) bool { e.invNextChar(-1); return true },
		cbAddr(0x0acd, 0x00eb): func(int, Event) bool { e.invNextChar(1); return true },
		cbAddr(0x0ef3, 0x25bd): func(a int, _ Event) bool { e.spellbookTab(a); return true },
		cbAddr(0x0ef3, 0x2929): func(a int, ev Event) bool { e.spellbookRow(a, ev); return true },
		cbAddr(0x0ef3, 0x08cf): func(a int, _ Event) bool { e.spellTargetClicked(a); return true },
		cbAddr(0x0acd, 0x078c): func(a int, _ Event) bool { // ports S0acd_078c; its S0d1c_0078/S0d1c_006d calls are empty stubs: ports S0d1c_0078 S0d1c_006d as no-ops
			if !e.charStatus(a, 1) {
				return true
			}
			e.selectChar(a)
			return false
		},
		cbAddr(0x0acd, 0x08d1): func(int, Event) bool { e.eatFood(int(e.SelectedChar)); return true }, // ports S0acd_08d1
		cbAddr(0x0acd, 0x0264): func(a int, _ Event) bool { return e.clickedCharName(a) },
		cbAddr(0x0acd, 0x0393): func(int, Event) bool { return e.cancelSwap() },
		cbAddr(0x2102, 0x0020): func(int, Event) bool { e.openCamp(); return true },
		cbAddr(0x0acd, 0x0415): func(a int, ev Event) bool { e.clickedCharHand(a, ev); return true },
	}
}

// calcNewBlock ports S0d1c_00da.
func (e *Engine) calcNewBlock(block, dir int) int {
	return (block + int(e.X.I8(dsDirBlockOffs+dir))) & 0x3ff
}

// checkMove ports S0d1c_0080: returns the new block or -1.
func (e *Engine) checkMove(block, dir int) int {
	nb := e.calcNewBlock(block, dir)
	w := e.Blocks[nb].Walls[e.u8(dsOppositeSide+dir)]
	if e.WllWallFlags[w]&1 == 0 {
		return -1
	}
	if e.Blocks[nb].Flags&7 != 0 {
		return -1
	}
	return nb
}

// moveRel ports the four step buttons S0acd_04f8 S0acd_0529 S0acd_05ba S0acd_05ff.
func (e *Engine) moveRel(rel int) {
	d := (e.CurrentDirection + rel) & 3
	nb := e.checkMove(e.CurrentBlock, d)
	if nb == -1 {
		e.cantGo()
		return
	}
	e.moveParty(nb)
}

// cantGo ports S0acd_0008.
func (e *Engine) cantGo() {
	e.printMessage(e.X.Str(dsStrCantGo))
	e.playSound(0x1d)
}

// moveParty ports S122e_0001.
func (e *Engine) moveParty(nb int) {
	e.updateAllMonsterDests()
	old := e.CurrentBlock
	e.CurrentBlock = nb
	e.runBlockScript(old, 2)
	e.runBlockScript(nb, 1)
	e.updateAllMonsterDests()
	e.StepCounter++
	e.kbdFloorPos = -1
	e.spendTurn()
	e.sceneUpdate = true
	e.printMessage(e.X.Str(0x5ab5))
	e.flyingObjectsHitParty()
}

// flyingObjectsHitParty ports S1414_0d63.
func (e *Engine) flyingObjectsHitParty() {
	for i := range e.FlyingObjects {
		o := &e.FlyingObjects[i]
		if o.Enable != 0 && e.flyingObjectHit(o, int(o.CurBlock), int(o.CurPos)) {
			e.endFlyingObject(o)
		}
	}
}

// turn ports the turn buttons S0acd_056c S0acd_0593.
func (e *Engine) turn(d int) {
	e.CurrentDirection = (e.CurrentDirection + d) & 3
	e.kbdFloorPos = -1
	e.sceneUpdate = true
	e.spendTurn()
}

// spendTurn lets one turn of game time run in turn-based mode (extension).
func (e *Engine) spendTurn() {
	if e.TurnBased {
		e.turnLeft = turnTicks
	}
}

// playSound ports S177d_0070: the platform maps id through ADLIB.DAT's table.
func (e *Engine) playSound(id int) {
	if s, ok := e.Sys.(SoundPlayer); ok && e.Opt80e6 != 0 {
		s.PlaySound(id)
	}
}

// playMusic plays SOUND.DAT track id: hook for the INTRO.EXE port (EOB.EXE
// itself never plays music; its Tunes flag 0x80e8 is only toggled/saved).
func (e *Engine) playMusic(id int) {
	if s, ok := e.Sys.(MusicPlayer); ok {
		s.PlayMusic(id)
	}
}

// activeButtons recomputes the list the original installs explicitly: it
// ports S0cf7_0007 (list reset) S0cf7_0028 (playfield / spellbook),
// ports S0cf7_00b4 (inventory) S0cf7_0140 (GuiMode 2) S0cf7_01cc (swap),
// ports S0cf7_0211 (spell target list ds:5462, set by S0ef3_059d).
func (e *Engine) activeButtons() []int {
	if e.book.waiting {
		return e.buttonList(0x5462, 7)
	}
	if e.SwapChar != -1 {
		return e.buttonList(dsListSwap, 7)
	}
	if e.GuiMode == 2 {
		if e.SpellbookOpen {
			return e.buttonList(0x528e, 0x13)
		}
		return e.buttonList(0x53c6, 0x20)
	}
	if e.GuiMode != 0 {
		if e.SpellbookOpen {
			return e.buttonList(0x52da, 0x3b)
		}
		return e.buttonList(0x51d2, 0x2f)
	}
	if e.SpellbookOpen {
		return e.buttonList(dsListSpellbook, 0x34)
	}
	return e.buttonList(dsListPlayfield, 0x26)
}

// Run is the engine main entry (runs on its own goroutine).
// Run ports S07c8_0001 S07a9_000a (startup/init; DOS memory, driver and
// buffer allocation replaced by Go/host).
// An empty savePath shows the main menu (load EOBDATA.SAV / new party).
func (e *Engine) Run(savePath string) (err error) {
	defer func() {
		if r := recover(); r == errQuit {
			err = nil
		} else if r != nil {
			panic(r)
		}
	}()
	if err := e.InitResources(); err != nil {
		return err
	}
	e.loadButtons()
	e.loadSpells()
	e.initHandlers()
	e.loadTimers()
	e.initTimerHandlers()
	e.seed = 1
	// S07a9_000a sets ds:f43c = 1 (bar graphs); ds:80e8 is 1 in the DS
	// image; S07c8_0037 sets ds:80e6 = (sound driver arg != '4'): on.
	e.Opt80e8, e.Opt80e6, e.OptF43c, e.BarGraphs = 1, 1, 1, true
	e.savePath = savePath
	if savePath == "" {
		if err := e.frontEnd(); err != nil {
			return err
		}
	} else {
		e.loadPlayfield()
		if err := e.LoadOriginalSave(savePath); err != nil {
			return err
		}
	}
	e.drawPage = 0
	e.DrawPlayfield()
	e.DrawScene(true)
	e.setDim(7)
	e.present()
	e.inGame = true
	e.ambientNext = e.ticks + uint32(e.rollDice(1, 10, 3)*18)
	for !e.quit {
		for {
			ev, ok := e.Sys.PollEvent()
			if !ok {
				break
			}
			if ev.Kind == EvQuit {
				return nil
			}
			if ev.Kind == EvDebugGoto {
				e.CurrentBlock, e.CurrentDirection = ev.X, ev.Y
				e.sceneUpdate = true
				continue
			}
			if ev.Kind == EvMouseDown || ev.Kind == EvMouseUp {
				e.mouseX, e.mouseY = ev.X, ev.Y
			}
			if ev.Kind == EvKeyDown && ev.Key == KeyTab { // extension
				e.MiniMap = !e.MiniMap
				continue
			}
			if ev.Kind == EvKeyDown && ev.Key == KeyAttack && e.GuiMode == 0 { // extension
				e.partyAttack()
				continue
			}
			if ev.Kind == EvKeyDown && ev.Key == KeySpace { // not in the original: DeepCrawlerForge-style turn mode
				e.TurnBased = !e.TurnBased
				e.turnLeft = 0
				continue
			}
			if !e.processButtons(ev, e.activeButtons()) && ev.Kind == EvKeyDown {
				e.kbdCommand(ev.Key) // ports S0acd_0e6c (leftover input -> S0acd_0d9d)
			}
		}
		if e.TurnBased && e.turnLeft == 0 { // world waits: hold game time
			e.tickOfs = e.ticks - e.Sys.Millis()/tickMillis
		}
		if nt := e.Sys.Millis()/tickMillis + e.tickOfs; nt != e.ticks {
			e.ticks = nt
			if e.turnLeft > 0 {
				e.turnLeft--
			}
			e.processTimers()
		}
		e.ambientSound()
		e.playSoundUpdate(0) // S0acd_0e8a flushes the queued sound each loop
		e.turnUndeadAuto()
		if e.sceneUpdate {
			e.DrawScene(true)
			e.sceneUpdate = false
		}
		e.present()
		e.Sys.Sleep(16)
	}
	return nil
}

func (e *Engine) present() {
	// composite the mouse cursor (item icon) onto a copy of page 0
	copy(e.Scr.Pages[15], e.Scr.Pages[0])
	mx, my := e.Sys.Mouse()
	shp := e.ItemIcons[0] // default pointer: ports O2106_0000 (ITEMICN 0,0,16x16 icon)
	hx, hy, hf := 0, 0, 0
	if e.HandItem != 0 {
		ic := int(byte(e.Items[e.HandItem].Icon))
		if ic < len(e.ItemIcons) {
			shp = e.ItemIcons[ic]
			hf = e.magicRemap(e.HandItem, ic)
		}
		hx, hy = 8, 8
	}
	if e.MiniMap && e.inGame && e.GuiMode == 0 {
		e.drawMiniMap(15)
	}
	if e.TurnBased { // turn-mode badge, top left of the 3D view; grey while a turn plays out
		fg := byte(0xf)
		if e.turnLeft > 0 {
			fg = 8
		}
		e.Scr.PrintText(e.font6, 15, "TURN", 3, 3, 0xc, 0) // shadow
		e.Scr.PrintText(e.font6, 15, "TURN", 2, 2, fg, 0)
	}
	if shp != nil {
		e.Scr.DrawShapeClip(15, shp, mx-hx, my-hy, 0, 0, 320, 200, hf)
	}
	e.Sys.Present(e.Scr.Pages[15], e.Scr.Pal[:])
}

// ambientSound ports the ambient part of S0acd_0e8a: on levels 1-3, every
// 4-13 seconds, sound 0x1e from a block near the party.
func (e *Engine) ambientSound() {
	if e.ambientNext >= e.ticks || e.CurrentLevel <= 0 || e.CurrentLevel >= 4 {
		return
	}
	e.ambientNext = e.ticks + uint32(e.rollDice(1, 10, 3)*18)
	e.playSoundAt(0x1e, e.CurrentBlock+e.rollDice(1, 12, -1))
}

// partyAttack (extension) is a right click on the primary hand of one
// front-row character, alternating between the two on each press. A
// character who is down, still recovering or holding a spellbook / holy
// symbol is passed over in favour of the other.
func (e *Engine) partyAttack() {
	for k := 0; k < 2; k++ {
		c := e.attackNext
		e.attackNext ^= 1
		ch := &e.Chars[c]
		if !e.charStatus(c, 0xd) || ch.DisabledSlots&1 != 0 {
			continue
		}
		if it := int(ch.Inventory[0]); it != 0 {
			if p := e.ItemTypes[e.Items[it].Type].ExtraProps & 0x7f; p == 5 || p == 6 {
				continue
			}
		}
		e.useItem(c, 0)
		return
	}
}
