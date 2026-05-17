package editor

import (
	"math"

	"sromapedit/internal/sromap"
)

// Default slope threshold: 0 disables the slope check entirely. When enabled,
// the threshold is measured in degrees. 60 degrees matches the map-only NVMs
// produced by Etherial Editor for the current test map set.
const DefaultNVMSlopeThreshold = 60.0

// computeWalkability returns a 96x96 walkability grid for the region.
// A tile is non-walkable if either:
//   - slopeThreshold > 0 and the tile slope exceeds it, or
//   - it falls inside any object placement's rotated XZ-AABB.
func (s *Server) computeWalkability(mesh *sromap.Mesh, placements []sromap.ObjectEntry, slopeThreshold float32) [sromap.NVMTotalTiles]bool {
	var walkable [sromap.NVMTotalTiles]bool
	for i := range walkable {
		walkable[i] = true
	}
	heights := mesh.UniqueHeightMap()

	const tileCount = sromap.NVMTileCount

	// Slope per tile (the tile spans grid points (ti, tj) to (ti+1, tj+1)).
	if slopeThreshold > 0 {
		for tj := 0; tj < tileCount; tj++ {
			for ti := 0; ti < tileCount; ti++ {
				if terrainTileSlopeDegrees(heights, ti, tj) > slopeThreshold {
					walkable[tj*tileCount+ti] = false
				}
			}
		}
	}

	// Object XZ-AABB (rotated by yaw around Y).
	for _, p := range placements {
		asset, _ := s.objCache.get(p.ObjID)
		if asset == nil {
			continue
		}
		var bmin, bmax [3]float32
		if asset.HasCollision {
			bmin = asset.CollisionBBoxMin
			bmax = asset.CollisionBBoxMax
		} else {
			bmin = asset.BBoxMin
			bmax = asset.BBoxMax
		}
		c := float32(math.Cos(float64(p.Yaw)))
		sn := float32(math.Sin(float64(p.Yaw)))
		rotMinX, rotMaxX := float32(math.MaxFloat32), float32(-math.MaxFloat32)
		rotMinZ, rotMaxZ := float32(math.MaxFloat32), float32(-math.MaxFloat32)
		for _, cx := range [2]float32{bmin[0], bmax[0]} {
			for _, cz := range [2]float32{bmin[2], bmax[2]} {
				rx := c*cx - sn*cz + p.X
				rz := sn*cx + c*cz + p.Z
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
		iMin := clampTile(int(rotMinX / sromap.NVMTileSize))
		iMax := clampTile(int(rotMaxX / sromap.NVMTileSize))
		jMin := clampTile(int(rotMinZ / sromap.NVMTileSize))
		jMax := clampTile(int(rotMaxZ / sromap.NVMTileSize))
		for tj := jMin; tj <= jMax; tj++ {
			for ti := iMin; ti <= iMax; ti++ {
				walkable[tj*tileCount+ti] = false
			}
		}
	}
	return walkable
}

func terrainTileSlopeDegrees(heights [sromap.MeshGridSize * sromap.MeshGridSize]float32, x, z int) float32 {
	h00 := heights[z*sromap.MeshGridSize+x]
	h10 := heights[z*sromap.MeshGridSize+x+1]
	h01 := heights[(z+1)*sromap.MeshGridSize+x]
	h11 := heights[(z+1)*sromap.MeshGridSize+x+1]
	const edge = float32(sromap.NVMTileSize)
	diag := edge * float32(math.Sqrt2)
	maxRise := float32(0)
	for _, rise := range [6]float32{
		absF(h10-h00) / edge,
		absF(h11-h01) / edge,
		absF(h01-h00) / edge,
		absF(h11-h10) / edge,
		absF(h11-h00) / diag,
		absF(h01-h10) / diag,
	} {
		if rise > maxRise {
			maxRise = rise
		}
	}
	return float32(math.Atan(float64(maxRise)) * 180 / math.Pi)
}

func absF(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (s *Server) loadNavRebuildPlacements(rx, ry int) []sromap.ObjectEntry {
	targetMin := float32(0)
	targetMax := float32(sromap.RegionSize)
	seen := make(map[uint64]bool)
	var placements []sromap.ObjectEntry

	for oy := ry - 1; oy <= ry+1; oy++ {
		if oy < 0 || oy > 127 {
			continue
		}
		for ox := rx - 1; ox <= rx+1; ox++ {
			if ox < 0 || ox > 255 {
				continue
			}
			sourceRegionID := uint16(oy)<<8 | uint16(ox)
			o2, err := sromap.LoadO2(sromap.O2Path(s.Root, ox, oy))
			if err != nil {
				continue
			}

			for _, e := range o2.Entries {
				// The owner region's .o2 entry is canonical. Host-block
				// duplicates in neighbor files should not drive NVM sync.
				if e.RegionID != sourceRegionID {
					continue
				}
				key := placementKey(e.RegionID, e.UID, e.ObjID)
				if seen[key] {
					continue
				}
				seen[key] = true

				asset, _ := s.objCache.get(e.ObjID)
				if asset == nil {
					continue
				}

				p := e
				ownerX := int(p.RegionID & 0x00ff)
				ownerY := int(p.RegionID >> 8)
				p.X += float32(ownerX-rx) * float32(sromap.RegionSize)
				p.Z += float32(ownerY-ry) * float32(sromap.RegionSize)

				minX, maxX, minZ, maxZ := placementRotatedBBox(p, asset)
				if maxX <= targetMin || minX >= targetMax || maxZ <= targetMin || minZ >= targetMax {
					continue
				}
				placements = append(placements, p)
			}
		}
	}
	return placements
}

func placementKey(regionID uint16, uid int16, objID uint32) uint64 {
	return uint64(regionID)<<48 | uint64(uint16(uid))<<32 | uint64(objID)
}

func loosePlacementKey(uid int16, objID uint32) uint64 {
	return uint64(uint16(uid))<<32 | uint64(objID)
}

func placementRotatedBBox(p sromap.ObjectEntry, asset *objectAsset) (minX, maxX, minZ, maxZ float32) {
	var bmin, bmax [3]float32
	if asset.HasCollision {
		bmin = asset.CollisionBBoxMin
		bmax = asset.CollisionBBoxMax
	} else {
		bmin = asset.BBoxMin
		bmax = asset.BBoxMax
	}
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

// addCustomNVMObjectsOnly synchronizes the NVMObject array with the latest
// .o2 placements for *custom* (user-imported, `res\custom\…`) assets:
//
//   - If a custom placement has no matching NVMObject (by UID+AssetID), append one.
//   - If a custom placement HAS a matching NVMObject, UPDATE its position+yaw to
//     match the .o2 — otherwise moving a placement in the editor leaves the
//     NVMObject pointing at the old location (the bug we found vs the rebuild
//     tool: our broken nvm had asset 3308 at (1543,1508) while .o2 already
//     said (1752,1478)).
//
// Stock NVMObjects are not touched — their positions are baked correctly.
func (s *Server) addCustomNVMObjectsOnly(nvm *sromap.NVM, placements []sromap.ObjectEntry) (added int) {
	// Index existing NVMObjects by (UID, AssetID) → array index.
	existingExact := make(map[uint64]int, len(nvm.Objects))
	existingLoose := make(map[uint64]int, len(nvm.Objects))
	looseAmbiguous := make(map[uint64]bool)
	for i, o := range nvm.Objects {
		existingExact[placementKey(o.RegionID, o.UID, o.AssetID)] = i
		k := loosePlacementKey(o.UID, o.AssetID)
		if _, ok := existingLoose[k]; ok {
			looseAmbiguous[k] = true
			continue
		}
		existingLoose[k] = i
	}

	wantedCustom := make(map[uint64]bool)
	for _, p := range placements {
		asset, _ := s.objCache.get(p.ObjID)
		if asset == nil || !asset.HasCollision {
			continue
		}
		if !pathLooksCustom(asset.Source) {
			continue
		}
		exactKey := placementKey(p.RegionID, p.UID, p.ObjID)
		wantedCustom[exactKey] = true
		looseKey := loosePlacementKey(p.UID, p.ObjID)
		idx, ok := existingExact[exactKey]
		if !ok && !looseAmbiguous[looseKey] {
			idx, ok = existingLoose[looseKey]
		}
		if ok {
			// UPDATE position/yaw if it drifted from the .o2.
			o := &nvm.Objects[idx]
			if o.X != p.X || o.Y != p.Y || o.Z != p.Z || o.Yaw != p.Yaw || o.RegionID != p.RegionID || o.IsBig != p.Big || o.IsStruct != p.Struct {
				o.X, o.Y, o.Z = p.X, p.Y, p.Z
				o.Yaw = p.Yaw
				o.RegionID = p.RegionID
				o.IsBig = p.Big
				o.IsStruct = p.Struct
				added++ // count as a sync
			}
			existingExact[exactKey] = idx
			continue
		}
		nvm.Objects = append(nvm.Objects, sromap.NVMObject{
			AssetID:  p.ObjID,
			X:        p.X,
			Y:        p.Y,
			Z:        p.Z,
			Yaw:      p.Yaw,
			Type:     -1,
			UID:      p.UID,
			Short0:   0,
			IsBig:    p.Big,
			IsStruct: p.Struct,
			RegionID: p.RegionID,
			Links:    nil,
		})
		existingExact[exactKey] = len(nvm.Objects) - 1
		if _, ok := existingLoose[looseKey]; ok {
			looseAmbiguous[looseKey] = true
		} else {
			existingLoose[looseKey] = len(nvm.Objects) - 1
		}
		added++
	}
	added += s.removeStaleCustomNVMObjects(nvm, wantedCustom)
	return
}

func (s *Server) removeStaleCustomNVMObjects(nvm *sromap.NVM, wanted map[uint64]bool) int {
	if len(nvm.Objects) == 0 {
		return 0
	}
	remap := make([]int, len(nvm.Objects))
	for i := range remap {
		remap[i] = -1
	}
	kept := make([]sromap.NVMObject, 0, len(nvm.Objects))
	removed := 0
	for oldIdx, obj := range nvm.Objects {
		asset, _ := s.objCache.get(obj.AssetID)
		isCustom := asset != nil && asset.HasCollision && pathLooksCustom(asset.Source)
		if isCustom && !wanted[placementKey(obj.RegionID, obj.UID, obj.AssetID)] {
			removed++
			continue
		}
		remap[oldIdx] = len(kept)
		kept = append(kept, obj)
	}
	if removed == 0 {
		return 0
	}
	nvm.Objects = kept
	remapCellObjectIndices(nvm, remap)
	return removed
}

func remapCellObjectIndices(nvm *sromap.NVM, remap []int) {
	for ci := range nvm.Cells {
		out := nvm.Cells[ci].ObjectIndices[:0]
		for _, idx := range nvm.Cells[ci].ObjectIndices {
			oldIdx := int(idx)
			if oldIdx < 0 || oldIdx >= len(remap) {
				continue
			}
			newIdx := remap[oldIdx]
			if newIdx < 0 || newIdx > 0xffff {
				continue
			}
			out = append(out, uint16(newIdx))
		}
		nvm.Cells[ci].ObjectIndices = out
	}
}

func (s *Server) clearCustomObjectCellIndices(nvm *sromap.NVM) {
	if len(nvm.Objects) == 0 {
		return
	}
	custom := make([]bool, len(nvm.Objects))
	for i, obj := range nvm.Objects {
		asset, _ := s.objCache.get(obj.AssetID)
		custom[i] = asset != nil && asset.HasCollision && pathLooksCustom(asset.Source)
	}
	for ci := range nvm.Cells {
		out := nvm.Cells[ci].ObjectIndices[:0]
		for _, idx := range nvm.Cells[ci].ObjectIndices {
			if int(idx) >= len(custom) || !custom[idx] {
				out = append(out, idx)
			}
		}
		nvm.Cells[ci].ObjectIndices = out
	}
}

// addCustomBboxObjectIndices is the post-repartition step that wires custom
// NVMObjects into the cell graph. For each NVMObject backed by a *custom*
// asset, it computes the rotated world bbox and appends the NVMObject's
// index to the `ObjectIndices` of every cell whose AABB intersects that
// bbox. This matches the rebuild tool's pattern (asset 3308 at (1752,1478)
// got `ObjectIndices=[9]` on the four 240×240 cells covering its footprint).
//
// Stock NVMObjects are left alone — their cell associations were already
// set by ApplyNVMNavRebuild's point-based logic, which is fine for the
// stock baked positions.
func (s *Server) addCustomBboxObjectIndices(nvm *sromap.NVM) {
	s.clearCustomObjectCellIndices(nvm)
	for objIdx, obj := range nvm.Objects {
		asset, _ := s.objCache.get(obj.AssetID)
		if asset == nil || !asset.HasCollision {
			continue
		}
		if !pathLooksCustom(asset.Source) {
			continue
		}
		bmin := asset.CollisionBBoxMin
		bmax := asset.CollisionBBoxMax
		c := float32(math.Cos(float64(obj.Yaw)))
		sn := float32(math.Sin(float64(obj.Yaw)))
		rotMinX, rotMaxX := float32(math.MaxFloat32), float32(-math.MaxFloat32)
		rotMinZ, rotMaxZ := float32(math.MaxFloat32), float32(-math.MaxFloat32)
		for _, cx := range [2]float32{bmin[0], bmax[0]} {
			for _, cz := range [2]float32{bmin[2], bmax[2]} {
				rx := c*cx - sn*cz + obj.X
				rz := sn*cx + c*cz + obj.Z
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

		// Min overlap threshold: a cell needs at least MinOverlap world units
		// of overlap with the asset's bbox on EACH axis before we link it.
		// Without this, a cell that only clips a few units of the bbox (e.g.,
		// cell 24 in the baseline 5c94 baseline, which overlapped asset 3308's
		// bbox by only 10 units in Z) gets the ObjectIndex appended and
		// confuses the engine into treating the whole cell as the asset's
		// footprint — breaking the navmesh around it.
		const minOverlap float32 = 20 // = one tile width
		for ci := range nvm.Cells {
			cell := &nvm.Cells[ci]
			// AABB-vs-AABB overlap (XZ) with min-overlap requirement.
			overlapX := minF(cell.MaxX, rotMaxX) - maxF(cell.MinX, rotMinX)
			overlapZ := minF(cell.MaxZ, rotMaxZ) - maxF(cell.MinZ, rotMinZ)
			if overlapX < minOverlap || overlapZ < minOverlap {
				continue
			}
			// Skip if already present (rare with our flow but safe).
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
			if len(cell.ObjectIndices) >= 255 {
				continue
			}
			cell.ObjectIndices = append(cell.ObjectIndices, uint16(objIdx))
		}
	}
}

// addCustomObjectWithCollision is the FULL collision-add path for each
// custom placement (a .o2 entry whose asset has collision but no baked
// NVMObject). For each one we:
//
//  1. Append an NVMObject pointing to the custom AssetID at world pos
//  2. Append a closed cell at the asset's footprint AABB
//     - Its `ObjectIndices` references the NEW NVMObject's index (key!)
//  3. Redirect tiles inside the AABB to point at the closed cell
//  4. Set Flag bit 0 on those tiles
//  5. Append per-tile wall edges around the perimeter
//
// Hypothesis (verified by reading the server binary's RTNavMeshObj.cpp
// strings — `(*itCellID) < Cells.size()`, `edge.GetLinkedCellCount() == 2`,
// `mapVLinks`, `EdgeIdDatas`): the SERVER builds `CRTNavMeshObj` (the
// per-asset collision template) AT BOOT by walking each region's .nvm and
// grouping cells/edges by which NVMObject their `ObjectIndices` reference.
// Our previous pNMI crashes were because we added the NVMObject for asset
// 3308 with NO cells back-referencing its index, so `CRTNavMeshObj` for
// 3308 came out null, which made `SNavMeshInst` null.
//
// This combined function fixes the chain: NVMObject + cells with
// `ObjectIndices=[newObjIdx]` + wall edges → `CRTNavMeshObj` builds from
// our cells → `SNavMeshInst` builds from the NVMObject + that template.
func (s *Server) addCustomObjectWithCollision(nvm *sromap.NVM, placements []sromap.ObjectEntry, rx, ry int) (added int) {
	have := make(map[uint64]bool, len(nvm.Objects))
	for _, o := range nvm.Objects {
		key := placementKey(o.RegionID, o.UID, o.AssetID)
		have[key] = true
	}

	for _, p := range placements {
		asset, _ := s.objCache.get(p.ObjID)
		if asset == nil || !asset.HasCollision {
			continue
		}
		// Only inject collision for genuinely custom (user-imported) assets.
		// Stock assets like asset 972 already have a CRTNavMeshObj built
		// from neighbor regions' baked NVMObjects — adding our own NVMObject
		// + cell for them would re-trigger the "Load Fail(NavMesh Obj)"
		// chain we just spent hours clearing out. We identify custom by the
		// object.ifo path containing "custom" (matches the layout we write
		// when exporting OBJ-imported objects: res\custom\<slug>\…).
		if !pathLooksCustom(asset.Source) {
			continue
		}
		key := placementKey(p.RegionID, p.UID, p.ObjID)
		if have[key] {
			continue
		}

		// ── 1. Append the NVMObject FIRST, capture its index. ──
		newObjIdx := uint16(len(nvm.Objects))
		nvm.Objects = append(nvm.Objects, sromap.NVMObject{
			AssetID:  p.ObjID,
			X:        p.X,
			Y:        p.Y,
			Z:        p.Z,
			Yaw:      p.Yaw,
			Type:     -1,
			UID:      p.UID,
			Short0:   0,
			IsBig:    p.Big,
			IsStruct: p.Struct,
			RegionID: p.RegionID,
			Links:    nil,
		})
		have[key] = true

		// ── 2. Compute the rotated XZ AABB of the collision mesh. ──
		c := float32(math.Cos(float64(p.Yaw)))
		sn := float32(math.Sin(float64(p.Yaw)))
		bmin := asset.CollisionBBoxMin
		bmax := asset.CollisionBBoxMax
		rotMinX, rotMaxX := float32(math.MaxFloat32), float32(-math.MaxFloat32)
		rotMinZ, rotMaxZ := float32(math.MaxFloat32), float32(-math.MaxFloat32)
		for _, cx := range [2]float32{bmin[0], bmax[0]} {
			for _, cz := range [2]float32{bmin[2], bmax[2]} {
				rx := c*cx - sn*cz + p.X
				rz := sn*cx + c*cz + p.Z
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
		tile := float32(sromap.NVMTileSize)
		alignedMinX := float32(math.Floor(float64(rotMinX/tile))) * tile
		alignedMinZ := float32(math.Floor(float64(rotMinZ/tile))) * tile
		alignedMaxX := float32(math.Ceil(float64(rotMaxX/tile))) * tile
		alignedMaxZ := float32(math.Ceil(float64(rotMaxZ/tile))) * tile

		// ── 3. Closed cell whose ObjectIndices points at our new NVMObject. ──
		newCellIdx := int32(len(nvm.Cells))
		nvm.Cells = append(nvm.Cells, sromap.NVMCell{
			MinX:          alignedMinX,
			MinZ:          alignedMinZ,
			MaxX:          alignedMaxX,
			MaxZ:          alignedMaxZ,
			ObjectIndices: []uint16{newObjIdx},
		})

		// Tile bounds.
		iMin := clampTile(int(alignedMinX / tile))
		iMax := clampTile(int(alignedMaxX/tile) - 1)
		jMin := clampTile(int(alignedMinZ / tile))
		jMax := clampTile(int(alignedMaxZ/tile) - 1)

		// ── 4. Redirect tiles to the closed cell + set Flag bit 0. ──
		for tj := jMin; tj <= jMax; tj++ {
			for ti := iMin; ti <= iMax; ti++ {
				idx := tj*sromap.NVMTileCount + ti
				nvm.Tiles[idx].CellID = newCellIdx
				nvm.Tiles[idx].Flag |= 1
			}
		}

		// ── 5. Wall edges + adjacent-cell ObjectIndices ──
		//
		// In the baseline NVM, every CLOSED cell that wraps an object's
		// footprint AND every nearby OPEN cell carry the same NVMObject
		// index in their `ObjectIndices`. We replicate that pattern: as
		// we walk the wall perimeter, we also accumulate the unique set
		// of adjacent open cells and append our newObjIdx to each. This
		// is the "object group" the server uses to assemble a complete
		// CRTNavMeshObj for the asset.
		const (
			wallFlag     uint8 = 0x02
			dirFromSouth uint8 = 0
			dirFromWest  uint8 = 1
			dirFromNorth uint8 = 2
			dirFromEast  uint8 = 3
		)
		adjacentOpenCells := map[int16]bool{}
		addWall := func(minX, minZ, maxX, maxZ float32, dir uint8, neighbourTile int) {
			if neighbourTile < 0 || neighbourTile >= sromap.NVMTotalTiles {
				return
			}
			cell0 := nvm.Tiles[neighbourTile].CellID
			if cell0 < 0 || uint32(cell0) >= nvm.OpenCellCount {
				return
			}
			nvm.InternalEdges = append(nvm.InternalEdges, sromap.NVMInternalEdge{
				MinX: minX, MinZ: minZ, MaxX: maxX, MaxZ: maxZ,
				Flag: wallFlag, Dir0: dir, Dir1: 0xFF,
				Cell0: int16(cell0), Cell1: -1,
			})
			adjacentOpenCells[int16(cell0)] = true
		}
		for ti := iMin; ti <= iMax; ti++ {
			tx := float32(ti) * tile
			if jMin > 0 {
				addWall(tx, alignedMinZ, tx+tile, alignedMinZ, dirFromSouth, (jMin-1)*sromap.NVMTileCount+ti)
			}
			if jMax+1 < sromap.NVMTileCount {
				addWall(tx, alignedMaxZ, tx+tile, alignedMaxZ, dirFromNorth, (jMax+1)*sromap.NVMTileCount+ti)
			}
		}
		for tj := jMin; tj <= jMax; tj++ {
			tz := float32(tj) * tile
			if iMin > 0 {
				addWall(alignedMinX, tz, alignedMinX, tz+tile, dirFromWest, tj*sromap.NVMTileCount+(iMin-1))
			}
			if iMax+1 < sromap.NVMTileCount {
				addWall(alignedMaxX, tz, alignedMaxX, tz+tile, dirFromEast, tj*sromap.NVMTileCount+(iMax+1))
			}
		}

		// ── 6. Append our NVMObject index to every adjacent open cell's
		// ObjectIndices, matching the baseline "object group" pattern. ──
		for cellIdx := range adjacentOpenCells {
			if int(cellIdx) < 0 || int(cellIdx) >= len(nvm.Cells) {
				continue
			}
			already := false
			for _, idx := range nvm.Cells[cellIdx].ObjectIndices {
				if idx == newObjIdx {
					already = true
					break
				}
			}
			if !already {
				nvm.Cells[cellIdx].ObjectIndices = append(nvm.Cells[cellIdx].ObjectIndices, newObjIdx)
			}
		}

		added++
	}
	return
}

// addCustomCollisionCells injects "terrain-style" closed cells into the
// navmesh for each placement whose asset has collision but doesn't already
// have a matching NVMObject entry. This is how custom (user-imported)
// objects get server-side collision without an NVMObject (NVMObject + a
// custom AssetID crashes the server with `pNMI` null because the engine
// has no baked NavMeshInstance for that asset).
//
// Pattern observed in baseline 5c94: closed cells with `ObjectIndices=[]`
// (terrain-only) successfully block player movement. We append one such
// closed cell per custom placement, sized to the asset's collision AABB,
// and point every tile inside the AABB to that new closed cell. The
// existing baked NVMObjects, cells and edges are left completely untouched.
func (s *Server) addCustomCollisionCells(nvm *sromap.NVM, placements []sromap.ObjectEntry) (addedCells int) {
	// Build the set of (UID, AssetID) for which an NVMObject already exists
	// — those are baked-in and we don't touch them.
	have := make(map[uint64]bool, len(nvm.Objects))
	for _, o := range nvm.Objects {
		key := uint64(uint16(o.UID))<<32 | uint64(o.AssetID)
		have[key] = true
	}

	for _, p := range placements {
		asset, _ := s.objCache.get(p.ObjID)
		if asset == nil || !asset.HasCollision {
			continue
		}
		key := uint64(uint16(p.UID))<<32 | uint64(p.ObjID)
		if have[key] {
			continue // baked-in placement; original closed cells already exist
		}

		// Rotated XZ AABB of the asset, centred at the placement's world pos.
		c := float32(math.Cos(float64(p.Yaw)))
		sn := float32(math.Sin(float64(p.Yaw)))
		bmin := asset.CollisionBBoxMin
		bmax := asset.CollisionBBoxMax
		rotMinX, rotMaxX := float32(math.MaxFloat32), float32(-math.MaxFloat32)
		rotMinZ, rotMaxZ := float32(math.MaxFloat32), float32(-math.MaxFloat32)
		for _, cx := range [2]float32{bmin[0], bmax[0]} {
			for _, cz := range [2]float32{bmin[2], bmax[2]} {
				rx := c*cx - sn*cz + p.X
				rz := sn*cx + c*cz + p.Z
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

		// Snap to tile grid (so the cell aligns with the tile boundaries the
		// game baker uses for terrain-only closed cells).
		tile := float32(sromap.NVMTileSize)
		alignedMinX := float32(math.Floor(float64(rotMinX/tile))) * tile
		alignedMinZ := float32(math.Floor(float64(rotMinZ/tile))) * tile
		alignedMaxX := float32(math.Ceil(float64(rotMaxX/tile))) * tile
		alignedMaxZ := float32(math.Ceil(float64(rotMaxZ/tile))) * tile

		// Append a closed cell that references an EXISTING baked NVMObject.
		// Experiment: terrain-style (ObjectIndices=[]) cells didn't enforce
		// collision in our last test, so try pointing at obj 0 — every
		// baseline closed cell with an NVMObject reference DOES block, so
		// this borrows a valid NMI without actually adding one.
		newCellIdx := int32(len(nvm.Cells))
		objRef := []uint16{}
		if len(nvm.Objects) > 0 {
			objRef = []uint16{0}
		}
		nvm.Cells = append(nvm.Cells, sromap.NVMCell{
			MinX:          alignedMinX,
			MinZ:          alignedMinZ,
			MaxX:          alignedMaxX,
			MaxZ:          alignedMaxZ,
			ObjectIndices: objRef,
		})
		addedCells++

		// Tile bounds (inclusive both ends).
		iMin := clampTile(int(alignedMinX / tile))
		iMax := clampTile(int(alignedMaxX/tile) - 1)
		jMin := clampTile(int(alignedMinZ / tile))
		jMax := clampTile(int(alignedMaxZ/tile) - 1)

		// Point every tile in the AABB at the new closed cell + set blocked Flag.
		for tj := jMin; tj <= jMax; tj++ {
			for ti := iMin; ti <= iMax; ti++ {
				idx := tj*sromap.NVMTileCount + ti
				nvm.Tiles[idx].CellID = newCellIdx
				nvm.Tiles[idx].Flag |= 1
			}
		}

		// Wall edges — the actual collision-enforcement signal in Silkroad's
		// nav graph. The baseline uses Cell0=adjacent_open_cell, Cell1=-1,
		// Flag=0x02 (BlockSrc2Dst) line segments at the perimeter of each
		// closed cell. We replicate that pattern: one segment per tile
		// along each side of the new closed cell. The dir0 field is the
		// direction from the adjacent open cell TO the wall.
		//
		// Z increases northward in Silkroad world coords (baseline pattern
		// confirmed: MinZ side of cell 207 had dir=0=North on the south-
		// side adjacent cell). So:
		//   - wall at house's MinZ (south side): adj cell is SOUTH, dir=0
		//   - wall at house's MaxZ (north side): adj cell is NORTH, dir=2
		//   - wall at house's MinX (west side):  adj cell is WEST,  dir=1
		//   - wall at house's MaxX (east side):  adj cell is EAST,  dir=3
		const (
			wallFlag     uint8 = 0x02
			dirFromSouth uint8 = 0 // adj cell is south of the wall → wall is N of it
			dirFromWest  uint8 = 1 // adj cell is west of the wall  → wall is E of it
			dirFromNorth uint8 = 2 // adj cell is north of the wall → wall is S of it
			dirFromEast  uint8 = 3 // adj cell is east of the wall  → wall is W of it
		)

		addWall := func(minX, minZ, maxX, maxZ float32, dir uint8, neighbourTile int) {
			if neighbourTile < 0 || neighbourTile >= sromap.NVMTotalTiles {
				return
			}
			cell0 := nvm.Tiles[neighbourTile].CellID
			// Only attach to OPEN cells — skip if the neighbour is already
			// the new closed cell or another closed one (would yield a
			// degenerate "wall between closed cells" edge).
			if cell0 < 0 || uint32(cell0) >= nvm.OpenCellCount {
				return
			}
			nvm.InternalEdges = append(nvm.InternalEdges, sromap.NVMInternalEdge{
				MinX: minX, MinZ: minZ, MaxX: maxX, MaxZ: maxZ,
				Flag: wallFlag, Dir0: dir, Dir1: 0xFF,
				Cell0: int16(cell0), Cell1: -1,
			})
		}

		// House's south side (z = alignedMinZ): adjacent tile is one row south
		// (smaller j). Wall direction from that tile = North.
		for ti := iMin; ti <= iMax; ti++ {
			tx := float32(ti) * tile
			neighbour := -1
			if jMin > 0 {
				neighbour = (jMin-1)*sromap.NVMTileCount + ti
			}
			addWall(tx, alignedMinZ, tx+tile, alignedMinZ, dirFromSouth, neighbour)
		}
		// House's north side (z = alignedMaxZ)
		for ti := iMin; ti <= iMax; ti++ {
			tx := float32(ti) * tile
			neighbour := -1
			if jMax+1 < sromap.NVMTileCount {
				neighbour = (jMax+1)*sromap.NVMTileCount + ti
			}
			addWall(tx, alignedMaxZ, tx+tile, alignedMaxZ, dirFromNorth, neighbour)
		}
		// House's west side (x = alignedMinX)
		for tj := jMin; tj <= jMax; tj++ {
			tz := float32(tj) * tile
			neighbour := -1
			if iMin > 0 {
				neighbour = tj*sromap.NVMTileCount + (iMin - 1)
			}
			addWall(alignedMinX, tz, alignedMinX, tz+tile, dirFromWest, neighbour)
		}
		// House's east side (x = alignedMaxX)
		for tj := jMin; tj <= jMax; tj++ {
			tz := float32(tj) * tile
			neighbour := -1
			if iMax+1 < sromap.NVMTileCount {
				neighbour = tj*sromap.NVMTileCount + (iMax + 1)
			}
			addWall(alignedMaxX, tz, alignedMaxX, tz+tile, dirFromEast, neighbour)
		}
	}
	return
}

// syncNVMObjects walks the region's .o2 placements and ensures every one
// whose asset has collision data also exists as an NVMObject entry inside
// the navmesh. Empirically the Silkroad server reads NVMObjects (not
// Tile.Flag bits and not closed cells alone) for AABB collision against
// physical props — without an NVMObject entry, the player walks straight
// through even if tile/cell signals are set.
//
// Existing NVMObjects (the originals baked by Joymax) are left alone; we
// only append the missing ones, keyed by (RegionID, ObjID, UID).
func (s *Server) syncNVMObjects(nvm *sromap.NVM, placements []sromap.ObjectEntry, rx, ry int) {
	have := make(map[uint64]bool, len(nvm.Objects))
	for _, o := range nvm.Objects {
		key := placementKey(o.RegionID, o.UID, o.AssetID)
		have[key] = true
	}
	added := 0
	for _, p := range placements {
		asset, _ := s.objCache.get(p.ObjID)
		if asset == nil {
			continue
		}
		// Only add for assets that actually have collision data — matches
		// what Joymax did: not every prop got an NVMObject entry, only
		// "solid" ones with collision_mesh references.
		if !asset.HasCollision {
			continue
		}
		key := placementKey(p.RegionID, p.UID, p.ObjID)
		if have[key] {
			continue
		}
		nvm.Objects = append(nvm.Objects, sromap.NVMObject{
			AssetID:  p.ObjID,
			X:        p.X,
			Y:        p.Y,
			Z:        p.Z,
			Yaw:      p.Yaw,
			Type:     -1,
			UID:      p.UID,
			Short0:   0,
			IsBig:    p.Big,
			IsStruct: p.Struct,
			RegionID: p.RegionID,
			Links:    nil,
		})
		have[key] = true
		added++
	}
	_ = added
}

// pathLooksCustom returns true if the asset.Source (from object.ifo)
// uses the `res\custom\` (or `res/custom/`) prefix we apply to user-
// imported objects. Stock Joymax assets live elsewhere (res\bldg\…,
// res\nature\…, etc) and must NOT be re-registered as collision
// objects — they already have CRTNavMeshObj entries built from the
// baked NVMObjects in neighbor regions.
func pathLooksCustom(p string) bool {
	if p == "" {
		return false
	}
	for i := 0; i+7 <= len(p); i++ {
		s := p[i : i+7]
		if (s[0] == 'c' || s[0] == 'C') && (s[1] == 'u' || s[1] == 'U') &&
			(s[2] == 's' || s[2] == 'S') && (s[3] == 't' || s[3] == 'T') &&
			(s[4] == 'o' || s[4] == 'O') && (s[5] == 'm' || s[5] == 'M') &&
			(s[6] == '\\' || s[6] == '/') {
			return true
		}
	}
	return false
}

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

func clampTile(v int) int {
	if v < 0 {
		return 0
	}
	if v > sromap.NVMTileCount-1 {
		return sromap.NVMTileCount - 1
	}
	return v
}
