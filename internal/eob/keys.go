package eob

// Westwood key codes used by the original button tables.
const (
	KeyEsc    = 110
	KeyEnter  = 43
	KeySpace  = 61
	KeyTab    = 16
	KeyAttack = 34    // F (extension: front-row attack, alternating)
	KeyMap    = 0x200 // M (extension: full-screen map; the wall click moved to R)
	KeyBack   = 15
	KeyMinus  = 12
	KeyEqual  = 13
	KeyKPSub  = 105
	KeyKPAdd  = 106
	KeyKP7    = 91 // turn left
	KeyKP4    = 92 // strafe left
	KeyKP1    = 93
	KeyKP8    = 96 // forward
	KeyKP5    = 97
	KeyKP2    = 98  // back
	KeyKP9    = 101 // turn right
	KeyKP6    = 102 // strafe right
	KeyKP3    = 103
	KeyF1     = 112
	KeyF2     = 113
	KeyF3     = 114
	KeyF4     = 115
	KeyF5     = 116
	KeyF6     = 117
	KeyLeftUp = 0x100 // release flag / shifted variants are matched ignoring this bit
)
