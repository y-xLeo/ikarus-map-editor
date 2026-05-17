package sromap

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestEncodeMinimalBMS_RoundTrip(t *testing.T) {
	verts := []BMSVertex{
		{X: -1, Y: 0, Z: -1, U: 0, V: 0},
		{X: 1, Y: 0, Z: -1, U: 1, V: 0},
		{X: 1, Y: 0, Z: 1, U: 1, V: 1},
		{X: -1, Y: 0, Z: 1, U: 0, V: 1},
	}
	indices := []uint16{0, 1, 2, 0, 2, 3}
	bboxMin := [3]float32{-1, 0, -1}
	bboxMax := [3]float32{1, 0, 1}

	data, err := EncodeMinimalBMS("test", "matA", verts, indices, bboxMin, bboxMax)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	bms, err := DecodeBMS(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if bms.Name != "test" {
		t.Errorf("name: got %q, want test", bms.Name)
	}
	if bms.Material != "matA" {
		t.Errorf("material: got %q, want matA", bms.Material)
	}
	if len(bms.Vertices) != 4 {
		t.Fatalf("vertex count: got %d, want 4", len(bms.Vertices))
	}
	if len(bms.Indices) != 6 {
		t.Fatalf("index count: got %d, want 6", len(bms.Indices))
	}
	for i, v := range verts {
		got := bms.Vertices[i]
		if got.X != v.X || got.Y != v.Y || got.Z != v.Z {
			t.Errorf("vertex %d position: got (%g,%g,%g), want (%g,%g,%g)",
				i, got.X, got.Y, got.Z, v.X, v.Y, v.Z)
		}
		// The encoder writes V untouched (matches the game's top-down
		// convention) but the decoder flips on read, so the round-trip
		// produces 1-V. Compare against the expected flipped value.
		if got.U != v.U || got.V != 1-v.V {
			t.Errorf("vertex %d uv: got (%g,%g), want (%g,%g)", i, got.U, got.V, v.U, 1-v.V)
		}
	}
	for i, idx := range indices {
		if bms.Indices[i] != idx {
			t.Errorf("index %d: got %d, want %d", i, bms.Indices[i], idx)
		}
	}
	if bms.BBoxMin != bboxMin || bms.BBoxMax != bboxMax {
		t.Errorf("bbox: got %v..%v, want %v..%v", bms.BBoxMin, bms.BBoxMax, bboxMin, bboxMax)
	}
	if !bms.HasNavMesh {
		t.Fatalf("expected encoded BMS to include navmesh")
	}
	navOff := binary.LittleEndian.Uint32(data[40:44])
	unk9Off := binary.LittleEndian.Uint32(data[48:52])
	if unk9Off == 0 || unk9Off+4 != navOff {
		t.Fatalf("unknown09 offset: got 0x%x, nav offset 0x%x; want unknown09+4 == nav", unk9Off, navOff)
	}
	if len(bms.NavVertices) != 4 || len(bms.NavCells) != 2 {
		t.Fatalf("navmesh size: got %d vertices, %d cells; want 4 vertices, 2 cells", len(bms.NavVertices), len(bms.NavCells))
	}
}

func TestEncodeMinimalBMS_NavOffset(t *testing.T) {
	verts := []BMSVertex{
		{X: -1, Y: 0, Z: -1, U: 0, V: 0},
		{X: 1, Y: 0, Z: -1, U: 1, V: 0},
		{X: 1, Y: 0, Z: 1, U: 1, V: 1},
		{X: -1, Y: 0, Z: 1, U: 0, V: 1},
	}
	indices := []uint16{0, 1, 2, 0, 2, 3}
	bboxMin := [3]float32{-1, 0, -1}
	bboxMax := [3]float32{1, 0, 1}
	opts := BMSNavOptions{OffsetX: 5, OffsetZ: -7}

	data, err := EncodeMinimalBMSWithOptions("test", "matA", verts, indices, bboxMin, bboxMax, opts)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	bms, err := DecodeBMS(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if bms.BBoxMin != [3]float32{-1, 0, -8} || bms.BBoxMax != [3]float32{6, 0, 1} {
		t.Fatalf("expanded bbox: got %v..%v", bms.BBoxMin, bms.BBoxMax)
	}
	if !bms.HasNavMesh {
		t.Fatalf("expected encoded BMS to include navmesh")
	}
	if bms.NavBBoxMin != [3]float32{4, 0, -8} || bms.NavBBoxMax != [3]float32{6, 0, -6} {
		t.Fatalf("nav bbox: got %v..%v", bms.NavBBoxMin, bms.NavBBoxMax)
	}
	if len(bms.NavVertices) != 4 || len(bms.NavCells) != 2 || len(bms.NavOutlineEdges) != 4 || len(bms.NavInlineEdges) != 1 {
		t.Fatalf("navmesh size: got %d vertices, %d cells, %d outline edges, %d inline edges",
			len(bms.NavVertices), len(bms.NavCells), len(bms.NavOutlineEdges), len(bms.NavInlineEdges))
	}

	navVerts, gridOrigin := readEncodedNavTestData(t, data)
	want := [][2]float32{
		{4, -8},
		{6, -8},
		{6, -6},
		{4, -6},
	}
	for i := range want {
		if navVerts[i] != want[i] {
			t.Fatalf("nav vertex %d: got %v, want %v", i, navVerts[i], want[i])
		}
	}
	if gridOrigin != [2]float32{4, -8} {
		t.Fatalf("lookup origin: got %v", gridOrigin)
	}
}

func TestEncodeMinimalBMS_CustomFootprint(t *testing.T) {
	verts := []BMSVertex{
		{X: -5, Y: 0, Z: -5, U: 0, V: 0},
		{X: 5, Y: 0, Z: -5, U: 1, V: 0},
		{X: 5, Y: 0, Z: 5, U: 1, V: 1},
		{X: -5, Y: 0, Z: 5, U: 0, V: 1},
	}
	indices := []uint16{0, 1, 2, 0, 2, 3}
	bboxMin := [3]float32{-5, 0, -5}
	bboxMax := [3]float32{5, 10, 5}
	opts := BMSNavOptions{
		OffsetX: 10,
		OffsetZ: -3,
		Footprint: [][2]float32{
			{-2, -1},
			{2, -1},
			{3, 1},
			{0, 2},
			{-2, 1},
		},
	}

	data, err := EncodeMinimalBMSWithOptions("test", "matA", verts, indices, bboxMin, bboxMax, opts)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	bms, err := DecodeBMS(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(bms.NavVertices) != 5 || len(bms.NavCells) != 3 || len(bms.NavOutlineEdges) != 5 || len(bms.NavInlineEdges) != 2 {
		t.Fatalf("navmesh size: got %d vertices, %d cells, %d outline edges, %d inline edges",
			len(bms.NavVertices), len(bms.NavCells), len(bms.NavOutlineEdges), len(bms.NavInlineEdges))
	}
	if bms.NavBBoxMin != [3]float32{8, 0, -4} || bms.NavBBoxMax != [3]float32{13, 0, -1} {
		t.Fatalf("nav bbox: got %v..%v", bms.NavBBoxMin, bms.NavBBoxMax)
	}
	if bms.NavLookupOrigin != [2]float32{8, -4} {
		t.Fatalf("lookup origin: got %v", bms.NavLookupOrigin)
	}
	for i, edge := range bms.NavInlineEdges {
		if edge.Flag != 0x04 {
			t.Fatalf("inline edge %d flag: got 0x%02x, want 0x04", i, edge.Flag)
		}
	}
}

func TestEncodeMinimalBMS_StockLikeNavLookup(t *testing.T) {
	verts := []BMSVertex{
		{X: -140, Y: 0, Z: -100, U: 0, V: 0},
		{X: 140, Y: 0, Z: -100, U: 1, V: 0},
		{X: 140, Y: 0, Z: 120, U: 1, V: 1},
		{X: -140, Y: 0, Z: 120, U: 0, V: 1},
	}
	indices := []uint16{0, 1, 2, 0, 2, 3}
	bboxMin := [3]float32{-140, 0, -100}
	bboxMax := [3]float32{140, 10, 120}

	data, err := EncodeMinimalBMSWithOptions("test", "matA", verts, indices, bboxMin, bboxMax, BMSNavOptions{})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	bms, err := DecodeBMS(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if bms.NavLookupWidth != 3 || bms.NavLookupHeight != 3 {
		t.Fatalf("lookup size: got %dx%d, want 3x3", bms.NavLookupWidth, bms.NavLookupHeight)
	}
	if len(bms.NavInlineEdges) != 1 {
		t.Fatalf("inline edges: got %d, want 1", len(bms.NavInlineEdges))
	}
	if bms.NavInlineEdges[0].Flag != 0x04 {
		t.Fatalf("inline flag: got 0x%02x, want 0x04", bms.NavInlineEdges[0].Flag)
	}
	for i, edge := range bms.NavOutlineEdges {
		if edge.Flag != 0x03 {
			t.Fatalf("outline edge %d flag: got 0x%02x, want 0x03", i, edge.Flag)
		}
	}
}

func readEncodedNavTestData(t *testing.T, data []byte) ([][2]float32, [2]float32) {
	t.Helper()
	if len(data) < 44 {
		t.Fatal("encoded BMS too short")
	}
	navOff := int(binary.LittleEndian.Uint32(data[40:44]))
	if navOff <= 0 || navOff >= len(data) {
		t.Fatalf("bad nav offset %d", navOff)
	}
	pos := navOff
	readU32 := func() uint32 {
		if pos+4 > len(data) {
			t.Fatalf("unexpected EOF at %d", pos)
		}
		v := binary.LittleEndian.Uint32(data[pos : pos+4])
		pos += 4
		return v
	}
	readF32 := func() float32 {
		return math.Float32frombits(readU32())
	}
	readU16 := func() uint16 {
		if pos+2 > len(data) {
			t.Fatalf("unexpected EOF at %d", pos)
		}
		v := binary.LittleEndian.Uint16(data[pos : pos+2])
		pos += 2
		return v
	}
	readByte := func() {
		if pos+1 > len(data) {
			t.Fatalf("unexpected EOF at %d", pos)
		}
		pos++
	}

	vertexCount := int(readU32())
	if vertexCount != 4 {
		t.Fatalf("nav vertex count: got %d", vertexCount)
	}
	verts := make([][2]float32, vertexCount)
	for i := range verts {
		x := readF32()
		_ = readF32()
		z := readF32()
		readByte()
		verts[i] = [2]float32{x, z}
	}
	cellCount := int(readU32())
	pos += cellCount * 8
	outlineCount := int(readU32())
	pos += outlineCount * 9
	inlineCount := int(readU32())
	pos += inlineCount * 9
	origin := [2]float32{readF32(), readF32()}
	_ = readU32() // width
	_ = readU32() // height
	gridCells := int(readU32())
	for i := 0; i < gridCells; i++ {
		outlines := int(readU32())
		for j := 0; j < outlines; j++ {
			_ = readU16()
		}
	}
	return verts, origin
}
