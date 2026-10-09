package eob

import "fmt"

// Resting (overlay O212a).

const (
	dsRestCounters  = 0x7f40 // 6 zero bytes copied into the per-char counters
	dsRestFmtGained = 0x7f46 // "%s gained %s.\r"
	dsRestFmtMemo   = 0x7f55 // "%s memorized %s.\r"
	dsRestFmtHeal   = 0x7f67 // "%s casts healing on %s.\r"
	dsRestFmtHours  = 0x7f80 // "Hours rested: %-4d"
	dsRestMonsters  = 0x7f93 // "\rYou can't rest here, monsters are near."
	restTicksHour   = 0x7ff8 // added to the game tick counter (ds:8492) per hour
)

// restParty ports O212a_0000. Returns true if monsters interrupted the rest.
func (e *Engine) restParty() bool {
	var cnt [6]byte
	copy(cnt[:], e.X.Bytes(dsRestCounters, 6))
	hours := 0
	if e.restMonstersNear() {
		return true
	}
	d := e.Scr.Dims[e.campMenu(0, 1)]
	e.campBox(d.X*8, d.Y, d.W*8, d.H, false, false)
	poisoned := false
	for c := range e.Chars {
		if e.charStatus(c, 1) && !e.charStatus(c, 0x10) {
			poisoned = true
		}
	}
	if poisoned && !e.campConfirm(0x3b) {
		return false
	}
	untilHealed, heal := false, false
	if e.mostInjured() > 0 {
		for c := range e.Chars {
			ch := &e.Chars[c]
			if e.classPartIndex(int(ch.Class), 2) != -1 && ch.HPCur > 0 && ch.Name[0] != 0 && ch.Food > 0 {
				heal = e.campConfirm(0x28)
				break
			}
		}
	}
	oldDim := e.txtDim
	e.setDimClear(7)
	injured := e.mostInjured()
	step := 0
	e.restShowHours(hours, true)
	e.setDim(7)
	e.setFont(e.font6)
	monsters := false
	for !e.quit {
		step++
		for c := range e.Chars {
			cnt[c]++
			ch := &e.Chars[c]
			if ch.Food == 0 || !e.charStatus(c, 5) {
				continue
			}
			if e.restLearn(c, &ch.ClericSpells, &cnt[c], dsRestFmtGained, 25) {
				e.waitTicks(5)
			}
			if e.findInvItem(c, 0x1d, -1) != -1 && e.restLearn(c, &ch.MageSpells, &cnt[c], dsRestFmtMemo, 0) {
				e.waitTicks(5)
			}
		}
		if heal && injured != 0 {
			e.restHeal(&cnt, &injured)
		}
		if step == 6 {
			step = 0
			hours++
			e.ticks += restTicksHour
			e.tickOfs += restTicksHour
			e.restShowHours(hours, false)
			for c := range e.Chars {
				if e.charStatus(c, 1) && !e.charStatus(c, 0x10) {
					e.damageChar(c, 10)
					e.seqDelay(5)
				}
			}
			if hours%8 == 0 {
				if e.restEightHours(hours) {
					ok := e.campConfirm(0x2f)
					e.restShowHours(hours, true)
					if !ok {
						injured = 0
						break
					}
				}
				injured = e.mostInjured()
			}
		}
		if !e.restUnlearned() && !untilHealed && injured != 0 {
			untilHealed = e.campConfirm(0x29)
			e.restShowHours(hours, true)
			if !untilHealed {
				break
			}
		}
		if e.restKeyPressed() {
			break
		}
		monsters = e.restMonstersNear()
		if !e.restUnlearned() && ((!untilHealed && !heal) || injured == 0) {
			break
		}
		if e.restKeyPressed() || monsters {
			break
		}
	}
	// original: S1aad_0480, setDim(4); O2102's camp loop later restores the
	// dim, which Go's camp does not track, so restore it here.
	e.flushInput()
	e.setDim(oldDim)
	e.setFont(e.font8)
	if injured == 0 && !monsters {
		e.campMessage(0x2b)
	}
	return monsters
}

// restLearn ports the spell learning loops of O212a_0000: the first
// unlearned (negative) spell whose level fits the counter becomes usable.
func (e *Engine) restLearn(c int, list *[30]int8, cnt *byte, fmtStr, spellBase int) bool {
	for i := 0; i < 30 && i/6+0x30 < int(*cnt); i++ {
		if list[i] >= 0 {
			continue
		}
		list[i] = -list[i]
		*cnt = 0x30
		e.printMessage(fmt.Sprintf(e.X.Str(fmtStr), e.Chars[c].NameStr(), e.Spells[spellBase+int(list[i])].Name))
		return true
	}
	return false
}

// restHeal ports the healing part of O212a_0000: clerics cast their cure
// spells on the most injured character.
func (e *Engine) restHeal(cnt *[6]byte, injured *int) {
	for c := range e.Chars {
		ch := &e.Chars[c]
		if e.classPartIndex(int(ch.Class), 2) == -1 {
			continue
		}
		if cnt[c] >= 0x30 {
			for _, v := range ch.ClericSpells {
				if v == -2 || v == -15 || v == -20 {
					return
				}
			}
		}
		for i := 0; i < 30 && *injured != 0; i++ {
			h := 0
			switch ch.ClericSpells[i] {
			case 2:
				h = e.rollDice(1, 8, 0)
			case 0xf:
				h = e.rollDice(2, 8, 1)
			case 0x14:
				h = e.rollDice(3, 8, 3)
			}
			if h == 0 {
				continue
			}
			ch.ClericSpells[i] = -ch.ClericSpells[i]
			cnt[c] = 0
			t := &e.Chars[*injured-1]
			e.printMessage(fmt.Sprintf(e.X.Str(dsRestFmtHeal), ch.NameStr(), t.NameStr()))
			e.waitTicks(5)
			t.HPCur = int8(min(int(t.HPCur)+h, int(t.HPMax)))
		}
		if *injured == 0 {
			return
		}
	}
}

// restEightHours ports the 8-hour block of O212a_0000: paladins regain lay
// on hands, fed characters regain 1 HP, starving ones take damage. Returns
// true if somebody is starving.
func (e *Engine) restEightHours(hours int) bool {
	starving := false
	for c := range e.Chars {
		ch := &e.Chars[c]
		if ch.Class == 2 {
			found := false
			for i := 0; i < 6; i++ {
				if ch.ClericSpells[i] == -0x18 {
					ch.ClericSpells[i], found = 0x18, true
					break
				}
				found = found || ch.ClericSpells[i] == 0x18
			}
			for i := 0; i < 6 && !found; i++ {
				if ch.ClericSpells[i] == 0 {
					ch.ClericSpells[i], found = 0x18, true
				}
			}
		}
		if !e.charStatus(c, 3) {
			continue
		}
		if ch.Food != 0 {
			if ch.HPCur < ch.HPMax {
				ch.HPCur++
				e.drawCharPanel(c)
			}
			continue // food decrease (ring check S0d1c_0163 < 0) is dead code
		}
		if hours%24 == 0 && ch.HPCur > -10 {
			e.damageChar(c, 1)
			starving = true
			e.drawCharPanel(c)
		}
	}
	return starving
}

// mostInjured ports O212a_07e2: 1 + index of the character with the
// lowest HP below maximum, 0 if nobody is hurt.
func (e *Engine) mostInjured() int {
	r, lo := -1, 900
	for c := range e.Chars {
		ch := &e.Chars[c]
		if e.charStatus(c, 3) && ch.HPCur < ch.HPMax && int(ch.HPCur) < lo {
			r, lo = c, int(ch.HPCur)
		}
	}
	return r + 1
}

// restShowHours ports O212a_0858.
func (e *Engine) restShowHours(hours int, full bool) {
	defer e.setFont(e.setFont(e.font8))
	d := e.Scr.Dims[0xa]
	if full {
		e.campBox(d.X*8, d.Y, d.W*8, d.H, false, false)
		e.campPrint(e.campStr(0x2a), (d.X+1)*8, d.Y+5, 9, 0)
	}
	e.printShadow(fmt.Sprintf(e.X.Str(dsRestFmtHours), hours), (d.X+1)*8, d.Y+0x14, 0xf, e.col.BoxFill)
	e.present()
	e.waitTicks(10)
}

// restMonstersNear ports O212a_09d2: lets the monsters move, then refuses
// to rest with a monster within one block.
func (e *Engine) restMonstersNear() bool {
	defer e.setFont(e.setFont(e.font6))
	e.noMonsterSound = true
	e.updateMonsters(0)
	e.updateMonsters(1)
	e.noMonsterSound = false
	for i := range e.Monsters {
		if blockDistance(e.CurrentBlock, int(e.Monsters[i].Block)) < 2 {
			old := e.txtDim
			e.setDim(7)
			e.printMessage(e.X.Str(dsRestMonsters))
			e.setDim(old)
			return true
		}
	}
	return false
}

// restUnlearned ports O212a_0a92: does anybody still have spells to learn?
func (e *Engine) restUnlearned() bool {
	neg := func(l *[30]int8) bool {
		for _, v := range l {
			if v < 0 {
				return true
			}
		}
		return false
	}
	for c := range e.Chars {
		ch := &e.Chars[c]
		if ch.HPCur <= 0 || ch.Food == 0 {
			continue
		}
		if e.classPartIndex(int(ch.Class), 2) != -1 || e.classPartIndex(int(ch.Class), 4) != 0 {
			if neg(&ch.ClericSpells) {
				return true
			}
		}
		if e.classPartIndex(int(ch.Class), 1) != -1 && e.findInvItem(c, 0x1d, -1) != -1 && neg(&ch.MageSpells) {
			return true
		}
	}
	return false
}

// restKeyPressed ports the S1aad_038e checks (any key or click aborts).
func (e *Engine) restKeyPressed() bool {
	e.present()
	ev, ok := e.poll()
	if ok && ev.Kind == EvQuit {
		e.quit = true
	}
	return ok && (ev.Kind == EvKeyDown || ev.Kind == EvMouseDown || ev.Kind == EvQuit)
}
