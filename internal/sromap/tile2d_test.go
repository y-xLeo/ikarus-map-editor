package sromap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTile2DInfo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tile2d.ifo")
	body := "JMXV2DTI1001\r\n" +
		"4\r\n" +
		"00000 0x00000000 \"CJfild\" \"c_dust_fld_01.ddj\"\r\n" +
		"00001 0x0000000a \"CJfild\" \"c_grass_fld_03.ddj\"\r\n" +
		"00007 0x0000000a \"HMfild\" \"c_grass_hmfld_01.ddj\" {757,64}\r\n" +
		"00269 0x00000000 \"Alexandria\" \"alex_dust_01.ddj\"\r\n"
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	tiles, err := LoadTile2DInfo(path)
	if err != nil {
		t.Fatalf("LoadTile2DInfo: %v", err)
	}
	if len(tiles) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(tiles))
	}
	if got := tiles[0].Filename; got != "c_dust_fld_01.ddj" {
		t.Errorf("entry 0 filename = %q", got)
	}
	if got := tiles[1].TileType; got != 10 {
		t.Errorf("entry 1 tileType = %d", got)
	}
	if got := tiles[7].Grass; got != "{757,64}" {
		t.Errorf("entry 7 grass = %q", got)
	}
	if got := tiles[269].Folder; got != "Alexandria" {
		t.Errorf("entry 269 folder = %q", got)
	}
}

func TestLoadTile2DInfoBadSignature(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tile2d.ifo")
	if err := os.WriteFile(path, []byte("NOT_A_TILE2D_FILE\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTile2DInfo(path); err == nil {
		t.Fatal("expected error for bad signature, got nil")
	}
}
