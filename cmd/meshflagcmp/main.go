package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) != 5 {
		fmt.Fprintln(os.Stderr, "usage: meshflagcmp <root> <rx> <ry> <nvm>")
		os.Exit(2)
	}
	rx, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad rx:", err)
		os.Exit(2)
	}
	ry, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad ry:", err)
		os.Exit(2)
	}
	mesh, err := sromap.LoadMesh(sromap.MeshPath(os.Args[1], rx, ry))
	if err != nil {
		fmt.Fprintln(os.Stderr, "load mesh:", err)
		os.Exit(1)
	}
	nvm, err := sromap.LoadNVM(os.Args[4])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load nvm:", err)
		os.Exit(1)
	}
	flags := mesh.TileFlagMap()
	type count struct{ open, closed int }
	counts := map[uint16]count{}
	for i, f := range flags {
		c := counts[f]
		if uint32(nvm.Tiles[i].CellID) < nvm.OpenCellCount {
			c.open++
		} else {
			c.closed++
		}
		counts[f] = c
	}
	keys := make([]int, 0, len(counts))
	for k := range counts {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		c := counts[uint16(k)]
		fmt.Printf("flag=0x%04x open=%d closed=%d\n", k, c.open, c.closed)
	}
}
