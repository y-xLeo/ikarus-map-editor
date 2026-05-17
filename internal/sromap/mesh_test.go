package sromap

import (
	"math"
	"testing"
)

func TestBrushWeight(t *testing.T) {
	b := Brush{CenterX: 100, CenterZ: 100, Radius: 100, Delta: 1, Falloff: "none"}
	if got := brushWeight(b, 200, 100); got != 1 {
		t.Fatalf("edge with none falloff = %v, want 1", got)
	}
	b.Falloff = "linear"
	if got := brushWeight(b, 200, 100); got != 0 {
		t.Fatalf("edge with linear falloff = %v, want 0", got)
	}
	if got := brushWeight(b, 100, 100); got != 1 {
		t.Fatalf("center with linear falloff = %v, want 1", got)
	}
}

func TestBlockAndVertexOffsets(t *testing.T) {
	if got := blockOffset(0, 0); got != 12 {
		t.Fatalf("blockOffset(0,0) = %d", got)
	}
	if got := blockOffset(5, 5) + MeshBlockSize; got != MeshExpectedSize {
		t.Fatalf("last block end = %d, want %d", got, MeshExpectedSize)
	}
	if got := vertexHeightOffset(12, 0, 0); got != 18 {
		t.Fatalf("first vertex height offset = %d, want 18", got)
	}
}

func TestApplyBrushAllGeneratedMesh(t *testing.T) {
	data := make([]byte, MeshExpectedSize)
	copy(data, []byte(meshSignature))
	mesh := &Mesh{Data: data}
	mesh.RecalculateBlockBounds()

	report, err := mesh.ApplyBrush(Brush{All: true, Delta: 3})
	if err != nil {
		t.Fatal(err)
	}
	if report.UniqueVerticesChanged != MeshGridSize*MeshGridSize {
		t.Fatalf("unique changed = %d", report.UniqueVerticesChanged)
	}
	stats := mesh.Stats()
	if math.Abs(float64(stats.MinHeight-3)) > 0.0001 || math.Abs(float64(stats.MaxHeight-3)) > 0.0001 {
		t.Fatalf("height range = %v..%v, want 3..3", stats.MinHeight, stats.MaxHeight)
	}
}
