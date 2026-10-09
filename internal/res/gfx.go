package res

import (
	"encoding/binary"
	"image/color"
)

// Palette converts a 768-byte 6-bit VGA palette to RGBA colors.
func Palette(p []byte) color.Palette {
	pal := make(color.Palette, 256)
	for i := 0; i < 256; i++ {
		if 3*i+2 >= len(p) {
			pal[i] = color.RGBA{0, 0, 0, 255}
			continue
		}
		c := func(v byte) uint8 { v &= 63; return v<<2 | v>>4 }
		pal[i] = color.RGBA{c(p[3*i]), c(p[3*i+1]), c(p[3*i+2]), 255}
	}
	return pal
}

// VCN is a set of 8x8 4bpp wall/floor blocks with colour lookup tables.
type VCN struct {
	ColBg   [16]byte // colour map for background (floor/ceiling) blocks
	ColWall [16]byte // colour map for wall blocks
	Blocks  []byte   // 32 bytes per block
	Count   int
}

// DecodeVCN parses a decompressed VCN payload.
func DecodeVCN(d []byte) *VCN {
	v := &VCN{Count: int(binary.LittleEndian.Uint16(d))}
	copy(v.ColBg[:], d[2:18])
	copy(v.ColWall[:], d[18:34])
	v.Blocks = d[34:]
	return v
}

// DecodeU16 parses a raw little-endian u16 array (VMP files: count, entries...).
func DecodeU16(d []byte) []uint16 {
	n := len(d) / 2
	out := make([]uint16, n)
	for i := range out {
		out[i] = binary.LittleEndian.Uint16(d[2*i:])
	}
	return out
}
