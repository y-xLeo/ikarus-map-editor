package main

import (
	"fmt"
	"os"
	"sromapedit/internal/sromap"
)

func main() {
	n, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		panic(err)
	}
	var walkable [sromap.NVMTotalTiles]bool
	for i := range walkable {
		walkable[i] = true
	}
	sromap.ApplyNVMNavRebuild(n, walkable)
	fmt.Printf("After ApplyNVMNavRebuild on %s:\n", os.Args[1])
	fmt.Printf("  Cells: %d (open=%d closed=%d)\n", len(n.Cells), n.OpenCellCount, uint32(len(n.Cells))-n.OpenCellCount)
	fmt.Printf("  IntEdges: %d\n", len(n.InternalEdges))
	sizes := map[string]int{}
	for _, c := range n.Cells {
		w := c.MaxX - c.MinX
		h := c.MaxZ - c.MinZ
		sizes[fmt.Sprintf("%.0fx%.0f", w, h)]++
	}
	fmt.Println("  Sizes:")
	for k, v := range sizes {
		fmt.Printf("    %s: %d\n", k, v)
	}
}
