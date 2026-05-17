package sromap

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestO2Roundtrip loads a real .o2 file, saves it back, and verifies the
// bytes are identical. Skips if no .o2 file is available in the working tree.
func TestO2Roundtrip(t *testing.T) {
	candidates := []string{
		"../../../../Map/100/100.o2",
		"../../../../Map/148/92.o2",
	}
	var src string
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			src = c
			break
		}
	}
	if src == "" {
		t.Skip("no .o2 sample available")
	}

	original, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read sample: %v", err)
	}
	o2, err := LoadO2(src)
	if err != nil {
		t.Fatalf("LoadO2: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "out.o2")
	if err := o2.Save(dst); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read saved: %v", err)
	}
	if !bytes.Equal(original, saved) {
		t.Fatalf("round-trip mismatch: original %d bytes, saved %d bytes", len(original), len(saved))
	}
}
