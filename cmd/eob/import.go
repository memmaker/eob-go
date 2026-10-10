package main

import (
	"errors"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/felix/goeob/internal/eob"
	"github.com/felix/goeob/internal/res"
)

// importGame (-import) finds an Eye of the Beholder install anywhere under
// src, copies its directory into dst and writes dst/COVER.PNG from the title
// picture for the floppy label.
func importGame(src, dst string) error {
	var dir string
	filepath.WalkDir(src, func(p string, d fs.DirEntry, _ error) error {
		if d != nil && strings.EqualFold(d.Name(), "EOB.EXE") {
			if ok, _ := filepath.Glob(filepath.Join(filepath.Dir(p), "*.PAK")); len(ok) > 0 || hasPak(filepath.Dir(p)) {
				dir = filepath.Dir(p)
				return fs.SkipAll
			}
		}
		return nil
	})
	if dir == "" {
		return errors.New("no Eye of the Beholder found")
	}
	os.MkdirAll(dst, 0o755)
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if e.IsDir() || !gameFile[strings.ToUpper(filepath.Ext(e.Name()))] {
			continue
		}
		if b, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
			if err := os.WriteFile(filepath.Join(dst, strings.ToUpper(e.Name())), b, 0o644); err != nil {
				return err
			}
		}
	}
	e, err := eob.New(dst, &headlessNop{})
	if err != nil {
		return err
	}
	if _, err := e.LoadCPS("INTRO", 0); err != nil {
		return err
	}
	pal, err := e.FS.Read("EOBPAL.COL") // the main menu's palette
	if err != nil {
		return err
	}
	img := image.NewPaletted(image.Rect(0, 0, 320, 200), res.Palette(pal))
	copy(img.Pix, e.Scr.Pages[0])
	f, err := os.Create(filepath.Join(dst, "COVER.PNG"))
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// gameFile are the extensions the engine reads (GOG folders also hold
// DOSBox, manuals and PDFs).
var gameFile = map[string]bool{".EXE": true, ".PAK": true, ".FNT": true, ".OVL": true, ".DAT": true, ".SAV": true, ".TMP": true, ".MAP": true}

// hasPak catches lower-case PAK names (Glob is case-sensitive).
func hasPak(dir string) bool {
	ents, _ := os.ReadDir(dir)
	for _, e := range ents {
		if strings.EqualFold(filepath.Ext(e.Name()), ".pak") {
			return true
		}
	}
	return false
}

type headlessNop struct{}

func (headlessNop) Present(pix, pal []byte)      {}
func (headlessNop) PollEvent() (eob.Event, bool) { return eob.Event{}, false }
func (headlessNop) Mouse() (int, int)            { return 0, 0 }
func (headlessNop) Millis() uint32               { return 0 }
func (headlessNop) Sleep(int)                    {}
