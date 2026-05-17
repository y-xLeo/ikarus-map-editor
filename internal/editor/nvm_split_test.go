package editor

import (
	"fmt"
	"testing"

	"sromapedit/internal/sromap"
)

// TestSplitCell6Baseline verifies that splitOpenCell, applied to baseline
// 5c94's cell 6 (480×480 open cell at (1440,1140)..(1920,1620)), produces 4
// sub-cells of 240×240 each, that the existing ObjectIndices [9] (asset 971
// at world (1703, 1239)) lands only in the sub-cell containing its center,
// and that tile.CellID values are re-pointed accordingly.
//
// This is the same case that the rebuild tool gets right (4 sub-cells, asset
// 971 only in the NE quadrant) and that our old default-mode flow got wrong
// (one big 480×480 cell with everything in it).
func TestSplitCell6Baseline(t *testing.T) {
	const baselinePath = `C:\Silkroad Stuff\Mapeditor\Data\navmesh\nv_5c94.nvm`
	nvm, err := sromap.LoadNVM(baselinePath)
	if err != nil {
		t.Skipf("baseline nvm not available: %v", err)
		return
	}

	// Cell 6 is the 480×480 open cell containing asset 971's center
	// (1703, 1239).  Verify the precondition first.
	const targetCi = 6
	if targetCi >= int(nvm.OpenCellCount) {
		t.Fatalf("baseline cell %d is not open", targetCi)
	}
	before := nvm.Cells[targetCi]
	if (before.MaxX-before.MinX != 480) || (before.MaxZ-before.MinZ != 480) {
		t.Fatalf("baseline cell %d is not 480×480: %+v", targetCi, before)
	}
	if len(before.ObjectIndices) != 1 || before.ObjectIndices[0] != 9 {
		t.Fatalf("baseline cell %d ObjectIndices != [9]: %v", targetCi, before.ObjectIndices)
	}
	if int(nvm.Objects[9].AssetID) != 971 {
		t.Fatalf("baseline object 9 is not asset 971: %d", nvm.Objects[9].AssetID)
	}
	asset971Center := [2]float32{nvm.Objects[9].X, nvm.Objects[9].Z}

	// Capture some edges + tiles that reference cell 6 to verify post-shift.
	var edgesToCi6 int
	for _, e := range nvm.InternalEdges {
		if int(e.Cell0) == targetCi || int(e.Cell1) == targetCi {
			edgesToCi6++
		}
	}
	var tilesToCi6 int
	for _, ti := range nvm.Tiles {
		if int(ti.CellID) == targetCi {
			tilesToCi6++
		}
	}
	totalCellsBefore := len(nvm.Cells)
	openCellsBefore := nvm.OpenCellCount

	// Snapshot some closed cells to confirm they shift correctly.
	var closedSnap []sromap.NVMCell
	if openCellsBefore < uint32(len(nvm.Cells)) {
		closedSnap = append(closedSnap, nvm.Cells[openCellsBefore:]...)
	}

	splitOpenCell(nvm, targetCi, 0x5c94)

	// Post-conditions:
	// 1) Cell count grew by 3 (one cell split into four; one stays at ci,
	//    three inserted).
	if got, want := len(nvm.Cells), totalCellsBefore+3; got != want {
		t.Fatalf("cell count = %d, want %d", got, want)
	}
	if got, want := nvm.OpenCellCount, openCellsBefore+3; got != want {
		t.Fatalf("OpenCellCount = %d, want %d", got, want)
	}

	// 2) Closed cells were shifted up by 3 and preserved verbatim.
	for i, c := range closedSnap {
		got := nvm.Cells[int(openCellsBefore)+3+i]
		if got.MinX != c.MinX || got.MinZ != c.MinZ || got.MaxX != c.MaxX || got.MaxZ != c.MaxZ {
			t.Fatalf("closed cell %d shifted wrong: got %+v want %+v", i, got, c)
		}
	}

	// 3) The 4 sub-cells are 240×240, cover the parent's AABB, and the one
	//    containing asset 971's center is the only one with [9].
	subIdx := []int{targetCi, int(openCellsBefore), int(openCellsBefore) + 1, int(openCellsBefore) + 2}
	expect := []struct {
		minX, minZ, maxX, maxZ float32
	}{
		{1440, 1140, 1680, 1380},
		{1680, 1140, 1920, 1380},
		{1440, 1380, 1680, 1620},
		{1680, 1380, 1920, 1620},
	}
	var hits971 int
	for i, idx := range subIdx {
		sc := nvm.Cells[idx]
		if sc.MinX != expect[i].minX || sc.MinZ != expect[i].minZ ||
			sc.MaxX != expect[i].maxX || sc.MaxZ != expect[i].maxZ {
			t.Fatalf("sub-cell %d (array index %d): got AABB (%.0f,%.0f)..(%.0f,%.0f) want (%.0f,%.0f)..(%.0f,%.0f)",
				i, idx, sc.MinX, sc.MinZ, sc.MaxX, sc.MaxZ,
				expect[i].minX, expect[i].minZ, expect[i].maxX, expect[i].maxZ)
		}
		contains971 := asset971Center[0] >= sc.MinX && asset971Center[0] < sc.MaxX &&
			asset971Center[1] >= sc.MinZ && asset971Center[1] < sc.MaxZ
		hasObjIdx9 := false
		for _, j := range sc.ObjectIndices {
			if j == 9 {
				hasObjIdx9 = true
			}
		}
		if hasObjIdx9 {
			hits971++
		}
		if contains971 && !hasObjIdx9 {
			t.Fatalf("sub-cell %d contains asset 971 but doesn't have [9]", i)
		}
		if !contains971 && hasObjIdx9 {
			t.Fatalf("sub-cell %d does NOT contain asset 971 but has [9]", i)
		}
		fmt.Printf("  sub-cell %d (array %d): (%.0f,%.0f)..(%.0f,%.0f) objIdx=%v contains971=%v\n",
			i, idx, sc.MinX, sc.MinZ, sc.MaxX, sc.MaxZ, sc.ObjectIndices, contains971)
	}
	if hits971 != 1 {
		t.Fatalf("expected asset 971 to land in exactly 1 sub-cell, got %d", hits971)
	}

	// 4) Tiles previously pointing at cell 6 now point at one of the 4 sub-cells.
	tilesByCell := map[int]int{}
	for _, ti := range nvm.Tiles {
		if int(ti.CellID) == targetCi ||
			int(ti.CellID) == int(openCellsBefore) ||
			int(ti.CellID) == int(openCellsBefore)+1 ||
			int(ti.CellID) == int(openCellsBefore)+2 {
			tilesByCell[int(ti.CellID)]++
		}
	}
	fmt.Printf("  tile counts per sub-cell: %v\n", tilesByCell)
	totalSubTiles := 0
	for _, c := range tilesByCell {
		totalSubTiles += c
	}
	// Each sub-cell is 12×12 = 144 tiles. Together = 576. But it's possible
	// some tiles in the parent's range were already pointing at non-cell-6
	// (e.g., closed cells overlapping the same tile range). We only require
	// >= tilesToCi6 to be redistributed across the new sub-cells.
	if totalSubTiles < tilesToCi6 {
		t.Fatalf("tiles redistributed = %d, want >= original %d", totalSubTiles, tilesToCi6)
	}
	// And no tile still references ci6 with the OLD cell 6 bounds (i.e.,
	// outside the new shrunken cell 6 area).
	tile := float32(sromap.NVMTileSize)
	for j := 0; j < sromap.NVMTileCount; j++ {
		for i := 0; i < sromap.NVMTileCount; i++ {
			if nvm.Tiles[j*sromap.NVMTileCount+i].CellID != int32(targetCi) {
				continue
			}
			tx := float32(i) * tile
			tz := float32(j) * tile
			c := nvm.Cells[targetCi]
			if tx < c.MinX || tx >= c.MaxX || tz < c.MinZ || tz >= c.MaxZ {
				t.Fatalf("tile (%d,%d) still references cell %d but lies outside its new bounds %+v", i, j, targetCi, c)
			}
		}
	}

	// 5) Edges that were against ci6 are either kept and re-pointed to a
	//    sub-cell or split into pieces.
	var stillRefCi6Edges int
	for _, e := range nvm.InternalEdges {
		if int(e.Cell0) == targetCi || int(e.Cell1) == targetCi {
			// The piece must lie within cell 6's NEW (shrunken) bounds, i.e.,
			// touch only its sides.
			c := nvm.Cells[targetCi]
			onBound := (e.MinX == c.MinX || e.MinX == c.MaxX || e.MinZ == c.MinZ || e.MinZ == c.MaxZ)
			if !onBound {
				t.Fatalf("edge still references cell %d but is not on its new boundary: %+v", targetCi, e)
			}
			stillRefCi6Edges++
		}
	}
	fmt.Printf("  edges before pointing at ci6: %d, after split: %d (some merged into sub-cells)\n", edgesToCi6, stillRefCi6Edges)

	// 6) New internal edges between sub-cells should exist.
	var aBcdEdges int
	for _, e := range nvm.InternalEdges {
		a := int(e.Cell0)
		b := int(e.Cell1)
		ok0 := contains(subIdx, a)
		ok1 := contains(subIdx, b)
		if ok0 && ok1 {
			aBcdEdges++
		}
	}
	// In a 2x2 grid, we expect 4 internal edges between sub-cells.
	if aBcdEdges != 4 {
		t.Errorf("expected 4 edges between the 4 sub-cells, got %d", aBcdEdges)
	}
	fmt.Printf("  new edges between sub-cells: %d (expect 4)\n", aBcdEdges)

	// 7) Global edges that previously referenced cell 6 must now reference
	//    a sub-cell that physically reaches the region boundary they sit on.
	//    Cell 6's east side is at X=1920 (region edge) so any global edge
	//    that pointed at cell 6 on the east side must now point at one of
	//    cells 171/173 (the east column of the 2×2 split).
	const regionExtent = float32(sromap.NVMTileCount * sromap.NVMTileSize)
	for _, e := range nvm.GlobalEdges {
		if int(e.Cell0) == targetCi || int(e.Cell1) == targetCi {
			// Verify this edge's segment actually lies on cell 6's new
			// (shrunken NW sub) boundary — otherwise it's a dangling edge.
			c := nvm.Cells[targetCi]
			onBound := (e.MinX == c.MinX || e.MaxX == c.MaxX ||
				e.MinZ == c.MinZ || e.MaxZ == c.MaxZ)
			if !onBound {
				t.Fatalf("global edge still references cell %d but lies outside its new bounds: %+v vs cell %+v", targetCi, e, c)
			}
		}
		// Any global edge on X=regionExtent (east region boundary) that
		// falls in cell 6's old Z-range (1140..1620) must now reference one
		// of the eastern sub-cells (171 NE, 173 SE), not the NW sub.
		if e.MinX == regionExtent && e.MaxX == regionExtent {
			if e.MinZ >= 1140 && e.MaxZ <= 1620 {
				ref := -1
				switch {
				case int(e.Cell0) == 171 || int(e.Cell1) == 171:
					ref = 171
				case int(e.Cell0) == 173 || int(e.Cell1) == 173:
					ref = 173
				case int(e.Cell0) == targetCi || int(e.Cell1) == targetCi:
					ref = targetCi // pre-split, shouldn't happen anymore
				}
				fmt.Printf("  east-boundary global edge Z=(%.0f..%.0f) -> ref cell %d\n",
					e.MinZ, e.MaxZ, ref)
				if ref == targetCi {
					t.Errorf("east-boundary edge Z=(%.0f..%.0f) still pointing at NW sub-cell %d", e.MinZ, e.MaxZ, targetCi)
				}
			}
		}
	}
}

func contains(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func TestSplitOpenCellPreservesNeighborGlobalCellIDs(t *testing.T) {
	const localRegion = uint16(0x5c94)
	const neighborRegion = uint16(0x5d94)
	nvm := &sromap.NVM{
		OpenCellCount: 1,
		Cells: []sromap.NVMCell{{
			MinX: 0, MinZ: 0,
			MaxX: 480, MaxZ: 240,
		}},
		GlobalEdges: []sromap.NVMGlobalEdge{
			{
				MinX: 480, MinZ: 0, MaxX: 480, MaxZ: 240,
				Flag: sromap.NVMGlobalEdgeFlag,
				Dir0: sromap.NVMDirEast, Dir1: sromap.NVMDirWest,
				Cell0: 0, Cell1: 200,
				Region0: int16(localRegion), Region1: int16(neighborRegion),
			},
			{
				MinX: 480, MinZ: 0, MaxX: 480, MaxZ: 240,
				Flag: sromap.NVMGlobalEdgeFlag,
				Dir0: sromap.NVMDirWest, Dir1: sromap.NVMDirEast,
				Cell0: 201, Cell1: 0,
				Region0: int16(neighborRegion), Region1: int16(localRegion),
			},
		},
	}

	splitOpenCell(nvm, 0, localRegion)

	if got, want := len(nvm.Cells), 2; got != want {
		t.Fatalf("cell count = %d, want %d", got, want)
	}
	if got, want := nvm.OpenCellCount, uint32(2); got != want {
		t.Fatalf("open count = %d, want %d", got, want)
	}
	if got, want := nvm.GlobalEdges[0].Cell0, int16(1); got != want {
		t.Fatalf("local Cell0 = %d, want %d", got, want)
	}
	if got, want := nvm.GlobalEdges[0].Cell1, int16(200); got != want {
		t.Fatalf("neighbor Cell1 shifted to %d, want %d", got, want)
	}
	if got, want := nvm.GlobalEdges[1].Cell0, int16(201); got != want {
		t.Fatalf("neighbor Cell0 shifted to %d, want %d", got, want)
	}
	if got, want := nvm.GlobalEdges[1].Cell1, int16(1); got != want {
		t.Fatalf("local Cell1 = %d, want %d", got, want)
	}
	if got, want := len(nvm.InternalEdges), 1; got != want {
		t.Fatalf("internal edge count = %d, want %d", got, want)
	}
	e := nvm.InternalEdges[0]
	if e.Dir0 != sromap.NVMDirEast || e.Dir1 != sromap.NVMDirWest {
		t.Fatalf("split internal dirs = (%d, %d), want east/west (%d, %d)", e.Dir0, e.Dir1, sromap.NVMDirEast, sromap.NVMDirWest)
	}
}
