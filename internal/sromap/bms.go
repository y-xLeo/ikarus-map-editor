package sromap

import (
	"fmt"
	"math"
	"os"
)

const bmsSignature = "JMXVBMS 0110"

type BMSVertex struct {
	X, Y, Z    float32
	U, V       float32
	NX, NY, NZ float32 // unit normal; written into the bytes BMS reserves at +12 after XYZ
}

type BMSNavVertex struct {
	X, Y, Z       float32
	BisectorIndex byte
}

type BMSNavCell struct {
	V0, V1, V2    uint16
	Flag          uint16
	EventZoneData byte
}

type BMSNavEdge struct {
	SrcVertex     uint16
	DstVertex     uint16
	SrcCell       uint16
	DstCell       uint16
	Flag          byte
	EventZoneData byte
}

type BMS struct {
	Path            string
	Name            string
	Material        string
	Vertices        []BMSVertex
	Indices         []uint16
	BBoxMin         [3]float32
	BBoxMax         [3]float32
	HasNavMesh      bool
	NavBBoxMin      [3]float32
	NavBBoxMax      [3]float32
	NavVertices     []BMSNavVertex
	NavCells        []BMSNavCell
	NavOutlineEdges []BMSNavEdge
	NavInlineEdges  []BMSNavEdge
	NavLookupOrigin [2]float32
	NavLookupWidth  uint32
	NavLookupHeight uint32
}

func LoadBMS(path string) (*BMS, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	bms, err := DecodeBMS(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	bms.Path = path
	return bms, nil
}

func DecodeBMS(data []byte) (*BMS, error) {
	if len(data) < 12 || string(data[:12]) != bmsSignature {
		return nil, fmt.Errorf("bms: bad signature")
	}
	r := NewBinReader(data)
	if err := r.Skip(12); err != nil {
		return nil, err
	}

	// 7 × uint32 file offsets (vertices, vertexGroups, faces, vertexClothes, edgeClothes, bbox, occlusion)
	if err := r.Skip(28); err != nil {
		return nil, err
	}
	offNavMesh, err := r.U32()
	if err != nil {
		return nil, err
	}
	// skinnedNavMesh + unknown09 offsets
	if err := r.Skip(8); err != nil {
		return nil, err
	}
	// unkUInt0
	if err := r.Skip(4); err != nil {
		return nil, err
	}
	navFlag, err := r.U32()
	if err != nil {
		return nil, err
	}
	// subPrimCount
	if err := r.Skip(4); err != nil {
		return nil, err
	}
	vertexFlag, err := r.U32()
	if err != nil {
		return nil, err
	}
	// unkUInt2
	if err := r.Skip(4); err != nil {
		return nil, err
	}

	name, err := r.LenString()
	if err != nil {
		return nil, err
	}
	material, err := r.LenString()
	if err != nil {
		return nil, err
	}
	if err := r.Skip(4); err != nil {
		return nil, err
	}

	vertexCount, err := r.U32()
	if err != nil {
		return nil, err
	}
	if vertexCount > 1<<20 {
		return nil, fmt.Errorf("bms: implausible vertex count %d", vertexCount)
	}
	vertices := make([]BMSVertex, vertexCount)
	for i := range vertices {
		x, err := r.F32()
		if err != nil {
			return nil, err
		}
		y, err := r.F32()
		if err != nil {
			return nil, err
		}
		z, err := r.F32()
		if err != nil {
			return nil, err
		}
		if err := r.Skip(12); err != nil {
			return nil, err
		}
		u, err := r.F32()
		if err != nil {
			return nil, err
		}
		v, err := r.F32()
		if err != nil {
			return nil, err
		}
		if vertexFlag&0x400 != 0 {
			if err := r.Skip(8); err != nil {
				return nil, err
			}
		}
		if vertexFlag&0x800 != 0 {
			if err := r.Skip(40); err != nil {
				return nil, err
			}
			u2, err := r.F32()
			if err != nil {
				return nil, err
			}
			v2, err := r.F32()
			if err != nil {
				return nil, err
			}
			u, v = u2, v2
		} else {
			if err := r.Skip(12); err != nil {
				return nil, err
			}
		}
		vertices[i] = BMSVertex{X: x, Y: y, Z: z, U: u, V: 1 - v}
	}
	if vertexFlag&0x400 != 0 {
		if _, err := r.LenString(); err != nil {
			return nil, err
		}
	}
	if vertexFlag&0x1000 != 0 {
		count, err := r.U32()
		if err != nil {
			return nil, err
		}
		if err := r.Skip(int(count) * 24); err != nil {
			return nil, err
		}
	}

	vgCount, err := r.U32()
	if err != nil {
		return nil, err
	}
	if vgCount > 0 {
		for i := uint32(0); i < vgCount; i++ {
			if _, err := r.LenString(); err != nil {
				return nil, err
			}
		}
		if err := r.Skip(int(vertexCount) * 6); err != nil {
			return nil, err
		}
	}

	faceCount, err := r.U32()
	if err != nil {
		return nil, err
	}
	if faceCount > 1<<22 {
		return nil, fmt.Errorf("bms: implausible face count %d", faceCount)
	}
	indices := make([]uint16, 0, faceCount*3)
	for i := uint32(0); i < faceCount; i++ {
		a, err := r.U16()
		if err != nil {
			return nil, err
		}
		b, err := r.U16()
		if err != nil {
			return nil, err
		}
		c, err := r.U16()
		if err != nil {
			return nil, err
		}
		indices = append(indices, a, b, c)
	}

	bms := &BMS{
		Name:     name,
		Material: material,
		Vertices: vertices,
		Indices:  indices,
	}

	if err := readBMSBounds(r, bms); err != nil {
		return bms, nil
	}
	_ = readBMSNav(data, offNavMesh, navFlag, bms)
	return bms, nil
}

func readBMSNav(data []byte, offNavMesh, navFlag uint32, bms *BMS) error {
	if offNavMesh == 0 || int(offNavMesh) >= len(data) {
		return nil
	}
	r := NewBinReader(data)
	if err := r.Seek(int(offNavMesh)); err != nil {
		return err
	}
	count, err := r.U32()
	if err != nil {
		return err
	}
	if count == 0 || count > 1<<20 {
		return nil
	}
	vertices := make([]BMSNavVertex, count)
	min := [3]float32{float32(math.Inf(1)), float32(math.Inf(1)), float32(math.Inf(1))}
	max := [3]float32{float32(math.Inf(-1)), float32(math.Inf(-1)), float32(math.Inf(-1))}
	for i := uint32(0); i < count; i++ {
		x, err := r.F32()
		if err != nil {
			return err
		}
		y, err := r.F32()
		if err != nil {
			return err
		}
		z, err := r.F32()
		if err != nil {
			return err
		}
		bisector, err := r.U8()
		if err != nil {
			return err
		}
		p := [3]float32{x, y, z}
		for j := 0; j < 3; j++ {
			if p[j] < min[j] {
				min[j] = p[j]
			}
			if p[j] > max[j] {
				max[j] = p[j]
			}
		}
		vertices[i] = BMSNavVertex{X: x, Y: y, Z: z, BisectorIndex: bisector}
	}

	cellCount, err := r.U32()
	if err != nil {
		return err
	}
	if cellCount > 1<<20 {
		return nil
	}
	cells := make([]BMSNavCell, cellCount)
	for i := uint32(0); i < cellCount; i++ {
		v0, err := r.U16()
		if err != nil {
			return err
		}
		v1, err := r.U16()
		if err != nil {
			return err
		}
		v2, err := r.U16()
		if err != nil {
			return err
		}
		flag, err := r.U16()
		if err != nil {
			return err
		}
		cell := BMSNavCell{V0: v0, V1: v1, V2: v2, Flag: flag}
		if navFlag&2 != 0 {
			ev, err := r.U8()
			if err != nil {
				return err
			}
			cell.EventZoneData = ev
		}
		cells[i] = cell
	}

	outlineEdges, err := readBMSNavEdges(r, navFlag)
	if err != nil {
		return err
	}
	inlineEdges, err := readBMSNavEdges(r, navFlag)
	if err != nil {
		return err
	}

	if navFlag&4 != 0 {
		eventCount, err := r.U32()
		if err != nil {
			return err
		}
		if eventCount > 1<<16 {
			return nil
		}
		for i := uint32(0); i < eventCount; i++ {
			if _, err := r.LenString(); err != nil {
				return err
			}
		}
	}

	originX, err := r.F32()
	if err != nil {
		return err
	}
	originZ, err := r.F32()
	if err != nil {
		return err
	}
	width, err := r.U32()
	if err != nil {
		return err
	}
	height, err := r.U32()
	if err != nil {
		return err
	}
	gridCellCount, err := r.U32()
	if err != nil {
		return err
	}
	if gridCellCount > 1<<20 {
		return nil
	}
	for i := uint32(0); i < gridCellCount; i++ {
		outlineCount, err := r.U32()
		if err != nil {
			return err
		}
		if outlineCount > 1<<20 {
			return nil
		}
		if err := r.Skip(int(outlineCount) * 2); err != nil {
			return err
		}
	}
	bms.HasNavMesh = true
	bms.NavBBoxMin = min
	bms.NavBBoxMax = max
	bms.NavVertices = vertices
	bms.NavCells = cells
	bms.NavOutlineEdges = outlineEdges
	bms.NavInlineEdges = inlineEdges
	bms.NavLookupOrigin = [2]float32{originX, originZ}
	bms.NavLookupWidth = width
	bms.NavLookupHeight = height
	return nil
}

func readBMSNavEdges(r *BinReader, navFlag uint32) ([]BMSNavEdge, error) {
	count, err := r.U32()
	if err != nil {
		return nil, err
	}
	if count > 1<<20 {
		return nil, fmt.Errorf("bms: implausible nav edge count %d", count)
	}
	edges := make([]BMSNavEdge, count)
	for i := uint32(0); i < count; i++ {
		srcVertex, err := r.U16()
		if err != nil {
			return nil, err
		}
		dstVertex, err := r.U16()
		if err != nil {
			return nil, err
		}
		srcCell, err := r.U16()
		if err != nil {
			return nil, err
		}
		dstCell, err := r.U16()
		if err != nil {
			return nil, err
		}
		flag, err := r.U8()
		if err != nil {
			return nil, err
		}
		edge := BMSNavEdge{
			SrcVertex: srcVertex,
			DstVertex: dstVertex,
			SrcCell:   srcCell,
			DstCell:   dstCell,
			Flag:      flag,
		}
		if navFlag&1 != 0 {
			ev, err := r.U8()
			if err != nil {
				return nil, err
			}
			edge.EventZoneData = ev
		}
		edges[i] = edge
	}
	return edges, nil
}

func readBMSBounds(r *BinReader, bms *BMS) error {
	// vertex cloths
	vcCount, err := r.U32()
	if err != nil {
		return err
	}
	if err := r.Skip(int(vcCount) * 8); err != nil {
		return err
	}
	// edge cloths
	ecCount, err := r.U32()
	if err != nil {
		return err
	}
	if ecCount > 0 {
		if err := r.Skip(int(ecCount)*12 + int(ecCount)*4); err != nil {
			return err
		}
		// cloth settings: 9 × float32/int
		if err := r.Skip(36); err != nil {
			return err
		}
	}
	for i := 0; i < 2; i++ {
		x, err := r.F32()
		if err != nil {
			return err
		}
		y, err := r.F32()
		if err != nil {
			return err
		}
		z, err := r.F32()
		if err != nil {
			return err
		}
		if i == 0 {
			bms.BBoxMin = [3]float32{x, y, z}
		} else {
			bms.BBoxMax = [3]float32{x, y, z}
		}
	}
	return nil
}
