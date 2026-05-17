package editor

import (
	"math"

	"sromapedit/internal/sromap"
)

// splitOversizedCellsForCustom finds every OPEN cell whose AABB meaningfully
// overlaps a *custom* NVMObject's rotated world bbox AND is larger than one
// 240×240 grid cell on either axis, and splits it into a 240-grid of
// sub-cells. This matches the rebuild tool's behaviour: the baseline 5c94
// has a single 480×480 open cell (cell 6) covering the area around custom
// asset 3308; the rebuild tool replaces it with 4 sub-cells of 240×240 and
// re-assigns each NVMObject to whichever sub-cell its center falls into.
//
// Without splitting, the new NVMObject's ObjectIndex lands on the big
// parent cell — which the engine then treats as "this whole 480×480 region
// is asset 3308's footprint", breaking pathfinding when the player approaches
// the asset.
//
// Constraints we satisfy:
//   - Closed cells are NEVER split (they encode stock objects' footprints).
//   - Existing edges that referenced the split cell are re-routed (and
//     split themselves when they span multiple sub-cells).
//   - New internal edges are added between adjacent sub-cells.
//   - Tile.CellID values inside the parent cell's footprint are re-pointed.
//   - Open-cell vs closed-cell ordering is preserved: new sub-cells are
//     inserted just before the closed cells, and all indices >= the
//     insertion point (in tiles, edges, and the cell array itself) are
//     shifted up by the number of new sub-cells.
//   - ObjectIndices inheritance: each sub-cell inherits the parent's
//     ObjectIndices, but only those whose NVMObject *center* lies inside
//     the sub-cell's AABB. This matches what the rebuild tool produces
//     (e.g., baseline cell 6 had ObjectIndices=[9]=asset 971 whose center
//     is at (1703,1239); after split, only the NE sub-cell retains [9]).
func (s *Server) splitOversizedCellsForCustom(nvm *sromap.NVM, regionID uint16) {
	type bbox struct {
		minX, maxX, minZ, maxZ float32
	}
	var customBboxes []bbox

	for _, obj := range nvm.Objects {
		asset, _ := s.objCache.get(obj.AssetID)
		if asset == nil || !asset.HasCollision {
			continue
		}
		if !pathLooksCustom(asset.Source) {
			continue
		}
		bmin := asset.CollisionBBoxMin
		bmax := asset.CollisionBBoxMax
		c := float32(math.Cos(float64(obj.Yaw)))
		sn := float32(math.Sin(float64(obj.Yaw)))
		var bb bbox
		bb.minX, bb.maxX = float32(math.MaxFloat32), float32(-math.MaxFloat32)
		bb.minZ, bb.maxZ = float32(math.MaxFloat32), float32(-math.MaxFloat32)
		for _, cx := range [2]float32{bmin[0], bmax[0]} {
			for _, cz := range [2]float32{bmin[2], bmax[2]} {
				rx := c*cx - sn*cz + obj.X
				rz := sn*cx + c*cz + obj.Z
				if rx < bb.minX {
					bb.minX = rx
				}
				if rx > bb.maxX {
					bb.maxX = rx
				}
				if rz < bb.minZ {
					bb.minZ = rz
				}
				if rz > bb.maxZ {
					bb.maxZ = rz
				}
			}
		}
		customBboxes = append(customBboxes, bb)
	}

	if len(customBboxes) == 0 {
		return
	}

	// Process cells in reverse so splitting doesn't invalidate higher indices
	// we haven't visited yet.
	for ci := int(nvm.OpenCellCount) - 1; ci >= 0; ci-- {
		cell := nvm.Cells[ci]
		w := cell.MaxX - cell.MinX
		h := cell.MaxZ - cell.MinZ
		const maxSize float32 = 240 // = NVMMaxCellTile * NVMTileSize
		if w <= maxSize && h <= maxSize {
			continue
		}

		// Does any custom bbox overlap meaningfully?
		overlaps := false
		for _, bb := range customBboxes {
			ovX := minF(cell.MaxX, bb.maxX) - maxF(cell.MinX, bb.minX)
			ovZ := minF(cell.MaxZ, bb.maxZ) - maxF(cell.MinZ, bb.minZ)
			if ovX >= 20 && ovZ >= 20 {
				overlaps = true
				break
			}
		}
		if !overlaps {
			continue
		}

		splitOpenCell(nvm, ci, regionID)
	}
}

// splitOpenCell replaces the open cell at index ci with a grid of sub-cells
// each up to 12×12 tiles (240×240 world units). The first sub-cell takes the
// old cell's index; the rest are inserted at the boundary between open and
// closed cells (so OpenCellCount grows but the open/closed split stays
// valid). All references (tiles, edges) are updated.
func splitOpenCell(nvm *sromap.NVM, ci int, regionID uint16) {
	cell := nvm.Cells[ci]
	tile := float32(sromap.NVMTileSize)
	minTX := int(cell.MinX / tile)
	minTZ := int(cell.MinZ / tile)
	maxTX := int(cell.MaxX/tile) - 1
	maxTZ := int(cell.MaxZ/tile) - 1

	wTiles := maxTX - minTX + 1
	hTiles := maxTZ - minTZ + 1
	const maxTiles = sromap.NVMMaxCellTile

	nX := (wTiles + maxTiles - 1) / maxTiles
	nZ := (hTiles + maxTiles - 1) / maxTiles
	if nX <= 1 && nZ <= 1 {
		return
	}

	// Build sub-cells in row-major order (zi outer, xi inner).
	type sub struct {
		minTX, minTZ, maxTX, maxTZ int
		ncell                      sromap.NVMCell
	}
	subs := make([]sub, 0, nX*nZ)
	for zi := 0; zi < nZ; zi++ {
		for xi := 0; xi < nX; xi++ {
			sMinTX := minTX + xi*maxTiles
			sMinTZ := minTZ + zi*maxTiles
			sMaxTX := sMinTX + maxTiles - 1
			if sMaxTX > maxTX {
				sMaxTX = maxTX
			}
			sMaxTZ := sMinTZ + maxTiles - 1
			if sMaxTZ > maxTZ {
				sMaxTZ = maxTZ
			}
			nc := sromap.NVMCell{
				MinX: float32(sMinTX) * tile,
				MinZ: float32(sMinTZ) * tile,
				MaxX: float32(sMaxTX+1) * tile,
				MaxZ: float32(sMaxTZ+1) * tile,
			}
			// Filter parent's ObjectIndices: each NVMObject lands in the
			// single sub-cell containing its center.
			for _, idx := range cell.ObjectIndices {
				if int(idx) < 0 || int(idx) >= len(nvm.Objects) {
					continue
				}
				obj := nvm.Objects[idx]
				if obj.X >= nc.MinX && obj.X < nc.MaxX && obj.Z >= nc.MinZ && obj.Z < nc.MaxZ {
					nc.ObjectIndices = append(nc.ObjectIndices, idx)
				}
			}
			subs = append(subs, sub{
				minTX: sMinTX, minTZ: sMinTZ,
				maxTX: sMaxTX, maxTZ: sMaxTZ,
				ncell: nc,
			})
		}
	}

	// Edge case: parent's ObjectIndices contains an entry whose center wasn't
	// inside any sub-cell (objects whose .X/.Z falls in the original cell but
	// rounds outside). Park them on the sub-cell whose AABB is closest to
	// the object — keeps the index from being dropped.
	for _, idx := range cell.ObjectIndices {
		if int(idx) < 0 || int(idx) >= len(nvm.Objects) {
			continue
		}
		placed := false
		for _, s := range subs {
			for _, j := range s.ncell.ObjectIndices {
				if j == idx {
					placed = true
					break
				}
			}
			if placed {
				break
			}
		}
		if placed {
			continue
		}
		obj := nvm.Objects[idx]
		bestI, bestD := 0, float32(math.MaxFloat32)
		for i, s := range subs {
			cx := (s.ncell.MinX + s.ncell.MaxX) / 2
			cz := (s.ncell.MinZ + s.ncell.MaxZ) / 2
			dx := obj.X - cx
			dz := obj.Z - cz
			d := dx*dx + dz*dz
			if d < bestD {
				bestD = d
				bestI = i
			}
		}
		subs[bestI].ncell.ObjectIndices = append(subs[bestI].ncell.ObjectIndices, idx)
	}

	// First sub-cell replaces the old cell at ci (index unchanged).
	nvm.Cells[ci] = subs[0].ncell

	insertAt := int(nvm.OpenCellCount) // closed cells start here
	extras := make([]sromap.NVMCell, 0, len(subs)-1)
	for i := 1; i < len(subs); i++ {
		extras = append(extras, subs[i].ncell)
	}
	n := len(extras)

	// Shift any reference >= insertAt up by n (closed cells move down the
	// array because we're inserting open cells before them).
	for ti := range nvm.Tiles {
		if int(nvm.Tiles[ti].CellID) >= insertAt {
			nvm.Tiles[ti].CellID += int32(n)
		}
	}
	for ei := range nvm.InternalEdges {
		e := &nvm.InternalEdges[ei]
		if int(e.Cell0) >= insertAt {
			e.Cell0 += int16(n)
		}
		if int(e.Cell1) >= insertAt {
			e.Cell1 += int16(n)
		}
	}
	for ei := range nvm.GlobalEdges {
		e := &nvm.GlobalEdges[ei]
		if globalEdgeSlotIsLocal(e.Region0, regionID) && int(e.Cell0) >= insertAt {
			e.Cell0 += int16(n)
		}
		if globalEdgeSlotIsLocal(e.Region1, regionID) && int(e.Cell1) >= insertAt {
			e.Cell1 += int16(n)
		}
	}

	// Splice extras into the cell array at insertAt.
	merged := make([]sromap.NVMCell, len(nvm.Cells)+n)
	copy(merged, nvm.Cells[:insertAt])
	copy(merged[insertAt:insertAt+n], extras)
	copy(merged[insertAt+n:], nvm.Cells[insertAt:])
	nvm.Cells = merged
	nvm.OpenCellCount += uint32(n)

	// Map sub index → cell array index after splice. subs[0] keeps ci.
	subIdx := make([]int, len(subs))
	subIdx[0] = ci
	for i := 1; i < len(subs); i++ {
		subIdx[i] = insertAt + i - 1
	}

	// Re-point every tile inside the parent cell's footprint to the matching
	// sub-cell.
	for tj := minTZ; tj <= maxTZ; tj++ {
		for ti := minTX; ti <= maxTX; ti++ {
			xi := (ti - minTX) / maxTiles
			zi := (tj - minTZ) / maxTiles
			if xi >= nX {
				xi = nX - 1
			}
			if zi >= nZ {
				zi = nZ - 1
			}
			nvm.Tiles[tj*sromap.NVMTileCount+ti].CellID = int32(subIdx[zi*nX+xi])
		}
	}

	// Re-route existing internal edges that touched the old cell. The cell's
	// 4 sides are: MinX (west), MaxX (east), MinZ (south), MaxZ (north).
	// For each edge that hits one of those sides, splice it into pieces, one
	// per sub-cell row/column overlapping the edge segment.
	var kept []sromap.NVMInternalEdge
	for _, e := range nvm.InternalEdges {
		touchesCi := false
		var keepSide uint8 // 0=W 1=E 2=S 3=N, used only if touchesCi
		// Vertical edge (MinX==MaxX): wall between east-of-edge and west-of-edge.
		if e.MinX == e.MaxX {
			x := e.MinX
			if int(e.Cell0) == ci {
				// Cell0 is on east side (a cell whose MinX is at this x)
				if x == cell.MinX {
					touchesCi = true
					keepSide = 0 // ci was east of edge → ci's west wall side
				}
				if x == cell.MaxX {
					touchesCi = true
					keepSide = 1
				}
			}
			if int(e.Cell1) == ci {
				if x == cell.MinX {
					touchesCi = true
					keepSide = 0
				}
				if x == cell.MaxX {
					touchesCi = true
					keepSide = 1
				}
			}
		}
		if e.MinZ == e.MaxZ {
			z := e.MinZ
			if int(e.Cell0) == ci {
				if z == cell.MinZ {
					touchesCi = true
					keepSide = 2
				}
				if z == cell.MaxZ {
					touchesCi = true
					keepSide = 3
				}
			}
			if int(e.Cell1) == ci {
				if z == cell.MinZ {
					touchesCi = true
					keepSide = 2
				}
				if z == cell.MaxZ {
					touchesCi = true
					keepSide = 3
				}
			}
		}
		if !touchesCi {
			kept = append(kept, e)
			continue
		}

		// Split the edge into pieces along the sub-cell grid.
		switch keepSide {
		case 0, 1: // vertical wall on west or east side
			xi := 0
			if keepSide == 1 {
				xi = nX - 1
			}
			for zi := 0; zi < nZ; zi++ {
				s := subs[zi*nX+xi]
				ovMinZ := maxF(e.MinZ, s.ncell.MinZ)
				ovMaxZ := minF(e.MaxZ, s.ncell.MaxZ)
				if ovMinZ >= ovMaxZ {
					continue
				}
				piece := e
				piece.MinZ = ovMinZ
				piece.MaxZ = ovMaxZ
				if int(e.Cell0) == ci {
					piece.Cell0 = int16(subIdx[zi*nX+xi])
				}
				if int(e.Cell1) == ci {
					piece.Cell1 = int16(subIdx[zi*nX+xi])
				}
				kept = append(kept, piece)
			}
		case 2, 3: // horizontal wall on south or north side
			zi := 0
			if keepSide == 3 {
				zi = nZ - 1
			}
			for xi := 0; xi < nX; xi++ {
				s := subs[zi*nX+xi]
				ovMinX := maxF(e.MinX, s.ncell.MinX)
				ovMaxX := minF(e.MaxX, s.ncell.MaxX)
				if ovMinX >= ovMaxX {
					continue
				}
				piece := e
				piece.MinX = ovMinX
				piece.MaxX = ovMaxX
				if int(e.Cell0) == ci {
					piece.Cell0 = int16(subIdx[zi*nX+xi])
				}
				if int(e.Cell1) == ci {
					piece.Cell1 = int16(subIdx[zi*nX+xi])
				}
				kept = append(kept, piece)
			}
		}
	}

	// Add new internal edges between adjacent sub-cells.
	// East-west neighbours.
	for zi := 0; zi < nZ; zi++ {
		for xi := 0; xi < nX-1; xi++ {
			a := subs[zi*nX+xi]
			b := subs[zi*nX+xi+1]
			ovMinZ := maxF(a.ncell.MinZ, b.ncell.MinZ)
			ovMaxZ := minF(a.ncell.MaxZ, b.ncell.MaxZ)
			if ovMinZ >= ovMaxZ {
				continue
			}
			kept = append(kept, sromap.NVMInternalEdge{
				MinX: a.ncell.MaxX, MaxX: a.ncell.MaxX,
				MinZ: ovMinZ, MaxZ: ovMaxZ,
				Cell0: int16(subIdx[zi*nX+xi]),
				Cell1: int16(subIdx[zi*nX+xi+1]),
				Dir0:  sromap.NVMDirEast, Dir1: sromap.NVMDirWest,
				Flag: sromap.NVMInternalEdgeFlag,
			})
		}
	}
	// North-south neighbours.
	for zi := 0; zi < nZ-1; zi++ {
		for xi := 0; xi < nX; xi++ {
			a := subs[zi*nX+xi]
			b := subs[(zi+1)*nX+xi]
			ovMinX := maxF(a.ncell.MinX, b.ncell.MinX)
			ovMaxX := minF(a.ncell.MaxX, b.ncell.MaxX)
			if ovMinX >= ovMaxX {
				continue
			}
			kept = append(kept, sromap.NVMInternalEdge{
				MinX: ovMinX, MaxX: ovMaxX,
				MinZ: a.ncell.MaxZ, MaxZ: a.ncell.MaxZ,
				Cell0: int16(subIdx[zi*nX+xi]),
				Cell1: int16(subIdx[(zi+1)*nX+xi]),
				Dir0:  sromap.NVMDirNorth, Dir1: sromap.NVMDirSouth,
				Flag: sromap.NVMInternalEdgeFlag,
			})
		}
	}

	nvm.InternalEdges = kept

	// Apply the same boundary-edge surgery to GLOBAL edges (region crossings).
	// Without this, a global edge that pointed at the parent cell's east
	// boundary stays pointing at the (now shrunken) parent — which after
	// split no longer reaches that boundary. The player then crosses into
	// the neighbor region with no cell linkage on our side and glitches out.
	//
	// We split the global edge into pieces along the sub-cell grid, just
	// like internal edges. The cross-region link (Region0/Region1 and the
	// Cell on the neighbor side) stays identical on every piece — the
	// neighbor region's nvm still has one edge but we have N matching
	// pieces, each anchored to the correct sub-cell on our side.
	var keptGlobal []sromap.NVMGlobalEdge
	for _, e := range nvm.GlobalEdges {
		touchesCi := false
		var keepSide uint8
		localSlot := localGlobalEdgeSlot(e, regionID)
		if localSlot < 0 {
			keptGlobal = append(keptGlobal, e)
			continue
		}
		localCell := e.Cell0
		if localSlot == 1 {
			localCell = e.Cell1
		}
		if e.MinX == e.MaxX {
			x := e.MinX
			if int(localCell) == ci {
				if x == cell.MinX {
					touchesCi = true
					keepSide = 0
				}
				if x == cell.MaxX {
					touchesCi = true
					keepSide = 1
				}
			}
		}
		if e.MinZ == e.MaxZ {
			z := e.MinZ
			if int(localCell) == ci {
				if z == cell.MinZ {
					touchesCi = true
					keepSide = 2
				}
				if z == cell.MaxZ {
					touchesCi = true
					keepSide = 3
				}
			}
		}
		if !touchesCi {
			keptGlobal = append(keptGlobal, e)
			continue
		}

		switch keepSide {
		case 0, 1:
			xi := 0
			if keepSide == 1 {
				xi = nX - 1
			}
			for zi := 0; zi < nZ; zi++ {
				s := subs[zi*nX+xi]
				ovMinZ := maxF(e.MinZ, s.ncell.MinZ)
				ovMaxZ := minF(e.MaxZ, s.ncell.MaxZ)
				if ovMinZ >= ovMaxZ {
					continue
				}
				piece := e
				piece.MinZ = ovMinZ
				piece.MaxZ = ovMaxZ
				if localSlot == 0 {
					piece.Cell0 = int16(subIdx[zi*nX+xi])
				}
				if localSlot == 1 {
					piece.Cell1 = int16(subIdx[zi*nX+xi])
				}
				keptGlobal = append(keptGlobal, piece)
			}
		case 2, 3:
			zi := 0
			if keepSide == 3 {
				zi = nZ - 1
			}
			for xi := 0; xi < nX; xi++ {
				s := subs[zi*nX+xi]
				ovMinX := maxF(e.MinX, s.ncell.MinX)
				ovMaxX := minF(e.MaxX, s.ncell.MaxX)
				if ovMinX >= ovMaxX {
					continue
				}
				piece := e
				piece.MinX = ovMinX
				piece.MaxX = ovMaxX
				if localSlot == 0 {
					piece.Cell0 = int16(subIdx[zi*nX+xi])
				}
				if localSlot == 1 {
					piece.Cell1 = int16(subIdx[zi*nX+xi])
				}
				keptGlobal = append(keptGlobal, piece)
			}
		}
	}
	nvm.GlobalEdges = keptGlobal
}

func globalEdgeSlotIsLocal(region int16, localRegionID uint16) bool {
	return region >= 0 && uint16(region) == localRegionID
}

func localGlobalEdgeSlot(e sromap.NVMGlobalEdge, localRegionID uint16) int {
	if globalEdgeSlotIsLocal(e.Region0, localRegionID) {
		return 0
	}
	if globalEdgeSlotIsLocal(e.Region1, localRegionID) {
		return 1
	}
	return -1
}
