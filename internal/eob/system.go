package eob

// EventKind classifies input events.
type EventKind int

// Input event kinds.
const (
	EvKeyDown EventKind = iota + 1
	EvMouseDown
	EvMouseUp
	EvQuit
	EvMouseMove
	EvDebugGoto // X=block, Y=dir (testing aid)
)

// Event is an input event delivered by the platform layer.
type Event struct {
	Kind   EventKind
	Key    int // platform-independent key code (see keys.go)
	Ascii  rune
	Button int // 1 left, 2 right
	X, Y   int
}

// System is the platform interface (window, input, timing).
type System interface {
	// Present shows page 0 with the current palette.
	Present(pix []byte, pal []byte)
	// PollEvent returns the next pending input event.
	PollEvent() (Event, bool)
	// Mouse returns the current mouse position in 320x200 coordinates.
	Mouse() (int, int)
	// Millis returns a monotonic millisecond clock.
	Millis() uint32
	// Sleep blocks for ms milliseconds.
	Sleep(ms int)
}

// SoundPlayer is optionally implemented by a System to play sound effects;
// id is the game's sound id (index into the ADLIB.DAT id table).
type SoundPlayer interface {
	PlaySound(id int)
}

// DistSoundPlayer is optionally implemented by a SoundPlayer to play a sound
// dist blocks from the party (AdLib branch of S177d_0106).
type DistSoundPlayer interface {
	PlaySoundAt(id, dist int)
}

// MusicPlayer is optionally implemented by a System to play SOUND.DAT tracks
// (same format and id table as ADLIB.DAT).
type MusicPlayer interface {
	PlayMusic(id int)
}
