// mtileflags: read .m file's per-tile Flag field and dump distribution.
// Layout:
//   12 bytes signature "JMXVMAPM1000"
//   36 blocks (6×6, zBlock-major then xBlock):
//     4   uint32  Flag (block)
//     2   uint16  EnvironmentId
//     17×17 × 7 = 2023 bytes vertices (Height/TextureData/Brightness)
//     1   sbyte   WaterType
//     1   uint8   WaterWaveType
//     4   float   WaterHeight
//     16×16 × 2 = 512 bytes tile flags  ← what we want
//     4   float   HeightMax
//     4   float   HeightMin
//     20  bytes   reserved
//   Total per block: 2575 bytes
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

const (
	HeaderSize    = 12
	BlockSize     = 2575
	BlockDim      = 6
	VertexDim     = 17
	TileDim       = 16
	TileFlagsBase = 6 + (VertexDim * VertexDim * 7) + 6 // = 6 + 2023 + 6 = 2035
)

func tileFlagFileOffset(gx, gz int) int {
	zBlock := gz / TileDim
	xBlock := gx / TileDim
	localZ := gz % TileDim
	localX := gx % TileDim
	blockOffset := HeaderSize + (zBlock*BlockDim+xBlock)*BlockSize
	return blockOffset + TileFlagsBase + (localZ*TileDim+localX)*2
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: mtileflags <m-file>")
		os.Exit(2)
	}
	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	if string(raw[:12]) != "JMXVMAPM1000" {
		panic("bad signature")
	}
	fmt.Printf("File size: %d (expected %d)\n", len(raw), 12+36*2575)

	// Read all 96x96 tile flags
	flagDist := map[uint16]int{}
	for gz := 0; gz < 96; gz++ {
		for gx := 0; gx < 96; gx++ {
			off := tileFlagFileOffset(gx, gz)
			f := binary.LittleEndian.Uint16(raw[off : off+2])
			flagDist[f]++
		}
	}
	fmt.Println("\n=== .m file tile.Flag distribution ===")
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
		fmt.Printf("  Flag=0x%04x (bin: %016b): %d tiles\n", e.k, e.k, e.v)
	}

	// Show flags around the house at world coords (1553, 1612)
	// Tile = world / 20. Tiles are 96x96 in a region of 1920x1920.
	houseGX := 77 // 1553 / 20
	houseGZ := 80 // 1612 / 20
	fmt.Printf("\n=== Tiles AROUND house position (gx=%d, gz=%d) ===\n", houseGX, houseGZ)
	for dz := -3; dz <= 3; dz++ {
		for dx := -3; dx <= 3; dx++ {
			gx := houseGX + dx
			gz := houseGZ + dz
			if gx < 0 || gx >= 96 || gz < 0 || gz >= 96 {
				fmt.Printf("    .")
				continue
			}
			off := tileFlagFileOffset(gx, gz)
			f := binary.LittleEndian.Uint16(raw[off : off+2])
			if f == 0 {
				fmt.Printf("  ___")
			} else {
				fmt.Printf("  %03x", f)
			}
		}
		fmt.Println()
	}
}
