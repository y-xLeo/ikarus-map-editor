package sromap

import "sort"

const (
	NVMTileSize    = 20 // world units per nav tile (region/96)
	NVMMaxCellTile = 12 // max tiles per cell side (12 tiles × 20u = 240 world units, matches baseline grid)

	NVMDirNorth = 0
	NVMDirEast  = 1
	NVMDirSouth = 2
	NVMDirWest  = 3
)

// PartitionedCell describes one cell of the navmesh in tile-space.
type PartitionedCell struct {
	MinTileX, MinTileZ, MaxTileX, MaxTileZ int
	Open                                   bool
}

// PartitionCells partitions the 96x96 walkability grid into axis-aligned
// rectangles inside fixed 12x12-tile macro blocks. Etherial's map-only NVM
// rebuild does not let cells cross those 240x240 world-unit block boundaries;
// matching that packing keeps cell IDs and cross-region edge topology stable.
// Open cells (walkable) are emitted before closed cells, so the first N entries
// of the returned slice are the open cells. Each tile gets the index of the
// cell containing it written to tileCellID.
func PartitionCells(walkable [NVMTotalTiles]bool) (cells []PartitionedCell, tileCellID [NVMTotalTiles]int32, openCount int) {
	var visited [NVMTotalTiles]bool

	openCells := partitionPass(walkable, &visited, true)
	closedCells := partitionPass(walkable, &visited, false)

	cells = make([]PartitionedCell, 0, len(openCells)+len(closedCells))
	cells = append(cells, openCells...)
	cells = append(cells, closedCells...)
	openCount = len(openCells)

	for cellIdx, cell := range cells {
		for tj := cell.MinTileZ; tj <= cell.MaxTileZ; tj++ {
			for ti := cell.MinTileX; ti <= cell.MaxTileX; ti++ {
				tileCellID[tj*NVMTileCount+ti] = int32(cellIdx)
			}
		}
	}
	return
}

func partitionPass(walkable [NVMTotalTiles]bool, visited *[NVMTotalTiles]bool, match bool) []PartitionedCell {
	var cells []PartitionedCell
	for blockZ := 0; blockZ < NVMTileCount; blockZ += NVMMaxCellTile {
		maxZ := minInt(blockZ+NVMMaxCellTile, NVMTileCount)
		for blockX := 0; blockX < NVMTileCount; blockX += NVMMaxCellTile {
			maxX := minInt(blockX+NVMMaxCellTile, NVMTileCount)
			for tj := blockZ; tj < maxZ; tj++ {
				for ti := blockX; ti < maxX; ti++ {
					idx := tj*NVMTileCount + ti
					if visited[idx] || walkable[idx] != match {
						continue
					}
					cells = append(cells, growRectInBlock(ti, tj, maxX, maxZ, match, walkable, visited))
				}
			}
		}
	}
	return cells
}

func growRectInBlock(si, sj, blockMaxX, blockMaxZ int, match bool, walkable [NVMTotalTiles]bool, visited *[NVMTotalTiles]bool) PartitionedCell {
	w := 0
	for si+w < blockMaxX {
		idx := sj*NVMTileCount + si + w
		if visited[idx] || walkable[idx] != match {
			break
		}
		w++
	}

	h := 1
	for sj+h < blockMaxZ {
		ok := true
		for k := 0; k < w; k++ {
			idx := (sj+h)*NVMTileCount + si + k
			if visited[idx] || walkable[idx] != match {
				ok = false
				break
			}
		}
		if !ok {
			break
		}
		h++
	}

	for tj := sj; tj < sj+h; tj++ {
		for ti := si; ti < si+w; ti++ {
			visited[tj*NVMTileCount+ti] = true
		}
	}
	return PartitionedCell{
		MinTileX: si, MinTileZ: sj,
		MaxTileX: si + w - 1, MaxTileZ: sj + h - 1,
		Open: match,
	}
}

// CellWorldBounds returns the world-space (region-local) AABB of a tile-space cell.
func CellWorldBounds(c PartitionedCell) (minX, minZ, maxX, maxZ float32) {
	minX = float32(c.MinTileX) * NVMTileSize
	minZ = float32(c.MinTileZ) * NVMTileSize
	maxX = float32(c.MaxTileX+1) * NVMTileSize
	maxZ = float32(c.MaxTileZ+1) * NVMTileSize
	return
}

// GenerateInternalEdges connects every pair of cells that share a tile edge.
// Closed-to-closed pairs are skipped — they're not navigable anyway and the
// game's pathfinder doesn't traverse them.
func GenerateInternalEdges(cells []PartitionedCell) []NVMInternalEdge {
	var edges []NVMInternalEdge
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			if !cells[i].Open && !cells[j].Open {
				continue
			}
			if e, ok := tryMakeInternalEdge(cells[i], cells[j], int16(i), int16(j)); ok {
				edges = append(edges, e)
			}
		}
	}
	return edges
}

// GenerateInternalEdgesWithWalls connects open cells to other open cells with
// normal traversable edges and emits one-way wall edges wherever an open cell
// touches a closed cell. This matches map-baked NVMs: closed cells are
// referenced by Tile.CellID, while the wall edge points from the adjacent open
// cell to Cell1=-1 with Flag=2. Collinear wall spans for the same open cell are
// merged after traversable edges are emitted, matching Etherial's topology.
func GenerateInternalEdgesWithWalls(cells []PartitionedCell) []NVMInternalEdge {
	var traversable []NVMInternalEdge
	var walls []NVMInternalEdge
	for i := 0; i < len(cells); i++ {
		for j := i + 1; j < len(cells); j++ {
			if !cells[i].Open && !cells[j].Open {
				continue
			}
			e, ok := tryMakeInternalEdge(cells[i], cells[j], int16(i), int16(j))
			if !ok {
				continue
			}
			switch {
			case cells[i].Open && cells[j].Open:
				normalizeEdgeCellOrder(&e, int16(i), int16(j))
				e.Flag = NVMInternalEdgeFlag
				traversable = append(traversable, e)
			case cells[int(e.Cell0)].Open && !cells[int(e.Cell1)].Open:
				e.Flag = NVMWallEdgeFlag
				e.Cell1 = -1
				e.Dir1 = 0xFF
				walls = append(walls, e)
			case !cells[int(e.Cell0)].Open && cells[int(e.Cell1)].Open:
				walls = append(walls, reverseWallEdge(e))
			}
		}
	}
	return append(traversable, mergeWallEdges(walls)...)
}

func normalizeEdgeCellOrder(e *NVMInternalEdge, a, b int16) {
	if e.Cell0 == a && e.Cell1 == b {
		return
	}
	e.Cell0, e.Cell1 = e.Cell1, e.Cell0
	e.Dir0, e.Dir1 = e.Dir1, e.Dir0
}

func reverseWallEdge(e NVMInternalEdge) NVMInternalEdge {
	e.Flag = NVMWallEdgeFlag
	e.Cell0, e.Cell1 = e.Cell1, -1
	e.Dir0, e.Dir1 = e.Dir1, 0xFF
	return e
}

func mergeWallEdges(edges []NVMInternalEdge) []NVMInternalEdge {
	sort.SliceStable(edges, func(i, j int) bool {
		return wallEdgeLess(edges[i], edges[j])
	})
	merged := make([]NVMInternalEdge, 0, len(edges))
	for _, e := range edges {
		if mergeIntoWallEdge(merged, e) {
			continue
		}
		merged = append(merged, e)
	}
	return merged
}

func wallEdgeLess(a, b NVMInternalEdge) bool {
	if a.Cell0 != b.Cell0 {
		return a.Cell0 < b.Cell0
	}
	if a.Dir0 != b.Dir0 {
		return a.Dir0 < b.Dir0
	}
	if a.MinX != b.MinX {
		return a.MinX < b.MinX
	}
	if a.MinZ != b.MinZ {
		return a.MinZ < b.MinZ
	}
	if a.MaxX != b.MaxX {
		return a.MaxX < b.MaxX
	}
	return a.MaxZ < b.MaxZ
}

func mergeIntoWallEdge(edges []NVMInternalEdge, e NVMInternalEdge) bool {
	for i := range edges {
		m := &edges[i]
		if m.Flag != NVMWallEdgeFlag || m.Cell1 != -1 || m.Cell0 != e.Cell0 || m.Dir0 != e.Dir0 {
			continue
		}
		switch e.Dir0 {
		case NVMDirNorth, NVMDirSouth:
			if m.MinZ != e.MinZ || m.MaxZ != e.MaxZ || m.MinX > e.MaxX || e.MinX > m.MaxX {
				continue
			}
			if e.MinX < m.MinX {
				m.MinX = e.MinX
			}
			if e.MaxX > m.MaxX {
				m.MaxX = e.MaxX
			}
			return true
		case NVMDirEast, NVMDirWest:
			if m.MinX != e.MinX || m.MaxX != e.MaxX || m.MinZ > e.MaxZ || e.MinZ > m.MaxZ {
				continue
			}
			if e.MinZ < m.MinZ {
				m.MinZ = e.MinZ
			}
			if e.MaxZ > m.MaxZ {
				m.MaxZ = e.MaxZ
			}
			return true
		}
	}
	return false
}

func tryMakeInternalEdge(a, b PartitionedCell, ai, bi int16) (NVMInternalEdge, bool) {
	// b directly east of a
	if a.MaxTileX+1 == b.MinTileX {
		ovMinZ := maxInt(a.MinTileZ, b.MinTileZ)
		ovMaxZ := minInt(a.MaxTileZ, b.MaxTileZ)
		if ovMinZ > ovMaxZ {
			return NVMInternalEdge{}, false
		}
		x := float32(b.MinTileX) * NVMTileSize
		return NVMInternalEdge{
			MinX: x, MaxX: x,
			MinZ:  float32(ovMinZ) * NVMTileSize,
			MaxZ:  float32(ovMaxZ+1) * NVMTileSize,
			Cell0: ai, Cell1: bi,
			Dir0: NVMDirEast, Dir1: NVMDirWest,
		}, true
	}
	if a.MinTileX == b.MaxTileX+1 {
		return tryMakeInternalEdge(b, a, bi, ai)
	}
	// b directly north of a
	if a.MaxTileZ+1 == b.MinTileZ {
		ovMinX := maxInt(a.MinTileX, b.MinTileX)
		ovMaxX := minInt(a.MaxTileX, b.MaxTileX)
		if ovMinX > ovMaxX {
			return NVMInternalEdge{}, false
		}
		z := float32(b.MinTileZ) * NVMTileSize
		return NVMInternalEdge{
			MinX: float32(ovMinX) * NVMTileSize,
			MaxX: float32(ovMaxX+1) * NVMTileSize,
			MinZ: z, MaxZ: z,
			Cell0: ai, Cell1: bi,
			Dir0: NVMDirNorth, Dir1: NVMDirSouth,
		}, true
	}
	if a.MinTileZ == b.MaxTileZ+1 {
		return tryMakeInternalEdge(b, a, bi, ai)
	}
	return NVMInternalEdge{}, false
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ApplyNVMTileFlags marks every non-walkable tile as belonging to a CLOSED
// cell so the game-server's pathfinder refuses movement through it.
//
// Empirically (see nvmtileinspect against real game NVMs), collision lives
// in the cell topology, not the per-tile Flag bit:
//
//	Original cj_ferry_box NVM:
//	  Flag bit 1 set on  34 tiles
//	  In closed cells:  1032 tiles
//	  Overlap:             0 tiles  ← Flag bit doesn't control collision
//
// So this function:
//
//  1. For each currently-blocked tile that's still pointing at an OPEN cell,
//     reassign its CellID to a freshly-appended single-tile CLOSED cell.
//  2. Tiles already in a closed cell are left alone (they're baked terrain).
//  3. Tiles transitioning back to walkable get reverted to a synthetic open
//     cell (cell 0) — coarse but matches what default-mode would have done
//     before, and avoids stranded closed-cell references.
//  4. The Tile.Flag bit is kept in sync as a hint for the editor's brush
//     preview, even though the game ignores it.
//
// We never touch existing cells' bounds, internal edges, global edges, or
// NVM objects — only append new closed cells and reassign tile pointers.
// That's invasive enough to add collision but localised enough that the
// server emulator accepts the file (verified vs Full Repartition crashes).
// ApplyNVMTileFlags flips the per-tile "blocked" bit so the server's
// pathfinder refuses movement onto non-walkable tiles.
//
// The blocked bit is **bit 0** (value 1), confirmed via the PathFinder.NET
// reference project: `IsBlocked => (Flag & 1) != 0`. Earlier we'd been
// setting bit 1 (value 2) based on a misread of the TileFlag enum, which is
// why default-mode "rebuild" wasn't producing collision against placed
// objects — we were toggling a different (unused?) bit entirely.
func ApplyNVMTileFlags(n *NVM, walkable [NVMTotalTiles]bool) {
	for i := range n.Tiles {
		if walkable[i] {
			n.Tiles[i].Flag &^= 1
		} else {
			n.Tiles[i].Flag |= 1
		}
	}
}

// ApplyNVMBlockTiles writes per-tile blocked Flag bit 0 AND redirects each
// non-walkable tile's CellID to point at a closed cell (any index >=
// OpenCellCount). The Silkroad server's pathfinder treats "tile's cell is a
// closed cell" as the actual blocked signal — Flag bit 0 alone is read by
// the client-side reference pathfinder but the server engine evaluates
// cell-state, so we need both.
//
// Constraints learned the hard way:
//   - CellID = -1 detaches the tile from the cell graph entirely, which
//     causes the server to throw an "overlap exception" at boot when it
//     tries to reconcile cell adjacency.
//   - Appending NEW closed cells changes the cell-count header and trips a
//     client popup ("Load Fail(NavMesh Obj) …").
//
// Compromise that satisfies both: leave the cell array untouched and just
// reroute the blocked tile's CellID to an existing closed cell index that's
// already in the .nvm. We pick OpenCellCount (the first closed cell) by
// default. If the navmesh has zero closed cells we fall back to leaving
// the tile's CellID alone (Flag bit only).
func ApplyNVMBlockTiles(n *NVM, walkable [NVMTotalTiles]bool) {
	var hasClosedCell bool
	var closedCellIdx int32
	if uint32(len(n.Cells)) > n.OpenCellCount {
		hasClosedCell = true
		closedCellIdx = int32(n.OpenCellCount)
	}
	for i := range n.Tiles {
		if walkable[i] {
			n.Tiles[i].Flag &^= 1
		} else {
			n.Tiles[i].Flag |= 1
			if hasClosedCell {
				n.Tiles[i].CellID = closedCellIdx
			}
		}
	}
}

// Edge flag values matched against original NVM files exported from the base
// game. A flag of 0 (which an earlier rebuild used) makes the server's
// pathfinder treat the edge as non-traversable, so make sure to set these.
const (
	NVMInternalEdgeFlag = 4
	NVMGlobalEdgeFlag   = 8
	NVMWallEdgeFlag     = 2
)

// ApplyNVMNavRebuild replaces the navmesh's cells, internal edges, and tile
// flags/cellIDs based on the supplied walkability grid. NVM objects are
// re-bound to whatever cell their XZ position falls into; their link arrays
// are cleared (referenced obsolete edge IDs). Global edges are best-effort
// re-pointed to new cells that physically touch the same boundary side.
//
// This is destructive: the original cell partition the base game baked is
// thrown away. Prefer ApplyNVMTileFlags when walkability hasn't structurally
// changed.
func ApplyNVMNavRebuild(n *NVM, walkable [NVMTotalTiles]bool) {
	cells, tileCellID, openCount := PartitionCells(walkable)
	edges := GenerateInternalEdges(cells)
	for i := range edges {
		edges[i].Flag = NVMInternalEdgeFlag
	}

	n.OpenCellCount = uint32(openCount)
	n.Cells = make([]NVMCell, len(cells))
	for i, c := range cells {
		minX, minZ, maxX, maxZ := CellWorldBounds(c)
		n.Cells[i] = NVMCell{
			MinX: minX, MinZ: minZ, MaxX: maxX, MaxZ: maxZ,
			ObjectIndices: nil,
		}
	}
	n.InternalEdges = edges

	// Update CellID pointers to the new partition. We deliberately do NOT
	// touch tile.Flag here: the rebuild tool we benchmarked against leaves
	// tile.Flag=0x00 around custom assets, and stamping Flag=1 on every
	// AABB-covered tile broke slope walkability (tiles under stock objects'
	// AABBs cover ramps/stairs the player needs to traverse). Collision for
	// new objects is supplied by the cell→ObjectIndices→BSR/BMS chain, not
	// the tile bits.
	for i := range n.Tiles {
		n.Tiles[i].CellID = tileCellID[i]
	}

	for i := range n.Objects {
		n.Objects[i].Links = nil
	}
	for objIdx, obj := range n.Objects {
		ci := findCellAt(n.Cells, obj.X, obj.Z)
		if ci < 0 || len(n.Cells[ci].ObjectIndices) >= 255 {
			continue
		}
		n.Cells[ci].ObjectIndices = append(n.Cells[ci].ObjectIndices, uint16(objIdx))
	}

	rebindGlobalEdges(n)
}

// rebindGlobalEdges re-points each original cross-region edge's Cell0 to an
// open cell in the new partition that physically touches the same boundary
// side (N/S/E/W) and overlaps the edge's segment.
func rebindGlobalEdges(n *NVM) {
	const regionExtent = float32(NVMTileCount) * NVMTileSize
	for i := range n.GlobalEdges {
		e := &n.GlobalEdges[i]
		if e.Flag == 0 {
			e.Flag = NVMGlobalEdgeFlag
		}
		var ci int
		switch {
		case e.MinZ == regionExtent && e.MaxZ == regionExtent: // north
			ci = findBoundaryCell(n.Cells, int(n.OpenCellCount), 'N', e.MinX, e.MaxX)
		case e.MinZ == 0 && e.MaxZ == 0: // south
			ci = findBoundaryCell(n.Cells, int(n.OpenCellCount), 'S', e.MinX, e.MaxX)
		case e.MinX == regionExtent && e.MaxX == regionExtent: // east
			ci = findBoundaryCell(n.Cells, int(n.OpenCellCount), 'E', e.MinZ, e.MaxZ)
		case e.MinX == 0 && e.MaxX == 0: // west
			ci = findBoundaryCell(n.Cells, int(n.OpenCellCount), 'W', e.MinZ, e.MaxZ)
		default:
			midX := (e.MinX + e.MaxX) / 2
			midZ := (e.MinZ + e.MaxZ) / 2
			ci = findCellAt(n.Cells, midX, midZ)
		}
		if ci >= 0 {
			e.Cell0 = int16(ci)
		}
	}
}

// findBoundaryCell returns the index of an open cell that sits flush against
// the given region side and overlaps the [lo, hi] segment along the
// perpendicular axis. -1 if none found.
func findBoundaryCell(cells []NVMCell, openCount int, side byte, lo, hi float32) int {
	const regionExtent = float32(NVMTileCount) * NVMTileSize
	if openCount > len(cells) {
		openCount = len(cells)
	}
	for i := 0; i < openCount; i++ {
		c := cells[i]
		switch side {
		case 'N':
			if c.MaxZ == regionExtent && c.MinX < hi && c.MaxX > lo {
				return i
			}
		case 'S':
			if c.MinZ == 0 && c.MinX < hi && c.MaxX > lo {
				return i
			}
		case 'E':
			if c.MaxX == regionExtent && c.MinZ < hi && c.MaxZ > lo {
				return i
			}
		case 'W':
			if c.MinX == 0 && c.MinZ < hi && c.MaxZ > lo {
				return i
			}
		}
	}
	return -1
}

func findCellAt(cells []NVMCell, x, z float32) int {
	for i, c := range cells {
		if x >= c.MinX && x < c.MaxX && z >= c.MinZ && z < c.MaxZ {
			return i
		}
	}
	return -1
}
