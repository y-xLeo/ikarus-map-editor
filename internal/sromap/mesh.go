package sromap

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

type Mesh struct {
	Path string
	Data []byte
}

type MeshStats struct {
	StoredVertices int
	UniqueVertices int
	MinHeight      float32
	MaxHeight      float32
	ExtraBytes     int
}

type Brush struct {
	All     bool
	CenterX float64
	CenterZ float64
	Radius  float64
	Delta   float32
	Falloff string
}

type EditReport struct {
	StoredVerticesChanged int
	UniqueVerticesChanged int
	MinHeightBefore       float32
	MaxHeightBefore       float32
	MinHeightAfter        float32
	MaxHeightAfter        float32
}

func LoadMesh(path string) (*Mesh, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < MeshExpectedSize {
		return nil, fmt.Errorf("mesh is too small: got %d, need at least %d", len(data), MeshExpectedSize)
	}
	if string(data[:12]) != meshSignature {
		return nil, fmt.Errorf("invalid mesh signature %q", string(data[:12]))
	}
	return &Mesh{Path: path, Data: data}, nil
}

func (m *Mesh) Save(path string) error {
	return os.WriteFile(path, m.Data, 0644)
}

// NewFlatMesh clones the given template's bytes and overwrites all vertex
// heights with `height`. Tile IDs are forced to `defaultTileID` (use 0 to
// clear, or pass any valid tile2d.ifo entry). Block bounds are recalculated.
// The caller decides where to save the result.
func NewFlatMesh(template *Mesh, height float32, defaultTileID uint16) (*Mesh, error) {
	if template == nil || len(template.Data) < MeshExpectedSize {
		return nil, fmt.Errorf("template mesh missing or too small")
	}
	out := &Mesh{Data: append([]byte(nil), template.Data...)}
	heights := make([]float32, MeshGridSize*MeshGridSize)
	for i := range heights {
		heights[i] = height
	}
	if err := out.SetUniqueHeightMap(heights); err != nil {
		return nil, err
	}
	ids := make([]uint16, MeshGridSize*MeshGridSize)
	for i := range ids {
		ids[i] = defaultTileID
	}
	if err := out.SetUniqueTextureIDs(ids); err != nil {
		return nil, err
	}
	return out, nil
}

func (m *Mesh) Stats() MeshStats {
	minH := float32(math.Inf(1))
	maxH := float32(math.Inf(-1))
	seen := make(map[int]struct{}, MeshGridSize*MeshGridSize)

	m.eachStoredVertex(func(gx, gz int, offset int) {
		h := readFloat32(m.Data[offset : offset+4])
		if h < minH {
			minH = h
		}
		if h > maxH {
			maxH = h
		}
		seen[gz*MeshGridSize+gx] = struct{}{}
	})

	return MeshStats{
		StoredVertices: MeshBlockCount * MeshBlockCount * MeshBlockVerts * MeshBlockVerts,
		UniqueVertices: len(seen),
		MinHeight:      minH,
		MaxHeight:      maxH,
		ExtraBytes:     max(0, len(m.Data)-MeshExpectedSize),
	}
}

func (m *Mesh) ApplyBrush(b Brush) (EditReport, error) {
	if err := validateBrush(b); err != nil {
		return EditReport{}, err
	}

	before := m.Stats()
	seenChanged := make(map[int]struct{}, MeshGridSize*MeshGridSize)
	report := EditReport{
		MinHeightBefore: before.MinHeight,
		MaxHeightBefore: before.MaxHeight,
	}

	m.eachStoredVertex(func(gx, gz int, offset int) {
		weight := brushWeight(b, float64(gx*CellSize), float64(gz*CellSize))
		if weight <= 0 {
			return
		}
		old := readFloat32(m.Data[offset : offset+4])
		writeFloat32(m.Data[offset:offset+4], old+b.Delta*float32(weight))
		report.StoredVerticesChanged++
		seenChanged[gz*MeshGridSize+gx] = struct{}{}
	})

	if report.StoredVerticesChanged == 0 {
		report.MinHeightAfter = before.MinHeight
		report.MaxHeightAfter = before.MaxHeight
		return report, nil
	}

	m.RecalculateBlockBounds()
	after := m.Stats()
	report.UniqueVerticesChanged = len(seenChanged)
	report.MinHeightAfter = after.MinHeight
	report.MaxHeightAfter = after.MaxHeight
	return report, nil
}

func (m *Mesh) RecalculateBlockBounds() {
	for bz := 0; bz < MeshBlockCount; bz++ {
		for bx := 0; bx < MeshBlockCount; bx++ {
			block := blockOffset(bx, bz)
			minH := float32(math.Inf(1))
			maxH := float32(math.Inf(-1))
			for vz := 0; vz < MeshBlockVerts; vz++ {
				for vx := 0; vx < MeshBlockVerts; vx++ {
					offset := vertexHeightOffset(block, vx, vz)
					h := readFloat32(m.Data[offset : offset+4])
					if h < minH {
						minH = h
					}
					if h > maxH {
						maxH = h
					}
				}
			}
			writeFloat32(m.Data[block+2547:block+2551], maxH)
			writeFloat32(m.Data[block+2551:block+2555], minH)
		}
	}
}

func (m *Mesh) UniqueHeightMap() [MeshGridSize * MeshGridSize]float32 {
	var heights [MeshGridSize * MeshGridSize]float32
	var seen [MeshGridSize * MeshGridSize]bool
	m.eachStoredVertex(func(gx, gz int, offset int) {
		idx := gz*MeshGridSize + gx
		if seen[idx] {
			return
		}
		heights[idx] = readFloat32(m.Data[offset : offset+4])
		seen[idx] = true
	})
	return heights
}

func (m *Mesh) UniqueTextureMap() (ids [MeshGridSize * MeshGridSize]uint16, scales [MeshGridSize * MeshGridSize]uint8, brightness [MeshGridSize * MeshGridSize]uint8) {
	var seen [MeshGridSize * MeshGridSize]bool
	m.eachStoredVertex(func(gx, gz int, offset int) {
		idx := gz*MeshGridSize + gx
		if seen[idx] {
			return
		}
		packed := binary.LittleEndian.Uint16(m.Data[offset+4 : offset+6])
		ids[idx] = packed & 0x7FF
		scales[idx] = uint8((packed >> 11) & 0x1F)
		brightness[idx] = m.Data[offset+6]
		seen[idx] = true
	})
	return ids, scales, brightness
}

// SetUniqueTextureIDs updates the 11-bit texture ID portion of each vertex's
// packed uint16, preserving the 5-bit scale. The same unique grid value is
// written to every duplicate stored vertex on shared block borders. Indices
// outside the 11-bit range (>= 2048) are clamped to keep the file valid.
func (m *Mesh) SetUniqueTextureIDs(ids []uint16) error {
	if len(ids) != MeshGridSize*MeshGridSize {
		return fmt.Errorf("texture map must contain %d values, got %d", MeshGridSize*MeshGridSize, len(ids))
	}
	m.eachStoredVertex(func(gx, gz int, offset int) {
		want := ids[gz*MeshGridSize+gx] & 0x7FF
		packed := binary.LittleEndian.Uint16(m.Data[offset+4 : offset+6])
		newPacked := (packed & 0xF800) | want
		binary.LittleEndian.PutUint16(m.Data[offset+4:offset+6], newPacked)
	})
	return nil
}

// TileFlagMap returns the 96x96 per-tile flag grid stored in the .m file.
func (m *Mesh) TileFlagMap() [NVMTotalTiles]uint16 {
	var flags [NVMTotalTiles]uint16
	for bz := 0; bz < MeshBlockCount; bz++ {
		for bx := 0; bx < MeshBlockCount; bx++ {
			block := blockOffset(bx, bz)
			base := block + meshBlockTileFlagsOffset
			for tz := 0; tz < MeshBlockTiles; tz++ {
				for tx := 0; tx < MeshBlockTiles; tx++ {
					gx := bx*MeshBlockTiles + tx
					gz := bz*MeshBlockTiles + tz
					flags[gz*NVMTileCount+gx] = binary.LittleEndian.Uint16(m.Data[base+(tz*MeshBlockTiles+tx)*2:])
				}
			}
		}
	}
	return flags
}

// NVMTileTextureMap derives the NVM's 96x96 texture-id grid from the terrain
// vertex texture IDs. The game/editor format stores texture IDs on vertices;
// the NVM tile table stores one ID per terrain tile, so use the south-west
// vertex of each tile as the stable representative.
func (m *Mesh) NVMTileTextureMap() [NVMTotalTiles]uint16 {
	ids, _, _ := m.UniqueTextureMap()
	var out [NVMTotalTiles]uint16
	for z := 0; z < NVMTileCount; z++ {
		for x := 0; x < NVMTileCount; x++ {
			out[z*NVMTileCount+x] = ids[z*MeshGridSize+x]
		}
	}
	return out
}

// PlaneMaps converts each .m block's water/ice plane metadata into the NVM
// plane arrays. A waterType of -1/0xff means no plane in the source block.
func (m *Mesh) PlaneMaps() (types [MeshBlockCount * MeshBlockCount]byte, heights [MeshBlockCount * MeshBlockCount]float32) {
	for bz := 0; bz < MeshBlockCount; bz++ {
		for bx := 0; bx < MeshBlockCount; bx++ {
			idx := bz*MeshBlockCount + bx
			block := blockOffset(bx, bz)
			waterType := int8(m.Data[block+meshBlockWaterTypeOffset])
			if waterType >= 0 {
				types[idx] = byte(waterType + 1)
			}
			heights[idx] = readFloat32(m.Data[block+meshBlockWaterHeightOffset : block+meshBlockWaterHeightOffset+4])
		}
	}
	return types, heights
}

func (m *Mesh) SetUniqueHeightMap(heights []float32) error {
	if len(heights) != MeshGridSize*MeshGridSize {
		return fmt.Errorf("height map must contain %d values, got %d", MeshGridSize*MeshGridSize, len(heights))
	}
	m.eachStoredVertex(func(gx, gz int, offset int) {
		writeFloat32(m.Data[offset:offset+4], heights[gz*MeshGridSize+gx])
	})
	m.RecalculateBlockBounds()
	return nil
}

func (m *Mesh) eachStoredVertex(fn func(gx, gz int, offset int)) {
	for bz := 0; bz < MeshBlockCount; bz++ {
		for bx := 0; bx < MeshBlockCount; bx++ {
			block := blockOffset(bx, bz)
			for vz := 0; vz < MeshBlockVerts; vz++ {
				for vx := 0; vx < MeshBlockVerts; vx++ {
					gx := bx*MeshBlockTiles + vx
					gz := bz*MeshBlockTiles + vz
					fn(gx, gz, vertexHeightOffset(block, vx, vz))
				}
			}
		}
	}
}

func validateBrush(b Brush) error {
	if b.Delta == 0 {
		return fmt.Errorf("delta must be non-zero")
	}
	if b.All {
		return nil
	}
	if b.Radius <= 0 {
		return fmt.Errorf("radius must be > 0 unless -all is used")
	}
	if b.CenterX < 0 || b.CenterX > RegionSize || b.CenterZ < 0 || b.CenterZ > RegionSize {
		return fmt.Errorf("brush center must be inside local region coordinates 0..%d", RegionSize)
	}
	switch b.Falloff {
	case "", "none", "linear", "smooth":
		return nil
	default:
		return fmt.Errorf("unsupported falloff %q; use none, linear, or smooth", b.Falloff)
	}
}

func brushWeight(b Brush, x, z float64) float64 {
	if b.All {
		return 1
	}
	dx, dz := x-b.CenterX, z-b.CenterZ
	dist := math.Sqrt(dx*dx + dz*dz)
	if dist > b.Radius {
		return 0
	}
	t := dist / b.Radius
	switch b.Falloff {
	case "none":
		return 1
	case "linear":
		return 1 - t
	default:
		smooth := t * t * (3 - 2*t)
		return 1 - smooth
	}
}

func blockOffset(bx, bz int) int {
	return 12 + (bz*MeshBlockCount+bx)*MeshBlockSize
}

func vertexHeightOffset(blockOffset, vx, vz int) int {
	return blockOffset + 6 + (vz*MeshBlockVerts+vx)*7
}

const (
	meshBlockWaterTypeOffset   = 6 + MeshBlockVerts*MeshBlockVerts*7
	meshBlockWaterHeightOffset = meshBlockWaterTypeOffset + 2
	meshBlockTileFlagsOffset   = meshBlockWaterTypeOffset + 6
)

func readFloat32(data []byte) float32 {
	return math.Float32frombits(binary.LittleEndian.Uint32(data))
}

func writeFloat32(data []byte, v float32) {
	binary.LittleEndian.PutUint32(data, math.Float32bits(v))
}
