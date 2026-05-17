package editor

import (
	"testing"

	"sromapedit/internal/sromap"
)

func TestReplaceReciprocalEdgesSplitsNeighborEdge(t *testing.T) {
	localID := uint16(0x5c95)
	neighborID := uint16(0x5c94)
	local := &sromap.NVM{
		GlobalEdges: []sromap.NVMGlobalEdge{
			{
				MinX: 0, MaxX: 0, MinZ: 1620, MaxZ: 1680,
				Flag: 8, Dir0: sromap.NVMDirWest, Dir1: sromap.NVMDirEast,
				Cell0: 13, Cell1: 24, Region0: int16(localID), Region1: int16(neighborID),
			},
			{
				MinX: 0, MaxX: 0, MinZ: 1680, MaxZ: 1760,
				Flag: 8, Dir0: sromap.NVMDirWest, Dir1: sromap.NVMDirEast,
				Cell0: 44, Cell1: 24, Region0: int16(localID), Region1: int16(neighborID),
			},
		},
	}
	neighbor := &sromap.NVM{
		GlobalEdges: []sromap.NVMGlobalEdge{
			{
				MinX: 1920, MaxX: 1920, MinZ: 1620, MaxZ: 1760,
				Flag: 8, Dir0: sromap.NVMDirEast, Dir1: sromap.NVMDirWest,
				Cell0: 24, Cell1: 13, Region0: int16(neighborID), Region1: int16(localID),
			},
		},
	}

	desired := reciprocalEdgesForNeighbor(local, 'W', localID, neighborID)
	if got, want := len(desired), 2; got != want {
		t.Fatalf("desired edges = %d, want %d", got, want)
	}
	if !replaceReciprocalEdges(neighbor, 'E', neighborID, localID, desired) {
		t.Fatal("replaceReciprocalEdges returned false")
	}
	if !globalEdgeSlicesEqual(neighbor.GlobalEdges, desired) {
		t.Fatalf("neighbor edges did not match desired:\n%+v\n%+v", neighbor.GlobalEdges, desired)
	}
	if replaceReciprocalEdges(neighbor, 'E', neighborID, localID, desired) {
		t.Fatal("second replace should be idempotent")
	}
}
