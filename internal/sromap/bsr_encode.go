package sromap

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// EncodeMinimalBSR writes a static-prop wrapper that references one BMS
// (the mesh) and one BMT (the material). Offsets are computed for the
// sections we know the game reads (collision-mesh, mat-count, mesh-count);
// the rest of the 8-slot offset table is zeroed.
//
// `bsrName` is the human-readable name embedded in the BSR (often the file
// stem, e.g. "japanese_house"). `materialPath` and `meshPath` are the
// relative resource paths to the BMT and BMS we just wrote, e.g.
// "prim\\mtrl\\custom\\japanese_house\\japanese_house.bmt".
//
// `collisionMeshPath` may be empty (no collision — player can walk through);
// for a static building you normally point it at the same BMS path used as
// the visual mesh.
func EncodeMinimalBSR(bsrName, materialPath, meshPath, collisionMeshPath string, bboxMin, bboxMax [3]float32) ([]byte, error) {
	if bsrName == "" {
		return nil, fmt.Errorf("bsr encode: bsrName is required")
	}
	if materialPath == "" || meshPath == "" {
		return nil, fmt.Errorf("bsr encode: materialPath and meshPath are required")
	}

	var buf bytes.Buffer
	buf.WriteString(bsrSignature) // 12 bytes

	offsetsStart := buf.Len()
	buf.Write(make([]byte, 8*4)) // reserved offsets

	// meshFlags (0 = no per-mesh extras), followed by 16 bytes of zeros
	// (4 × uint32 unknown header fields the parser skips).
	buf.Write(make([]byte, 4+16))

	// objType + a sibling uint16 right after it — observed as (2, 2) in
	// every static-prop BSR we've dumped (cj_ferry_box, _dam02, _wagon).
	// The parser treats the second uint16 as padding but real files store 2.
	var u16 [2]byte
	binary.LittleEndian.PutUint16(u16[:], 2)
	buf.Write(u16[:])
	binary.LittleEndian.PutUint16(u16[:], 2)
	buf.Write(u16[:])

	// Name (length-prefixed).
	writeLenString(&buf, bsrName)

	// 8 mystery bytes after the name — observed as (uint32 0, uint32 2)
	// in real static-prop BSRs. The parser skips these but the client may
	// rely on the "2" (possibly a subType or LOD-mode marker).
	var u32 [4]byte
	binary.LittleEndian.PutUint32(u32[:], 0)
	buf.Write(u32[:])
	binary.LittleEndian.PutUint32(u32[:], 2)
	buf.Write(u32[:])
	// 40 bytes mystery (the parser skips this block — leave it zeroed and
	// hope the game tolerates it for a static prop).
	buf.Write(make([]byte, 40))

	// --- collisionMesh (section #7) ---
	collisionOff := uint32(buf.Len())
	writeLenString(&buf, collisionMeshPath)

	// Collision boxes. Stock static props use the visual bbox for box0 and
	// an empty sentinel (+1e38..-1e38) for box1 when there is no second
	// collision volume.
	emptyMin := [3]float32{1e38, 1e38, 1e38}
	emptyMax := [3]float32{-1e38, -1e38, -1e38}
	for _, f := range []float32{
		bboxMin[0], bboxMin[1], bboxMin[2],
		bboxMax[0], bboxMax[1], bboxMax[2],
		emptyMin[0], emptyMin[1], emptyMin[2],
		emptyMax[0], emptyMax[1], emptyMax[2],
	} {
		var b [4]byte
		binary.LittleEndian.PutUint32(b[:], floatBits(f))
		buf.Write(b[:])
	}

	// hasMatrix flag (0 = no local-space transform).
	buf.Write(make([]byte, 4))

	// --- materials (section #0) ---
	matCountOff := uint32(buf.Len())
	{
		var c [4]byte
		binary.LittleEndian.PutUint32(c[:], 1)
		buf.Write(c[:])
		// Material type — 0 is the most common for diffuse-only props.
		binary.LittleEndian.PutUint32(c[:], 0)
		buf.Write(c[:])
		writeLenString(&buf, materialPath)
	}

	// --- meshes (section #1) ---
	meshCountOff := uint32(buf.Len())
	{
		var c [4]byte
		binary.LittleEndian.PutUint32(c[:], 1)
		buf.Write(c[:])
		writeLenString(&buf, meshPath)
		// meshFlags = 0 → no per-mesh trailer
	}

	// --- "default" trailer (sections #2..#6) ---
	// Real static-prop BSRs (cj_ferry_box, dam02, wagon — all verified)
	// carry an animation-states block after the meshes section. Without
	// it, the client tries to read past EOF for offsets[3..6] and crashes
	// when the object enters draw range. Layout matches cj_ferry_box.bsr
	// exactly, just with our string locations:
	//
	//   +0  uint32 0x00001000   (flag)
	//   +4  uint32 0
	//   +8  uint32 0
	//   +12 uint32 0            ← offset[2]
	//   +16 uint32 1            ← offset[4] (count of states = 1)
	//   +20 uint32 7            (length of "default")
	//   +24 "default"           (7 bytes)
	//   +31 uint32 1
	//   +35 uint32 0
	//   +39 uint32 0            ← offset[5]
	//   +43 uint32 0            ← offset[6]
	//   +47 uint32 0
	//   +51 byte 0
	//
	// Slot 3 (offset[3]) points at the start of this trailer.
	trailerStart := uint32(buf.Len())
	binary.LittleEndian.PutUint32(u32[:], 0x00001000)
	buf.Write(u32[:])
	buf.Write(make([]byte, 8)) // two zero uint32s
	off2 := uint32(buf.Len())
	buf.Write(make([]byte, 4)) // zero
	off4 := uint32(buf.Len())
	binary.LittleEndian.PutUint32(u32[:], 1)
	buf.Write(u32[:]) // count = 1
	binary.LittleEndian.PutUint32(u32[:], 7)
	buf.Write(u32[:]) // string length
	buf.WriteString("default")
	binary.LittleEndian.PutUint32(u32[:], 1)
	buf.Write(u32[:])
	buf.Write(make([]byte, 4)) // zero
	off5 := uint32(buf.Len())
	buf.Write(make([]byte, 4)) // zero
	off6 := uint32(buf.Len())
	buf.Write(make([]byte, 4)) // zero
	buf.Write(make([]byte, 4)) // zero
	buf.WriteByte(0)           // trailing single byte

	data := buf.Bytes()
	offsets := [8]uint32{
		matCountOff,  // [0]
		meshCountOff, // [1]
		off2,         // [2]
		trailerStart, // [3]
		off4,         // [4]
		off5,         // [5]
		off6,         // [6]
		collisionOff, // [7]
	}
	for i, off := range offsets {
		binary.LittleEndian.PutUint32(data[offsetsStart+i*4:offsetsStart+(i+1)*4], off)
	}
	return data, nil
}
