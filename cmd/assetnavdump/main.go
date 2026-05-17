package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: assetnavdump <root> <objID> [objID...]")
		os.Exit(2)
	}
	root := os.Args[1]
	infos, err := sromap.LoadObjectInfo(sromap.ObjectInfoPath(root))
	if err != nil {
		fmt.Fprintf(os.Stderr, "object.ifo: %v\n", err)
		os.Exit(1)
	}
	idx := sromap.NewAssetIndex(root)
	for _, arg := range os.Args[2:] {
		id64, err := strconv.ParseUint(arg, 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad objID %q\n", arg)
			continue
		}
		if err := dumpObject(root, idx, infos, uint32(id64)); err != nil {
			fmt.Printf("obj %d: ERROR: %v\n", id64, err)
		}
		fmt.Println()
	}
}

func dumpObject(root string, idx *sromap.AssetIndex, infos map[uint32]sromap.ObjectInfo, id uint32) error {
	info, ok := infos[id]
	if !ok {
		return fmt.Errorf("not present in %s", sromap.ObjectInfoPath(root))
	}
	fmt.Printf("obj %d\n", id)
	fmt.Printf("  object.ifo: flags=0x%08x path=%s\n", info.Flags, info.Path)
	resolved := idx.Resolve(info.Path)
	if resolved == "" {
		return fmt.Errorf("cannot resolve %s", info.Path)
	}
	if !info.IsCPD {
		return dumpBSR(idx, resolved)
	}
	cpd, err := sromap.LoadCPD(resolved)
	if err != nil {
		return err
	}
	fmt.Printf("  CPD: %s resources=%d collision=%q\n", rel(root, resolved), len(cpd.Resources), cpd.CollisionPath)
	for _, res := range cpd.Resources {
		if strings.HasSuffix(strings.ToLower(res), ".bsr") {
			if p := idx.Resolve(res); p != "" {
				if err := dumpBSR(idx, p); err != nil {
					fmt.Printf("  BSR %s: %v\n", res, err)
				}
			}
		}
	}
	return nil
}

func dumpBSR(idx *sromap.AssetIndex, bsrPath string) error {
	bsr, err := sromap.LoadBSR(bsrPath)
	if err != nil {
		return err
	}
	fmt.Printf("  BSR: %s name=%q type=%d meshes=%d collision=%q\n",
		rel(idx.Root, bsrPath), bsr.Name, bsr.Type, len(bsr.Meshes), bsr.CollisionMesh)
	fmt.Printf("  BSR collision boxes: box0=%s..%s box1=%s..%s matrix=%v\n",
		vec3(bsr.CollisionBBox0Min), vec3(bsr.CollisionBBox0Max),
		vec3(bsr.CollisionBBox1Min), vec3(bsr.CollisionBBox1Max), bsr.HasCollisionMatrix)
	if bsr.HasCollisionMatrix {
		fmt.Printf("    collision matrix rows: [%.3f %.3f %.3f %.3f] [%.3f %.3f %.3f %.3f] [%.3f %.3f %.3f %.3f] [%.3f %.3f %.3f %.3f]\n",
			bsr.CollisionMatrix[0], bsr.CollisionMatrix[1], bsr.CollisionMatrix[2], bsr.CollisionMatrix[3],
			bsr.CollisionMatrix[4], bsr.CollisionMatrix[5], bsr.CollisionMatrix[6], bsr.CollisionMatrix[7],
			bsr.CollisionMatrix[8], bsr.CollisionMatrix[9], bsr.CollisionMatrix[10], bsr.CollisionMatrix[11],
			bsr.CollisionMatrix[12], bsr.CollisionMatrix[13], bsr.CollisionMatrix[14], bsr.CollisionMatrix[15])
	}

	visualMin, visualMax := emptyBounds()
	visualCount := 0
	for _, meshRel := range bsr.Meshes {
		meshPath := idx.Resolve(meshRel)
		if meshPath == "" {
			fmt.Printf("    mesh: %s unresolved\n", meshRel)
			continue
		}
		bms, err := sromap.LoadBMS(meshPath)
		if err != nil {
			fmt.Printf("    mesh: %s %v\n", meshRel, err)
			continue
		}
		min, max := vertexBounds(bms)
		visualMin, visualMax = unionBounds(visualMin, visualMax, min, max)
		visualCount++
		fmt.Printf("    mesh: %s verts=%d faces=%d vertexBBox=%s..%s fileBBox=%s..%s\n",
			rel(idx.Root, meshPath), len(bms.Vertices), len(bms.Indices)/3, vec3(min), vec3(max), vec3(bms.BBoxMin), vec3(bms.BBoxMax))
	}
	if visualCount > 0 {
		fmt.Printf("  visual combined: %s..%s center=%s size=%s\n",
			vec3(visualMin), vec3(visualMax), vec3(center3(visualMin, visualMax)), vec3(size3(visualMin, visualMax)))
	}

	if bsr.CollisionMesh == "" {
		fmt.Println("  collision: none")
		return nil
	}
	collisionPath := idx.Resolve(bsr.CollisionMesh)
	if collisionPath == "" {
		fmt.Printf("  collision: %s unresolved\n", bsr.CollisionMesh)
		return nil
	}
	collision, err := sromap.LoadBMS(collisionPath)
	if err != nil {
		return fmt.Errorf("collision BMS: %w", err)
	}
	cMin, cMax := vertexBounds(collision)
	fmt.Printf("  collision BMS: %s verts=%d faces=%d vertexBBox=%s..%s fileBBox=%s..%s\n",
		rel(idx.Root, collisionPath), len(collision.Vertices), len(collision.Indices)/3, vec3(cMin), vec3(cMax), vec3(collision.BBoxMin), vec3(collision.BBoxMax))
	if collision.HasNavMesh {
		fmt.Printf("  navmesh: verts=%d cells=%d outline=%d inline=%d navBBox=%s..%s center=%s size=%s lookup=(%.2f, %.2f) %dx%d\n",
			len(collision.NavVertices), len(collision.NavCells), len(collision.NavOutlineEdges), len(collision.NavInlineEdges),
			vec3(collision.NavBBoxMin), vec3(collision.NavBBoxMax), vec3(center3(collision.NavBBoxMin, collision.NavBBoxMax)),
			vec3(size3(collision.NavBBoxMin, collision.NavBBoxMax)), collision.NavLookupOrigin[0], collision.NavLookupOrigin[1],
			collision.NavLookupWidth, collision.NavLookupHeight)
		if len(collision.NavCells) > 0 || len(collision.NavOutlineEdges) > 0 || len(collision.NavInlineEdges) > 0 {
			fmt.Printf("  nav flags: cells=%s outline=%s inline=%s\n",
				firstCellFlags(collision.NavCells), firstEdgeFlags(collision.NavOutlineEdges), firstEdgeFlags(collision.NavInlineEdges))
		}
		if visualCount > 0 {
			vCenter := center3(visualMin, visualMax)
			nCenter := center3(collision.NavBBoxMin, collision.NavBBoxMax)
			fmt.Printf("  nav-vs-visual center delta: %s\n", vec3([3]float32{nCenter[0] - vCenter[0], nCenter[1] - vCenter[1], nCenter[2] - vCenter[2]}))
		}
	} else {
		fmt.Println("  navmesh: none")
	}
	return nil
}

func vertexBounds(bms *sromap.BMS) ([3]float32, [3]float32) {
	if len(bms.Vertices) == 0 {
		return bms.BBoxMin, bms.BBoxMax
	}
	min := [3]float32{bms.Vertices[0].X, bms.Vertices[0].Y, bms.Vertices[0].Z}
	max := min
	for _, v := range bms.Vertices[1:] {
		p := [3]float32{v.X, v.Y, v.Z}
		for i := 0; i < 3; i++ {
			if p[i] < min[i] {
				min[i] = p[i]
			}
			if p[i] > max[i] {
				max[i] = p[i]
			}
		}
	}
	return min, max
}

func emptyBounds() ([3]float32, [3]float32) {
	return [3]float32{float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(1))},
		[3]float32{float32(math.Inf(-1)), float32(math.Inf(-1)), float32(math.Inf(-1))}
}

func unionBounds(aMin, aMax, bMin, bMax [3]float32) ([3]float32, [3]float32) {
	for i := 0; i < 3; i++ {
		if bMin[i] < aMin[i] {
			aMin[i] = bMin[i]
		}
		if bMax[i] > aMax[i] {
			aMax[i] = bMax[i]
		}
	}
	return aMin, aMax
}

func center3(min, max [3]float32) [3]float32 {
	return [3]float32{(min[0] + max[0]) / 2, (min[1] + max[1]) / 2, (min[2] + max[2]) / 2}
}

func size3(min, max [3]float32) [3]float32 {
	return [3]float32{max[0] - min[0], max[1] - min[1], max[2] - min[2]}
}

func vec3(v [3]float32) string {
	return fmt.Sprintf("(%.2f, %.2f, %.2f)", v[0], v[1], v[2])
}

func firstCellFlags(cells []sromap.BMSNavCell) string {
	n := len(cells)
	if n > 8 {
		n = 8
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("0x%04x", cells[i].Flag))
	}
	if len(cells) > n {
		out = append(out, "...")
	}
	return strings.Join(out, ",")
}

func firstEdgeFlags(edges []sromap.BMSNavEdge) string {
	n := len(edges)
	if n > 8 {
		n = 8
	}
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, fmt.Sprintf("0x%02x", edges[i].Flag))
	}
	if len(edges) > n {
		out = append(out, "...")
	}
	return strings.Join(out, ",")
}

func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(p)
}
