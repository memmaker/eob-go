package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// One .bin for the whole disc: each track ends where the next one's first INDEX starts.
func TestParseCue(t *testing.T) {
	cue := filepath.Join(t.TempDir(), "disc.cue")
	os.WriteFile(cue, []byte(`FILE "Disc Image.bin" BINARY
  TRACK 01 MODE1/2352
    INDEX 01 00:00:00
  TRACK 08 AUDIO
    INDEX 00 00:10:00
    INDEX 01 00:12:00
  TRACK 09 AUDIO
    INDEX 01 01:00:00
`), 0o644)
	ts, err := parseCue(cue)
	if err != nil || len(ts) != 3 {
		t.Fatal(ts, err)
	}
	if ts[0].end != 750*2352 || ts[1].n != 8 || !ts[1].audio || ts[1].start != 900*2352 || ts[1].end != 4500*2352 || ts[2].end != -1 {
		t.Fatalf("%+v", ts)
	}
	if filepath.Base(ts[1].file) != "Disc Image.bin" {
		t.Fatal(ts[1].file)
	}
}

// The FLAC tracks decode back to the exact CD samples, again after a loop.
func TestFLACRoundTrip(t *testing.T) {
	pcm := make([]byte, 4*10000) // not a multiple of the block size
	for i := range pcm {
		pcm[i] = byte(i*7 + i/97)
	}
	var b bytes.Buffer
	if err := encodeFLAC(&b, pcm); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "T.FLA")
	os.WriteFile(p, b.Bytes(), 0o644)
	r, size, err := openFLAC(p)
	if err != nil || size != int64(len(pcm)) {
		t.Fatal(size, err)
	}
	defer r.Close()
	for range 2 {
		got, _ := io.ReadAll(r)
		if !bytes.Equal(got, pcm) {
			t.Fatal("decoded audio differs")
		}
		r.Seek(0, io.SeekStart)
	}
	// the audio player also asks for its position and seeks around
	if at, err := r.Seek(30000, io.SeekStart); at != 30000 || err != nil {
		t.Fatal(at, err)
	}
	if at, _ := r.Seek(0, io.SeekCurrent); at != 30000 {
		t.Fatal(at)
	}
	r.Seek(-8, io.SeekEnd)
	if tail, _ := io.ReadAll(r); !bytes.Equal(tail, pcm[len(pcm)-8:]) {
		t.Fatal("seek from the end")
	}
}
