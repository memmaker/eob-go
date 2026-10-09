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
// (S16a5_00bd): keys match key/key2; mouse buttons trigger on press or
// release depending on button flags (0x800/0x100 left, 0x8000/0x1000 right).
// A callback returning false lets processing continue with the next button.
func (e *Engine) processButtons(ev Event, list []int) bool {
	for _, k := range list {
		b := &e.ButtonDefs[k]
		hit := false
		switch ev.Kind {
		case EvKeyDown:
			key := ev.Key
			hit = key != 0 && ((b.Key&0xff) == key || (b.Key2 != 0 && (b.Key2&0xff) == key))
		case EvMouseDown, EvMouseUp:
			if b.W <= 0 || b.H <= 0 || ev.X < b.X || ev.X >= b.X+b.W || ev.Y < b.Y || ev.Y >= b.Y+b.H {
				continue
			}
			var mask int
			switch {
			case ev.Kind == EvMouseDown && ev.Button == 1:
				mask = 0x800
			case ev.Kind == EvMouseUp && ev.Button == 1:
				mask = 0x100
			case ev.Kind == EvMouseDown && ev.Button == 2:
				mask = 0x8000
			case ev.Kind == EvMouseUp && ev.Button == 2:
				mask = 0x1000
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
