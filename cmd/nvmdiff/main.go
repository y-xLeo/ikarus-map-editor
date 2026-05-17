package main

import (
	"fmt"
	"os"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: nvmdiff <nvm-path> [<nvm-path-b>]")
		os.Exit(2)
	}
	a, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load a:", err)
		os.Exit(1)
	}
	dump("A "+os.Args[1], a)
	if len(os.Args) >= 3 {
		b, err := sromap.LoadNVM(os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "load b:", err)
			os.Exit(1)
		}
		dump("B "+os.Args[2], b)
	}
}

func dump(label string, n *sromap.NVM) {
	fmt.Println("===", label, "===")
	fmt.Printf("  Objects:       %d\n", len(n.Objects))
	fmt.Printf("  Cells:         %d (open=%d)\n", len(n.Cells), n.OpenCellCount)
	fmt.Printf("  GlobalEdges:   %d\n", len(n.GlobalEdges))
	fmt.Printf("  InternalEdges: %d\n", len(n.InternalEdges))

	// Sample of cells.
	if len(n.Cells) > 0 {
		fmt.Println("  First 3 cells:")
		for i := 0; i < 3 && i < len(n.Cells); i++ {
			c := n.Cells[i]
			fmt.Printf("    [%d] (%.1f..%.1f, %.1f..%.1f) objIdx=%v\n",
				i, c.MinX, c.MaxX, c.MinZ, c.MaxZ, c.ObjectIndices)
		}
	}

	// Region edges (touching boundaries).
	north, south, east, west := 0, 0, 0, 0
	for _, e := range n.GlobalEdges {
		switch {
		case e.MinX == 0 && e.MaxX == 0:
			west++
		case e.MinX == 1920 && e.MaxX == 1920:
			east++
		case e.MinZ == 0 && e.MaxZ == 0:
			south++
		case e.MinZ == 1920 && e.MaxZ == 1920:
			north++
		}
	}
	fmt.Printf("  GlobalEdges by side: N=%d S=%d E=%d W=%d (other=%d)\n",
		north, south, east, west, len(n.GlobalEdges)-(north+south+east+west))

	if len(n.GlobalEdges) > 0 {
		fmt.Println("  First 4 global edges:")
		for i := 0; i < 4 && i < len(n.GlobalEdges); i++ {
			e := n.GlobalEdges[i]
			fmt.Printf("    flag=%d dir=%d/%d cells=%d/%d regions=%04x/%04x  (%.1f..%.1f, %.1f..%.1f)\n",
				e.Flag, e.Dir0, e.Dir1, e.Cell0, e.Cell1, uint16(e.Region0), uint16(e.Region1),
				e.MinX, e.MaxX, e.MinZ, e.MaxZ)
		}
	}

	if len(n.InternalEdges) > 0 {
		fmt.Println("  First 3 internal edges:")
		for i := 0; i < 3 && i < len(n.InternalEdges); i++ {
			e := n.InternalEdges[i]
			fmt.Printf("    flag=%d dir=%d/%d cells=%d/%d (%.1f..%.1f, %.1f..%.1f)\n",
				e.Flag, e.Dir0, e.Dir1, e.Cell0, e.Cell1,
				e.MinX, e.MaxX, e.MinZ, e.MaxZ)
		}
	}

	// First object detail.
	if len(n.Objects) > 0 {
		o := n.Objects[0]
		fmt.Printf("  Obj[0]: asset=%d region=%04x pos=(%.1f,%.1f,%.1f) yaw=%.2f links=%d type=%d uid=%d short=%d big=%v struct=%v\n",
			o.AssetID, o.RegionID, o.X, o.Y, o.Z, o.Yaw, len(o.Links), o.Type, o.UID, o.Short0, o.IsBig, o.IsStruct)
	}

	// Tile / plane sanity.
	tileCellMin, tileCellMax := int32(1<<30), int32(-1<<30)
	for _, t := range n.Tiles {
		if t.CellID < tileCellMin {
			tileCellMin = t.CellID
		}
		if t.CellID > tileCellMax {
			tileCellMax = t.CellID
		}
	}
	fmt.Printf("  Tiles cellID range: %d..%d\n", tileCellMin, tileCellMax)

	st := n.Stats()
	fmt.Printf("  Heights range: %.1f..%.1f\n", st.MinHeight, st.MaxHeight)
}
