// Package pak reads Westwood PAK archives used by Eye of the Beholder.
package pak

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Archive is an in-memory PAK archive.
type Archive struct {
	Name  string
	Files map[string][]byte
	Order []string
}

// Parse decodes a PAK archive. Format: repeated {u32 offset, NUL name}, terminated
// by an offset equal to the file size (or zero), followed by file data.
func Parse(name string, data []byte) (*Archive, error) {
	a := &Archive{Name: name, Files: map[string][]byte{}}
	type ent struct {
		name string
		off  uint32
	}
	var ents []ent
	p := 0
	first := uint32(0)
	for {
		if p+4 > len(data) {
			return nil, fmt.Errorf("pak %s: truncated header", name)
		}
		off := binary.LittleEndian.Uint32(data[p:])
		p += 4
		if len(ents) == 0 {
			first = off
		}
		if off == 0 || int(off) >= len(data) || (first != 0 && p > int(first)) {
			if off != 0 && int(off) <= len(data) && len(ents) > 0 {
				ents = append(ents, ent{"", off})
			}
			break
		}
		e := strings.IndexByte(string(data[p:]), 0)
		if e < 0 {
			return nil, fmt.Errorf("pak %s: bad name", name)
		}
		ents = append(ents, ent{string(data[p : p+e]), off})
		p += e + 1
	}
	for i, e := range ents {
		if e.name == "" {
			continue
		}
		end := uint32(len(data))
		if i+1 < len(ents) {
			end = ents[i+1].off
		}
		if e.off > end || int(end) > len(data) {
			return nil, fmt.Errorf("pak %s: bad entry %s", name, e.name)
		}
		u := strings.ToUpper(e.name)
		a.Files[u] = data[e.off:end]
		a.Order = append(a.Order, u)
	}
	return a, nil
}

// FS is a virtual filesystem over the game directory and its PAK archives.
type FS struct {
	Dir   string
	files map[string][]byte
}

// Open loads every *.PAK in dir.
func Open(dir string) (*FS, error) {
	fs := &FS{Dir: dir, files: map[string][]byte{}}
	m, _ := filepath.Glob(filepath.Join(dir, "*"))
	sort.Strings(m)
	for _, f := range m {
		if !strings.EqualFold(filepath.Ext(f), ".pak") || strings.EqualFold(filepath.Base(f), "EYE.PAK") {
			continue
		}
		d, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		a, err := Parse(filepath.Base(f), d)
		if err != nil {
			return nil, err
		}
		for _, n := range a.Order {
			fs.files[n] = a.Files[n]
		}
	}
	return fs, nil
}

// Read returns a file, looking first in loose files then PAKs.
// Read ports S1c0e_00db S1c0e_0400 S1c0e_03bb (open/read/close of the DOS
// PAK-aware file layer, collapsed to a whole-file read; the original
// searches PAK entries before loose files).
func (f *FS) Read(name string) ([]byte, error) {
	u := strings.ToUpper(name)
	if m, _ := filepath.Glob(filepath.Join(f.Dir, "*")); m != nil {
		for _, p := range m {
			if strings.EqualFold(filepath.Base(p), u) {
				return os.ReadFile(p)
			}
		}
	}
	if d, ok := f.files[u]; ok {
		return d, nil
	}
	return nil, fmt.Errorf("file not found: %s", name)
}

// Names lists all PAK-contained files.
func (f *FS) Names() []string {
	var n []string
	for k := range f.files {
		n = append(n, k)
	}
	sort.Strings(n)
	return n
}
