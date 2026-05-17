package sromap

import "testing"

func TestPartitionCellsFullyWalkable(t *testing.T) {
	var walkable [NVMTotalTiles]bool
	for i := range walkable {
		walkable[i] = true
	}
	cells, _, openCount := PartitionCells(walkable)
	if len(cells) != 64 {
		t.Fatalf("expected 64 capped cells, got %d", len(cells))
	}
	if openCount != 64 {
		t.Fatalf("expected openCount=64, got %d", openCount)
	}
	for _, c := range cells {
		if c.MaxTileX-c.MinTileX+1 > NVMMaxCellTile || c.MaxTileZ-c.MinTileZ+1 > NVMMaxCellTile {
			t.Fatalf("cell exceeds cap: %+v", c)
		}
	}
}

func TestPartitionCellsSplitsBlocked(t *testing.T) {
	var walkable [NVMTotalTiles]bool
	for i := range walkable {
		walkable[i] = true
	}
	// Block a 2x2 patch in the middle.
	for tj := 4; tj < 6; tj++ {
		for ti := 4; ti < 6; ti++ {
			walkable[tj*NVMTileCount+ti] = false
		}
	}
	cells, tileCellID, openCount := PartitionCells(walkable)
	if openCount < 2 {
		t.Fatalf("expected ≥2 open cells around the hole, got %d", openCount)
	}
	if len(cells) <= openCount {
		t.Fatalf("expected at least one closed cell, got %d total / %d open", len(cells), openCount)
	}
	// Tile inside the hole should map to a closed cell index ≥ openCount.
	holeIdx := 4*NVMTileCount + 4
	if int(tileCellID[holeIdx]) < openCount {
		t.Fatalf("hole tile mapped to open cell %d (openCount=%d)", tileCellID[holeIdx], openCount)
	}
}

func TestGenerateInternalEdgesAdjacentCells(t *testing.T) {
	// Two cells side by side: a covers tiles [0,0..1,0], b covers [2,0..3,0].
	a := PartitionedCell{MinTileX: 0, MinTileZ: 0, MaxTileX: 1, MaxTileZ: 0, Open: true}
	b := PartitionedCell{MinTileX: 2, MinTileZ: 0, MaxTileX: 3, MaxTileZ: 0, Open: true}
	edges := GenerateInternalEdges([]PartitionedCell{a, b})
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	e := edges[0]
	if e.Cell0 != 0 || e.Cell1 != 1 {
		t.Fatalf("edge cells = (%d, %d)", e.Cell0, e.Cell1)
	}
	if e.MinX != 40 || e.MaxX != 40 {
		t.Fatalf("expected shared X edge at 40, got (%v..%v)", e.MinX, e.MaxX)
	}
	if e.Dir0 != NVMDirEast || e.Dir1 != NVMDirWest {
		t.Fatalf("edge dirs = (%d, %d), want east/west (%d, %d)", e.Dir0, e.Dir1, NVMDirEast, NVMDirWest)
	}
}

func TestGenerateInternalEdgesNorthSouthDirections(t *testing.T) {
	a := PartitionedCell{MinTileX: 0, MinTileZ: 0, MaxTileX: 1, MaxTileZ: 0, Open: true}
	b := PartitionedCell{MinTileX: 0, MinTileZ: 1, MaxTileX: 1, MaxTileZ: 1, Open: true}
	edges := GenerateInternalEdges([]PartitionedCell{a, b})
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	e := edges[0]
	if e.MinZ != 20 || e.MaxZ != 20 {
		t.Fatalf("expected shared Z edge at 20, got (%v..%v)", e.MinZ, e.MaxZ)
	}
	if e.Dir0 != NVMDirNorth || e.Dir1 != NVMDirSouth {
		t.Fatalf("edge dirs = (%d, %d), want north/south (%d, %d)", e.Dir0, e.Dir1, NVMDirNorth, NVMDirSouth)
	}
}

func TestMergeWallEdges(t *testing.T) {
	edges := mergeWallEdges([]NVMInternalEdge{
		{MinX: 1100, MinZ: 840, MaxX: 1140, MaxZ: 840, Flag: NVMWallEdgeFlag, Dir0: NVMDirSouth, Dir1: 0xFF, Cell0: 127, Cell1: -1},
		{MinX: 1140, MinZ: 840, MaxX: 1200, MaxZ: 840, Flag: NVMWallEdgeFlag, Dir0: NVMDirSouth, Dir1: 0xFF, Cell0: 127, Cell1: -1},
	})
	if len(edges) != 1 {
		t.Fatalf("len(edges) = %d, want 1", len(edges))
	}
	if edges[0].MinX != 1100 || edges[0].MaxX != 1200 {
		t.Fatalf("merged bounds = %.0f..%.0f, want 1100..1200", edges[0].MinX, edges[0].MaxX)
	}
}
