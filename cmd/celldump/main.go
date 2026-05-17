package main

import (
	"fmt"
	"os"
	"strconv"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 2 || len(os.Args) > 3 {
		fmt.Fprintln(os.Stderr, "usage: celldump <nvm> [count]")
		os.Exit(2)
	}
	count := 40
	if len(os.Args) == 3 {
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad count:", err)
			os.Exit(2)
		}
		count = v
	}
	nvm, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load nvm:", err)
		os.Exit(1)
	}
	if count > len(nvm.Cells) {
		count = len(nvm.Cells)
	}
	fmt.Printf("%s objects=%d cells=%d open=%d globals=%d internals=%d\n",
		os.Args[1], len(nvm.Objects), len(nvm.Cells), nvm.OpenCellCount, len(nvm.GlobalEdges), len(nvm.InternalEdges))
	for i := 0; i < count; i++ {
		c := nvm.Cells[i]
		kind := "OPEN"
		if uint32(i) >= nvm.OpenCellCount {
			kind = "CLOSED"
		}
		fmt.Printf("[%3d] %-6s (%.0f,%.0f)..(%.0f,%.0f) size=%.0fx%.0f objs=%v\n",
			i, kind, c.MinX, c.MinZ, c.MaxX, c.MaxZ, c.MaxX-c.MinX, c.MaxZ-c.MinZ, c.ObjectIndices)
	}
}
