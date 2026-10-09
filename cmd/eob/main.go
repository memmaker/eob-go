// Command eob runs the Eye of the Beholder reimplementation. It requires the
// original game files (EOB.EXE, *.PAK, FONT*.FNT) from your own copy.
package main

import (
	"flag"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text" //nolint:staticcheck // v1 text API is enough here
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"

	"github.com/felix/goeob/internal/adlib"
	"github.com/felix/goeob/internal/eob"
	"github.com/felix/goeob/internal/pak"
	"github.com/felix/goeob/internal/pcspk"
)

type platform struct {
	mu     sync.Mutex
	frame  []byte
	events []eob.Event
	mx, my int
	start  time.Time
	done   bool
	help   bool
	paused time.Duration // game time frozen while help is open
	helpAt time.Time
	snd    soundDriver
	mus    *adlib.Player
}

// soundDriver is the AdLib (adlib) or PC speaker (pcspk) effects player.
type soundDriver interface {
	Play(id int)
	PlayAt(id, dist int)
}

func (p *platform) PlayMusic(id int) {
	if p.mus != nil {
		p.mus.Play(id)
	}
}

func (p *platform) PlaySoundAt(id, dist int) {
	if p.snd != nil {
		p.snd.PlayAt(id, dist)
	}
}

func (p *platform) PlaySound(id int) {
	if p.snd != nil {
		p.snd.Play(id)
	}
}

func (p *platform) Present(pix, pal []byte) {
	p.mu.Lock()
	for i, c := range pix {
		r, g, b := pal[3*int(c)]&63, pal[3*int(c)+1]&63, pal[3*int(c)+2]&63
		p.frame[4*i], p.frame[4*i+1], p.frame[4*i+2], p.frame[4*i+3] = r<<2|r>>4, g<<2|g>>4, b<<2|b>>4, 255
	}
	p.mu.Unlock()
}

func (p *platform) PollEvent() (eob.Event, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.events) == 0 {
		return eob.Event{}, false
	}
	ev := p.events[0]
	p.events = p.events[1:]
	return ev, true
}

func (p *platform) Mouse() (int, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.mx, p.my
}

func (p *platform) Millis() uint32 {
	p.mu.Lock()
	defer p.mu.Unlock()
	d := time.Since(p.start) - p.paused
	if p.help {
		d -= time.Since(p.helpAt)
	}
	return uint32(d.Milliseconds())
}
func (p *platform) Sleep(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

var keyMap = map[ebiten.Key]int{
	ebiten.KeyArrowUp: eob.KeyKP8, ebiten.KeyArrowDown: eob.KeyKP2, ebiten.KeyArrowLeft: eob.KeyKP4, ebiten.KeyArrowRight: eob.KeyKP6,
	ebiten.KeyNumpad8: eob.KeyKP8, ebiten.KeyNumpad2: eob.KeyKP2, ebiten.KeyNumpad4: eob.KeyKP4, ebiten.KeyNumpad6: eob.KeyKP6,
	ebiten.KeyNumpad7: eob.KeyKP7, ebiten.KeyNumpad9: eob.KeyKP9, ebiten.KeyNumpad1: eob.KeyKP1, ebiten.KeyNumpad3: eob.KeyKP3,
	ebiten.KeyW: eob.KeyKP8, ebiten.KeyS: eob.KeyKP2, ebiten.KeyA: eob.KeyKP4, ebiten.KeyD: eob.KeyKP6, ebiten.KeyQ: eob.KeyKP7, ebiten.KeyE: eob.KeyKP9,
	ebiten.KeyEscape: eob.KeyEsc, ebiten.KeyEnter: eob.KeyEnter, ebiten.KeySpace: eob.KeySpace, ebiten.KeyBackspace: eob.KeyBack,
	ebiten.KeyMinus: eob.KeyMinus, ebiten.KeyEqual: eob.KeyEqual, ebiten.KeyNumpadSubtract: eob.KeyKPSub, ebiten.KeyNumpadAdd: eob.KeyKPAdd,
	ebiten.KeyF1: eob.KeyF1, ebiten.KeyF2: eob.KeyF2, ebiten.KeyF3: eob.KeyF3, ebiten.KeyF4: eob.KeyF4, ebiten.KeyF5: eob.KeyF5, ebiten.KeyF6: eob.KeyF6,
	// letter commands, as original key numbers (WASD/QE stay movement)
	ebiten.KeyC: 48, ebiten.KeyI: 24, ebiten.KeyP: 26, ebiten.KeyU: 23, ebiten.KeyT: 21, ebiten.KeyG: 35, ebiten.KeyR: 52, ebiten.KeyM: eob.KeyMap,
	ebiten.KeyDigit1: 2, ebiten.KeyDigit2: 3, ebiten.KeyDigit3: 4, ebiten.KeyDigit4: 5, ebiten.KeyDigit5: 6,
	ebiten.KeyF: eob.KeyAttack, ebiten.KeyTab: eob.KeyTab, ebiten.KeyComma: 53, ebiten.KeyPeriod: 54,
}

// helpText is the Ctrl+H overlay (extension; the game clock pauses while open).
var helpText = []string{
	"KEYS          Ctrl+H / pad Back: close",
	"W S  Up Dn    forward, back",
	"A D  Lt Rt    step left, right",
	"Q E           turn left, right",
	"F1-F6 inventory  I inventory  P info",
	"C camp  Esc close  1-5 , . spells",
	"F attack  Space turn mode  Tab minimap",
	"M or minimap click: map, POI notes",
	"Kbd mode: U use T throw G get R wall",
	"Ctrl+F or F7-F10    fullscreen",
	"Pad: stick cursor, A/B click,",
	" D-pad move/turn, LB/RB strafe,",
	" X attack, L3 turn mode, Y map,",
	" Start camp",
}

// extAscii holds the characters the original keyboard driver reports for
// editing keys (read by the line editor S082c_000b).
var extAscii = map[ebiten.Key]rune{ebiten.KeyArrowLeft: 0xb5, ebiten.KeyArrowRight: 0xb3}

type game struct {
	p              *platform
	img            *ebiten.Image
	vx, vy         float64 // cursor, driven by the mouse or the left stick
	lastMX, lastMY int
}

// padKeys maps standard-layout gamepad buttons to game keys (extension).
var padKeys = map[ebiten.StandardGamepadButton]int{
	ebiten.StandardGamepadButtonLeftTop: eob.KeyKP8, ebiten.StandardGamepadButtonLeftBottom: eob.KeyKP2,
	ebiten.StandardGamepadButtonLeftLeft: eob.KeyKP7, ebiten.StandardGamepadButtonLeftRight: eob.KeyKP9,
	ebiten.StandardGamepadButtonFrontTopLeft: eob.KeyKP4, ebiten.StandardGamepadButtonFrontTopRight: eob.KeyKP6,
	ebiten.StandardGamepadButtonRightLeft: eob.KeyAttack, ebiten.StandardGamepadButtonLeftStick: eob.KeySpace, ebiten.StandardGamepadButtonRightTop: eob.KeyTab,
	ebiten.StandardGamepadButtonCenterRight: 48, // camp
	ebiten.StandardGamepadButtonRightStick:  eob.KeyEsc,
}

// padButtons maps gamepad buttons to mouse buttons at the cursor.
var padButtons = map[ebiten.StandardGamepadButton]int{
	ebiten.StandardGamepadButtonRightBottom: 1, ebiten.StandardGamepadButtonRightRight: 2,
}

func (g *game) Update() error {
	p := g.p
	mx, my := ebiten.CursorPosition()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done {
		return ebiten.Termination
	}
	if mx != g.lastMX || my != g.lastMY { // the real mouse moved: it wins
		g.vx, g.vy, g.lastMX, g.lastMY = float64(mx), float64(my), mx, my
	}
	var pads []ebiten.GamepadID
	pads = ebiten.AppendGamepadIDs(pads)
	padJust := func(b ebiten.StandardGamepadButton) bool {
		for _, id := range pads {
			if ebiten.IsStandardGamepadLayoutAvailable(id) && inpututil.IsStandardGamepadButtonJustPressed(id, b) {
				return true
			}
		}
		return false
	}
	for _, id := range pads {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		ax := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		ay := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		if ax*ax+ay*ay > 0.04 { // dead zone
			g.vx = min(max(g.vx+ax*4, 0), 319)
			g.vy = min(max(g.vy+ay*4, 0), 199)
		}
	}
	mx, my = int(g.vx), int(g.vy)
	p.mx, p.my = mx, my
	if ebiten.IsKeyPressed(ebiten.KeyControl) && inpututil.IsKeyJustPressed(ebiten.KeyH) || padJust(ebiten.StandardGamepadButtonCenterLeft) {
		if p.help = !p.help; p.help {
			p.helpAt = time.Now()
		} else {
			p.paused += time.Since(p.helpAt)
		}
		return nil
	}
	if p.help { // swallow input while help is shown
		ebiten.AppendInputChars(nil)
		return nil
	}
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
	if ctrl && inpututil.IsKeyJustPressed(ebiten.KeyF) || inpututil.IsKeyJustPressed(ebiten.KeyF7) ||
		inpututil.IsKeyJustPressed(ebiten.KeyF8) || inpututil.IsKeyJustPressed(ebiten.KeyF9) || inpututil.IsKeyJustPressed(ebiten.KeyF10) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen()) // F1-F6 are game keys
		return nil
	}
	for k, code := range keyMap {
		if inpututil.IsKeyJustPressed(k) && !(ctrl && k == ebiten.KeyF) {
			p.events = append(p.events, eob.Event{Kind: eob.EvKeyDown, Key: code, Ascii: extAscii[k]})
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDelete) {
		p.events = append(p.events, eob.Event{Kind: eob.EvKeyDown, Ascii: 0x7f})
	}
	for b, code := range padKeys {
		if padJust(b) {
			p.events = append(p.events, eob.Event{Kind: eob.EvKeyDown, Key: code})
		}
	}
	for b, n := range padButtons {
		for _, id := range pads {
			if !ebiten.IsStandardGamepadLayoutAvailable(id) {
				continue
			}
			if inpututil.IsStandardGamepadButtonJustPressed(id, b) {
				p.events = append(p.events, eob.Event{Kind: eob.EvMouseDown, Button: n, X: mx, Y: my})
			}
			if inpututil.IsStandardGamepadButtonJustReleased(id, b) {
				p.events = append(p.events, eob.Event{Kind: eob.EvMouseUp, Button: n, X: mx, Y: my})
			}
		}
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		p.events = append(p.events, eob.Event{Kind: eob.EvKeyDown, Ascii: r})
	}
	for b, n := range map[ebiten.MouseButton]int{ebiten.MouseButtonLeft: 1, ebiten.MouseButtonRight: 2} {
		if inpututil.IsMouseButtonJustPressed(b) {
			p.events = append(p.events, eob.Event{Kind: eob.EvMouseDown, Button: n, X: mx, Y: my})
		}
		if inpututil.IsMouseButtonJustReleased(b) {
			p.events = append(p.events, eob.Event{Kind: eob.EvMouseUp, Button: n, X: mx, Y: my})
		}
	}
	return nil
}

func (g *game) Draw(screen *ebiten.Image) {
	g.p.mu.Lock()
	g.img.WritePixels(g.p.frame)
	help := g.p.help
	g.p.mu.Unlock()
	screen.DrawImage(g.img, nil)
	if help {
		vector.DrawFilledRect(screen, 4, 4, 312, 192, color.RGBA{0, 0, 0, 220}, false)
		for i, l := range helpText {
			text.Draw(screen, l, basicfont.Face7x13, 10, 18+i*14, color.White)
		}
	}
}

func (g *game) Layout(int, int) (int, int) { return 320, 200 }

func main() {
	dir := flag.String("data", os.Getenv("EOB_DATA"), "path to your original Eye of the Beholder directory")
	save := flag.String("load", "", "original save file to load directly (default: main menu)")
	sound := flag.Int("sound", 1, "sound driver as EOB.EXE's second argument: 1 AdLib/Sound Blaster, 2 PC speaker, 3 Tandy (3-voice speaker), 4 none")
	flag.Parse()
	if *dir == "" {
		log.Fatal("set -data or EOB_DATA to your original Eye of the Beholder directory")
	}
	p := &platform{frame: make([]byte, 64000*4), start: time.Now()}
	if fs, err := pak.Open(*dir); err == nil {
		ctx := audio.NewContext(44100)
		switch *sound {
		case 1:
			if pl := startAdlib(ctx, fs, *dir, "ADLIB.DAT", "EOB.EXE"); pl != nil {
				p.snd = pl
			}
		case 2, 3:
			if pl := startSpeaker(ctx, fs, *dir, *sound); pl != nil {
				p.snd = pl
			}
		}
		p.mus = startAdlib(ctx, fs, *dir, "SOUND.DAT", "INTRO.EXE")
	}
	e, err := eob.New(*dir, p)
	if err != nil {
		log.Fatal(err)
	}
	e.NoSound = *sound == 4
	go func() {
		if err := e.Run(*save); err != nil {
			log.Println(err)
		}
		p.mu.Lock()
		p.done = true
		p.mu.Unlock()
	}()
	ebiten.SetWindowSize(960, 600)
	ebiten.SetWindowTitle("Eye of the Beholder")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetCursorMode(ebiten.CursorModeHidden) // the game draws its own cursor
	if err := ebiten.RunGame(&game{p: p, img: ebiten.NewImage(320, 200)}); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}

// startSpeaker loads PCSOUND.DAT for the PC speaker driver (sound 2: one
// voice, 3: the Tandy option's three voices) and streams it.
func startSpeaker(ctx *audio.Context, fs *pak.FS, dir string, sound int) *pcspk.Player {
	d, err := fs.Read("PCSOUND.DAT")
	if err != nil {
		return nil
	}
	exe, err := os.ReadFile(filepath.Join(dir, "EOB.EXE"))
	if err != nil {
		return nil
	}
	voices := 1
	if sound == 3 {
		voices = 3
	}
	pl, err := pcspk.New(d, exe, 44100, voices)
	if err != nil {
		log.Printf("pcspk: %v; sound disabled", err)
		return nil
	}
	if ap, err := ctx.NewPlayer(pl); err == nil {
		ap.SetBufferSize(50 * time.Millisecond)
		ap.Play()
	}
	return pl
}

// startAdlib loads an ADLIB.DAT-format file and streams its player, using
// the driver tables of the executable that plays it.
func startAdlib(ctx *audio.Context, fs *pak.FS, dir, name, exeName string) *adlib.Player {
	d, err := fs.Read(name)
	if err != nil {
		return nil
	}
	pl, err := adlib.New(d, 44100)
	if err != nil {
		return nil
	}
	exe, err := os.ReadFile(filepath.Join(dir, exeName))
	if err != nil || !pl.LoadDriver(exe) {
		log.Printf("adlib: driver tables not found in %s; sound disabled", exeName)
		return nil
	}
	if ap, err := ctx.NewPlayer(pl); err == nil {
		ap.SetBufferSize(50 * time.Millisecond)
		ap.Play()
	}
	return pl
}
