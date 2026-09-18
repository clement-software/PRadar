package main

import (
	"image/color"
	"testing"
)

func TestDraw_ProducesTheMarkOnATransparentPlate(t *testing.T) {
	t.Parallel()
	const size = 256
	img := draw(size)
	at := func(x, y int) color.NRGBA {
		r, g, b, a := img.At(x, y).RGBA()
		return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
	}
	if corner := at(2, 2); corner.A != 0 {
		t.Errorf("the corner outside the rounded plate must be transparent, got %+v", corner)
	}
	if edge := at(size/2, 20); edge != brand {
		t.Errorf("the plate border must be the brand colour, got %+v", edge)
	}
	if inside := at(size-60, 60); inside != plate {
		t.Errorf("the plate must be white where the mark is absent, got %+v", inside)
	}
	if dot := at(size*40/100, size*62/100); dot != brand {
		t.Errorf("the radar dot must be the brand colour, got %+v", dot)
	}
	for _, size := range []int{16, 32, 1024} {
		if bounds := draw(size).Bounds(); bounds.Dx() != size || bounds.Dy() != size {
			t.Errorf("draw(%d) produced %v", size, bounds)
		}
	}
}
