package editor

import (
	"math"
	"sort"

	"sromapedit/internal/sromap"
)

func customObjectCollisionFootprint(mesh *sromap.OBJMesh) [][2]float32 {
	if mesh == nil || len(mesh.Vertices) == 0 {
		return nil
	}
	minY := mesh.BBoxMin[1]
	height := mesh.BBoxMax[1] - mesh.BBoxMin[1]
	thresholds := []float32{
		maxFloat32(4.0, height*0.03),
		maxFloat32(6.0, height*0.05),
		maxFloat32(8.0, height*0.08),
		maxFloat32(12.0, height*0.12),
	}
	var best [][2]float32
	bestArea := float32(0)
	for _, threshold := range thresholds {
		points := makeGroundPoints(mesh, minY+threshold)
		hull := convexHull2D(points)
		area := float32(math.Abs(float64(polygonArea2D(hull))))
		if len(hull) >= 3 && area > bestArea {
			best = hull
			bestArea = area
		}
	}
	if len(best) >= 3 && bestArea > 1e-3 {
		return best
	}
	return [][2]float32{
		{mesh.BBoxMin[0], mesh.BBoxMin[2]},
		{mesh.BBoxMax[0], mesh.BBoxMin[2]},
		{mesh.BBoxMax[0], mesh.BBoxMax[2]},
		{mesh.BBoxMin[0], mesh.BBoxMax[2]},
	}
}

func customObjectCollisionNav(meta *CustomObjectMeta, mesh *sromap.OBJMesh) ([3]float32, [3]float32, []float32, []uint16, []uint16) {
	footprint := customObjectCollisionFootprint(mesh)
	if len(footprint) < 3 {
		min, max := customObjectCollisionBounds(meta)
		verts, indices, outline := collisionQuadNav(min, max)
		return min, max, verts, indices, outline
	}
	return navFromFootprint(mesh.BBoxMin[1], meta.CollisionOffsetX, meta.CollisionOffsetZ, footprint)
}

func customObjectCollisionBMSMesh(mesh *sromap.OBJMesh, footprint [][2]float32, offsetX, offsetZ float32) ([]sromap.BMSVertex, []uint16, [3]float32, [3]float32) {
	if mesh == nil {
		return nil, nil, [3]float32{}, [3]float32{}
	}
	if len(footprint) < 3 {
		footprint = [][2]float32{
			{mesh.BBoxMin[0], mesh.BBoxMin[2]},
			{mesh.BBoxMax[0], mesh.BBoxMin[2]},
			{mesh.BBoxMax[0], mesh.BBoxMax[2]},
			{mesh.BBoxMin[0], mesh.BBoxMax[2]},
		}
	}

	bottomY := mesh.BBoxMin[1]
	topY := mesh.BBoxMax[1]
	if topY <= bottomY {
		topY = bottomY + 40
	}
	n := len(footprint)
	verts := make([]sromap.BMSVertex, 0, n*2)
	min := [3]float32{float32(math.Inf(1)), bottomY, float32(math.Inf(1))}
	max := [3]float32{float32(math.Inf(-1)), topY, float32(math.Inf(-1))}
	for _, y := range []float32{bottomY, topY} {
		for _, p := range footprint {
			x := p[0] + offsetX
			z := p[1] + offsetZ
			verts = append(verts, sromap.BMSVertex{X: x, Y: y, Z: z, U: 0, V: 0, NX: 0, NY: 1, NZ: 0})
			if x < min[0] {
				min[0] = x
			}
			if x > max[0] {
				max[0] = x
			}
			if z < min[2] {
				min[2] = z
			}
			if z > max[2] {
				max[2] = z
			}
		}
	}

	indices := make([]uint16, 0, (n-2)*6+n*6)
	for i := 1; i < n-1; i++ {
		// Ground-level cap. Some collision paths rasterize actual BMS faces,
		// so keep a floor triangle set in addition to the nav section.
		indices = append(indices, 0, uint16(i), uint16(i+1))
		// Top cap, opposite winding.
		indices = append(indices, uint16(n), uint16(n+i+1), uint16(n+i))
	}
	for i := 0; i < n; i++ {
		j := (i + 1) % n
		bi := uint16(i)
		bj := uint16(j)
		ti := uint16(n + i)
		tj := uint16(n + j)
		indices = append(indices, bi, bj, tj, bi, tj, ti)
	}
	return verts, indices, min, max
}

func navFromFootprint(floorY, offsetX, offsetZ float32, footprint [][2]float32) ([3]float32, [3]float32, []float32, []uint16, []uint16) {
	min := [3]float32{float32(math.Inf(1)), floorY, float32(math.Inf(1))}
	max := [3]float32{float32(math.Inf(-1)), floorY, float32(math.Inf(-1))}
	verts := make([]float32, 0, len(footprint)*3)
	for _, p := range footprint {
		x := p[0] + offsetX
		z := p[1] + offsetZ
		verts = append(verts, x, floorY, z)
		if x < min[0] {
			min[0] = x
		}
		if x > max[0] {
			max[0] = x
		}
		if z < min[2] {
			min[2] = z
		}
		if z > max[2] {
			max[2] = z
		}
	}

	indices := make([]uint16, 0, (len(footprint)-2)*3)
	for i := 1; i < len(footprint)-1; i++ {
		indices = append(indices, 0, uint16(i), uint16(i+1))
	}
	outline := make([]uint16, 0, len(footprint)*2)
	for i := range footprint {
		outline = append(outline, uint16(i), uint16((i+1)%len(footprint)))
	}
	return min, max, verts, indices, outline
}

func makeGroundPoints(mesh *sromap.OBJMesh, maxY float32) [][2]float32 {
	type key struct {
		x int64
		z int64
	}
	seen := make(map[key]bool)
	points := make([][2]float32, 0, len(mesh.Vertices))
	for _, v := range mesh.Vertices {
		if v.Y > maxY {
			continue
		}
		k := key{
			x: int64(math.Round(float64(v.X) * 1000)),
			z: int64(math.Round(float64(v.Z) * 1000)),
		}
		if seen[k] {
			continue
		}
		seen[k] = true
		points = append(points, [2]float32{v.X, v.Z})
	}
	return points
}

func convexHull2D(points [][2]float32) [][2]float32 {
	if len(points) <= 1 {
		return points
	}
	pts := append([][2]float32(nil), points...)
	sort.Slice(pts, func(i, j int) bool {
		if pts[i][0] == pts[j][0] {
			return pts[i][1] < pts[j][1]
		}
		return pts[i][0] < pts[j][0]
	})
	unique := pts[:0]
	for _, p := range pts {
		if len(unique) == 0 || p[0] != unique[len(unique)-1][0] || p[1] != unique[len(unique)-1][1] {
			unique = append(unique, p)
		}
	}
	if len(unique) <= 2 {
		return unique
	}
	pts = unique

	lower := make([][2]float32, 0, len(pts))
	for _, p := range pts {
		for len(lower) >= 2 && cross2D(lower[len(lower)-2], lower[len(lower)-1], p) <= 1e-5 {
			lower = lower[:len(lower)-1]
		}
		lower = append(lower, p)
	}
	upper := make([][2]float32, 0, len(pts))
	for i := len(pts) - 1; i >= 0; i-- {
		p := pts[i]
		for len(upper) >= 2 && cross2D(upper[len(upper)-2], upper[len(upper)-1], p) <= 1e-5 {
			upper = upper[:len(upper)-1]
		}
		upper = append(upper, p)
	}
	hull := append(lower[:len(lower)-1], upper[:len(upper)-1]...)
	if polygonArea2D(hull) < 0 {
		for i, j := 0, len(hull)-1; i < j; i, j = i+1, j-1 {
			hull[i], hull[j] = hull[j], hull[i]
		}
	}
	return hull
}

func cross2D(a, b, c [2]float32) float32 {
	return (b[0]-a[0])*(c[1]-a[1]) - (b[1]-a[1])*(c[0]-a[0])
}

func polygonArea2D(points [][2]float32) float32 {
	var area float32
	for i, p := range points {
		q := points[(i+1)%len(points)]
		area += p[0]*q[1] - q[0]*p[1]
	}
	return area * 0.5
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
