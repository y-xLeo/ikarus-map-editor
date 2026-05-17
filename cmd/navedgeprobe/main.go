package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: navedgeprobe <root> <nvm> <object-index> [edge-index...]")
		os.Exit(2)
	}
	root := os.Args[1]
	nvm, err := sromap.LoadNVM(os.Args[2])
	if err != nil {
		die(err)
	}
	objIdx, err := strconv.Atoi(os.Args[3])
	if err != nil || objIdx < 0 || objIdx >= len(nvm.Objects) {
		die("bad object index")
	}
	wanted := map[int]bool{}
	for _, arg := range os.Args[4:] {
		v, err := strconv.Atoi(arg)
		if err != nil {
			die("bad edge index")
		}
		wanted[v] = true
	}
	info, err := sromap.LoadObjectInfo(sromap.ObjectInfoPath(root))
	if err != nil {
		die(err)
	}
	idx := sromap.NewAssetIndex(root)
	obj := nvm.Objects[objIdx]
	edges, err := outlineEdges(idx, info[obj.AssetID].Path, obj)
	if err != nil {
		die(err)
	}
	fmt.Printf("object[%d] asset=%d uid=%d edges=%d\n", objIdx, obj.AssetID, obj.UID, len(edges))
	for _, e := range edges {
		if len(wanted) > 0 && !wanted[e.index] {
			continue
		}
		fmt.Printf("edge[%03d] (%.3f,%.3f)..(%.3f,%.3f)\n", e.index, e.x0, e.z0, e.x1, e.z1)
	}
}

type edge struct {
	index  int
	x0, z0 float32
	x1, z1 float32
}

func outlineEdges(idx *sromap.AssetIndex, objectPath string, obj sromap.NVMObject) ([]edge, error) {
	bsrPath := idx.Resolve(objectPath)
	if bsrPath == "" {
		return nil, fmt.Errorf("resolve %s", objectPath)
	}
	bsr, err := sromap.LoadBSR(bsrPath)
	if err != nil {
		return nil, err
	}
	meshPath := idx.Resolve(bsr.CollisionMesh)
	if meshPath == "" && len(bsr.Meshes) > 0 {
		meshPath = idx.Resolve(bsr.Meshes[0])
	}
	if meshPath == "" {
		return nil, fmt.Errorf("resolve collision mesh for %s", filepath.Base(bsrPath))
	}
	bms, err := sromap.LoadBMS(meshPath)
	if err != nil {
		return nil, err
	}
	c := float32(math.Cos(float64(obj.Yaw)))
	sn := float32(math.Sin(float64(obj.Yaw)))
	transform := func(v sromap.BMSNavVertex) (float32, float32) {
		return c*v.X - sn*v.Z + obj.X, sn*v.X + c*v.Z + obj.Z
	}
	out := make([]edge, 0, len(bms.NavOutlineEdges))
	for i, oe := range bms.NavOutlineEdges {
		if int(oe.SrcVertex) >= len(bms.NavVertices) || int(oe.DstVertex) >= len(bms.NavVertices) {
			continue
		}
		x0, z0 := transform(bms.NavVertices[oe.SrcVertex])
		x1, z1 := transform(bms.NavVertices[oe.DstVertex])
		out = append(out, edge{index: i, x0: x0, z0: z0, x1: x1, z1: z1})
	}
	return out, nil
}

func die(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
