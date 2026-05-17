// fullrebuildtest: load baseline, sync NVMObject 3308 to .o2 position,
// run ApplyNVMNavRebuild with our fixed 240x240 partition + bbox-spread
// ObjectIndices, then compare to the rebuild tool's output.
package main

import (
	"fmt"
	"math"
	"os"

	"sromapedit/internal/sromap"
)

func main() {
	// Load baseline (10 stock NVMObjects, no asset 3308)
	src := os.Args[1]
	n, err := sromap.LoadNVM(src)
	if err != nil {
		panic(err)
	}

	// Manually inject asset 3308 (custom) at the .o2 position
	n.Objects = append(n.Objects, sromap.NVMObject{
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

	// Treat all tiles as walkable (no slope check, no other blockers in this test)
	var walkable [sromap.NVMTotalTiles]bool
	for i := range walkable {
		walkable[i] = true
	}

	// Run full repartition with our fixed 240x240 partition
	sromap.ApplyNVMNavRebuild(n, walkable)

	// Manually spread ObjectIndices for asset 3308 (the custom one).
	// Mirror what addCustomBboxObjectIndices would do, with hardcoded bbox
	// since we don't have objCache here.
	objIdx := len(n.Objects) - 1 // asset 3308 is the last one
	obj := n.Objects[objIdx]
	bmin := [3]float32{-150, 0, -142.38269}
	bmax := [3]float32{150, 246.11717, 142.38269}
	c := float32(math.Cos(float64(obj.Yaw)))
	s := float32(math.Sin(float64(obj.Yaw)))
	rotMinX, rotMaxX := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	rotMinZ, rotMaxZ := float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, cx := range [2]float32{bmin[0], bmax[0]} {
		for _, cz := range [2]float32{bmin[2], bmax[2]} {
			rx := c*cx - s*cz + obj.X
			rz := s*cx + c*cz + obj.Z
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

	for ci := range n.Cells {
		cell := &n.Cells[ci]
		if cell.MinX >= rotMaxX || cell.MaxX <= rotMinX ||
			cell.MinZ >= rotMaxZ || cell.MaxZ <= rotMinZ {
			continue
		}
		already := false
		for _, idx := range cell.ObjectIndices {
			if int(idx) == objIdx {
				already = true
				break
			}
		}
		if already {
			continue
		}
		cell.ObjectIndices = append(cell.ObjectIndices, uint16(objIdx))
	}

	// Save and report
	out := os.Args[2]
	if err := n.Save(out); err != nil {
		panic(err)
	}

	fmt.Printf("=== %s → %s ===\n", src, out)
	fmt.Printf("NVMObjects=%d Cells=%d (open=%d closed=%d) IntEdges=%d\n",
		len(n.Objects), len(n.Cells), n.OpenCellCount, uint32(len(n.Cells))-n.OpenCellCount,
		len(n.InternalEdges))
	fmt.Printf("Asset 3308 NVMObject pos=(%.2f, %.2f, %.2f) yaw=%.4f\n", obj.X, obj.Y, obj.Z, obj.Yaw)
	fmt.Printf("Rotated world bbox: X=(%.0f..%.0f) Z=(%.0f..%.0f)\n", rotMinX, rotMaxX, rotMinZ, rotMaxZ)
	fmt.Println("\nCells with ObjectIndices=[9]:")
	for ci, c := range n.Cells {
		for _, idx := range c.ObjectIndices {
			if int(idx) == objIdx {
				kind := "OPEN"
				if uint32(ci) >= n.OpenCellCount {
					kind = "CLOSED"
				}
				fmt.Printf("  cell %d (%s): AABB=(%.0f,%.0f)..(%.0f,%.0f) size=%.0fx%.0f\n",
					ci, kind, c.MinX, c.MinZ, c.MaxX, c.MaxZ, c.MaxX-c.MinX, c.MaxZ-c.MinZ)
				break
			}
		}
	}
}
