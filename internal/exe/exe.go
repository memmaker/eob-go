// Package exe gives access to data tables and strings inside the original
// EOB.EXE (DOS, Turbo C++ 1990). Nothing is copied into this program: all
// values are read from the user's executable at runtime.
package exe

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
)

// EXE is a loaded EOB.EXE image.
type EXE struct {
	File []byte
	DS   int // file offset of DGROUP
}

var dgroupSig = []byte("Turbo C++ - Copyright 1990 Borland")

// Load reads EOB.EXE and locates its data segment.
func Load(path string) (*EXE, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	i := bytes.Index(b, dgroupSig)
	if i < 4 {
		return nil, errors.New("EOB.EXE: data segment signature not found (unsupported version)")
	}
	return &EXE{File: b, DS: i - 4}, nil
}

// U8 reads a byte at DS offset o.
func (e *EXE) U8(o int) uint8 { return e.File[e.DS+o] }

// I8 reads a signed byte at DS offset o.
func (e *EXE) I8(o int) int8 { return int8(e.File[e.DS+o]) }

// U16 reads a little-endian word at DS offset o.
func (e *EXE) U16(o int) uint16 { return binary.LittleEndian.Uint16(e.File[e.DS+o:]) }

// I16 reads a signed little-endian word at DS offset o.
func (e *EXE) I16(o int) int16 { return int16(e.U16(o)) }

// Bytes copies n bytes at DS offset o.
func (e *EXE) Bytes(o, n int) []byte { return append([]byte(nil), e.File[e.DS+o:e.DS+o+n]...) }

// I8s copies n signed bytes.
func (e *EXE) I8s(o, n int) []int8 {
	r := make([]int8, n)
	for i := range r {
		r[i] = e.I8(o + i)
	}
	return r
}

// I16s copies n signed words.
func (e *EXE) I16s(o, n int) []int16 {
	r := make([]int16, n)
	for i := range r {
		r[i] = e.I16(o + 2*i)
	}
	return r
}

// U16s copies n words.
func (e *EXE) U16s(o, n int) []uint16 {
	r := make([]uint16, n)
	for i := range r {
		r[i] = e.U16(o + 2*i)
	}
	return r
}

// Str reads a NUL-terminated string at DS offset o.
func (e *EXE) Str(o int) string {
	p := e.DS + o
	q := bytes.IndexByte(e.File[p:], 0)
	return string(e.File[p : p+q])
}

// FarPtrStr reads a string through a far pointer (off,seg) stored at DS offset o,
// assuming the segment is DGROUP.
func (e *EXE) FarPtrStr(o int) string { return e.Str(int(e.U16(o))) }

// FileU8 reads a byte at an absolute file offset.
func (e *EXE) FileU8(o int) uint8 { return e.File[o] }

// SegOffset returns the file offset of segment seg (load-relative) + off.
func (e *EXE) SegOffset(seg, off int) int { return 0x2800 + seg*16 + off }

// ImmStoreByte scans code at file range [from,to) for "mov byte ptr [ds:addr], imm8"
// (C6 06 lo hi imm) and returns the first immediate found.
func (e *EXE) ImmStoreByte(from, to, addr int) (byte, bool) {
	for p := from; p+5 <= to && p+5 <= len(e.File); p++ {
		if e.File[p] == 0xC6 && e.File[p+1] == 0x06 && int(e.File[p+2])|int(e.File[p+3])<<8 == addr {
			return e.File[p+4], true
		}
	}
	return 0, false
}

// OverlayBase returns the file offset of an overlay segment's code (stub seg).
func (e *EXE) OverlayBase(stubSeg int) int {
	p := 0x2800 + stubSeg*16
	fo := int(e.File[p+4]) | int(e.File[p+5])<<8 | int(e.File[p+6])<<16 | int(e.File[p+7])<<24
	return 0x33b60 + fo
}
