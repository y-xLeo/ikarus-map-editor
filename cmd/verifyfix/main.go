// verifyfix: programmatically execute the default rebuild path with the
// min-overlap threshold and dump cell-by-cell ObjectIndices changes so we
// can verify the fix is producing the right output without needing the
// editor GUI / in-game testing.
package main

import (
	"fmt"
	"math"
	"os"
	"reflect"

	"sromapedit/internal/sromap"
)

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func main() {
	// Load the baseline (Mapeditor's stock 5c94)
	baseline, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		panic(err)
	}

	// Capture the pre-modification ObjectIndices to diff after
	preObj := make([][]uint16, len(baseline.Cells))
	for i, c := range baseline.Cells {
		preObj[i] = append([]uint16(nil), c.ObjectIndices...)
	}

	// Step 1: Append asset 3308 NVMObject (as addCustomNVMObjectsOnly would)
	newObjIdx := len(baseline.Objects)
	baseline.Objects = append(baseline.Objects, sromap.NVMObject{
		AssetID:  3308,
		X:        1752.40,
		Y:        206.21,
		Z:        1477.64,
		Yaw:      -1.5882,
		Type:     -1,
		UID:      29698,
		Short0:   0,
		IsBig:    false,
		IsStruct: false,
		RegionID: 0x5C94,
		Links:    nil,
	})
	fmt.Printf("Appended asset 3308 NVMObject at index %d, pos=(1752,1478)\n\n", newObjIdx)

	// Step 2: Compute rotated bbox (as addCustomBboxObjectIndices would)
	bmin := [3]float32{-150, 0, -142.38269}
	bmax := [3]float32{150, 246.11717, 142.38269}
	yaw := float32(-1.5882)
	c := float32(math.Cos(float64(yaw)))
	s := float32(math.Sin(float64(yaw)))
	rotMinX, rotMaxX := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	rotMinZ, rotMaxZ := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, cx := range [2]float32{bmin[0], bmax[0]} {
		for _, cz := range [2]float32{bmin[2], bmax[2]} {
			rx := c*cx - s*cz + 1752.40
			rz := s*cx + c*cz + 1477.64
			if rx < rotMinX {
				rotMinX = rx
			}
			if rx > rotMaxX {
				rotMaxX = rx
			}
			if rz < rotMinZ {
				rotMinZ = rz
			}
			if rz > rotMaxZ {
				rotMaxZ = rz
			}
		}
	}
	fmt.Printf("Asset 3308 rotated world bbox: X=(%.0f..%.0f) Z=(%.0f..%.0f)\n\n", rotMinX, rotMaxX, rotMinZ, rotMaxZ)

	// Step 3: Apply min-overlap-threshold ObjectIndices spread
	const minOverlap float32 = 20
	added, skippedTinyOverlap := 0, 0
	for ci := range baseline.Cells {
		cell := &baseline.Cells[ci]
		overlapX := minF(cell.MaxX, rotMaxX) - maxF(cell.MinX, rotMinX)
		overlapZ := minF(cell.MaxZ, rotMaxZ) - maxF(cell.MinZ, rotMinZ)
		if overlapX <= 0 || overlapZ <= 0 {
			continue // no overlap at all
		}
		if overlapX < minOverlap || overlapZ < minOverlap {
			skippedTinyOverlap++
			fmt.Printf("  SKIP cell %d AABB=(%.0f,%.0f)..(%.0f,%.0f) — overlap X=%.0f Z=%.0f (below %.0f)\n",
				ci, cell.MinX, cell.MinZ, cell.MaxX, cell.MaxZ, overlapX, overlapZ, minOverlap)
			continue
		}
		cell.ObjectIndices = append(cell.ObjectIndices, uint16(newObjIdx))
		added++
		fmt.Printf("  ADD  cell %d AABB=(%.0f,%.0f)..(%.0f,%.0f) — overlap X=%.0f Z=%.0f, ObjIdx now %v\n",
			ci, cell.MinX, cell.MinZ, cell.MaxX, cell.MaxZ, overlapX, overlapZ, cell.ObjectIndices)
	}

	fmt.Printf("\nResult: added asset 3308 (idx %d) to %d cells, skipped %d cells with overlap below %0.f units\n",
		newObjIdx, added, skippedTinyOverlap, minOverlap)

	// Print all changed cells
	fmt.Println("\n--- Verification ---")
	for i, c := range baseline.Cells {
		if !reflect.DeepEqual(c.ObjectIndices, preObj[i]) {
			fmt.Printf("  cell %d: %v -> %v (AABB %.0f,%.0f..%.0f,%.0f)\n",
				i, preObj[i], c.ObjectIndices, c.MinX, c.MinZ, c.MaxX, c.MaxZ)
		}
	}
}
