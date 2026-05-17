package sromap

import "testing"

func TestEncodeMinimalBSR_RoundTrip(t *testing.T) {
	data, err := EncodeMinimalBSR("test_house",
		"prim\\mtrl\\custom\\test\\test.bmt",
		"prim\\mesh\\custom\\test\\test.bms",
		"prim\\mesh\\custom\\test\\test.bms",
		[3]float32{-1, 0, -1}, [3]float32{1, 2, 1})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	bsr, err := DecodeBSR(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if bsr.Name != "test_house" {
		t.Errorf("name: got %q, want test_house", bsr.Name)
	}
	if bsr.CollisionMesh != "prim\\mesh\\custom\\test\\test.bms" {
		t.Errorf("collisionMesh: got %q", bsr.CollisionMesh)
	}
	if bsr.CollisionBBox0Min != [3]float32{-1, 0, -1} || bsr.CollisionBBox0Max != [3]float32{1, 2, 1} {
		t.Errorf("collision box0: got %v..%v", bsr.CollisionBBox0Min, bsr.CollisionBBox0Max)
	}
	if bsr.CollisionBBox1Min[0] < 1e37 || bsr.CollisionBBox1Max[0] > -1e37 {
		t.Errorf("collision box1 should be empty sentinel, got %v..%v", bsr.CollisionBBox1Min, bsr.CollisionBBox1Max)
	}
	if len(bsr.Materials) != 1 || bsr.Materials[0].Path != "prim\\mtrl\\custom\\test\\test.bmt" {
		t.Errorf("materials: got %+v", bsr.Materials)
	}
	if len(bsr.Meshes) != 1 || bsr.Meshes[0] != "prim\\mesh\\custom\\test\\test.bms" {
		t.Errorf("meshes: got %+v", bsr.Meshes)
	}
}
