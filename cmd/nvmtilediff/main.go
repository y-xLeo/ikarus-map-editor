package main

import (
	"fmt"
	"os"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: nvmtilediff <a.nvm> <b.nvm>")
		os.Exit(2)
	}
	a, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load a:", err)
		os.Exit(1)
	}
	b, err := sromap.LoadNVM(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load b:", err)
		os.Exit(1)
	}
	var sameOpen, sameClosed, aOpenBClosed, aClosedBOpen int
	var aOpenBClosedBox, aClosedBOpenBox bbox
	for z := 0; z < sromap.NVMTileCount; z++ {
		for x := 0; x < sromap.NVMTileCount; x++ {
			idx := z*sromap.NVMTileCount + x
			aw := uint32(a.Tiles[idx].CellID) < a.OpenCellCount
			bw := uint32(b.Tiles[idx].CellID) < b.OpenCellCount
			switch {
			case aw && bw:
				sameOpen++
			case !aw && !bw:
				sameClosed++
			case aw && !bw:
				aOpenBClosed++
				aOpenBClosedBox.add(x, z)
			case !aw && bw:
				aClosedBOpen++
				aClosedBOpenBox.add(x, z)
			}
		}
	}
	fmt.Printf("sameOpen=%d sameClosed=%d aOpen_bClosed=%d aClosed_bOpen=%d\n",
		sameOpen, sameClosed, aOpenBClosed, aClosedBOpen)
	fmt.Printf("aOpen_bClosed tiles: %s\n", aOpenBClosedBox)
	fmt.Printf("aClosed_bOpen tiles: %s\n", aClosedBOpenBox)
}

type bbox struct {
	set        bool
	minX, minZ int
	maxX, maxZ int
}

func (b *bbox) add(x, z int) {
	if !b.set {
		b.set = true
		b.minX, b.maxX = x, x
		b.minZ, b.maxZ = z, z
		return
	}
	if x < b.minX {
		b.minX = x
	}
	if x > b.maxX {
		b.maxX = x
	}
	if z < b.minZ {
		b.minZ = z
	}
	if z > b.maxZ {
		b.maxZ = z
	}
}

func (b bbox) String() string {
	if !b.set {
		return "<none>"
	}
	return fmt.Sprintf("tiles=(%d,%d)..(%d,%d) world=(%d,%d)..(%d,%d)",
		b.minX, b.minZ, b.maxX, b.maxZ,
		b.minX*sromap.NVMTileSize, b.minZ*sromap.NVMTileSize,
		(b.maxX+1)*sromap.NVMTileSize, (b.maxZ+1)*sromap.NVMTileSize)
}
