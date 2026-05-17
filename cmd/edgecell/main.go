package main

import (
	"fmt"
	"os"
	"strconv"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: edgecell <nvm> <cell>")
		os.Exit(2)
	}
	cell, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad cell:", err)
		os.Exit(2)
	}
	nvm, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load nvm:", err)
		os.Exit(1)
	}
	for i, e := range nvm.InternalEdges {
		if int(e.Cell0) != cell && int(e.Cell1) != cell {
			continue
		}
		fmt.Printf("[%3d] flag=%d dir=%d/%d cells=%d/%d (%.3f,%.3f)..(%.3f,%.3f)\n",
			i, e.Flag, e.Dir0, e.Dir1, e.Cell0, e.Cell1, e.MinX, e.MinZ, e.MaxX, e.MaxZ)
	}
}
