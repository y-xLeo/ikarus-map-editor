package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"sromapedit/internal/sromap"
)

type objectKey struct {
	assetID  uint32
	uid      int16
	regionID uint16
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: nvmpreservemetadata <reference-navmesh-dir> <target-navmesh-dir> [<target-navmesh-dir>...]")
		os.Exit(2)
	}
	refDir := os.Args[1]
	refFiles, err := filepath.Glob(filepath.Join(refDir, "nv_*.nvm"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "glob:", err)
		os.Exit(1)
	}
	if len(refFiles) == 0 {
		fmt.Fprintln(os.Stderr, "no reference nv_*.nvm files in", refDir)
		os.Exit(1)
	}

	totalChanged := 0
	for _, targetDir := range os.Args[2:] {
		for _, refPath := range refFiles {
			targetPath := filepath.Join(targetDir, filepath.Base(refPath))
			if _, err := os.Stat(targetPath); err != nil {
				continue
			}
			changed, err := repairOne(refPath, targetPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", targetPath, err)
				os.Exit(1)
			}
			if changed {
				totalChanged++
			}
		}
	}
	fmt.Printf("updated %d NVM file(s)\n", totalChanged)
}

func repairOne(refPath, targetPath string) (bool, error) {
	ref, err := sromap.LoadNVM(refPath)
	if err != nil {
		return false, fmt.Errorf("load reference: %w", err)
	}
	target, err := sromap.LoadNVM(targetPath)
	if err != nil {
		return false, fmt.Errorf("load target: %w", err)
	}

	changed := false
	for i := range target.Objects {
		if len(target.Objects[i].Links) != 0 {
			target.Objects[i].Links = nil
			changed = true
		}
	}

	refByKey, refDup := indexObjects(ref.Objects)
	targetByKey, targetDup := indexObjects(target.Objects)
	matched := make(map[int]int)
	for key, refIdx := range refByKey {
		if refDup[key] || targetDup[key] {
			continue
		}
		targetIdx, ok := targetByKey[key]
		if !ok || !samePlacement(ref.Objects[refIdx], target.Objects[targetIdx]) {
			continue
		}
		src := ref.Objects[refIdx]
		dst := &target.Objects[targetIdx]
		if dst.Type != src.Type || dst.Short0 != src.Short0 || dst.IsBig != src.IsBig || dst.IsStruct != src.IsStruct {
			dst.Type = src.Type
			dst.Short0 = src.Short0
			dst.IsBig = src.IsBig
			dst.IsStruct = src.IsStruct
			changed = true
		}
		matched[refIdx] = targetIdx
	}

	for refIdx, targetIdx := range matched {
		links := remapLinks(ref.Objects[refIdx].Links, len(ref.Objects), matched)
		if !linksEqual(target.Objects[targetIdx].Links, links) {
			target.Objects[targetIdx].Links = links
			changed = true
		}
	}
	if !changed {
		return false, nil
	}
	if err := target.Save(targetPath); err != nil {
		return false, fmt.Errorf("save target: %w", err)
	}
	fmt.Printf("%s: repaired object metadata from %s\n", targetPath, refPath)
	return true, nil
}

func indexObjects(objects []sromap.NVMObject) (map[objectKey]int, map[objectKey]bool) {
	byKey := make(map[objectKey]int, len(objects))
	duplicates := make(map[objectKey]bool)
	for i, obj := range objects {
		key := objectKey{assetID: obj.AssetID, uid: obj.UID, regionID: obj.RegionID}
		if _, ok := byKey[key]; ok {
			duplicates[key] = true
			continue
		}
		byKey[key] = i
	}
	return byKey, duplicates
}

func remapLinks(refLinks []sromap.NVMObjectLink, refObjectCount int, matched map[int]int) []sromap.NVMObjectLink {
	if len(refLinks) == 0 {
		return nil
	}
	out := make([]sromap.NVMObjectLink, 0, len(refLinks))
	for _, link := range refLinks {
		refLinkedIdx := int(link.LinkedObjectID)
		if refLinkedIdx < 0 || refLinkedIdx >= refObjectCount {
			continue
		}
		targetLinkedIdx, ok := matched[refLinkedIdx]
		if !ok {
			continue
		}
		out = append(out, sromap.NVMObjectLink{
			LinkedObjectID: int16(targetLinkedIdx),
			LinkedEdgeID:   link.LinkedEdgeID,
			EdgeID:         link.EdgeID,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func linksEqual(a, b []sromap.NVMObjectLink) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func samePlacement(a, b sromap.NVMObject) bool {
	const positionTolerance = float32(0.05)
	const heightTolerance = float32(0.25)
	const yawTolerance = float32(0.001)
	return abs32(a.X-b.X) <= positionTolerance &&
		abs32(a.Y-b.Y) <= heightTolerance &&
		abs32(a.Z-b.Z) <= positionTolerance &&
		angleDelta(a.Yaw, b.Yaw) <= yawTolerance
}

func angleDelta(a, b float32) float32 {
	const twoPi = 2 * math.Pi
	d := math.Mod(float64(a-b), twoPi)
	if d < 0 {
		d = -d
	}
	if d > math.Pi {
		d = twoPi - d
	}
	return float32(d)
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
