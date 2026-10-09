package eob

import "encoding/binary"

const charSize = 0xf3

// Character is the original 243-byte character record.
type Character struct {
	ID, Flags        byte
	Name             [11]byte
	StrCur, StrMax   int8
	StrExtCur        int8
	StrExtMax        int8
	IntCur, IntMax   int8
	WisCur, WisMax   int8
	DexCur, DexMax   int8
	ConCur, ConMax   int8
	ChaCur, ChaMax   int8
	HPCur, HPMax     int8
	AC               int8
	DisabledSlots    byte
	RaceSex          byte
	Class            byte
	Alignment        byte
	Portrait         int8
	Food             byte
	Level            [3]byte
	Experience       [3]uint32
	MageSpells       [30]int8
	ClericSpells     [30]int8
	MageSpellsAvail  uint32
	Inventory        [27]uint16
	Timers           [10]uint32
	Events           [10]int8
	EffectsRemainder [4]byte
	EffectFlags      uint32
	DamageTaken      byte
	SlotStatus       [5]int8
	Tail             [6]byte

	Face interface{} // runtime portrait shape (*gfx.Shape)
}

// NameStr returns the character name.
func (c *Character) NameStr() string { return cstr(c.Name[:]) }

// Active reports whether this slot holds a party member (flags & 1).
func (c *Character) Active() bool { return c.Flags&1 != 0 }

func (c *Character) unmarshal(b []byte) {
	c.ID, c.Flags = b[0], b[1]
	copy(c.Name[:], b[2:13])
	s := []*int8{&c.StrCur, &c.StrMax, &c.StrExtCur, &c.StrExtMax, &c.IntCur, &c.IntMax, &c.WisCur, &c.WisMax,
		&c.DexCur, &c.DexMax, &c.ConCur, &c.ConMax, &c.ChaCur, &c.ChaMax, &c.HPCur, &c.HPMax, &c.AC}
	for i, p := range s {
		*p = int8(b[13+i])
	}
	c.DisabledSlots, c.RaceSex, c.Class, c.Alignment = b[0x1e], b[0x1f], b[0x20], b[0x21]
	c.Portrait, c.Food = int8(b[0x22]), b[0x23]
	copy(c.Level[:], b[0x24:])
	for i := 0; i < 3; i++ {
		c.Experience[i] = binary.LittleEndian.Uint32(b[0x27+4*i:])
	}
	for i := 0; i < 30; i++ {
		c.MageSpells[i] = int8(b[0x37+i])
		c.ClericSpells[i] = int8(b[0x55+i])
	}
	c.MageSpellsAvail = binary.LittleEndian.Uint32(b[0x73:])
	for i := 0; i < 27; i++ {
		c.Inventory[i] = binary.LittleEndian.Uint16(b[0x77+2*i:])
	}
	for i := 0; i < 10; i++ {
		c.Timers[i] = binary.LittleEndian.Uint32(b[0xad+4*i:])
		c.Events[i] = int8(b[0xd5+i])
	}
	copy(c.EffectsRemainder[:], b[0xdf:])
	c.EffectFlags = binary.LittleEndian.Uint32(b[0xe3:])
	c.DamageTaken = b[0xe7]
	for i := 0; i < 5; i++ {
		c.SlotStatus[i] = int8(b[0xe8+i])
	}
	copy(c.Tail[:], b[0xed:0xf3])
}

func (c *Character) marshal(b []byte) {
	b[0], b[1] = c.ID, c.Flags
	copy(b[2:13], c.Name[:])
	s := []int8{c.StrCur, c.StrMax, c.StrExtCur, c.StrExtMax, c.IntCur, c.IntMax, c.WisCur, c.WisMax,
		c.DexCur, c.DexMax, c.ConCur, c.ConMax, c.ChaCur, c.ChaMax, c.HPCur, c.HPMax, c.AC}
	for i, v := range s {
		b[13+i] = byte(v)
	}
	b[0x1e], b[0x1f], b[0x20], b[0x21] = c.DisabledSlots, c.RaceSex, c.Class, c.Alignment
	b[0x22], b[0x23] = byte(c.Portrait), c.Food
	copy(b[0x24:], c.Level[:])
	for i := 0; i < 3; i++ {
		binary.LittleEndian.PutUint32(b[0x27+4*i:], c.Experience[i])
	}
	for i := 0; i < 30; i++ {
		b[0x37+i] = byte(c.MageSpells[i])
		b[0x55+i] = byte(c.ClericSpells[i])
	}
	binary.LittleEndian.PutUint32(b[0x73:], c.MageSpellsAvail)
	for i := 0; i < 27; i++ {
		binary.LittleEndian.PutUint16(b[0x77+2*i:], c.Inventory[i])
	}
	for i := 0; i < 10; i++ {
		binary.LittleEndian.PutUint32(b[0xad+4*i:], c.Timers[i])
		b[0xd5+i] = byte(c.Events[i])
	}
	copy(b[0xdf:], c.EffectsRemainder[:])
	binary.LittleEndian.PutUint32(b[0xe3:], c.EffectFlags)
	b[0xe7] = c.DamageTaken
	for i := 0; i < 5; i++ {
		b[0xe8+i] = byte(c.SlotStatus[i])
	}
	copy(b[0xed:0xf3], c.Tail[:])
}
