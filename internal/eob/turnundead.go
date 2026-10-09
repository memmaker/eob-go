package eob

import "fmt"

// turnUndeadAuto ports S0d1c_0a43: on levels 2 and 7 the best cleric or
// paladin holding a holy symbol turns undead in front of the party.
func (e *Engine) turnUndeadAuto() {
	if e.CurrentLevel != 2 && e.CurrentLevel != 7 {
		return
	}
	blk := e.calcNewBlock(e.CurrentBlock, e.CurrentDirection)
	if e.Blocks[blk].Flags&7 == 0 {
		return
	}
	t := e.bestTurner()
	if t == 0 {
		return
	}
	level, who := t&0xff, t>>8
	turned := false
	roll := e.rollDice(1, 0x14, 0)
	for _, mi := range e.monsterList(blk, 4, 4, true, true) {
		m := &e.Monsters[mi]
		if e.mprop(m, 0xf)&4 == 0 || m.Flags&0x10 != 0 {
			continue
		}
		e.batchHits = true
		m.Flags |= 0x10
		turned = e.turnMonster(m, roll, level) || turned
	}
	if turned {
		e.printMessage(fmt.Sprintf(e.X.Str(0x550e), e.Chars[who].NameStr()))
		e.playSound(0x5f)
		e.wandSparkle()
		e.flushMonsterHits()
	}
	e.batchHits = false
}

// bestTurner ports S0d1c_0903: char<<8 | effective turn level (0 = none).
func (e *Engine) bestTurner() int {
	best, who := 0, 0
	for c := 0; c < 6; c++ {
		ch := &e.Chars[c]
		if !e.charStatus(c, 0xd) || !e.holdsHolySymbol(ch) {
			continue
		}
		if i := e.classPartIndex(int(ch.Class), 2); i != -1 {
			if l := int(int8(ch.Level[i])); l > best {
				best, who = l, c
			}
		} else if i := e.classPartIndex(int(ch.Class), 4); i != -1 {
			if l := int(int8(ch.Level[i])) - 2; l > best {
				best, who = l, c
			}
		}
	}
	return who<<8 + best
}

// holdsHolySymbol ports S0d1c_08b1.
func (e *Engine) holdsHolySymbol(ch *Character) bool {
	for h := 0; h < 2; h++ {
		if it := ch.Inventory[h]; it != 0 && e.ItemTypes[e.Items[it].Type].ExtraProps == 6 {
			return true
		}
	}
	return false
}

// turnMonster ports S0d1c_09cb: -1 in the turn table destroys, otherwise
// a roll at or above it makes the monster flee.
func (e *Engine) turnMonster(m *Monster, roll, level int) bool {
	need := e.turnTable(level, e.mprop(m, 0x19))
	if need == -1 {
		e.damageMonster(m, 100)
		return true
	}
	if roll < need {
		return false
	}
	m.Flags |= 8
	m.Dir = int8((e.blockDirection8(int(m.Block), e.CurrentBlock) ^ 4) >> 1)
	return true
}

// turnTable ports S0a72_0597.
func (e *Engine) turnTable(level, kind int) int {
	return int(e.X.I8(0x3618 + kind*14 + min(level, 14)))
}
