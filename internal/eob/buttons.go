package eob

// Button is an original GUI button definition (0x3c-byte struct in DGROUP).
type Button struct {
	ID         int
	Key, Key2  int
	Flags      int
	X, Y, W, H int
	Flags2     int
	CB         uint32 // seg<<16 | off of original callback
	Arg        int
}

const (
	dsButtons     = 0x3a62
	buttonSize    = 0x3c
	numButtonDefs = 94

	dsListPlayfield = 0x506a // 38 far pointers
	dsListSpellbook = 0x5102 // 52 far pointers (spell menu open)
)

func (e *Engine) loadButtons() {
	x := e.X
	e.ButtonDefs = make([]Button, numButtonDefs)
	for i := range e.ButtonDefs {
		o := dsButtons + i*buttonSize
		e.ButtonDefs[i] = Button{
			ID: int(x.U16(o + 4)), Key: int(x.U16(o + 6)), Key2: int(x.U16(o + 8)), Flags: int(x.U16(o + 0xe)),
			X: int(x.I16(o + 0x1e)), Y: int(x.I16(o + 0x20)), W: int(x.I16(o + 0x22)), H: int(x.I16(o + 0x24)),
			Flags2: int(x.U16(o + 0x2e)), CB: uint32(x.U16(o+0x32))<<16 | uint32(x.U16(o+0x30)), Arg: int(x.U16(o + 0x38)),
		}
	}
}

// buttonByID returns a button definition by its original id.
// buttonByID ports S1658_01a2 (find by id, NULL if absent).
func (e *Engine) buttonByID(id int) *Button {
	for i := range e.ButtonDefs {
		if e.ButtonDefs[i].ID == id {
			return &e.ButtonDefs[i]
		}
	}
	return nil
}

// buttonList reads an original list of far pointers to button structs and
// returns button indices (into ButtonDefs) in processing order.
func (e *Engine) buttonList(ds, n int) []int {
	r := make([]int, 0, n)
	for i := 0; i < n; i++ {
		a := int(e.X.U16(ds + i*4))
		k := (a - dsButtons) / buttonSize
		if k >= 0 && k < len(e.ButtonDefs) {
			r = append(r, k)
		}
	}
	return r
}

// processButtons ports the matching rules of the original GUI processor
// (ports S16a5_00bd; its pre-callback hook ds:fc7c is only ever set to NULL,
// by S16a5_0d78 at startup, so it is omitted: ports S16a5_0d78): keys match
// key/key2 exactly (word compare; shifted 0x1xx codes only match with shift,
// which the host never sends). Mouse: left press 0x100, held 0x200, release
// 0x400, up 0x800; right 0x1000/0x2000/0x4000/0x8000. Every static and
// runtime-built (O20f6_03b2, O2102_123c) button uses 0x1100 or 0x1000, so
// only the press triggers occur; their 0x2e toggle redraw (S16a5_07c3) has
// draw type 0 and shows nothing, and none sets hover 4 / drag 0x40.
// A callback returning false lets processing continue with the next button.
func (e *Engine) processButtons(ev Event, list []int) bool {
	for _, k := range list {
		b := &e.ButtonDefs[k]
		hit := false
		switch ev.Kind {
		case EvKeyDown:
			key := ev.Key
			hit = key != 0 && (b.Key == key || b.Key2 == key)
		case EvMouseDown, EvMouseUp:
			if b.W <= 0 || b.H <= 0 || ev.X < b.X || ev.X >= b.X+b.W || ev.Y < b.Y || ev.Y >= b.Y+b.H {
				continue
			}
			var mask int
			switch {
			case ev.Kind == EvMouseDown && ev.Button == 1:
				mask = 0x100
			case ev.Kind == EvMouseUp && ev.Button == 1:
				mask = 0x400
			case ev.Kind == EvMouseDown && ev.Button == 2:
				mask = 0x1000
			case ev.Kind == EvMouseUp && ev.Button == 2:
				mask = 0x4000
			}
			hit = b.Flags&mask != 0
		}
		if !hit {
			continue
		}
		h, ok := e.handlers[b.CB]
		if !ok {
			return true
		}
		if h(b.Arg, ev) {
			return true
		}
	}
	return false
}
