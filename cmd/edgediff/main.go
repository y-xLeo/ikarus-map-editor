package main

import (
	"fmt"
	"os"
	"sort"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: edgediff <a.nvm> <b.nvm>")
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
	cellDiffs := 0
	if len(a.Cells) != len(b.Cells) || a.OpenCellCount != b.OpenCellCount {
		fmt.Printf("cell header differs: %d/%d vs %d/%d\n", len(a.Cells), a.OpenCellCount, len(b.Cells), b.OpenCellCount)
	}
	for i := 0; i < len(a.Cells) && i < len(b.Cells); i++ {
		if a.Cells[i].MinX != b.Cells[i].MinX || a.Cells[i].MinZ != b.Cells[i].MinZ ||
			a.Cells[i].MaxX != b.Cells[i].MaxX || a.Cells[i].MaxZ != b.Cells[i].MaxZ {
			if cellDiffs < 10 {
				fmt.Printf("cell[%d] bounds differ: a=(%.0f,%.0f)..(%.0f,%.0f) b=(%.0f,%.0f)..(%.0f,%.0f)\n",
					i, a.Cells[i].MinX, a.Cells[i].MinZ, a.Cells[i].MaxX, a.Cells[i].MaxZ,
					b.Cells[i].MinX, b.Cells[i].MinZ, b.Cells[i].MaxX, b.Cells[i].MaxZ)
			}
			cellDiffs++
		}
	}
	fmt.Printf("cell bound diffs=%d\n", cellDiffs)
	tileCellDiffs := 0
	tileFlagDiffs := 0
	tileTextureDiffs := 0
	for i := range a.Tiles {
		if a.Tiles[i].CellID != b.Tiles[i].CellID {
			tileCellDiffs++
		}
		if a.Tiles[i].Flag != b.Tiles[i].Flag {
			tileFlagDiffs++
		}
		if a.Tiles[i].TextureID != b.Tiles[i].TextureID {
			tileTextureDiffs++
		}
	}
	fmt.Printf("tile diffs: cell=%d flag=%d texture=%d\n", tileCellDiffs, tileFlagDiffs, tileTextureDiffs)
	dumpFlags("a", a.InternalEdges)
	dumpFlags("b", b.InternalEdges)
	am := edgeMap(a.InternalEdges)
	bm := edgeMap(b.InternalEdges)
	printOnlyInternal("only a", am, bm)
	printOnlyInternal("only b", bm, am)

	dumpGlobal("a", a.GlobalEdges)
	dumpGlobal("b", b.GlobalEdges)
	agm := globalEdgeMap(a.GlobalEdges)
	bgm := globalEdgeMap(b.GlobalEdges)
	printOnlyGlobal("global only a", agm, bgm)
	printOnlyGlobal("global only b", bgm, agm)
}

func dumpFlags(label string, edges []sromap.NVMInternalEdge) {
	counts := map[uint8]int{}
	for _, e := range edges {
		counts[e.Flag]++
	}
	fmt.Printf("%s internal=%d", label, len(edges))
	keys := make([]int, 0, len(counts))
	for k := range counts {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Printf(" flag%d=%d", k, counts[uint8(k)])
	}
	fmt.Println()
}

func edgeMap(edges []sromap.NVMInternalEdge) map[string]sromap.NVMInternalEdge {
	out := make(map[string]sromap.NVMInternalEdge, len(edges))
	for _, e := range edges {
		out[edgeKey(e)] = e
	}
	return out
}

func edgeKey(e sromap.NVMInternalEdge) string {
	return fmt.Sprintf("%.0f,%.0f,%.0f,%.0f|%d,%d,%d,%d,%d",
		e.MinX, e.MinZ, e.MaxX, e.MaxZ, e.Flag, e.Dir0, e.Dir1, e.Cell0, e.Cell1)
}

func printOnlyInternal(label string, a, b map[string]sromap.NVMInternalEdge) {
	keys := make([]string, 0)
	for k := range a {
		if _, ok := b[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	fmt.Printf("%s=%d\n", label, len(keys))
	for i, k := range keys {
		if i >= 20 {
			break
		}
		e := a[k]
		fmt.Printf("  flag=%d dir=%d/%d cells=%d/%d (%.0f,%.0f)..(%.0f,%.0f)\n",
			e.Flag, e.Dir0, e.Dir1, e.Cell0, e.Cell1, e.MinX, e.MinZ, e.MaxX, e.MaxZ)
	}
}

func dumpGlobal(label string, edges []sromap.NVMGlobalEdge) {
	counts := map[uint8]int{}
	for _, e := range edges {
		counts[e.Flag]++
	}
	fmt.Printf("%s global=%d", label, len(edges))
	keys := make([]int, 0, len(counts))
	for k := range counts {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Printf(" flag%d=%d", k, counts[uint8(k)])
	}
	fmt.Println()
}

func globalEdgeMap(edges []sromap.NVMGlobalEdge) map[string]sromap.NVMGlobalEdge {
	out := make(map[string]sromap.NVMGlobalEdge, len(edges))
	for _, e := range edges {
		out[globalEdgeKey(e)] = e
	}
	return out
}

func globalEdgeKey(e sromap.NVMGlobalEdge) string {
	return fmt.Sprintf("%.0f,%.0f,%.0f,%.0f|%d,%d,%d,%d,%d,%d,%d",
		e.MinX, e.MinZ, e.MaxX, e.MaxZ, e.Flag, e.Dir0, e.Dir1, e.Cell0, e.Cell1, e.Region0, e.Region1)
}

func printOnlyGlobal(label string, a, b map[string]sromap.NVMGlobalEdge) {
	keys := make([]string, 0)
	for k := range a {
		if _, ok := b[k]; !ok {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	fmt.Printf("%s=%d\n", label, len(keys))
	for i, k := range keys {
		if i >= 20 {
			break
		}
		e := a[k]
		fmt.Printf("  flag=%d dir=%d/%d cells=%d/%d regions=%04x/%04x (%.0f,%.0f)..(%.0f,%.0f)\n",
			e.Flag, e.Dir0, e.Dir1, e.Cell0, e.Cell1, uint16(e.Region0), uint16(e.Region1),
			e.MinX, e.MinZ, e.MaxX, e.MaxZ)
	}
}
