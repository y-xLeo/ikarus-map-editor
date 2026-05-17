// bsrdiff loads a real BSR, re-encodes it through our EncodeMinimalBSR
// using the parsed values, and prints a byte-by-byte diff. Any field we're
// silently corrupting shows up here.
package main

import (
	"fmt"
	"os"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: bsrdiff <real-bsr-path>")
		os.Exit(2)
	}
	real, err := os.ReadFile(os.Args[1])
	if err != nil {
		die("read:", err)
	}
	bsr, err := sromap.DecodeBSR(real)
	if err != nil {
		die("decode:", err)
	}
	if len(bsr.Materials) == 0 || len(bsr.Meshes) == 0 {
		die("bsr has no materials or meshes")
	}
	// Use first material + first mesh; the encoder only emits one of each.
	emitted, err := sromap.EncodeMinimalBSR(
		bsr.Name,
		bsr.Materials[0].Path,
		bsr.Meshes[0],
		bsr.CollisionMesh,
		[3]float32{0, 0, 0}, [3]float32{0, 0, 0}, // bbox unknown to caller; doesn't matter for diff structure
	)
	if err != nil {
		die("encode:", err)
	}
	fmt.Printf("real:    %d bytes\n", len(real))
	fmt.Printf("emitted: %d bytes\n", len(emitted))
	maxLen := len(real)
	if len(emitted) > maxLen {
		maxLen = len(emitted)
	}
	diffs := 0
	for i := 0; i < maxLen; i++ {
		var a, b byte
		var aok, bok bool
		if i < len(real) {
			a = real[i]
			aok = true
		}
		if i < len(emitted) {
			b = emitted[i]
			bok = true
		}
		if a != b || aok != bok {
			diffs++
			if diffs <= 60 {
				fmt.Printf("  0x%04X: real=%02x  emitted=%02x\n", i, a, b)
			}
		}
	}
	if diffs > 60 {
		fmt.Printf("  ... %d more diffs\n", diffs-60)
	}
	fmt.Printf("total diffs: %d\n", diffs)
}

func die(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
