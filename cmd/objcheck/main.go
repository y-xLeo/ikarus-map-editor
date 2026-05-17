package main

import (
	"fmt"
	"os"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: objcheck <path.obj>")
		os.Exit(2)
	}
	m, err := sromap.LoadOBJ(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse:", err)
		os.Exit(1)
	}
	fmt.Printf("vertices (unique v+vt combos): %d\n", len(m.Vertices))
	fmt.Printf("indices (triangle * 3): %d  -> %d tris\n", len(m.Indices), len(m.Indices)/3)
	fmt.Printf("bbox: min=%v max=%v\n", m.BBoxMin, m.BBoxMax)
	dx := m.BBoxMax[0] - m.BBoxMin[0]
	dy := m.BBoxMax[1] - m.BBoxMin[1]
	dz := m.BBoxMax[2] - m.BBoxMin[2]
	fmt.Printf("extents: x=%.4f y=%.4f z=%.4f\n", dx, dy, dz)
}
