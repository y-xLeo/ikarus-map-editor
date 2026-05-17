package sromap

import (
	"os"
	"path/filepath"
	"testing"
)

func loadSample(t *testing.T) *O2 {
	t.Helper()
	candidates := []string{
		"../../../../Map/100/100.o2",
		"../../../../Map/148/92.o2",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err != nil {
			continue
		}
		o2, err := LoadO2(c)
		if err != nil {
			continue
		}
		return o2
	}
	t.Skip("no .o2 sample available")
	return nil
}

func TestApplyDeletesRemovesAllMatchingEntries(t *testing.T) {
	o2 := loadSample(t)
	if len(o2.Entries) == 0 {
		t.Skip("no entries in sample")
	}
	first := o2.Entries[0]
	key := ObjectKey{ObjID: first.ObjID, UID: first.UID, RegionID: first.RegionID}
	originalCount := 0
	for _, e := range o2.Entries {
		if e.ObjID == key.ObjID && e.UID == key.UID && e.RegionID == key.RegionID {
			originalCount++
		}
	}
	deleted := o2.ApplyDeletes([]ObjectKey{key})
	if deleted != originalCount {
		t.Fatalf("expected %d deletions, got %d", originalCount, deleted)
	}
	for _, e := range o2.Entries {
		if e.ObjID == key.ObjID && e.UID == key.UID && e.RegionID == key.RegionID {
			t.Fatalf("entry survived delete: %+v", e)
		}
	}
}

func TestApplyAddsAssignsUniqueUIDs(t *testing.T) {
	o2 := loadSample(t)
	regionID := uint16(25700)
	if len(o2.Entries) > 0 {
		regionID = o2.Entries[0].RegionID
	}
	adds := []ObjectAdd{
		{ObjID: 25, RegionID: regionID, X: 200, Y: 50, Z: 200, Yaw: 0},
		{ObjID: 25, RegionID: regionID, X: 500, Y: 60, Z: 500, Yaw: 1.5},
	}
	result := o2.ApplyAdds(adds)
	if len(result) != 2 {
		t.Fatalf("expected 2 added results, got %d", len(result))
	}
	if result[0].UID == result[1].UID {
		t.Fatalf("expected unique UIDs, got %d %d", result[0].UID, result[1].UID)
	}
}

func TestApplyAddsDeduplicatesClampedBigBlocks(t *testing.T) {
	o2 := &O2{}
	o2.ApplyAdds([]ObjectAdd{{
		ObjID: 100, RegionID: 0x5c94,
		X: 1700, Y: 20, Z: 1700,
		IsBig: true,
	}})
	want := map[[2]int]bool{
		{4, 4}: true,
		{5, 4}: true,
		{4, 5}: true,
		{5, 5}: true,
	}
	if len(o2.Entries) != len(want) {
		t.Fatalf("entries = %d, want %d unique clamped blocks", len(o2.Entries), len(want))
	}
	for _, e := range o2.Entries {
		if !want[[2]int{e.XBlock, e.ZBlock}] {
			t.Fatalf("unexpected block after add: %+v", e)
		}
		delete(want, [2]int{e.XBlock, e.ZBlock})
	}
	if len(want) != 0 {
		t.Fatalf("missing blocks after add: %#v", want)
	}
}

func TestApplyEditsMovesHostBlockFootprint(t *testing.T) {
	o2 := &O2{Entries: []ObjectEntry{
		{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 100, Y: 10, Z: 100, Yaw: 0.5, Static: -1, XBlock: 0, ZBlock: 0, LODGroup: 2},
		{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 100, Y: 10, Z: 100, Yaw: 0.5, Static: -1, XBlock: 1, ZBlock: 0, LODGroup: 2},
		{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 100, Y: 10, Z: 100, Yaw: 0.5, Static: -1, XBlock: 0, ZBlock: 1, LODGroup: 2},
		{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 100, Y: 10, Z: 100, Yaw: 0.5, Static: -1, XBlock: 1, ZBlock: 1, LODGroup: 2},
		{ObjID: 200, UID: 8, RegionID: 0x5c94, X: 100, Y: 10, Z: 100, XBlock: 0, ZBlock: 0, LODGroup: 2},
	}}
	updated := o2.ApplyEdits([]ObjectEdit{{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 1000, Y: 20, Z: 1000, Yaw: 1.25}})
	if updated != 4 {
		t.Fatalf("updated = %d, want 4", updated)
	}
	wantBlocks := map[[3]int]bool{
		{3, 3, 2}: true,
		{4, 3, 2}: true,
		{3, 4, 2}: true,
		{4, 4, 2}: true,
	}
	gotBlocks := map[[3]int]bool{}
	for _, e := range o2.Entries {
		if e.ObjID != 100 {
			continue
		}
		if e.X != 1000 || e.Y != 20 || e.Z != 1000 || e.Yaw != 1.25 {
			t.Fatalf("edited entry did not receive new transform: %+v", e)
		}
		gotBlocks[[3]int{e.XBlock, e.ZBlock, e.LODGroup}] = true
	}
	if len(gotBlocks) != len(wantBlocks) {
		t.Fatalf("block count = %d, want %d: %#v", len(gotBlocks), len(wantBlocks), gotBlocks)
	}
	for b := range wantBlocks {
		if !gotBlocks[b] {
			t.Fatalf("missing moved block %v in %#v", b, gotBlocks)
		}
	}
}

func TestApplyEditsDeduplicatesClampedHostBlocks(t *testing.T) {
	o2 := &O2{Entries: []ObjectEntry{
		{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 1700, Y: 10, Z: 1700, XBlock: 4, ZBlock: 4, LODGroup: 2},
		{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 1700, Y: 10, Z: 1700, XBlock: 5, ZBlock: 4, LODGroup: 2},
		{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 1700, Y: 10, Z: 1700, XBlock: 4, ZBlock: 5, LODGroup: 2},
		{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 1700, Y: 10, Z: 1700, XBlock: 5, ZBlock: 5, LODGroup: 2},
	}}
	updated := o2.ApplyEdits([]ObjectEdit{{ObjID: 100, UID: 7, RegionID: 0x5c94, X: 60, Y: 20, Z: 60, Yaw: 0}})
	if updated != 4 {
		t.Fatalf("updated = %d, want 4", updated)
	}
	if len(o2.Entries) != 1 {
		t.Fatalf("entries = %d, want 1 deduplicated clamped entry", len(o2.Entries))
	}
	for _, e := range o2.Entries {
		if e.XBlock != 0 || e.ZBlock != 0 {
			t.Fatalf("unexpected clamped block after edit: %+v", e)
		}
	}
}

func TestApplyChangesRoundtrip(t *testing.T) {
	o2 := loadSample(t)
	adds := []ObjectAdd{
		{ObjID: 25, RegionID: o2.Entries[0].RegionID, X: 100, Y: 50, Z: 100, Yaw: 0},
	}
	res := o2.ApplyChanges(nil, nil, adds)
	if len(res.Added) != 1 {
		t.Fatalf("expected 1 added, got %d", len(res.Added))
	}
	tmp := filepath.Join(t.TempDir(), "out.o2")
	if err := o2.Save(tmp); err != nil {
		t.Fatalf("Save: %v", err)
	}
	reloaded, err := LoadO2(tmp)
	if err != nil {
		t.Fatalf("LoadO2 after save: %v", err)
	}
	// New entry should be findable
	found := false
	want := res.Added[0]
	for _, e := range reloaded.Entries {
		if e.ObjID == want.ObjID && e.UID == want.UID && e.RegionID == want.RegionID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("added entry missing after roundtrip")
	}
}
