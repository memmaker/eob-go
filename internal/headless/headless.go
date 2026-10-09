// Package headless provides a scripted System for automated testing.
package headless

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"sync"

	"github.com/felix/goeob/internal/eob"
)

// Sys feeds scripted events and captures frames.
type Sys struct {
	mu     sync.Mutex
	Events []eob.Event
	Frame  []byte
	Pal    []byte
	t      uint32
	Idle   int // number of empty polls after the script ran out
	mx, my int
	OnIdle func()
	quit   bool // EvQuit sent: keep the last frame shown before it
}

func (s *Sys) Present(pix, pal []byte) {
	s.mu.Lock()
	if s.quit {
		s.mu.Unlock()
		return
	}
	s.Frame = append(s.Frame[:0], pix...)
	s.Pal = append(s.Pal[:0], pal...)
	s.mu.Unlock()
}

func (s *Sys) PollEvent() (eob.Event, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.Events) == 0 {
		s.Idle++
		if s.Idle > 5 {
			s.quit = true
			return eob.Event{Kind: eob.EvQuit}, true
		}
		return eob.Event{}, false
	}
	ev := s.Events[0]
	s.Events = s.Events[1:]
	if ev.Kind == eob.EvMouseMove { // scripted wait: one empty frame
		return eob.Event{}, false
	}
	if ev.Kind == eob.EvMouseDown || ev.Kind == eob.EvMouseUp {
		s.mx, s.my = ev.X, ev.Y
	}
	return ev, true
}

func (s *Sys) Mouse() (int, int) { return s.mx, s.my }
func (s *Sys) Millis() uint32    { return s.t }
func (s *Sys) Sleep(ms int)      { s.t += uint32(ms) }

// SavePNG writes the last frame scaled by k.
func (s *Sys) SavePNG(path string, k int) error {
	img := image.NewRGBA(image.Rect(0, 0, 320*k, 200*k))
	for y := 0; y < 200*k; y++ {
		for x := 0; x < 320*k; x++ {
			c := int(s.Frame[(y/k)*320+x/k])
			r, g, b := s.Pal[3*c]&63, s.Pal[3*c+1]&63, s.Pal[3*c+2]&63
			img.Set(x, y, color.RGBA{r<<2 | r>>4, g<<2 | g>>4, b<<2 | b>>4, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
