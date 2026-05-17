package main

import (
	"fmt"
	"os"
	"sort"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: nvmclosure <terrain-only.nvm> <target.nvm>")
		os.Exit(2)
	}
	terrain, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load terrain:", err)
		os.Exit(1)
	}
	target, err := sromap.LoadNVM(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load target:", err)
		os.Exit(1)
	}
	counts := map[int]int{}
	noObject := 0
	extraClosed := 0
	for i := 0; i < sromap.NVMTotalTiles; i++ {
		terrainWalk := uint32(terrain.Tiles[i].CellID) < terrain.OpenCellCount
		targetWalk := uint32(target.Tiles[i].CellID) < target.OpenCellCount
		if !terrainWalk || targetWalk {
			continue
		}
		extraClosed++
		cellID := int(target.Tiles[i].CellID)
		if cellID < 0 || cellID >= len(target.Cells) || len(target.Cells[cellID].ObjectIndices) == 0 {
			noObject++
			continue
		}
		for _, idx := range target.Cells[cellID].ObjectIndices {
			counts[int(idx)]++
		}
	}
	fmt.Printf("extraClosed=%d noObject=%d\n", extraClosed, noObject)
	keys := make([]int, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, idx := range keys {
		obj := target.Objects[idx]
		fmt.Printf("object[%d] asset=%d uid=%d tiles=%d\n", idx, obj.AssetID, obj.UID, counts[idx])
	}
}
