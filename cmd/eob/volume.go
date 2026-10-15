package main

import (
	"encoding/binary"
	"io"
)

// faded is an s16 stream turned down by the goplay desk's fader for kind
// ("music" or "sfx", volume_js.go).
type faded struct {
	r    io.Reader
	kind string
}

func (f faded) Read(p []byte) (int, error) {
	n, err := f.r.Read(p)
	if g := volume(f.kind); g < 1 {
		for i := 0; i+1 < n; i += 2 {
			v := float64(int16(binary.LittleEndian.Uint16(p[i:])))
			binary.LittleEndian.PutUint16(p[i:], uint16(int16(v*g)))
		}
	}
	return n, err
}
