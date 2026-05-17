package sromap

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNVMRoundtrip(t *testing.T) {
	candidates := []string{
		"../../../../changed/Data/Navmesh/nv_4ab6.nvm",
		"../../../../SR_GameServer/Data/navmesh/nv_4ab6.nvm",
		"../../../../Data/Navmesh/nv_4ab6.nvm",
	}
	var src string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			src = c
			break
		}
	}
	if src == "" {
		t.Skip("no NVM sample available")
	}

	original, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	nvm, err := LoadNVM(src)
	if err != nil {
		t.Fatalf("LoadNVM: %v", err)
	}
	dst := filepath.Join(t.TempDir(), "out.nvm")
	if err := nvm.Save(dst); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, saved) {
		t.Fatalf("NVM round-trip mismatch: original=%d saved=%d", len(original), len(saved))
	}
}
