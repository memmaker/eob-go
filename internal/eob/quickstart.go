package eob

// Quick start (extension, not in the original): a fourth main menu entry
// that skips party creation with a balanced party - two fighters up front,
// a cleric and a fighter/mage behind. Stats are the best of 30 rolls.

var quickParty = []struct {
	name                  string
	raceSex, class, align byte
}{
	{"ALDRIC", 0, 2, 0},  // human male paladin, lawful good
	{"BRUNDAR", 6, 0, 0}, // dwarf male fighter, lawful good
	{"ISOLDE", 1, 4, 1},  // human female cleric, neutral good
	{"ELARION", 2, 8, 2}, // elf male fighter/mage, chaotic good
}

// quickStart fills e.Chars like chargen would.
func (e *Engine) quickStart() {
	e.cgInit() // face shapes
	for c, q := range quickParty {
		e.cg.slot = c
		ch := &e.Chars[c]
		*ch = Character{}
		ch.RaceSex, ch.Class, ch.Alignment = q.raceSex, q.class, q.align
		best, bestScore := *ch, -1
		for i := 0; i < 30; i++ {
			e.cgRollStats(c)
			if s := quickScore(ch); s > bestScore {
				best, bestScore = *ch, s
			}
		}
		*ch = best
		e.cg.sex = int(q.raceSex) % 2
		start := 0
		if e.cg.sex != 0 {
			start = 0x1a
		}
		p := e.cgNextFace(start+c*3, 1)
		e.cg.used[c] = p
		ch.Portrait, ch.Face = int8(p), e.cg.faces[p]
		copy(ch.Name[:], q.name)
	}
	e.cgFinish()
}

// quickScore favours hit points, then all attributes.
func quickScore(ch *Character) int {
	return int(ch.HPMax)*4 + int(ch.StrCur) + int(ch.StrExtCur)/10 + int(ch.IntCur) + int(ch.WisCur) +
		int(ch.DexCur) + int(ch.ConCur)*2 + int(ch.ChaCur)/2
}
