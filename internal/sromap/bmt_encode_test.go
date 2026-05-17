package sromap

import "testing"

func TestEncodeMinimalBMT_RoundTrip(t *testing.T) {
	data, err := EncodeMinimalBMT("matA", "res\\custom\\house\\diffuse.ddj", false)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	bmt, err := DecodeBMT(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(bmt.Materials) != 1 {
		t.Fatalf("material count: got %d, want 1", len(bmt.Materials))
	}
	m := bmt.Materials[0]
	if m.Name != "matA" {
		t.Errorf("name: got %q, want matA", m.Name)
	}
	if m.TextureFile != "res\\custom\\house\\diffuse.ddj" {
		t.Errorf("texFile: got %q", m.TextureFile)
	}
	if m.IsAbsolutePath {
		t.Errorf("IsAbsolutePath: got true, want false")
	}
}
