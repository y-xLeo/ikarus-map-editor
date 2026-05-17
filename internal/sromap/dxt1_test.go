package sromap

import (
	"image"
	"testing"
)

// TestDXT1Roundtrip encodes a synthetic gradient and decodes it back, then
// checks that pixels are within a small tolerance of the source. DXT1 is
// lossy but should reproduce smooth gradients with low error.
func TestDXT1Roundtrip(t *testing.T) {
	const w, h = 64, 64
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			off := (y*w + x) * 4
			v := byte((x * 255) / (w - 1))
			src.Pix[off] = v
			src.Pix[off+1] = v
			src.Pix[off+2] = v
			src.Pix[off+3] = 255
		}
	}
	encoded, err := EncodeDXT1(src.Pix, w, h)
	if err != nil {
		t.Fatalf("EncodeDXT1: %v", err)
	}
	if len(encoded) != w*h/2 {
		t.Fatalf("encoded size = %d, want %d", len(encoded), w*h/2)
	}

	decoded := make([]byte, w*h*4)
	if err := decodeDXT1(encoded, w, h, decoded); err != nil {
		t.Fatalf("decodeDXT1: %v", err)
	}
	// Check a few sample pixels for reasonable similarity (DXT1 tolerance ~20)
	const tol = 20
	for _, x := range []int{0, 8, 16, 24, 32, 40, 48, 56, 63} {
		off := x * 4 // first row
		want := byte((x * 255) / (w - 1))
		got := decoded[off]
		if abs8(int(got)-int(want)) > tol {
			t.Errorf("pixel x=%d: got=%d want=%d", x, got, want)
		}
	}
}

func abs8(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
