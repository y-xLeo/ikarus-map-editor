package sromap

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFlatNVMRoundTrip writes a freshly-generated flat NVM and re-parses it
// to verify our encoder produces something our decoder can read. Stricter
// than a struct comparison: every byte goes through Save → LoadNVM exactly
// like the server emulator's parser would.
func TestFlatNVMRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nv_0001.nvm")
	n := NewFlatNVM(123.5)
	if err := n.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadNVM(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.OpenCellCount != 1 || len(got.Cells) != 1 {
		t.Fatalf("cells: got count=%d open=%d, want 1/1",
			len(got.Cells), got.OpenCellCount)
	}
	c := got.Cells[0]
	if c.MinX != 0 || c.MinZ != 0 || c.MaxX != float32(NVMTileCount*NVMTileSize) || c.MaxZ != float32(NVMTileCount*NVMTileSize) {
		t.Errorf("cell bounds: got (%g..%g, %g..%g), want full region",
			c.MinX, c.MaxX, c.MinZ, c.MaxZ)
	}
	if len(got.GlobalEdges) != 0 || len(got.InternalEdges) != 0 {
		t.Errorf("edges should be empty: got %d global, %d internal",
			len(got.GlobalEdges), len(got.InternalEdges))
	}
	for i := range got.Tiles {
		if got.Tiles[i].CellID != 0 || got.Tiles[i].Flag != 0 {
			t.Fatalf("tile %d: cellID=%d flag=%d, want 0/0",
				i, got.Tiles[i].CellID, got.Tiles[i].Flag)
		}
	}
	for i, h := range got.Heights {
		if h != 123.5 {
			t.Fatalf("height %d: got %g, want 123.5", i, h)
		}
	}
	info, _ := os.Stat(path)
	t.Logf("flat NVM size: %d bytes", info.Size())
}
