package main

import (
	"fmt"
	"os"
	"strconv"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: nvmlinkdump <nvm> [object-index|all]")
		os.Exit(2)
	}
	nvm, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load nvm:", err)
		os.Exit(1)
	}
	start, end := 0, len(nvm.Objects)
	if len(os.Args) >= 3 && os.Args[2] != "all" {
		idx, err := strconv.Atoi(os.Args[2])
		if err != nil || idx < 0 || idx >= len(nvm.Objects) {
			fmt.Fprintln(os.Stderr, "bad object index")
			os.Exit(2)
		}
		start, end = idx, idx+1
	}
	fmt.Printf("%s objects=%d cells=%d open=%d\n", os.Args[1], len(nvm.Objects), len(nvm.Cells), nvm.OpenCellCount)
	for i := start; i < end; i++ {
		o := nvm.Objects[i]
		open, closed := objectCellCounts(nvm, i)
		fmt.Printf("[%02d] asset=%d uid=%d type=%d short0=%d big=%t struct=%t region=0x%04x pos=(%.2f,%.2f,%.2f) yaw=%.4f links=%d cells=%d open=%d closed=%d\n",
			i, o.AssetID, o.UID, o.Type, o.Short0, o.IsBig, o.IsStruct, o.RegionID, o.X, o.Y, o.Z, o.Yaw, len(o.Links), open+closed, open, closed)
		for j, l := range o.Links {
			fmt.Printf("    link[%02d] linkedObj=%d linkedEdge=%d edge=%d\n", j, l.LinkedObjectID, l.LinkedEdgeID, l.EdgeID)
		}
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
