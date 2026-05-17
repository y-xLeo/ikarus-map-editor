package editor

import (
	"math"
	"sort"

	"sromapedit/internal/sromap"
)

const o2BlockWorldSize = float32(sromap.RegionSize / sromap.MeshBlockCount)

type visibilityBlock struct {
	x, z, lod int
}

type visibilityObject struct {
	key      uint64
	template sromap.ObjectEntry
	blocks   map[visibilityBlock]bool
	lods     map[int]bool
}

// RepairO2Visibility expands map-side object records so every object is stored
// in the block buckets covered by its rotated visual bbox. These buckets drive
// client-side render visibility; missing buckets can make large bridges vanish
// when the camera turns even though the mesh is still in front of the camera.
func (s *Server) RepairO2Visibility(rx, ry int, assetFilter map[uint32]bool) (int, error) {
	path := sromap.O2Path(s.Root, rx, ry)
	o2, err := sromap.LoadO2(path)
	if err != nil {
		return 0, err
	}
	changed := s.addMissingO2VisibilityBlocks(o2, rx, ry, assetFilter)
	if changed == 0 {
		return 0, nil
	}
	if err := backupOnce(path); err != nil {
		return 0, err
	}
	if err := o2.Save(path); err != nil {
		return 0, err
	}
	_ = s.mirrorToExport(path)
	return changed, nil
}

func (s *Server) normalizeChangedO2Visibility(o2 *sromap.O2, rx, ry int, keys map[uint64]bool) int {
	if len(keys) == 0 {
		return 0
	}
	objects := s.collectVisibilityObjects(o2, keys, nil)
	if len(objects) == 0 {
		return 0
	}

	replacements := make(map[uint64][]sromap.ObjectEntry)
	for key, obj := range objects {
		asset, _ := s.objCache.get(obj.template.ObjID)
		desired := visibilityBlocksForEntry(obj.template, asset, rx, ry)
		if len(desired) == 0 {
			continue
		}
		if sameVisibilityBlocks(obj.blocks, desired, obj.lods) {
			continue
		}
		replacements[key] = visibilityEntries(obj.template, desired, obj.lods)
	}
	if len(replacements) == 0 {
		return 0
	}

	kept := o2.Entries[:0]
	for _, e := range o2.Entries {
		if _, ok := replacements[placementKey(e.RegionID, e.UID, e.ObjID)]; ok {
			continue
		}
		kept = append(kept, e)
	}
	keysSorted := make([]uint64, 0, len(replacements))
	for key := range replacements {
		keysSorted = append(keysSorted, key)
	}
	sort.Slice(keysSorted, func(i, j int) bool { return keysSorted[i] < keysSorted[j] })
	for _, key := range keysSorted {
		kept = append(kept, replacements[key]...)
	}
	o2.Entries = kept
	return len(replacements)
}

func (s *Server) addMissingO2VisibilityBlocks(o2 *sromap.O2, rx, ry int, assetFilter map[uint32]bool) int {
	objects := s.collectVisibilityObjects(o2, nil, assetFilter)
	added := 0
	for _, obj := range objects {
		asset, _ := s.objCache.get(obj.template.ObjID)
		desired := visibilityBlocksForEntry(obj.template, asset, rx, ry)
		for block := range desired {
			for lod := range obj.lods {
				k := visibilityBlock{x: block[0], z: block[1], lod: lod}
				if obj.blocks[k] {
					continue
				}
				e := obj.template
				e.XBlock = block[0]
				e.ZBlock = block[1]
				e.LODGroup = lod
				o2.Entries = append(o2.Entries, e)
				obj.blocks[k] = true
				added++
			}
		}
	}
	return added
}

func (s *Server) collectVisibilityObjects(o2 *sromap.O2, keys map[uint64]bool, assetFilter map[uint32]bool) map[uint64]*visibilityObject {
	objects := make(map[uint64]*visibilityObject)
	for _, e := range o2.Entries {
		if assetFilter != nil && !assetFilter[e.ObjID] {
			continue
		}
		key := placementKey(e.RegionID, e.UID, e.ObjID)
		if keys != nil && !keys[key] {
			continue
		}
		obj := objects[key]
		if obj == nil {
			obj = &visibilityObject{
				key:      key,
				template: e,
				blocks:   make(map[visibilityBlock]bool),
				lods:     make(map[int]bool),
			}
			objects[key] = obj
		}
		obj.blocks[visibilityBlock{x: e.XBlock, z: e.ZBlock, lod: e.LODGroup}] = true
		obj.lods[e.LODGroup] = true
	}
	return objects
}

func visibilityBlocksForEntry(e sromap.ObjectEntry, asset *objectAsset, fileX, fileY int) map[[2]int]bool {
	if asset == nil {
		return nil
	}
	minX, maxX, minZ, maxZ := placementVisualRotatedBBox(e, asset)
	ownerX := int(e.RegionID & 0x00ff)
	ownerY := int(e.RegionID >> 8)
	shiftX := float32(ownerX-fileX) * float32(sromap.RegionSize)
	shiftZ := float32(ownerY-fileY) * float32(sromap.RegionSize)
	minX += shiftX
	maxX += shiftX
	minZ += shiftZ
	maxZ += shiftZ

	if maxX <= 0 || minX >= sromap.RegionSize || maxZ <= 0 || minZ >= sromap.RegionSize {
		return nil
	}
	minX = maxF(minX, 0)
	maxX = minF(maxX, sromap.RegionSize)
	minZ = maxF(minZ, 0)
	maxZ = minF(maxZ, sromap.RegionSize)

	x0 := clampO2Block(int(math.Floor(float64(minX / o2BlockWorldSize))))
	x1 := clampO2Block(int(math.Floor(float64((maxX - 0.001) / o2BlockWorldSize))))
	z0 := clampO2Block(int(math.Floor(float64(minZ / o2BlockWorldSize))))
	z1 := clampO2Block(int(math.Floor(float64((maxZ - 0.001) / o2BlockWorldSize))))

	blocks := make(map[[2]int]bool, (x1-x0+1)*(z1-z0+1))
	for z := z0; z <= z1; z++ {
		for x := x0; x <= x1; x++ {
			blocks[[2]int{x, z}] = true
		}
	}
	return blocks
}

func placementVisualRotatedBBox(p sromap.ObjectEntry, asset *objectAsset) (minX, maxX, minZ, maxZ float32) {
	bmin := asset.BBoxMin
	bmax := asset.BBoxMax
	c := float32(math.Cos(float64(p.Yaw)))
	sn := float32(math.Sin(float64(p.Yaw)))
	minX, maxX = float32(math.MaxFloat32), float32(-math.MaxFloat32)
	minZ, maxZ = float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, cx := range [2]float32{bmin[0], bmax[0]} {
		for _, cz := range [2]float32{bmin[2], bmax[2]} {
			rx := c*cx - sn*cz + p.X
			rz := sn*cx + c*cz + p.Z
			if rx < minX {
				minX = rx
			}
			if rx > maxX {
				maxX = rx
			}
			if rz < minZ {
				minZ = rz
			}
			if rz > maxZ {
				maxZ = rz
			}
		}
	}
	return minX, maxX, minZ, maxZ
}

func sameVisibilityBlocks(got map[visibilityBlock]bool, desired map[[2]int]bool, lods map[int]bool) bool {
	wantCount := len(desired) * len(lods)
	if len(got) != wantCount {
		return false
	}
	for block := range desired {
		for lod := range lods {
			if !got[visibilityBlock{x: block[0], z: block[1], lod: lod}] {
				return false
			}
		}
	}
	return true
}

func visibilityEntries(template sromap.ObjectEntry, blocks map[[2]int]bool, lods map[int]bool) []sromap.ObjectEntry {
	blockList := make([][2]int, 0, len(blocks))
	for block := range blocks {
		blockList = append(blockList, block)
	}
	sort.Slice(blockList, func(i, j int) bool {
		if blockList[i][1] != blockList[j][1] {
			return blockList[i][1] < blockList[j][1]
		}
		return blockList[i][0] < blockList[j][0]
	})
	lodList := make([]int, 0, len(lods))
	for lod := range lods {
		lodList = append(lodList, lod)
	}
	sort.Ints(lodList)

	entries := make([]sromap.ObjectEntry, 0, len(blockList)*len(lodList))
	for _, block := range blockList {
		for _, lod := range lodList {
			e := template
			e.XBlock = block[0]
			e.ZBlock = block[1]
			e.LODGroup = lod
			entries = append(entries, e)
		}
	}
	return entries
}

func clampO2Block(v int) int {
	if v < 0 {
		return 0
	}
	if v >= sromap.MeshBlockCount {
		return sromap.MeshBlockCount - 1
	}
	return v
}
