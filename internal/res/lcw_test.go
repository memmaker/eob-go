package res

import (
	"bytes"
	"testing"
)

func TestEncodeLCWRoundTrip(t *testing.T) {
	var src []byte
	for i := range 5000 {
		switch {
		case i < 300:
			src = append(src, 7)
		case i < 2000:
			src = append(src, byte(i*i>>5))
		default:
			src = append(src, src[i%977])
		}
	}
	for _, n := range []int{1, 2, 3, 65, 66, 67, 300, 301, 5000} {
		if got := LCW(EncodeLCW(src[:n]), n); !bytes.Equal(got, src[:n]) {
			t.Fatalf("n=%d: round trip mismatch", n)
		}
	}
}
