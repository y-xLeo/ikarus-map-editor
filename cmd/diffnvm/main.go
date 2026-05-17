// diffnvm: compare two .nvm files cell-by-cell, focusing on cells that
// overlap a target world bbox. Use this to see exactly which cells the
// working "rebuild" tool produces around asset 3308 vs the baseline.
package main

import (
	"fmt"
	"math"
	"os"
	"sort"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: diffnvm <baseline.nvm> <rebuild.nvm>")
		os.Exit(1)
	}
	base, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		panic(err)
	}
	reb, err := sromap.LoadNVM(os.Args[2])
	if err != nil {
		panic(err)
	}

	fmt.Printf("Baseline: %d objects, %d cells, %d internal edges, %d global edges\n",
		len(base.Objects), len(base.Cells), len(base.InternalEdges), len(base.GlobalEdges))
	fmt.Printf("Rebuild : %d objects, %d cells, %d internal edges, %d global edges\n\n",
		len(reb.Objects), len(reb.Cells), len(reb.InternalEdges), len(reb.GlobalEdges))

	// Asset 3308 rotated bbox (computed previously)
	rotMinX, rotMaxX := float32(1607), float32(1897)
	rotMinZ, rotMaxZ := float32(1325), float32(1630)
	fmt.Printf("Target bbox: X=(%.0f..%.0f) Z=(%.0f..%.0f)\n\n", rotMinX, rotMaxX, rotMinZ, rotMaxZ)

	dumpCellsInBbox("BASELINE", base, rotMinX, rotMaxX, rotMinZ, rotMaxZ)
	fmt.Println()
	dumpCellsInBbox("REBUILD ", reb, rotMinX, rotMaxX, rotMinZ, rotMaxZ)

	// Object list compare
	fmt.Println("\n--- NVMObjects ---")
	for i, o := range base.Objects {
		fmt.Printf("  base[%2d] asset=%d pos=(%.0f,%.0f,%.0f) yaw=%.4f UID=%d\n", i, o.AssetID, o.X, o.Y, o.Z, o.Yaw, o.UID)
	}
	fmt.Println()
	for i, o := range reb.Objects {
		fmt.Printf("  reb [%2d] asset=%d pos=(%.0f,%.0f,%.0f) yaw=%.4f UID=%d\n", i, o.AssetID, o.X, o.Y, o.Z, o.Yaw, o.UID)
	}
}

func ifStr(b bool, a, c string) string {
	if b {
		return a
	}
	return c
}

func dumpCellsInBbox(label string, n *sromap.NVM, xMin, xMax, zMin, zMax float32) {
	type ce struct {
		idx                    int
		minX, minZ, maxX, maxZ float32
		objIdx                 []uint16
		ovX, ovZ               float32
	}
	var hits []ce
	for i, c := range n.Cells {
		ox := math.Min(float64(c.MaxX), float64(xMax)) - math.Max(float64(c.MinX), float64(xMin))
		oz := math.Min(float64(c.MaxZ), float64(zMax)) - math.Max(float64(c.MinZ), float64(zMin))
		if ox <= 0 || oz <= 0 {
			continue
		}
		hits = append(hits, ce{
			idx: i, minX: c.MinX, minZ: c.MinZ, maxX: c.MaxX, maxZ: c.MaxZ,
			objIdx: c.ObjectIndices,
			ovX:    float32(ox), ovZ: float32(oz),
		})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].idx < hits[j].idx })
	fmt.Printf("=== %s : %d cells overlap target bbox ===\n", label, len(hits))
	for _, h := range hits {
		w := h.maxX - h.minX
		ht := h.maxZ - h.minZ
		closed := h.idx >= int(n.OpenCellCount)
		fmt.Printf("  cell %3d %s AABB=(%.0f,%.0f)..(%.0f,%.0f)  size=%.0fx%.0f  ov=(%.0f,%.0f)  objIdx=%v\n",
			h.idx, ifStr(closed, "CLOSED", "OPEN  "), h.minX, h.minZ, h.maxX, h.maxZ, w, ht, h.ovX, h.ovZ, h.objIdx)
	}
}
