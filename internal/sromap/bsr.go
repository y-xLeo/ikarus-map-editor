package sromap

import (
	"fmt"
	"os"
)

const (
	bsrSignature = "JMXVRES 0109"
)

type BSRMaterialRef struct {
	Type uint32
	Path string
}

type BSR struct {
	Path               string
	Name               string
	Type               uint16
	CollisionMesh      string
	CollisionBBox0Min  [3]float32
	CollisionBBox0Max  [3]float32
	CollisionBBox1Min  [3]float32
	CollisionBBox1Max  [3]float32
	HasCollisionMatrix bool
	CollisionMatrix    [16]float32
	Materials          []BSRMaterialRef
	Meshes             []string
}

func LoadBSR(path string) (*BSR, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b, err := DecodeBSR(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	b.Path = path
	return b, nil
}

func DecodeBSR(data []byte) (*BSR, error) {
	if len(data) < 12 || string(data[:12]) != bsrSignature {
		return nil, fmt.Errorf("bsr: bad signature")
	}
	r := NewBinReader(data)
	if err := r.Skip(12); err != nil {
		return nil, err
	}
	// 8 × uint32 file offsets
	if err := r.Skip(8 * 4); err != nil {
		return nil, err
	}
	meshFlags, err := r.U32()
	if err != nil {
		return nil, err
	}
	if err := r.Skip(4 * 4); err != nil {
		return nil, err
	}

	objType, err := r.U16()
	if err != nil {
		return nil, err
	}
	if err := r.Skip(2); err != nil {
		return nil, err
	}
	name, err := r.LenString()
	if err != nil {
		return nil, err
	}
	if err := r.Skip(4 * 2); err != nil {
		return nil, err
	}
	if err := r.Skip(40); err != nil {
		return nil, err
	}

	collisionMesh, err := r.LenString()
	if err != nil {
		return nil, err
	}
	// 2 × bbox (3 × float32) = 24 bytes
	collisionBBox0Min, collisionBBox0Max, err := readBSRBBox(r)
	if err != nil {
		return nil, err
	}
	collisionBBox1Min, collisionBBox1Max, err := readBSRBBox(r)
	if err != nil {
		return nil, err
	}
	hasMatrix, err := r.U32()
	if err != nil {
		return nil, err
	}
	var collisionMatrix [16]float32
	if hasMatrix != 0 {
		for i := range collisionMatrix {
			if collisionMatrix[i], err = r.F32(); err != nil {
				return nil, err
			}
		}
	}

	matCount, err := r.U32()
	if err != nil {
		return nil, err
	}
	mats := make([]BSRMaterialRef, 0, matCount)
	for i := uint32(0); i < matCount; i++ {
		t, err := r.U32()
		if err != nil {
			return nil, err
		}
		p, err := r.LenString()
		if err != nil {
			return nil, err
		}
		mats = append(mats, BSRMaterialRef{Type: t, Path: p})
	}

	meshCount, err := r.U32()
	if err != nil {
		return nil, err
	}
	meshes := make([]string, 0, meshCount)
	for i := uint32(0); i < meshCount; i++ {
		p, err := r.LenString()
		if err != nil {
			return nil, err
		}
		if meshFlags&1 != 0 {
			if err := r.Skip(4); err != nil {
				return nil, err
			}
		}
		meshes = append(meshes, p)
	}

	return &BSR{
		Name:               name,
		Type:               objType,
		CollisionMesh:      collisionMesh,
		CollisionBBox0Min:  collisionBBox0Min,
		CollisionBBox0Max:  collisionBBox0Max,
		CollisionBBox1Min:  collisionBBox1Min,
		CollisionBBox1Max:  collisionBBox1Max,
		HasCollisionMatrix: hasMatrix != 0,
		CollisionMatrix:    collisionMatrix,
		Materials:          mats,
		Meshes:             meshes,
	}, nil
}

func readBSRBBox(r *BinReader) ([3]float32, [3]float32, error) {
	var min, max [3]float32
	for i := 0; i < 3; i++ {
		v, err := r.F32()
		if err != nil {
			return min, max, err
		}
		min[i] = v
	}
	for i := 0; i < 3; i++ {
		v, err := r.F32()
		if err != nil {
			return min, max, err
		}
		max[i] = v
	}
	return min, max, nil
}
