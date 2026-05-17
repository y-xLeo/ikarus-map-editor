package sromap

import (
	"math"
	"sort"
)

// Triangle stores three world-space corners.
type Triangle struct {
	A, B, C [3]float32
}

type bvhNode struct {
	Min, Max [3]float32
	// For an internal node: Left/Right are node indices, TriCount = 0.
	// For a leaf: Left = TriStart, TriCount > 0.
	Left, Right int32
	TriCount    int32
}

// BVH is a simple axis-aligned bounding-volume hierarchy of triangles built
// by recursive median split. Designed for ANY-hit shadow rays (early exit).
type BVH struct {
	triangles []Triangle
	order     []int32
	nodes     []bvhNode
}

func (b *BVH) Empty() bool { return len(b.nodes) == 0 }

func BuildBVH(triangles []Triangle) *BVH {
	n := len(triangles)
	bvh := &BVH{triangles: triangles, order: make([]int32, n)}
	if n == 0 {
		return bvh
	}
	for i := range bvh.order {
		bvh.order[i] = int32(i)
	}
	bvh.nodes = make([]bvhNode, 0, 2*n)
	bvh.buildRange(0, n)
	return bvh
}

const bvhLeafSize = 8

func (b *BVH) buildRange(start, end int) int32 {
	idx := int32(len(b.nodes))
	b.nodes = append(b.nodes, bvhNode{})
	bmin, bmax := b.boundsFor(start, end)
	b.nodes[idx].Min = bmin
	b.nodes[idx].Max = bmax

	count := end - start
	if count <= bvhLeafSize {
		b.nodes[idx].Left = int32(start)
		b.nodes[idx].Right = 0
		b.nodes[idx].TriCount = int32(count)
		return idx
	}
	// Centroid bounds → pick longest axis → median split.
	cmin, cmax := b.centroidBounds(start, end)
	axis := 0
	if cmax[1]-cmin[1] > cmax[axis]-cmin[axis] {
		axis = 1
	}
	if cmax[2]-cmin[2] > cmax[axis]-cmin[axis] {
		axis = 2
	}
	slice := b.order[start:end]
	sort.Slice(slice, func(i, j int) bool {
		return triCentroid(b.triangles[slice[i]])[axis] < triCentroid(b.triangles[slice[j]])[axis]
	})
	mid := start + count/2
	left := b.buildRange(start, mid)
	right := b.buildRange(mid, end)
	b.nodes[idx].Left = left
	b.nodes[idx].Right = right
	b.nodes[idx].TriCount = 0
	return idx
}

func (b *BVH) boundsFor(start, end int) ([3]float32, [3]float32) {
	const inf = float32(math.MaxFloat32)
	bmin := [3]float32{inf, inf, inf}
	bmax := [3]float32{-inf, -inf, -inf}
	for i := start; i < end; i++ {
		t := b.triangles[b.order[i]]
		for _, v := range [3][3]float32{t.A, t.B, t.C} {
			for k := 0; k < 3; k++ {
				if v[k] < bmin[k] {
					bmin[k] = v[k]
				}
				if v[k] > bmax[k] {
					bmax[k] = v[k]
				}
			}
		}
	}
	return bmin, bmax
}

func (b *BVH) centroidBounds(start, end int) ([3]float32, [3]float32) {
	const inf = float32(math.MaxFloat32)
	cmin := [3]float32{inf, inf, inf}
	cmax := [3]float32{-inf, -inf, -inf}
	for i := start; i < end; i++ {
		c := triCentroid(b.triangles[b.order[i]])
		for k := 0; k < 3; k++ {
			if c[k] < cmin[k] {
				cmin[k] = c[k]
			}
			if c[k] > cmax[k] {
				cmax[k] = c[k]
			}
		}
	}
	return cmin, cmax
}

func triCentroid(t Triangle) [3]float32 {
	return [3]float32{
		(t.A[0] + t.B[0] + t.C[0]) / 3,
		(t.A[1] + t.B[1] + t.C[1]) / 3,
		(t.A[2] + t.B[2] + t.C[2]) / 3,
	}
}

// AnyHit returns true if a ray from ro in direction rd (need not be normalized)
// hits any triangle at distance > epsilon and ≤ maxT.
func (b *BVH) AnyHit(ro, rd [3]float32, maxT float32) bool {
	if b.Empty() {
		return false
	}
	var invD [3]float32
	for i := 0; i < 3; i++ {
		if rd[i] != 0 {
			invD[i] = 1.0 / rd[i]
		} else {
			invD[i] = float32(math.Inf(1))
		}
	}
	var stack [64]int32
	stack[0] = 0
	sp := 1
	for sp > 0 {
		sp--
		nodeIdx := stack[sp]
		node := &b.nodes[nodeIdx]
		if !rayAABBPrecomp(ro, invD, node.Min, node.Max, maxT) {
			continue
		}
		if node.TriCount > 0 {
			end := node.Left + node.TriCount
			for i := node.Left; i < end; i++ {
				t := b.triangles[b.order[i]]
				if rayTriangleHit(ro, rd, t.A, t.B, t.C, maxT) {
					return true
				}
			}
		} else if sp+1 < len(stack) {
			stack[sp] = node.Right
			stack[sp+1] = node.Left
			sp += 2
		}
	}
	return false
}

func rayAABBPrecomp(ro, invD, bmin, bmax [3]float32, maxT float32) bool {
	tmin := float32(0)
	tmax := maxT
	for i := 0; i < 3; i++ {
		t0 := (bmin[i] - ro[i]) * invD[i]
		t1 := (bmax[i] - ro[i]) * invD[i]
		if t0 > t1 {
			t0, t1 = t1, t0
		}
		if t0 > tmin {
			tmin = t0
		}
		if t1 < tmax {
			tmax = t1
		}
		if tmin > tmax {
			return false
		}
	}
	return tmax >= 0
}

// rayTriangleHit returns true if the ray hits the triangle at 0 < t ≤ maxT.
// Möller–Trumbore, single-sided culling disabled (hits either face).
func rayTriangleHit(ro, rd, v0, v1, v2 [3]float32, maxT float32) bool {
	e1 := [3]float32{v1[0] - v0[0], v1[1] - v0[1], v1[2] - v0[2]}
	e2 := [3]float32{v2[0] - v0[0], v2[1] - v0[1], v2[2] - v0[2]}
	p := [3]float32{
		rd[1]*e2[2] - rd[2]*e2[1],
		rd[2]*e2[0] - rd[0]*e2[2],
		rd[0]*e2[1] - rd[1]*e2[0],
	}
	det := e1[0]*p[0] + e1[1]*p[1] + e1[2]*p[2]
	if det > -1e-6 && det < 1e-6 {
		return false
	}
	invDet := 1.0 / det
	s := [3]float32{ro[0] - v0[0], ro[1] - v0[1], ro[2] - v0[2]}
	u := (s[0]*p[0] + s[1]*p[1] + s[2]*p[2]) * invDet
	if u < 0 || u > 1 {
		return false
	}
	q := [3]float32{
		s[1]*e1[2] - s[2]*e1[1],
		s[2]*e1[0] - s[0]*e1[2],
		s[0]*e1[1] - s[1]*e1[0],
	}
	v := (rd[0]*q[0] + rd[1]*q[1] + rd[2]*q[2]) * invDet
	if v < 0 || u+v > 1 {
		return false
	}
	t := (e2[0]*q[0] + e2[1]*q[1] + e2[2]*q[2]) * invDet
	return t > 1e-4 && t <= maxT
}
