package eob

// Gamepad (extension): the platform reports the A/B face buttons and the
// D-pad as EvPadDown/EvPadUp (Key = Pad*, X/Y = cursor); poll and pollGame
// turn them into what the reading loop understands.
const (
	PadA = iota + 1
	PadB
	PadUp
	PadDown
	PadLeft
	PadRight
)

// poll is Sys.PollEvent for menus and dialogs: the D-pad moves the
// highlight, A confirms it (or clicks, once the cursor has moved) and B
// cancels.
func (e *Engine) poll() (Event, bool) { return e.pollPad(false) }

// pollGame is Sys.PollEvent for the 3D view and its panels: A/B click at the
// cursor, the D-pad moves and turns.
func (e *Engine) pollGame() (Event, bool) { return e.pollPad(true) }

func (e *Engine) pollPad(game bool) (Event, bool) {
	for {
		ev, ok := e.Sys.PollEvent()
		if !ok || ev.Kind != EvPadDown && ev.Kind != EvPadUp {
			return ev, ok
		}
		if ev, ok := e.padEvent(ev, game); ok {
			return ev, true
		}
	}
}

func (e *Engine) padEvent(ev Event, game bool) (Event, bool) {
	if ev.X != e.padX || ev.Y != e.padY { // the cursor moved: point and click
		e.padNav = false
	}
	if ev.Kind == EvPadUp {
		if ev.Key > PadB || !e.padClick[ev.Key] {
			return ev, false
		}
		e.padClick[ev.Key] = false
		return Event{Kind: EvMouseUp, Button: ev.Key, X: ev.X, Y: ev.Y}, true
	}
	e.padX, e.padY = ev.X, ev.Y
	k := 0
	switch {
	case ev.Key <= PadB && (game || ev.Key == PadA && !e.padNav):
		e.padClick[ev.Key] = true
		return Event{Kind: EvMouseDown, Button: ev.Key, X: ev.X, Y: ev.Y}, true
	case game:
		k = [...]int{PadUp: KeyKP8, PadDown: KeyKP2, PadLeft: KeyKP7, PadRight: KeyKP9}[ev.Key]
	default:
		k = [...]int{PadA: KeyEnter, PadB: KeyEsc, PadUp: KeyKP8, PadDown: KeyKP2, PadLeft: KeyKP4, PadRight: KeyKP6}[ev.Key]
		e.padNav = true
	}
	return Event{Kind: EvKeyDown, Key: k}, true
}

// mouse is Sys.Mouse for menu hover: while the D-pad drives the highlight
// and the cursor stays put, the cursor hovers nothing.
func (e *Engine) mouse() (int, int) {
	x, y := e.Sys.Mouse()
	if e.padNav && x == e.padX && y == e.padY {
		return -1, -1
	}
	e.padNav = false
	return x, y
}
