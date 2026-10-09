package eob

import "strings"

// sprintfC implements the small subset of C printf used by the game's format
// strings (%s, %d, %u, %x, %c, %%, with optional width / '-' flag).
func sprintfC(f string, args ...any) string {
	var b strings.Builder
	ai := 0
	next := func() any {
		if ai < len(args) {
			a := args[ai]
			ai++
			return a
		}
		return 0
	}
	for i := 0; i < len(f); i++ {
		c := f[i]
		if c != '%' {
			b.WriteByte(c)
			continue
		}
		i++
		if i >= len(f) {
			break
		}
		left, zero := false, false
		for i < len(f) && (f[i] == '-' || f[i] == '0') {
			if f[i] == '-' {
				left = true
			} else {
				zero = true
			}
			i++
		}
		w := 0
		for i < len(f) && f[i] >= '0' && f[i] <= '9' {
			w = w*10 + int(f[i]-'0')
			i++
		}
		for i < len(f) && (f[i] == 'l' || f[i] == 'h') {
			i++
		}
		if i >= len(f) {
			break
		}
		var s string
		switch f[i] {
		case 's':
			s, _ = next().(string)
		case 'd', 'i', 'u':
			s = itoa(toInt(next()))
		case 'x', 'X':
			s = strings.ToLower(hex(toInt(next())))
			if f[i] == 'X' {
				s = strings.ToUpper(s)
			}
		case 'c':
			s = string(rune(toInt(next())))
		case '%':
			s = "%"
		}
		for len(s) < w {
			if left {
				s += " "
			} else if zero {
				s = "0" + s
			} else {
				s = " " + s
			}
		}
		b.WriteString(s)
	}
	return b.String()
}

func toInt(a any) int {
	switch v := a.(type) {
	case int:
		return v
	case int8:
		return int(v)
	case int16:
		return int(v)
	case int32:
		return int(v)
	case uint8:
		return int(v)
	case uint16:
		return int(v)
	case uint32:
		return int(v)
	}
	return 0
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var d []byte
	for v > 0 {
		d = append([]byte{byte('0' + v%10)}, d...)
		v /= 10
	}
	if neg {
		return "-" + string(d)
	}
	return string(d)
}

func hex(v int) string {
	const h = "0123456789abcdef"
	if v == 0 {
		return "0"
	}
	u := uint16(v)
	var d []byte
	for u > 0 {
		d = append([]byte{h[u&15]}, d...)
		u >>= 4
	}
	return string(d)
}
