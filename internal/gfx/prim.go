package gfx

// DrawLine draws a horizontal or vertical line (the game only uses those).
func (s *Screen) DrawLine(page, x1, y1, x2, y2 int, c byte) {
	if y1 == y2 {
		if x1 > x2 {
			x1, x2 = x2, x1
		}
		s.FillRect(page, x1, y1, x2, y1, c)
		return
	}
	if x1 == x2 {
		if y1 > y2 {
			y1, y2 = y2, y1
		}
		s.FillRect(page, x1, y1, x1, y2, c)
		return
	}
	// general Bresenham
	dx, dy := abs(x2-x1), -abs(y2-y1)
	sx, sy := 1, 1
	if x1 > x2 {
		sx = -1
	}
	if y1 > y2 {
		sy = -1
	}
	e := dx + dy
	for {
		s.FillRect(page, x1, y1, x1, y1, c)
		if x1 == x2 && y1 == y2 {
			break
		}
		e2 := 2 * e
		if e2 >= dy {
			e += dy
			x1 += sx
		}
		if e2 <= dx {
			e += dx
			y1 += sy
		}
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// DrawBox3D ports S082c_09d5: shaded box with optional fill (fill < 0: none).
func (s *Screen) DrawBox3D(page, x1, y1, x2, y2 int, c1, c2 byte, fill int) {
	if fill != -1 {
		s.FillRect(page, x1+1, y1+1, x2-1, y2-1, byte(fill))
	}
	s.DrawLine(page, x1+1, y1, x2, y1, c1)
	s.DrawLine(page, x2, y1, x2, y2-1, c1)
	s.DrawLine(page, x1, y1, x1, y2, c2)
	s.DrawLine(page, x1, y2, x2, y2, c2)
}

// DrawRect ports S082c_04c4: rectangle outline.
func (s *Screen) DrawRect(page, x1, y1, x2, y2 int, c byte) {
	s.DrawLine(page, x1, y1, x2, y1, c)
	s.DrawLine(page, x1, y2, x2, y2, c)
	s.DrawLine(page, x1, y1, x1, y2, c)
	s.DrawLine(page, x2, y1, x2, y2, c)
}
