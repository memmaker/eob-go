package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"syscall/js"

	"github.com/hajimehoshi/ebiten/v2"
)

// hardwareCursor: in a browser the game's pointer is the page's own cursor (a CSS cursor
// image), so it moves with the hand at the browser's speed instead of the game's tick.
const hardwareCursor = true

// showCursor makes img (in 320x200 pixels, hot spot at hotX, hotY), scaled as the picture
// is in an outW x outH window, the cursor over the canvas; a nil img hides the cursor.
func showCursor(img *image.RGBA, hotX, hotY, outW, outH int) {
	if img == nil || outW == 0 {
		setCursor(nil, 0, 0)
		return
	}
	k := min(float64(outW)/320, float64(outH)/200)
	b := img.Bounds()
	w, h := max(1, int(float64(b.Dx())*k)), max(1, int(float64(b.Dy())*k))
	big := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			copy(big.Pix[big.PixOffset(x, y):][:4], img.Pix[img.PixOffset(int(float64(x)/k), int(float64(y)/k)):])
		}
	}
	setCursor(big, int(float64(hotX)*k), int(float64(hotY)*k))
}

// the cursor set last: the picture (trimmed) and hot spot, its CSS, and the CSS the page
// has now, which is "none" while the picture is drawn into the game's instead
var (
	cursorImg              *image.RGBA
	cursorPic              *ebiten.Image
	cursorHotX, cursorHotY int
	cursorCSS, cursorKey   string
	shownCSS               string
)

// the pointer's place in the page (CSS pixels), followed from the page's own events
var (
	pointerX, pointerY float64
	pointerKnown       bool
	pointerFollowed    bool
)

// followPointer tracks the pointer, and on every move switches between the page's cursor
// and the drawn one at once, before the browser shows its default cursor at an edge.
func followPointer() {
	if pointerFollowed {
		return
	}
	pointerFollowed = true
	js.Global().Call("addEventListener", "pointermove", js.FuncOf(func(_ js.Value, a []js.Value) any {
		pointerX, pointerY, pointerKnown = a[0].Get("clientX").Float(), a[0].Get("clientY").Float(), true
		showCSS()
		return nil
	}), true)
}

// maxWholeCursor is the largest cursor browsers show across the viewport's edge (Chrome
// shows its default cursor for a larger one that would cross it); edgeMargin keeps the
// switch to the drawn cursor ahead of the pointer.
const (
	maxWholeCursor = 32
	edgeMargin     = 16
)

// cssFits reports whether the page can show the cursor where the pointer is: a small one
// anywhere, a large one only well inside the viewport.
func cssFits() bool {
	b := cursorImg.Bounds()
	if b.Dx() <= maxWholeCursor && b.Dy() <= maxWholeCursor || !pointerKnown {
		return true
	}
	w, h := js.Global().Get("innerWidth").Float(), js.Global().Get("innerHeight").Float()
	x, y := pointerX-float64(cursorHotX), pointerY-float64(cursorHotY)
	return x >= edgeMargin && y >= edgeMargin && x+float64(b.Dx()) <= w-edgeMargin && y+float64(b.Dy()) <= h-edgeMargin
}

// showCSS gives the page the cursor's CSS, or none while drawCursor draws it.
func showCSS() {
	css := "none"
	if cursorImg != nil && cssFits() {
		css = cursorCSS
	}
	if css == shownCSS {
		return
	}
	shownCSS = css
	js.Global().Get("document").Get("body").Get("style").Set("cursor", css)
	if c := js.Global().Get("document").Call("querySelector", "canvas"); c.Truthy() {
		c.Get("style").Set("cursor", css)
	}
}

// opaqueBox cuts img to the box around its visible pixels and the hot spot: transparent
// rows and columns still count as cursor to the browser, so a cursor image larger than its
// shape would reach the viewport's edge before the shape does.
func opaqueBox(img image.Image, hotX, hotY int) (image.Image, int, int) {
	b := img.Bounds()
	box := image.Rect(hotX, hotY, hotX+1, hotY+1).Add(b.Min)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0 {
				box = box.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	box = box.Intersect(b)
	if box == b {
		return img, hotX, hotY
	}
	out := image.NewRGBA(image.Rect(0, 0, box.Dx(), box.Dy()))
	draw.Draw(out, out.Bounds(), img, box.Min, draw.Src)
	return out, hotX - (box.Min.X - b.Min.X), hotY - (box.Min.Y - b.Min.Y)
}

// setCursor makes img (in CSS pixels), with its hot spot at (hotX, hotY), the cursor over
// the canvas; a nil img hides the cursor.
func setCursor(img image.Image, hotX, hotY int) {
	followPointer()
	if img == nil {
		cursorImg, cursorKey = nil, ""
		showCSS()
		return
	}
	img, hotX, hotY = opaqueBox(img, hotX, hotY)
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba.Bounds().Min != (image.Point{}) {
		rgba = image.NewRGBA(image.Rect(0, 0, img.Bounds().Dx(), img.Bounds().Dy()))
		draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Src)
	}
	if key := fmt.Sprint(rgba.Bounds(), hotX, hotY) + string(rgba.Pix); key != cursorKey {
		var b bytes.Buffer
		if png.Encode(&b, rgba) != nil {
			return
		}
		cursorKey = key
		cursorCSS = fmt.Sprintf("url(data:image/png;base64,%s) %d %d, default",
			base64.StdEncoding.EncodeToString(b.Bytes()), hotX, hotY)
		cursorPic = nil
	}
	cursorImg, cursorHotX, cursorHotY = rgba, hotX, hotY
	showCSS()
}

// drawCursor draws the cursor into screen where the page can't show it (at the viewport's
// edge, see cssFits), at the pointer's latest place; call it last in Draw.
func drawCursor(screen *ebiten.Image) {
	if cursorImg == nil || cssFits() {
		return
	}
	if cursorPic == nil {
		cursorPic = ebiten.NewImageFromImage(cursorImg)
	}
	// Ebitengine fits the screen in the page, centred: k CSS pixels a screen pixel
	w, h := js.Global().Get("innerWidth").Float(), js.Global().Get("innerHeight").Float()
	sw, sh := float64(screen.Bounds().Dx()), float64(screen.Bounds().Dy())
	k := min(w/sw, h/sh)
	ox, oy := (w-sw*k)/2, (h-sh*k)/2
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(1/k, 1/k)
	op.GeoM.Translate((pointerX-float64(cursorHotX)-ox)/k, (pointerY-float64(cursorHotY)-oy)/k)
	screen.DrawImage(cursorPic, op)
}
