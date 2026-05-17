package editor

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"sromapedit/internal/sromap"
)

func TestNavRebuildPlacementsTrackMovedNeighborOwner(t *testing.T) {
	root := os.Getenv("SROMAPEDIT_INTEGRATION_ROOT")
	if root == "" {
		t.Skip("set SROMAPEDIT_INTEGRATION_ROOT to run against an exported PK2 tree")
	}

	srv, err := NewServer(root)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	owner := findO2Entry(t, root, 149, 92, 3308, 29698)
	if owner.RegionID != 0x5c95 {
		t.Fatalf("owner RegionID = 0x%04x, want 0x5c95", owner.RegionID)
	}
	asset, _ := srv.objCache.get(3308)
	if asset == nil {
		t.Fatalf("asset 3308 not loadable")
	}

	ownerPlacements := srv.loadNavRebuildPlacements(149, 92)
	ownerPlacement, ok := findPlacement(ownerPlacements, 3308, 29698)
	if !ok {
		t.Fatalf("asset 3308 uid 29698 missing from owner-region rebuild placements")
	}
	assertPlacement(t, ownerPlacement, owner)

	westOwner := owner
	westOwner.X += sromap.RegionSize
	westPlacements := srv.loadNavRebuildPlacements(148, 92)
	westPlacement, westFound := findPlacement(westPlacements, 3308, 29698)
	minX, maxX, minZ, maxZ := placementRotatedBBox(westOwner, asset)
	overlapsWest := maxX > 0 && minX < sromap.RegionSize && maxZ > 0 && minZ < sromap.RegionSize
	if overlapsWest {
		if !westFound {
			t.Fatalf("asset 3308 overlaps west region but is missing from west rebuild placements")
		}
		assertPlacement(t, westPlacement, westOwner)
	} else if westFound {
		t.Fatalf("asset 3308 does not overlap west region but rebuild still included pos(%.2f, %.2f, %.2f)", westPlacement.X, westPlacement.Y, westPlacement.Z)
	}

	westNVM, err := sromap.LoadNVM(filepath.Join(root, "export", "Data", "Navmesh", "nv_5c94.nvm"))
	if err != nil {
		t.Fatalf("load exported nv_5c94.nvm: %v", err)
	}
	srv.addCustomNVMObjectsOnly(westNVM, westPlacements)
	westObjIdx, westHasObj := findNVMObject(westNVM, 3308, 29698)
	if overlapsWest {
		if !westHasObj {
			t.Fatalf("asset 3308 should remain in west NVM after sync")
		}
		assertNVMObject(t, westNVM.Objects[westObjIdx], westOwner)
	} else if westHasObj {
		t.Fatalf("asset 3308 should have been pruned from west NVM, still at index %d", westObjIdx)
	}

	ownerNVM, err := sromap.LoadNVM(filepath.Join(root, "export", "Data", "Navmesh", "nv_5c95.nvm"))
	if err != nil {
		t.Fatalf("load exported nv_5c95.nvm: %v", err)
	}
	srv.addCustomNVMObjectsOnly(ownerNVM, ownerPlacements)
	srv.addCustomBboxObjectIndices(ownerNVM)
	ownerObjIdx, ok := findNVMObject(ownerNVM, 3308, 29698)
	if !ok {
		t.Fatalf("asset 3308 missing from owner NVM after sync")
	}
	assertNVMObject(t, ownerNVM.Objects[ownerObjIdx], owner)
	assertObjectCellRefsOverlap(t, ownerNVM, uint16(ownerObjIdx), owner, asset)
}

func findO2Entry(t *testing.T, root string, x, y int, objID uint32, uid int16) sromap.ObjectEntry {
	t.Helper()
	o2, err := sromap.LoadO2(sromap.O2Path(root, x, y))
	if err != nil {
		t.Fatalf("LoadO2: %v", err)
	}
	for _, e := range o2.Entries {
		if e.ObjID == objID && e.UID == uid {
			return e
		}
	}
	t.Fatalf("objID %d uid %d not found in %s", objID, uid, sromap.O2Path(root, x, y))
	return sromap.ObjectEntry{}
}

func findPlacement(placements []sromap.ObjectEntry, objID uint32, uid int16) (sromap.ObjectEntry, bool) {
	for _, p := range placements {
		if p.ObjID == objID && p.UID == uid {
			return p, true
		}
	}
	return sromap.ObjectEntry{}, false
}

func findNVMObject(nvm *sromap.NVM, assetID uint32, uid int16) (int, bool) {
	for i, o := range nvm.Objects {
		if o.AssetID == assetID && o.UID == uid {
			return i, true
		}
	}
	return 0, false
}

func assertPlacement(t *testing.T, got, want sromap.ObjectEntry) {
	t.Helper()
	if got.RegionID != want.RegionID || !near(got.X, want.X) || !near(got.Y, want.Y) || !near(got.Z, want.Z) || !near(got.Yaw, want.Yaw) {
		t.Fatalf("placement = region 0x%04x pos(%.2f, %.2f, %.2f) yaw %.4f, want region 0x%04x pos(%.2f, %.2f, %.2f) yaw %.4f",
			got.RegionID, got.X, got.Y, got.Z, got.Yaw, want.RegionID, want.X, want.Y, want.Z, want.Yaw)
	}
}

func assertNVMObject(t *testing.T, got sromap.NVMObject, want sromap.ObjectEntry) {
	t.Helper()
	if got.RegionID != want.RegionID || !near(got.X, want.X) || !near(got.Y, want.Y) || !near(got.Z, want.Z) || !near(got.Yaw, want.Yaw) {
		t.Fatalf("NVMObject = region 0x%04x pos(%.2f, %.2f, %.2f) yaw %.4f, want region 0x%04x pos(%.2f, %.2f, %.2f) yaw %.4f",
			got.RegionID, got.X, got.Y, got.Z, got.Yaw, want.RegionID, want.X, want.Y, want.Z, want.Yaw)
	}
}

func assertObjectCellRefsOverlap(t *testing.T, nvm *sromap.NVM, objIdx uint16, placement sromap.ObjectEntry, asset *objectAsset) {
	t.Helper()
	minX, maxX, minZ, maxZ := placementRotatedBBox(placement, asset)
	refs := 0
	for ci, cell := range nvm.Cells {
		has := false
		for _, idx := range cell.ObjectIndices {
			if idx == objIdx {
				has = true
				break
			}
		}
		if !has {
			continue
		}
		refs++
		overlapX := minF(cell.MaxX, maxX) - maxF(cell.MinX, minX)
		overlapZ := minF(cell.MaxZ, maxZ) - maxF(cell.MinZ, minZ)
		if overlapX < 20 || overlapZ < 20 {
			t.Fatalf("cell %d references object %d but does not overlap current bbox enough: overlap %.2fx%.2f cell=(%.0f,%.0f)..(%.0f,%.0f) bbox=(%.2f,%.2f)..(%.2f,%.2f)",
				ci, objIdx, overlapX, overlapZ, cell.MinX, cell.MinZ, cell.MaxX, cell.MaxZ, minX, minZ, maxX, maxZ)
		}
	}
	if refs == 0 {
		t.Fatalf("object %d has no cell references", objIdx)
	}
}

func near(got, want float32) bool {
	return math.Abs(float64(got-want)) < 0.02
}
