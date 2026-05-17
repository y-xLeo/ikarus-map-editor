package main

import (
	"fmt"
	"os"
	"sort"

	"sromapedit/internal/sromap"
)

func main() {
	n, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		panic(err)
	}
	flagDist := map[uint16]int{}
	flag1Dist := map[uint8]int{}
	flag2Dist := map[uint8]int{}
	textureDist := map[uint16]int{}
	cellIDDist := map[int32]int{}
	for _, t := range n.Tiles {
		flagDist[t.Flag]++
		flag1Dist[uint8(t.Flag&0xFF)]++
		flag2Dist[uint8((t.Flag>>8)&0xFF)]++
		textureDist[t.TextureID]++
		cellIDDist[t.CellID]++
	}
	fmt.Println("=== Tile.Flag (uint16) distribution ===")
	type kv struct {
		k uint16
		v int
	}
	var entries []kv
	for k, v := range flagDist {
		entries = append(entries, kv{k, v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].v > entries[j].v })
	for _, e := range entries {
		fmt.Printf("  Flag=0x%04x (b1=0x%02x b2=0x%02x): %d tiles\n", e.k, byte(e.k&0xFF), byte((e.k>>8)&0xFF), e.v)
	}
	fmt.Println("\n=== Low byte (flags1) distribution ===")
	for k, v := range flag1Dist {
		fmt.Printf("  flags1=0x%02x: %d tiles\n", k, v)
	}
	fmt.Println("\n=== High byte (flags2) distribution ===")
	for k, v := range flag2Dist {
		fmt.Printf("  flags2=0x%02x: %d tiles\n", k, v)
	}
	fmt.Println("\n=== CellID range ===")
	var cellKeys []int32
	for k := range cellIDDist {
		cellKeys = append(cellKeys, k)
	}
	sort.Slice(cellKeys, func(i, j int) bool { return cellKeys[i] < cellKeys[j] })
	if len(cellKeys) > 0 {
		fmt.Printf("  unique cell IDs used: %d (range %d..%d)\n", len(cellKeys), cellKeys[0], cellKeys[len(cellKeys)-1])
	}
}
