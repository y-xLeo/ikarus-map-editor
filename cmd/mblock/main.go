// mblock: set the .m file's per-tile Flag bit 0 (= "manually blocked") for
// a rectangle of tiles. This is the per-region collision system we missed —
// .nvm tile flags are some other thing; .m tile flags are what the client
// reads for walkability.
//
// usage: mblock <src.m> <dst.m> <gxMin> <gzMin> <gxMax> <gzMax>
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
)

const (
	HeaderSize    = 12
	BlockSize     = 2575
	BlockDim      = 6
	VertexDim     = 17
	TileDim       = 16
	TileFlagsBase = 6 + (VertexDim * VertexDim * 7) + 6 // = 2035
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
	if len(os.Args) < 7 {
		fmt.Println("usage: mblock <src.m> <dst.m> <gxMin> <gzMin> <gxMax> <gzMax>")
		os.Exit(2)
	}
	src := os.Args[1]
	dst := os.Args[2]
	gxMin, _ := strconv.Atoi(os.Args[3])
	gzMin, _ := strconv.Atoi(os.Args[4])
	gxMax, _ := strconv.Atoi(os.Args[5])
	gzMax, _ := strconv.Atoi(os.Args[6])

	raw, err := os.ReadFile(src)
	if err != nil {
		panic(err)
	}
	if string(raw[:12]) != "JMXVMAPM1000" {
		panic("bad signature")
	}
	if len(raw) != 92712 {
		fmt.Printf("WARNING: file size %d, expected 92712\n", len(raw))
	}

	changed := 0
	for gz := gzMin; gz <= gzMax; gz++ {
		for gx := gxMin; gx <= gxMax; gx++ {
			if gx < 0 || gx >= 96 || gz < 0 || gz >= 96 {
				continue
			}
			off := tileFlagFileOffset(gx, gz)
			old := binary.LittleEndian.Uint16(raw[off : off+2])
			newF := old | 1 // set bit 0 = manually blocked
			binary.LittleEndian.PutUint16(raw[off:off+2], newF)
			if old != newF {
				changed++
			}
		}
	}
	if err := os.WriteFile(dst, raw, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Set Flag bit 0 on %d tiles in (%d,%d)..(%d,%d). Wrote %s.\n",
		changed, gxMin, gzMin, gxMax, gzMax, dst)
}
