package editor

import (
	"os"
	"path/filepath"
	"testing"

	"sromapedit/internal/sromap"
)

func TestMapOnlyBuild5C94Integration(t *testing.T) {
	root := os.Getenv("SROMAPEDIT_INTEGRATION_ROOT")
	if root == "" {
		t.Skip("set SROMAPEDIT_INTEGRATION_ROOT to run map-only NVM integration test")
	}
	srv, err := NewServer(root)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	const rx, ry = 148, 92
	const slope = float32(DefaultNVMSlopeThreshold)
	mesh, err := sromap.LoadMesh(sromap.MeshPath(root, rx, ry))
	if err != nil {
		t.Fatalf("load mesh: %v", err)
	}
	terrainOnly := srv.buildMapOnlyNVM(rx, ry, mesh, nil, slope, false).NVM
	terrainClosed := 0
	for _, tile := range terrainOnly.Tiles {
		if uint32(tile.CellID) >= terrainOnly.OpenCellCount {
			terrainClosed++
		}
	}
	t.Logf("terrain-only stats: cells=%d open=%d internals=%d closedTiles=%d",
		len(terrainOnly.Cells), terrainOnly.OpenCellCount, len(terrainOnly.InternalEdges), terrainClosed)

	placements := srv.loadMapOnlyPlacements(rx, ry)
	assertHasPlacement(t, placements, 946, -21502, 0x5b93)
	for _, p := range placements {
		asset, _ := srv.objCache.get(p.ObjID)
		if !assetHasMapOnlyCollision(asset) {
			continue
		}
		one := srv.buildMapOnlyNVM(rx, ry, mesh, []sromap.ObjectEntry{p}, slope, false).NVM
		closed := 0
		for _, tile := range one.Tiles {
			if uint32(tile.CellID) >= one.OpenCellCount {
				closed++
			}
		}
		t.Logf("one-object asset=%d uid=%d region=%04x pos=(%.1f,%.1f) closedDelta=%d cells=%d",
			p.ObjID, p.UID, p.RegionID, p.X, p.Z, closed-terrainClosed, len(one.Cells))
	}
	built := srv.buildMapOnlyNVM(rx, ry, mesh, placements, slope, true)
	nvm := built.NVM

	if len(nvm.Objects) == 0 {
		t.Fatalf("map-only build produced no NVMObjects from %d placements", len(placements))
	}
	if len(nvm.Cells) == 0 || nvm.OpenCellCount == 0 || int(nvm.OpenCellCount) >= len(nvm.Cells) {
		t.Fatalf("unexpected cell counts: cells=%d open=%d", len(nvm.Cells), nvm.OpenCellCount)
	}
	if len(nvm.InternalEdges) == 0 {
		t.Fatalf("map-only build produced no internal edges")
	}
	if len(nvm.GlobalEdges) == 0 {
		t.Fatalf("map-only build produced no global edges")
	}
	var closedTiles, wallEdges int
	for _, tile := range nvm.Tiles {
		if uint32(tile.CellID) >= nvm.OpenCellCount {
			closedTiles++
		}
	}
	for _, edge := range nvm.InternalEdges {
		if edge.Flag == sromap.NVMWallEdgeFlag {
			wallEdges++
		}
	}
	if closedTiles == 0 || wallEdges == 0 {
		t.Fatalf("expected closed collision tiles and wall edges, got closedTiles=%d wallEdges=%d", closedTiles, wallEdges)
	}
	t.Logf("map-only stats: objects=%d cells=%d open=%d globals=%d internals=%d closedTiles=%d wallEdges=%d",
		len(nvm.Objects), len(nvm.Cells), nvm.OpenCellCount, len(nvm.GlobalEdges), len(nvm.InternalEdges), closedTiles, wallEdges)

	refPath := filepath.Join(root, "new_rebuild", "Data", "navmesh", sromap.NVMFileName(rx, ry))
	if ref, err := sromap.LoadNVM(refPath); err == nil && len(ref.GlobalEdges) == 0 {
		t.Fatalf("reference NVM has no global edges at %s", refPath)
	}
}

func assertHasPlacement(t *testing.T, placements []sromap.ObjectEntry, objID uint32, uid int16, regionID uint16) {
	t.Helper()
	for _, p := range placements {
		if p.ObjID == objID && p.UID == uid && p.RegionID == regionID {
			return
		}
	}
	t.Fatalf("missing placement obj=%d uid=%d region=0x%04x from %d map-only placements", objID, uid, regionID, len(placements))
}
