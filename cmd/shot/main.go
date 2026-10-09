// Command shot drives the engine headlessly with scripted input and writes a PNG.
//
//	shot -data DIR -keys "up,up,right" -click 100,50 -o out.png
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/felix/goeob/internal/eob"
	"github.com/felix/goeob/internal/headless"
)

var names = map[string]int{"up": eob.KeyKP8, "down": eob.KeyKP2, "left": eob.KeyKP4, "right": eob.KeyKP6,
	"tl": eob.KeyKP7, "tr": eob.KeyKP9, "esc": eob.KeyEsc, "enter": eob.KeyEnter, "space": eob.KeySpace, "tab": eob.KeyTab,
	"f1": eob.KeyF1, "f2": eob.KeyF2, "f3": eob.KeyF3, "f4": eob.KeyF4, "minus": eob.KeyMinus, "plus": eob.KeyEqual}

func main() {
	dir := flag.String("data", os.Getenv("EOB_DATA"), "game dir")
	script := flag.String("s", "", "comma separated: key names, c:x:y (left click), r:x:y (right click), t:text")
	out := flag.String("o", "shot.png", "")
	scale := flag.Int("scale", 2, "")
	save := flag.String("load", "", "save file")
	intro := flag.Int("intro", -1, "play the intro and shoot the frame shown at this many ms")
	dump := flag.String("dump", "", "after the run, log party position and blocks x:y;x:y")
	flag.Parse()
	if *save == "" {
		*save = filepath.Join(*dir, "EOBDATA.SAV")
	}
	if *save == "menu" { // main menu / character generation
		*save = ""
	}
	s := &headless.Sys{}
	for _, tok := range strings.Split(*script, ",") {
		if tok == "" {
			continue
		}
		if k, ok := names[tok]; ok {
			s.Events = append(s.Events, eob.Event{Kind: eob.EvKeyDown, Key: k})
			continue
		}
		p := strings.Split(tok, ":")
		switch p[0] {
		case "c", "r":
			x, _ := strconv.Atoi(p[1])
			y, _ := strconv.Atoi(p[2])
			b := 1
			if p[0] == "r" {
				b = 2
			}
			s.Events = append(s.Events, eob.Event{Kind: eob.EvMouseDown, Button: b, X: x, Y: y}, eob.Event{Kind: eob.EvMouseUp, Button: b, X: x, Y: y})
		case "g":
			x, _ := strconv.Atoi(p[1])
			y, _ := strconv.Atoi(p[2])
			d, _ := strconv.Atoi(p[3])
			s.Events = append(s.Events, eob.Event{Kind: eob.EvDebugGoto, X: y<<5 | x, Y: d})
		case "w":
			n, _ := strconv.Atoi(p[1])
			for i := 0; i < n; i++ {
				s.Events = append(s.Events, eob.Event{Kind: eob.EvMouseMove})
			}
		case "t":
			for _, r := range p[1] {
				s.Events = append(s.Events, eob.Event{Kind: eob.EvKeyDown, Ascii: r})
			}
		}
	}
	if *intro >= 0 {
		is := &introSys{Sys: s, at: uint32(*intro)}
		e, err := eob.New(*dir, is)
		if err != nil {
			log.Fatal(err)
		}
		e.PlayIntro()
		if *dump != "" { // party position and the walls/flags of the listed blocks (x:y;x:y)
			log.Printf("level %d block %d,%d dir %d", e.CurrentLevel, e.CurrentBlock&31, e.CurrentBlock>>5, e.CurrentDirection)
			for _, xy := range strings.Split(*dump, ";") {
				var x, y int
				fmt.Sscanf(xy, "%d:%d", &x, &y)
				b := e.Blocks[y<<5|x]
				log.Printf("%d,%d walls % x flags %x", x, y, b.Walls, b.Flags)
			}
		}
		if err := s.SavePNG(*out, *scale); err != nil {
			log.Fatal(err)
		}
		return
	}
	e, err := eob.New(*dir, s)
	if err != nil {
		log.Fatal(err)
	}
	if err := e.Run(*save); err != nil {
		log.Fatal(err)
	}
	if *dump != "" { // party position and the walls/flags of the listed blocks (x:y;x:y)
		log.Printf("level %d block %d,%d dir %d", e.CurrentLevel, e.CurrentBlock&31, e.CurrentBlock>>5, e.CurrentDirection)
		for _, xy := range strings.Split(*dump, ";") {
			var x, y int
			fmt.Sscanf(xy, "%d:%d", &x, &y)
			b := e.Blocks[y<<5|x]
			log.Printf("%d,%d walls % x flags %x", x, y, b.Walls, b.Flags)
		}
	}
	if err := s.SavePNG(*out, *scale); err != nil {
		log.Fatal(err)
	}
}

// introSys presses Esc once the virtual clock reaches at, ending the intro.
type introSys struct {
	*headless.Sys
	at uint32
}

func (s *introSys) PollEvent() (eob.Event, bool) {
	if s.Millis() >= s.at {
		return eob.Event{Kind: eob.EvKeyDown, Key: eob.KeyEsc}, true
	}
	return eob.Event{}, false
}

func (s *introSys) Present(pix, pal []byte) {
	if s.Millis() <= s.at {
		s.Sys.Present(pix, pal)
	}
}
