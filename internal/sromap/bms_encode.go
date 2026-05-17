package sromap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

const bmsNavLookupCellSize = float32(100)

// BMSNavOptions controls the generated object NavMesh section. The offset is
// in BMS local X/Z space and moves only the collision/nav footprint; the visual
// mesh vertices are written unchanged.
type BMSNavOptions struct {
	OffsetX   float32
	OffsetZ   float32
	Footprint [][2]float32 // optional local X/Z polygon; falls back to bbox rectangle
}

// BMSBoundsWithNavOptions returns the local-space bbox that encloses both the
// visual mesh bbox and the generated NavMesh bbox after applying opts.
func BMSBoundsWithNavOptions(bboxMin, bboxMax [3]float32, opts BMSNavOptions) ([3]float32, [3]float32) {
	outMin := bboxMin
	outMax := bboxMax
	navMin, navMax := bmsNavFootprintBounds(bboxMin, bboxMax, opts)
	if navMin[0] < outMin[0] {
		outMin[0] = navMin[0]
	}
	if navMax[0] > outMax[0] {
		outMax[0] = navMax[0]
	}
	if navMin[2] < outMin[2] {
		outMin[2] = navMin[2]
	}
	if navMax[2] > outMax[2] {
		outMax[2] = navMax[2]
	}
	return outMin, outMax
}

func bmsNavFootprint(bboxMin, bboxMax [3]float32, opts BMSNavOptions) [][2]float32 {
	if len(opts.Footprint) >= 3 {
		out := make([][2]float32, len(opts.Footprint))
		for i, p := range opts.Footprint {
			out[i] = [2]float32{p[0] + opts.OffsetX, p[1] + opts.OffsetZ}
		}
		return out
	}
	return [][2]float32{
		{bboxMin[0] + opts.OffsetX, bboxMin[2] + opts.OffsetZ},
		{bboxMax[0] + opts.OffsetX, bboxMin[2] + opts.OffsetZ},
		{bboxMax[0] + opts.OffsetX, bboxMax[2] + opts.OffsetZ},
		{bboxMin[0] + opts.OffsetX, bboxMax[2] + opts.OffsetZ},
	}
}

func bmsNavFootprintBounds(bboxMin, bboxMax [3]float32, opts BMSNavOptions) ([3]float32, [3]float32) {
	footprint := bmsNavFootprint(bboxMin, bboxMax, opts)
	min := [3]float32{footprint[0][0], bboxMin[1], footprint[0][1]}
	max := min
	for _, p := range footprint[1:] {
		if p[0] < min[0] {
			min[0] = p[0]
		}
		if p[0] > max[0] {
			max[0] = p[0]
		}
		if p[1] < min[2] {
			min[2] = p[1]
		}
		if p[1] > max[2] {
			max[2] = p[1]
		}
	}
	return min, max
}

// EncodeMinimalBMS writes a static-prop BMS file with no skinning, no cloth,
// no vertex groups, and the simplest possible vertex layout (44 bytes per
// vertex: XYZ + 12 zero "normal" bytes + UV + 12 zero "tangent" bytes).
//
// `material` is the case-insensitive material name that the BSR's BMT must
// also define. `name` is a human-readable mesh name (often the file stem).
//
// The 7 file-section offsets at the top of the BMS are computed so the game
// engine can index directly. Our own parser walks sequentially and ignores
// them, but the game client almost certainly consumes them.
//
// This format is deliberately minimal — anything that requires DXT5 alpha,
// skinning rigs, cloth, navmesh, or occlusion data is out of scope. The
// game's BMS loader is undocumented, so this is best-effort and will
// inevitably need iteration against actual in-game behaviour.
func EncodeMinimalBMS(name, material string, verts []BMSVertex, indices []uint16, bboxMin, bboxMax [3]float32) ([]byte, error) {
	return EncodeMinimalBMSWithOptions(name, material, verts, indices, bboxMin, bboxMax, BMSNavOptions{})
}

// EncodeMinimalBMSWithOptions is EncodeMinimalBMS plus collision/nav options.
func EncodeMinimalBMSWithOptions(name, material string, verts []BMSVertex, indices []uint16, bboxMin, bboxMax [3]float32, opts BMSNavOptions) ([]byte, error) {
	if len(verts) > 65535 {
		return nil, fmt.Errorf("bms encode: vertex count %d exceeds uint16 index space", len(verts))
	}
	if len(indices)%3 != 0 {
		return nil, fmt.Errorf("bms encode: index count %d not a multiple of 3", len(indices))
	}

	var buf bytes.Buffer
	buf.WriteString(bmsSignature) // 12 bytes

	// Reserve 28 bytes for the 7 section offsets (vertices, vertexGroups,
	// faces, vertexClothes, edgeClothes, bbox, occlusion) — patched at end.
	offsetsStart := buf.Len()
	buf.Write(make([]byte, 28))

	// navMesh / skinnedNavMesh / unknown09 offsets — patched at end.
	navOffsetsStart := buf.Len()
	buf.Write(make([]byte, 12))

	// unkUInt0 (0), navFlag (0), subPrimCount (1), vertexFlag (0), unkUInt2 (0).
	var smallHeader [4 + 4 + 4 + 4 + 4]byte
	binary.LittleEndian.PutUint32(smallHeader[8:12], 1) // subPrimCount = 1
	buf.Write(smallHeader[:])

	writeLenString(&buf, name)
	writeLenString(&buf, material)
	// 4 unknown bytes after the material name.
	buf.Write(make([]byte, 4))

	// --- vertex array ---
	// Per-vertex layout for vertexFlag=0 (44 bytes total), matched against
	// dumps of real BMS files (e.g. cj_ferry_box.bms):
	//   +0  XYZ (3 × f32)
	//   +12 normal (3 × f32, unit length) — zero normals → engine NaNs → crash
	//   +24 UV (2 × f32)
	//   +32 trailing 12 bytes: 4 zero, 4 bytes vertex color (BGRA), 4 zero.
	//                          Color must be opaque white (FFFFFFFF) for a
	//                          textured material — zero alpha = invisible / crash.
	verticesOffset := uint32(buf.Len())
	var vc [4]byte
	binary.LittleEndian.PutUint32(vc[:], uint32(len(verts)))
	buf.Write(vc[:])
	for _, v := range verts {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], floatBits(v.X))
		buf.Write(b[:])
		binary.LittleEndian.PutUint32(b[:], floatBits(v.Y))
		buf.Write(b[:])
		binary.LittleEndian.PutUint32(b[:], floatBits(v.Z))
		buf.Write(b[:])
		// Normal — fall back to up-vector if not supplied so we never write zeros.
		nx, ny, nz := v.NX, v.NY, v.NZ
		if nx*nx+ny*ny+nz*nz < 1e-6 {
			nx, ny, nz = 0, 1, 0
		}
		binary.LittleEndian.PutUint32(b[:], floatBits(nx))
		buf.Write(b[:])
		binary.LittleEndian.PutUint32(b[:], floatBits(ny))
		buf.Write(b[:])
		binary.LittleEndian.PutUint32(b[:], floatBits(nz))
		buf.Write(b[:])
		// UV — write struct.V directly. The OBJ parser already converted
		// OBJ's bottom-up V into top-down (= 1 - raw_obj_v) and the game
		// expects top-down, so no further flip is needed. Our own BMS
		// decoder does a `1-v` flip on read; that means a roundtrip
		// through it produces 1-struct.V, but in-game V is correct.
		binary.LittleEndian.PutUint32(b[:], floatBits(v.U))
		buf.Write(b[:])
		binary.LittleEndian.PutUint32(b[:], floatBits(v.V))
		buf.Write(b[:])
		// Trailing 12 bytes: pad + opaque white + pad.
		buf.Write([]byte{0, 0, 0, 0, 0xFF, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0})
	}

	// --- vertex groups (none) ---
	vertexGroupsOffset := uint32(buf.Len())
	buf.Write(make([]byte, 4)) // count = 0

	// --- faces ---
	facesOffset := uint32(buf.Len())
	binary.LittleEndian.PutUint32(vc[:], uint32(len(indices)/3))
	buf.Write(vc[:])
	for i := 0; i+2 < len(indices); i += 3 {
		var p [6]byte
		binary.LittleEndian.PutUint16(p[0:2], indices[i])
		binary.LittleEndian.PutUint16(p[2:4], indices[i+1])
		binary.LittleEndian.PutUint16(p[4:6], indices[i+2])
		buf.Write(p[:])
	}

	// --- vertex cloths (none) ---
	vertexClothesOffset := uint32(buf.Len())
	buf.Write(make([]byte, 4)) // count = 0

	// --- edge cloths (none) ---
	edgeClothesOffset := uint32(buf.Len())
	buf.Write(make([]byte, 4)) // count = 0

	fileBBoxMin, fileBBoxMax := BMSBoundsWithNavOptions(bboxMin, bboxMax, opts)

	// --- bounding box ---
	bboxOffset := uint32(buf.Len())
	for _, v := range []float32{fileBBoxMin[0], fileBBoxMin[1], fileBBoxMin[2], fileBBoxMax[0], fileBBoxMax[1], fileBBoxMax[2]} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], floatBits(v))
		buf.Write(b[:])
	}

	// --- occlusion portal ---
	occlusionOffset := uint32(buf.Len())
	buf.Write(make([]byte, 4)) // hasPortal = 0

	// --- unk9 ---
	unk9Offset := uint32(buf.Len())
	buf.Write(make([]byte, 4)) // unk9Count = 0

	// --- NavMesh section ---
	// Emit an object collision footprint as RTNavMeshObj-style local
	// vertices/cells/edges. Stock props use this section for server collision;
	// without it the player walks straight through any object referencing this
	// asset.
	navMeshOffset := uint32(buf.Len())
	floorY := bboxMin[1]
	footprint := bmsNavFootprint(bboxMin, bboxMax, opts)
	navMin, navMax := bmsNavFootprintBounds(bboxMin, bboxMax, opts)
	// nav vertex count + vertices (each 12 bytes pos + 1 byte bisector).
	binary.LittleEndian.PutUint32(vc[:], uint32(len(footprint)))
	buf.Write(vc[:])
	for _, p := range footprint {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], floatBits(p[0]))
		buf.Write(b[:])
		binary.LittleEndian.PutUint32(b[:], floatBits(floorY))
		buf.Write(b[:])
		binary.LittleEndian.PutUint32(b[:], floatBits(p[1]))
		buf.Write(b[:])
		buf.WriteByte(0) // BisectorIndex
	}
	// 2 cells (triangles). NavFlag=0 → no extra event byte.
	cellCount := len(footprint) - 2
	binary.LittleEndian.PutUint32(vc[:], uint32(cellCount))
	buf.Write(vc[:])
	for i := 1; i < len(footprint)-1; i++ {
		binary.LittleEndian.PutUint16(vc[:2], 0)
		buf.Write(vc[:2])
		binary.LittleEndian.PutUint16(vc[:2], uint16(i))
		buf.Write(vc[:2])
		binary.LittleEndian.PutUint16(vc[:2], uint16(i+1))
		buf.Write(vc[:2])
		binary.LittleEndian.PutUint16(vc[:2], 0) // cell.Flag = 0
		buf.Write(vc[:2])
	}
	// Outline edges (perimeter, every one fully blocked = BlockSrcToDst|BlockDstToSrc = 0x3).
	// SrcCell points at the triangle owning each edge; DstCell = 0xFFFF (no cell outside).
	binary.LittleEndian.PutUint32(vc[:], uint32(len(footprint)))
	buf.Write(vc[:])
	for i := range footprint {
		src := uint16(i)
		dst := uint16((i + 1) % len(footprint))
		srcCell := uint16(0)
		switch {
		case i == 0:
			srcCell = 0
		case i == len(footprint)-1:
			srcCell = uint16(cellCount - 1)
		default:
			srcCell = uint16(i - 1)
		}
		binary.LittleEndian.PutUint16(vc[:2], src)
		buf.Write(vc[:2])
		binary.LittleEndian.PutUint16(vc[:2], dst)
		buf.Write(vc[:2])
		binary.LittleEndian.PutUint16(vc[:2], srcCell)
		buf.Write(vc[:2])
		binary.LittleEndian.PutUint16(vc[:2], 0xFFFF)
		buf.Write(vc[:2])
		buf.WriteByte(0x03) // EdgeFlag.Blocked (= BlockSrcToDst | BlockDstToSrc)
	}
	// Fan diagonals are inline edges shared by adjacent cells.
	inlineCount := len(footprint) - 3
	binary.LittleEndian.PutUint32(vc[:], uint32(inlineCount))
	buf.Write(vc[:])
	for i := 0; i < inlineCount; i++ {
		binary.LittleEndian.PutUint16(vc[:2], 0)
		buf.Write(vc[:2])
		binary.LittleEndian.PutUint16(vc[:2], uint16(i+2))
		buf.Write(vc[:2])
		binary.LittleEndian.PutUint16(vc[:2], uint16(i))
		buf.Write(vc[:2])
		binary.LittleEndian.PutUint16(vc[:2], uint16(i+1))
		buf.Write(vc[:2])
		buf.WriteByte(0x04)
	}
	gridWidth := uint32(math.Ceil(float64((navMax[0] - navMin[0]) / bmsNavLookupCellSize)))
	if gridWidth == 0 {
		gridWidth = 1
	}
	gridHeight := uint32(math.Ceil(float64((navMax[2] - navMin[2]) / bmsNavLookupCellSize)))
	if gridHeight == 0 {
		gridHeight = 1
	}
	gridCellCount := gridWidth * gridHeight

	// OutlineLookupGrid uses roughly 100 local units per cell in stock BMS
	// files. Each generated cell keeps the full outline list; this is a small,
	// conservative candidate set and avoids missing perimeter edges.
	var f [4]byte
	binary.LittleEndian.PutUint32(f[:], floatBits(navMin[0]))
	buf.Write(f[:])
	binary.LittleEndian.PutUint32(f[:], floatBits(navMin[2]))
	buf.Write(f[:])
	binary.LittleEndian.PutUint32(vc[:], gridWidth)
	buf.Write(vc[:])
	binary.LittleEndian.PutUint32(vc[:], gridHeight)
	buf.Write(vc[:])
	binary.LittleEndian.PutUint32(vc[:], gridCellCount)
	buf.Write(vc[:])
	for cell := uint32(0); cell < gridCellCount; cell++ {
		binary.LittleEndian.PutUint32(vc[:], uint32(len(footprint)))
		buf.Write(vc[:])
		for i := range footprint {
			binary.LittleEndian.PutUint16(vc[:2], uint16(i))
			buf.Write(vc[:2])
		}
	}

	data := buf.Bytes()
	offsets := []uint32{
		verticesOffset, vertexGroupsOffset, facesOffset,
		vertexClothesOffset, edgeClothesOffset, bboxOffset, occlusionOffset,
	}
	for i, off := range offsets {
		binary.LittleEndian.PutUint32(data[offsetsStart+i*4:offsetsStart+(i+1)*4], off)
	}
	// Header's three "nav-related" offsets:
	// [navMesh, skinnedNavMesh, unknown09]. Stock BMS files point unknown09
	// at the 4-byte unk9Count immediately before the NavMesh section.
	binary.LittleEndian.PutUint32(data[navOffsetsStart+0:navOffsetsStart+4], navMeshOffset)
	binary.LittleEndian.PutUint32(data[navOffsetsStart+8:navOffsetsStart+12], unk9Offset)
	return data, nil
}

func writeLenString(buf *bytes.Buffer, s string) {
	var l [4]byte
	binary.LittleEndian.PutUint32(l[:], uint32(len(s)))
	buf.Write(l[:])
	buf.WriteString(s)
}

func floatBits(f float32) uint32 { return math.Float32bits(f) }
