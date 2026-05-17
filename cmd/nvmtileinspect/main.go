// nvmtileinspect cross-references each tile's blocked-flag with the cell it
// points to, to figure out whether server-side collision relies on
// Tile.Flag bit 1 or on the cell being beyond OpenCellCount (closed cell).
package main

import (
	"fmt"
	"os"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: nvmtileinspect <nvm-path>")
		os.Exit(2)
	}
	n, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		die("load:", err)
	}
	fmt.Printf("Cells: %d (open=%d, closed=%d)\n",
		len(n.Cells), n.OpenCellCount, uint32(len(n.Cells))-n.OpenCellCount)
	fmt.Printf("NVMObjects: %d\n", len(n.Objects))
	for i, o := range n.Objects {
		fmt.Printf("  [%d] asset=%d region=%04x pos=(%.1f, %.1f, %.1f) yaw=%.2f uid=%d type=%d isBig=%v isStruct=%v links=%d\n",
			i, o.AssetID, o.RegionID, o.X, o.Y, o.Z, o.Yaw, o.UID, o.Type, o.IsBig, o.IsStruct, len(o.Links))
	}
	fmt.Println()

	// For each tile, check the two collision signals:
	//   A: Tile.Flag bit 1 set (= "blocked")
	//   B: Tile.CellID >= OpenCellCount (= in a closed cell)
	flagBlocked := 0
	cellBlocked := 0
	bothBlocked := 0
	flagOnly := 0
	cellOnly := 0
	neither := 0
	for i := range n.Tiles {
		t := n.Tiles[i]
		a := (t.Flag & 1) != 0
		b := uint32(t.CellID) >= n.OpenCellCount && t.CellID >= 0
		switch {
		case a && b:
			bothBlocked++
		case a && !b:
			flagOnly++
		case !a && b:
			cellOnly++
		default:
			neither++
		}
		if a {
			flagBlocked++
		}
		if b {
			cellBlocked++
		}
	}
	fmt.Printf("Flag-blocked tiles:    %d\n", flagBlocked)
	fmt.Printf("Closed-cell tiles:     %d\n", cellBlocked)
	fmt.Printf("  both signals:        %d\n", bothBlocked)
	fmt.Printf("  flag only:           %d\n", flagOnly)
	fmt.Printf("  closed-cell only:    %d\n", cellOnly)
	fmt.Printf("  walkable:            %d\n", neither)
}

func die(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
