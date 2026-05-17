package main

import (
	"fmt"
	"os"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: nvmcellfill <nvm>")
		os.Exit(2)
	}
	nvm, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load nvm:", err)
		os.Exit(1)
	}
	nonFull := 0
	for ci, c := range nvm.Cells {
		tiles := 0
		for _, t := range nvm.Tiles {
			if int(t.CellID) == ci {
				tiles++
			}
		}
		boxTiles := int((c.MaxX - c.MinX) / sromap.NVMTileSize * (c.MaxZ - c.MinZ) / sromap.NVMTileSize)
		if tiles != boxTiles {
			nonFull++
			if nonFull <= 30 {
				kind := "OPEN"
				if uint32(ci) >= nvm.OpenCellCount {
					kind = "CLOSED"
				}
				fmt.Printf("cell[%d] %s bboxTiles=%d actualTiles=%d aabb=(%.0f,%.0f)..(%.0f,%.0f)\n",
					ci, kind, boxTiles, tiles, c.MinX, c.MinZ, c.MaxX, c.MaxZ)
			}
		}
	}
	fmt.Printf("nonFull=%d of %d\n", nonFull, len(nvm.Cells))
}
