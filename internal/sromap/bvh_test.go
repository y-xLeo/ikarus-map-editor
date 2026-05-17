package sromap

import "testing"

func TestBVHHitMiss(t *testing.T) {
	// Single triangle at z=0
	tris := []Triangle{
		{A: [3]float32{-1, 0, 0}, B: [3]float32{1, 0, 0}, C: [3]float32{0, 2, 0}},
	}
	bvh := BuildBVH(tris)

	// Ray straight at the triangle from (0, 1, -5) going +Z
	if !bvh.AnyHit([3]float32{0, 1, -5}, [3]float32{0, 0, 1}, 100) {
		t.Errorf("expected hit on ray through triangle")
	}
	// Ray missing the triangle (to the side)
	if bvh.AnyHit([3]float32{10, 1, -5}, [3]float32{0, 0, 1}, 100) {
		t.Errorf("expected miss off to the side")
	}
	// Ray facing away from the triangle
	if bvh.AnyHit([3]float32{0, 1, 5}, [3]float32{0, 0, 1}, 100) {
		t.Errorf("expected miss for ray pointing away")
	}
}

func TestBVHManyTriangles(t *testing.T) {
	// Build a grid of small triangles
	tris := make([]Triangle, 0, 100)
	for ix := 0; ix < 10; ix++ {
		for iz := 0; iz < 10; iz++ {
			x := float32(ix * 10)
			z := float32(iz * 10)
			tris = append(tris, Triangle{
				A: [3]float32{x, 0, z},
				B: [3]float32{x + 1, 0, z},
				C: [3]float32{x, 0, z + 1},
			})
		}
	}
	bvh := BuildBVH(tris)
	// Ray down through one specific triangle area
	if !bvh.AnyHit([3]float32{50.2, 10, 50.2}, [3]float32{0, -1, 0}, 100) {
		t.Errorf("expected hit straight down on grid")
	}
	if bvh.AnyHit([3]float32{200, 10, 200}, [3]float32{0, -1, 0}, 100) {
		t.Errorf("expected miss outside grid")
	}
}
