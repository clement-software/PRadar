// Command mkicon draws the PRadar application icon into an iconset directory.
// The mark is drawn here rather than shipped as an image so the icon is
// reproducible from a clean checkout and needs no drawing tool in the build.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// The palette matches the interface: a blue mark on a white plate.
var (
	brand = color.NRGBA{R: 0x1f, G: 0x4f, B: 0xe0, A: 0xff}
	plate = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

// sizes are the iconset entries macOS expects, each with its scale.
var sizes = []struct {
	name   string
	pixels int
}{
	{"icon_16x16.png", 16}, {"icon_16x16@2x.png", 32},
	{"icon_32x32.png", 32}, {"icon_32x32@2x.png", 64},
	{"icon_128x128.png", 128}, {"icon_128x128@2x.png", 256},
	{"icon_256x256.png", 256}, {"icon_256x256@2x.png", 512},
	{"icon_512x512.png", 512}, {"icon_512x512@2x.png", 1024},
}

func main() {
	out := flag.String("out", "", "iconset directory to write")
	flag.Parse()
	if *out == "" {
		fmt.Fprintln(os.Stderr, "usage: mkicon --out PRadar.iconset")
		os.Exit(2)
	}
	if err := os.MkdirAll(*out, 0o750); err != nil {
		fmt.Fprintln(os.Stderr, "mkicon:", err)
		os.Exit(1)
	}
	for _, size := range sizes {
		if err := writePNG(filepath.Join(*out, size.name), draw(size.pixels)); err != nil {
			fmt.Fprintln(os.Stderr, "mkicon:", err)
			os.Exit(1)
		}
	}
}

func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// draw renders the mark at the given size, supersampled so the arcs and the
// rounded plate stay smooth at 16 points.
func draw(size int) image.Image {
	const sample = 4
	big := size * sample
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			var r, g, b, a int
			for sy := range sample {
				for sx := range sample {
					c := markAt(float64(x*sample+sx)+0.5, float64(y*sample+sy)+0.5, float64(big))
					r, g, b, a = r+int(c.R), g+int(c.G), b+int(c.B), a+int(c.A)
				}
			}
			n := sample * sample
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(r / n), G: uint8(g / n), B: uint8(b / n), A: uint8(a / n)})
		}
	}
	return img
}

// markAt returns the colour of one supersampled point: a rounded plate with a
// blue border, a filled dot and the two radar arcs of the wordmark.
func markAt(x, y, size float64) color.NRGBA {
	unit := size / 1024
	inset := 72 * unit
	radius := 180 * unit
	border := 40 * unit

	distance := roundedRectDistance(x, y, inset, inset, size-inset, size-inset, radius)
	switch {
	case distance > 0:
		return color.NRGBA{} // outside the plate: transparent
	case distance > -border:
		return brand
	}

	dotX, dotY := size*0.40, size*0.62
	if math.Hypot(x-dotX, y-dotY) <= 62*unit {
		return brand
	}
	for _, arc := range []float64{190, 330} {
		distance := math.Abs(math.Hypot(x-dotX, y-dotY) - arc*unit)
		angle := math.Atan2(dotY-y, x-dotX)
		if distance <= 34*unit && angle > -0.15 && angle < math.Pi/2+0.15 {
			return brand
		}
	}
	return plate
}

// roundedRectDistance is negative inside the rounded rectangle and positive
// outside it, in pixels.
func roundedRectDistance(x, y, left, top, right, bottom, radius float64) float64 {
	halfWidth, halfHeight := (right-left)/2, (bottom-top)/2
	centreX, centreY := left+halfWidth, top+halfHeight
	dx := math.Abs(x-centreX) - (halfWidth - radius)
	dy := math.Abs(y-centreY) - (halfHeight - radius)
	outside := math.Hypot(math.Max(dx, 0), math.Max(dy, 0))
	inside := math.Min(math.Max(dx, dy), 0)
	return outside + inside - radius
}
