package sromap

import (
	"fmt"
	"math"
	"os"
)

const (
	NVMTileCount       = 96 // tiles per region edge
	NVMTotalTiles      = NVMTileCount * NVMTileCount
	nvmObjectFixedSize = 30 // size of an NVMObject before its variable Links array
)

type NVMObjectLink struct {
	LinkedObjectID int16
	LinkedEdgeID   int16
	EdgeID         int16
}

type NVMObject struct {
	AssetID  uint32
	X, Y, Z  float32
	Type     int16
	Yaw      float32
	UID      int16
	Short0   int16
	IsBig    bool
	IsStruct bool
	RegionID uint16
	Links    []NVMObjectLink
}

type NVMCell struct {
	MinX, MinZ, MaxX, MaxZ float32
	ObjectIndices          []uint16
}

type NVMGlobalEdge struct {
	MinX, MinZ, MaxX, MaxZ float32
	Flag, Dir0, Dir1       uint8
	Cell0, Cell1           int16
	Region0, Region1       int16
}

type NVMInternalEdge struct {
	MinX, MinZ, MaxX, MaxZ float32
	Flag, Dir0, Dir1       uint8
	Cell0, Cell1           int16
}

type NVMTile struct {
	CellID    int32
	Flag      uint16
	TextureID uint16
}

type NVM struct {
	Path          string
	Objects       []NVMObject
	OpenCellCount uint32
	Cells         []NVMCell
	GlobalEdges   []NVMGlobalEdge
	InternalEdges []NVMInternalEdge
	Tiles         [NVMTotalTiles]NVMTile
	Heights       [MeshGridSize * MeshGridSize]float32
	PlaneType     [MeshBlockCount * MeshBlockCount]byte
	PlaneHeight   [MeshBlockCount * MeshBlockCount]float32
}

type NVMStats struct {
	MinHeight float32
	MaxHeight float32
}

func LoadNVM(path string) (*NVM, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 || string(data[:12]) != nvmSignature {
		return nil, fmt.Errorf("invalid NVM signature")
	}
	n := &NVM{Path: path}
	r := NewBinReader(data)
	if err := r.Skip(12); err != nil {
		return nil, err
	}
	if err := n.parseFrom(r); err != nil {
		return nil, err
	}
	return n, nil
}

func (n *NVM) parseFrom(r *BinReader) error {
	objCount, err := r.U16()
	if err != nil {
		return err
	}
	n.Objects = make([]NVMObject, objCount)
	for i := range n.Objects {
		obj := &n.Objects[i]
		if obj.AssetID, err = r.U32(); err != nil {
			return err
		}
		if obj.X, err = r.F32(); err != nil {
			return err
		}
		if obj.Y, err = r.F32(); err != nil {
			return err
		}
		if obj.Z, err = r.F32(); err != nil {
			return err
		}
		t, err := r.U16()
		if err != nil {
			return err
		}
		obj.Type = int16(t)
		if obj.Yaw, err = r.F32(); err != nil {
			return err
		}
		u, err := r.U16()
		if err != nil {
			return err
		}
		obj.UID = int16(u)
		s, err := r.U16()
		if err != nil {
			return err
		}
		obj.Short0 = int16(s)
		bigByte, err := r.U8()
		if err != nil {
			return err
		}
		obj.IsBig = bigByte != 0
		structByte, err := r.U8()
		if err != nil {
			return err
		}
		obj.IsStruct = structByte != 0
		if obj.RegionID, err = r.U16(); err != nil {
			return err
		}
		linkCount, err := r.U16()
		if err != nil {
			return err
		}
		if linkCount > 1<<14 {
			return fmt.Errorf("NVM: implausible link count %d", linkCount)
		}
		obj.Links = make([]NVMObjectLink, linkCount)
		for j := range obj.Links {
			a, err := r.U16()
			if err != nil {
				return err
			}
			b, err := r.U16()
			if err != nil {
				return err
			}
			c, err := r.U16()
			if err != nil {
				return err
			}
			obj.Links[j].LinkedObjectID = int16(a)
			obj.Links[j].LinkedEdgeID = int16(b)
			obj.Links[j].EdgeID = int16(c)
		}
	}

	cellCount, err := r.U32()
	if err != nil {
		return err
	}
	if cellCount > 1<<20 {
		return fmt.Errorf("NVM: implausible cell count %d", cellCount)
	}
	if n.OpenCellCount, err = r.U32(); err != nil {
		return err
	}
	n.Cells = make([]NVMCell, cellCount)
	for i := range n.Cells {
		c := &n.Cells[i]
		if c.MinX, err = r.F32(); err != nil {
			return err
		}
		if c.MinZ, err = r.F32(); err != nil {
			return err
		}
		if c.MaxX, err = r.F32(); err != nil {
			return err
		}
		if c.MaxZ, err = r.F32(); err != nil {
			return err
		}
		count, err := r.U8()
		if err != nil {
			return err
		}
		c.ObjectIndices = make([]uint16, count)
		for j := range c.ObjectIndices {
			if c.ObjectIndices[j], err = r.U16(); err != nil {
				return err
			}
		}
	}

	geCount, err := r.U32()
	if err != nil {
		return err
	}
	if geCount > 1<<20 {
		return fmt.Errorf("NVM: implausible global edge count %d", geCount)
	}
	n.GlobalEdges = make([]NVMGlobalEdge, geCount)
	for i := range n.GlobalEdges {
		e := &n.GlobalEdges[i]
		if err := readEdgeBounds(r, &e.MinX, &e.MinZ, &e.MaxX, &e.MaxZ); err != nil {
			return err
		}
		if e.Flag, err = r.U8(); err != nil {
			return err
		}
		if e.Dir0, err = r.U8(); err != nil {
			return err
		}
		if e.Dir1, err = r.U8(); err != nil {
			return err
		}
		c0, err := r.U16()
		if err != nil {
			return err
		}
		c1, err := r.U16()
		if err != nil {
			return err
		}
		r0, err := r.U16()
		if err != nil {
			return err
		}
		r1, err := r.U16()
		if err != nil {
			return err
		}
		e.Cell0 = int16(c0)
		e.Cell1 = int16(c1)
		e.Region0 = int16(r0)
		e.Region1 = int16(r1)
	}

	ieCount, err := r.U32()
	if err != nil {
		return err
	}
	if ieCount > 1<<20 {
		return fmt.Errorf("NVM: implausible internal edge count %d", ieCount)
	}
	n.InternalEdges = make([]NVMInternalEdge, ieCount)
	for i := range n.InternalEdges {
		e := &n.InternalEdges[i]
		if err := readEdgeBounds(r, &e.MinX, &e.MinZ, &e.MaxX, &e.MaxZ); err != nil {
			return err
		}
		if e.Flag, err = r.U8(); err != nil {
			return err
		}
		if e.Dir0, err = r.U8(); err != nil {
			return err
		}
		if e.Dir1, err = r.U8(); err != nil {
			return err
		}
		c0, err := r.U16()
		if err != nil {
			return err
		}
		c1, err := r.U16()
		if err != nil {
			return err
		}
		e.Cell0 = int16(c0)
		e.Cell1 = int16(c1)
	}

	for i := range n.Tiles {
		cid, err := r.U32()
		if err != nil {
			return err
		}
		n.Tiles[i].CellID = int32(cid)
		if n.Tiles[i].Flag, err = r.U16(); err != nil {
			return err
		}
		if n.Tiles[i].TextureID, err = r.U16(); err != nil {
			return err
		}
	}

	for i := range n.Heights {
		if n.Heights[i], err = r.F32(); err != nil {
			return err
		}
	}

	for i := range n.PlaneType {
		b, err := r.U8()
		if err != nil {
			return err
		}
		n.PlaneType[i] = b
	}

	for i := range n.PlaneHeight {
		if n.PlaneHeight[i], err = r.F32(); err != nil {
			return err
		}
	}

	return nil
}

func readEdgeBounds(r *BinReader, minX, minZ, maxX, maxZ *float32) error {
	v, err := r.F32()
	if err != nil {
		return err
	}
	*minX = v
	if v, err = r.F32(); err != nil {
		return err
	}
	*minZ = v
	if v, err = r.F32(); err != nil {
		return err
	}
	*maxX = v
	if v, err = r.F32(); err != nil {
		return err
	}
	*maxZ = v
	return nil
}

func (n *NVM) Save(path string) error {
	buf := make([]byte, 0, 200000)
	buf = append(buf, []byte(nvmSignature)...)

	buf = appendU16LE(buf, uint16(len(n.Objects)))
	for _, obj := range n.Objects {
		buf = appendU32LE(buf, obj.AssetID)
		buf = appendF32LE(buf, obj.X)
		buf = appendF32LE(buf, obj.Y)
		buf = appendF32LE(buf, obj.Z)
		buf = appendU16LE(buf, uint16(obj.Type))
		buf = appendF32LE(buf, obj.Yaw)
		buf = appendU16LE(buf, uint16(obj.UID))
		buf = appendU16LE(buf, uint16(obj.Short0))
		buf = append(buf, boolByte(obj.IsBig), boolByte(obj.IsStruct))
		buf = appendU16LE(buf, obj.RegionID)
		buf = appendU16LE(buf, uint16(len(obj.Links)))
		for _, l := range obj.Links {
			buf = appendU16LE(buf, uint16(l.LinkedObjectID))
			buf = appendU16LE(buf, uint16(l.LinkedEdgeID))
			buf = appendU16LE(buf, uint16(l.EdgeID))
		}
	}

	buf = appendU32LE(buf, uint32(len(n.Cells)))
	buf = appendU32LE(buf, n.OpenCellCount)
	for _, c := range n.Cells {
		buf = appendF32LE(buf, c.MinX)
		buf = appendF32LE(buf, c.MinZ)
		buf = appendF32LE(buf, c.MaxX)
		buf = appendF32LE(buf, c.MaxZ)
		buf = append(buf, byte(len(c.ObjectIndices)))
		for _, idx := range c.ObjectIndices {
			buf = appendU16LE(buf, idx)
		}
	}

	buf = appendU32LE(buf, uint32(len(n.GlobalEdges)))
	for _, e := range n.GlobalEdges {
		buf = appendF32LE(buf, e.MinX)
		buf = appendF32LE(buf, e.MinZ)
		buf = appendF32LE(buf, e.MaxX)
		buf = appendF32LE(buf, e.MaxZ)
		buf = append(buf, e.Flag, e.Dir0, e.Dir1)
		buf = appendU16LE(buf, uint16(e.Cell0))
		buf = appendU16LE(buf, uint16(e.Cell1))
		buf = appendU16LE(buf, uint16(e.Region0))
		buf = appendU16LE(buf, uint16(e.Region1))
	}

	buf = appendU32LE(buf, uint32(len(n.InternalEdges)))
	for _, e := range n.InternalEdges {
		buf = appendF32LE(buf, e.MinX)
		buf = appendF32LE(buf, e.MinZ)
		buf = appendF32LE(buf, e.MaxX)
		buf = appendF32LE(buf, e.MaxZ)
		buf = append(buf, e.Flag, e.Dir0, e.Dir1)
		buf = appendU16LE(buf, uint16(e.Cell0))
		buf = appendU16LE(buf, uint16(e.Cell1))
	}

	for _, t := range n.Tiles {
		buf = appendU32LE(buf, uint32(t.CellID))
		buf = appendU16LE(buf, t.Flag)
		buf = appendU16LE(buf, t.TextureID)
	}
	for _, h := range n.Heights {
		buf = appendF32LE(buf, h)
	}
	buf = append(buf, n.PlaneType[:]...)
	for _, h := range n.PlaneHeight {
		buf = appendF32LE(buf, h)
	}

	return os.WriteFile(path, buf, 0644)
}

func boolByte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func (n *NVM) Stats() NVMStats {
	minH := float32(math.Inf(1))
	maxH := float32(math.Inf(-1))
	for _, h := range n.Heights {
		if h < minH {
			minH = h
		}
		if h > maxH {
			maxH = h
		}
	}
	return NVMStats{MinHeight: minH, MaxHeight: maxH}
}

func (n *NVM) ApplyBrush(b Brush) (int, error) {
	if err := validateBrush(b); err != nil {
		return 0, err
	}
	changed := 0
	for gz := 0; gz < MeshGridSize; gz++ {
		for gx := 0; gx < MeshGridSize; gx++ {
			weight := brushWeight(b, float64(gx*CellSize), float64(gz*CellSize))
			if weight <= 0 {
				continue
			}
			idx := gz*MeshGridSize + gx
			n.Heights[idx] += b.Delta * float32(weight)
			changed++
		}
	}
	return changed, nil
}

func (n *NVM) SetHeightMap(heights []float32) error {
	if len(heights) != MeshGridSize*MeshGridSize {
		return fmt.Errorf("height map must contain %d values, got %d", MeshGridSize*MeshGridSize, len(heights))
	}
	for i, h := range heights {
		n.Heights[i] = h
	}
	return nil
}

// NewFlatNVM returns a minimal but structurally complete NVM for a brand-new
// region with no obstacles: one cell spanning the full 96×96 tile grid, no
// global or internal edges, all tiles walkable and pointing at cell 0, and
// every height/plane sample set to `heightY`. Server emulators accept this
// because it's the smallest valid shape — no out-of-range cell IDs, no
// dangling edge references, no object table.
//
// For new regions, write this alongside the new .m so the server has a
// playable navmesh. Per-object collision is added later via ApplyNVMTileFlags
// when placements get saved (same path that already works for stock NVMs).
func NewFlatNVM(heightY float32) *NVM {
	n := &NVM{}
	regionExtent := float32(NVMTileCount * NVMTileSize)
	n.Cells = []NVMCell{{
		MinX: 0, MinZ: 0,
		MaxX: regionExtent, MaxZ: regionExtent,
		ObjectIndices: nil,
	}}
	n.OpenCellCount = 1
	for i := range n.Tiles {
		n.Tiles[i] = NVMTile{CellID: 0, Flag: 0, TextureID: 0}
	}
	for i := range n.Heights {
		n.Heights[i] = heightY
	}
	for i := range n.PlaneHeight {
		n.PlaneHeight[i] = heightY
	}
	return n
}
