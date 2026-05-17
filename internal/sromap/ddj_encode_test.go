package sromap

import (
	"image"
	"image/color"
	"testing"
)

// Encode a known image, decode through our DDJ parser, verify the colors are
// roughly preserved (DXT1 is lossy so we allow some channel slack).
func TestEncodeDDJ_RoundTrip(t *testing.T) {
	w, h := 16, 16
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	// Make a small 4-quadrant pattern: red, green, blue, white.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var c color.RGBA
			switch {
			case x < w/2 && y < h/2:
				c = color.RGBA{255, 0, 0, 255}
			case x >= w/2 && y < h/2:
				c = color.RGBA{0, 255, 0, 255}
			case x < w/2 && y >= h/2:
				c = color.RGBA{0, 0, 255, 255}
			default:
				c = color.RGBA{255, 255, 255, 255}
			}
			src.Set(x, y, c)
		}
	}
	data, err := EncodeDDJ(src)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(data) != 20+128+(w/4)*(h/4)*8 {
		t.Fatalf("size mismatch: got %d, want %d", len(data), 20+128+(w/4)*(h/4)*8)
	}
	img, err := DecodeDDJ(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if img.Width != w || img.Height != h {
		t.Fatalf("dimensions: got %dx%d, want %dx%d", img.Width, img.Height, w, h)
	}
	if img.Format != "DXT1" {
		t.Fatalf("format: got %s, want DXT1", img.Format)
	}
	// Check each quadrant's centre pixel — DXT1 channel error is < 32.
	checks := []struct {
		x, y int
		r, g, b byte
	}{
		{w / 4, h / 4, 255, 0, 0},
		{w * 3 / 4, h / 4, 0, 255, 0},
		{w / 4, h * 3 / 4, 0, 0, 255},
		{w * 3 / 4, h * 3 / 4, 255, 255, 255},
	}
	for _, c := range checks {
		off := (c.y*w + c.x) * 4
		gotR := img.RGBA.Pix[off]
		gotG := img.RGBA.Pix[off+1]
		gotB := img.RGBA.Pix[off+2]
		if absDelta(gotR, c.r) > 32 || absDelta(gotG, c.g) > 32 || absDelta(gotB, c.b) > 32 {
			t.Errorf("pixel (%d,%d): got rgb(%d,%d,%d), want rgb(%d,%d,%d)",
				c.x, c.y, gotR, gotG, gotB, c.r, c.g, c.b)
		}
	}
}

func absDelta(a, b byte) int {
	d := int(a) - int(b)
	if d < 0 {
		return -d
	}
	return d
}
