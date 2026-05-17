package editor

import (
	"testing"

	"sromapedit/internal/sromap"
)

func TestVisibilityBlocksForBridgeVisualBBox(t *testing.T) {
	asset := &objectAsset{
		BBoxMin: [3]float32{-342.04, -7.24, -659.06},
		BBoxMax: [3]float32{342.77, 968.32, 661.43},
	}
	entry := sromap.ObjectEntry{
		ObjID:    947,
		UID:      -30718,
		RegionID: 0x5c94,
		X:        952.87,
		Y:        166.66,
		Z:        1331.41,
		Yaw:      0,
	}

	ownerBlocks := visibilityBlocksForEntry(entry, asset, 148, 92)
	wantOwner := rectBlocks(1, 4, 2, 5)
	assertBlockSet(t, ownerBlocks, wantOwner)

	northBlocks := visibilityBlocksForEntry(entry, asset, 148, 93)
	wantNorth := rectBlocks(1, 4, 0, 0)
	assertBlockSet(t, northBlocks, wantNorth)
}

func rectBlocks(x0, x1, z0, z1 int) map[[2]int]bool {
	out := make(map[[2]int]bool)
	for z := z0; z <= z1; z++ {
		for x := x0; x <= x1; x++ {
			out[[2]int{x, z}] = true
		}
	}
	return out
}

func assertBlockSet(t *testing.T, got, want map[[2]int]bool) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("block count = %d, want %d; got %#v", len(got), len(want), got)
	}
	for block := range want {
		if !got[block] {
			t.Fatalf("missing block %v in %#v", block, got)
		}
	}
}
