package editor

import (
	"os"
	"path/filepath"
	"testing"

	"sromapedit/internal/sromap"
)

func TestPreserveMapOnlyObjectMetadataRemapsLinks(t *testing.T) {
	root := t.TempDir()
	navDir := filepath.Join(root, "Data", "Navmesh")
	if err := os.MkdirAll(navDir, 0755); err != nil {
		t.Fatal(err)
	}
	ref := &sromap.NVM{
		Objects: []sromap.NVMObject{
			{
				AssetID: 100, UID: 1, RegionID: 0x0201,
				X: 10, Y: 20, Z: 30, Type: -1, Short0: 7, IsBig: true,
				Links: []sromap.NVMObjectLink{{LinkedObjectID: 1, LinkedEdgeID: 22, EdgeID: 11}},
			},
			{
				AssetID: 200, UID: 2, RegionID: 0x0201,
				X: 40, Y: 50, Z: 60,
				Links: []sromap.NVMObjectLink{{LinkedObjectID: 0, LinkedEdgeID: 11, EdgeID: 22}},
			},
		},
	}
	if err := ref.Save(filepath.Join(navDir, sromap.NVMFileName(1, 2))); err != nil {
		t.Fatal(err)
	}

	built := &sromap.NVM{
		Objects: []sromap.NVMObject{
			{AssetID: 200, UID: 2, RegionID: 0x0201, X: 40, Y: 50, Z: 60},
			{AssetID: 100, UID: 1, RegionID: 0x0201, X: 10, Y: 20, Z: 30},
		},
	}
	srv := &Server{Root: root}
	srv.preserveMapOnlyObjectMetadata(1, 2, built)

	if got := built.Objects[1].Type; got != -1 {
		t.Fatalf("Type was not preserved: got %d", got)
	}
	if got := built.Objects[1].Short0; got != 7 {
		t.Fatalf("Short0 was not preserved: got %d", got)
	}
	if !built.Objects[1].IsBig {
		t.Fatalf("IsBig was not preserved")
	}
	if len(built.Objects[1].Links) != 1 {
		t.Fatalf("object 1 links = %d, want 1", len(built.Objects[1].Links))
	}
	if got := built.Objects[1].Links[0]; got != (sromap.NVMObjectLink{LinkedObjectID: 0, LinkedEdgeID: 22, EdgeID: 11}) {
		t.Fatalf("object 1 link = %+v", got)
	}
	if got := built.Objects[0].Links[0]; got != (sromap.NVMObjectLink{LinkedObjectID: 1, LinkedEdgeID: 11, EdgeID: 22}) {
		t.Fatalf("object 0 link = %+v", got)
	}
}

func TestPreserveMapOnlyObjectMetadataSkipsMovedObjects(t *testing.T) {
	root := t.TempDir()
	navDir := filepath.Join(root, "Data", "Navmesh")
	if err := os.MkdirAll(navDir, 0755); err != nil {
		t.Fatal(err)
	}
	ref := &sromap.NVM{
		Objects: []sromap.NVMObject{{
			AssetID: 100, UID: 1, RegionID: 0x0201,
			X: 10, Y: 20, Z: 30, Type: -1,
			Links: []sromap.NVMObjectLink{{LinkedObjectID: 0, LinkedEdgeID: 2, EdgeID: 1}},
		}},
	}
	if err := ref.Save(filepath.Join(navDir, sromap.NVMFileName(1, 2))); err != nil {
		t.Fatal(err)
	}

	built := &sromap.NVM{
		Objects: []sromap.NVMObject{{AssetID: 100, UID: 1, RegionID: 0x0201, X: 11, Y: 20, Z: 30, Type: 0}},
	}
	srv := &Server{Root: root}
	srv.preserveMapOnlyObjectMetadata(1, 2, built)

	if built.Objects[0].Type != 0 {
		t.Fatalf("moved object Type was overwritten: %d", built.Objects[0].Type)
	}
	if len(built.Objects[0].Links) != 0 {
		t.Fatalf("moved object links were preserved: %+v", built.Objects[0].Links)
	}
}
