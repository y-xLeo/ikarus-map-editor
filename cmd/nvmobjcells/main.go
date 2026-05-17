package main

import (
	"fmt"
	"os"
	"strconv"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: nvmobjcells <nvm> <object-index|asset-id|all>")
		os.Exit(2)
	}
	nvm, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load nvm:", err)
		os.Exit(1)
	}
	if os.Args[2] == "all" {
		for objIdx, obj := range nvm.Objects {
			open, closed := objectCellCounts(nvm, objIdx)
			fmt.Printf("object[%d] asset=%d uid=%d pos=(%.2f, %.2f, %.2f) yaw=%.4f region=0x%04x linked=%d open=%d closed=%d\n",
				objIdx, obj.AssetID, obj.UID, obj.X, obj.Y, obj.Z, obj.Yaw, obj.RegionID, open+closed, open, closed)
		}
		return
	}
	want, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad object selector:", err)
		os.Exit(2)
	}

	for objIdx, obj := range nvm.Objects {
		if objIdx != want && int(obj.AssetID) != want {
			continue
		}
		fmt.Printf("object[%d] asset=%d uid=%d pos=(%.2f, %.2f, %.2f) yaw=%.4f region=0x%04x\n",
			objIdx, obj.AssetID, obj.UID, obj.X, obj.Y, obj.Z, obj.Yaw, obj.RegionID)
		count := 0
		for ci, cell := range nvm.Cells {
			for _, idx := range cell.ObjectIndices {
				if int(idx) != objIdx {
					continue
				}
				kind := "OPEN"
				if uint32(ci) >= nvm.OpenCellCount {
					kind = "CLOSED"
				}
				fmt.Printf("  cell[%d] %-6s aabb=(%.0f, %.0f)..(%.0f, %.0f) size=%.0fx%.0f objs=%v\n",
					ci, kind, cell.MinX, cell.MinZ, cell.MaxX, cell.MaxZ,
					cell.MaxX-cell.MinX, cell.MaxZ-cell.MinZ, cell.ObjectIndices)
				count++
				break
			}
		}
		fmt.Printf("  linked cells: %d\n", count)
	}
}

func objectCellCounts(nvm *sromap.NVM, objIdx int) (open, closed int) {
	for ci, cell := range nvm.Cells {
		for _, idx := range cell.ObjectIndices {
			if int(idx) != objIdx {
				continue
			}
			if uint32(ci) < nvm.OpenCellCount {
				open++
			} else {
				closed++
			}
			break
		}
	}
	return open, closed
}
