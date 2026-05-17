package editor

import (
	"math"
	"os"
	"strings"

	"sromapedit/internal/sromap"
)

type mapOnlyBuild struct {
	NVM           *sromap.NVM
	WalkableCount int
}

func (s *Server) BuildMapOnlyNVMFromMaps(rx, ry int, slopeThreshold float32, withGlobals bool) (*sromap.NVM, int, error) {
	mesh, err := sromap.LoadMesh(sromap.MeshPath(s.Root, rx, ry))
	if err != nil {
		return nil, 0, err
	}
	placements := s.loadMapOnlyPlacements(rx, ry)
	built := s.buildMapOnlyNVM(rx, ry, mesh, placements, slopeThreshold, withGlobals)
	return built.NVM, built.WalkableCount, nil
}

func (s *Server) BuildTerrainOnlyNVMFromMaps(rx, ry int, slopeThreshold float32, withGlobals bool) (*sromap.NVM, int, error) {
	mesh, err := sromap.LoadMesh(sromap.MeshPath(s.Root, rx, ry))
	if err != nil {
		return nil, 0, err
	}
	built := s.buildMapOnlyNVM(rx, ry, mesh, nil, slopeThreshold, withGlobals)
	return built.NVM, built.WalkableCount, nil
}

func (s *Server) loadMapOnlyPlacements(rx, ry int) []sromap.ObjectEntry {
	const targetMin = float32(0)
	const targetMax = float32(sromap.RegionSize)
	const scanRadius = 2
	seen := make(map[uint64]bool)
	var placements []sromap.ObjectEntry

	for dy := -scanRadius; dy <= scanRadius; dy++ {
		for dx := -scanRadius; dx <= scanRadius; dx++ {
			ox, oy := rx+dx, ry+dy
			if ox < 0 || ox > 255 || oy < 0 || oy > 127 {
				continue
			}
			o2, err := sromap.LoadO2(sromap.O2Path(s.Root, ox, oy))
			if err != nil {
				continue
			}
			for _, e := range o2.Entries {
				placements = s.appendMapOnlyPlacement(placements, seen, e, rx, ry, targetMin, targetMax)
			}
		}
	}
	return placements
}

func (s *Server) appendMapOnlyPlacement(placements []sromap.ObjectEntry, seen map[uint64]bool, e sromap.ObjectEntry, rx, ry int, targetMin, targetMax float32) []sromap.ObjectEntry {
	key := placementKey(e.RegionID, e.UID, e.ObjID)
	if seen[key] {
		return placements
	}
	seen[key] = true

	asset, _ := s.objCache.get(e.ObjID)
	if !assetHasMapOnlyCollision(asset) {
		return placements
	}

	p := e
	ownerX := int(p.RegionID & 0x00ff)
	ownerY := int(p.RegionID >> 8)
	p.X += float32(ownerX-rx) * float32(sromap.RegionSize)
	p.Z += float32(ownerY-ry) * float32(sromap.RegionSize)

	minX, maxX, minZ, maxZ := placementRotatedBBox(p, asset)
	if maxX <= targetMin || minX >= targetMax || maxZ <= targetMin || minZ >= targetMax {
		return placements
	}
	return append(placements, p)
}

func (s *Server) buildMapOnlyNVM(rx, ry int, mesh *sromap.Mesh, placements []sromap.ObjectEntry, slopeThreshold float32, withGlobals bool) mapOnlyBuild {
	walkable := s.computeMapOnlyWalkability(mesh, placements, slopeThreshold)
	walkableCount := 0
	for _, ok := range walkable {
		if ok {
			walkableCount++
		}
	}

	cells, tileCellID, openCount := sromap.PartitionCells(walkable)
	nvm := &sromap.NVM{}
	nvm.OpenCellCount = uint32(openCount)
	nvm.Cells = make([]sromap.NVMCell, len(cells))
	for i, c := range cells {
		minX, minZ, maxX, maxZ := sromap.CellWorldBounds(c)
		nvm.Cells[i] = sromap.NVMCell{
			MinX: minX, MinZ: minZ,
			MaxX: maxX, MaxZ: maxZ,
		}
	}
	nvm.InternalEdges = sromap.GenerateInternalEdgesWithWalls(cells)

	for _, p := range placements {
		asset, _ := s.objCache.get(p.ObjID)
		if !assetHasMapOnlyCollision(asset) {
			continue
		}
		nvm.Objects = append(nvm.Objects, sromap.NVMObject{
			AssetID:  p.ObjID,
			X:        p.X,
			Y:        p.Y,
			Z:        p.Z,
			Type:     p.Static,
			Yaw:      p.Yaw,
			UID:      p.UID,
			Short0:   p.Short0,
			IsBig:    p.Big,
			IsStruct: p.Struct,
			RegionID: p.RegionID,
		})
	}
	s.addMapOnlyObjectIndices(nvm)
	s.preserveMapOnlyObjectMetadata(rx, ry, nvm)

	tileFlags := mesh.TileFlagMap()
	tileTextures := mesh.NVMTileTextureMap()
	for i := range nvm.Tiles {
		flag := tileFlags[i] &^ uint16(1)
		if !walkable[i] {
			flag |= 1
		}
		nvm.Tiles[i] = sromap.NVMTile{
			CellID:    tileCellID[i],
			Flag:      flag,
			TextureID: tileTextures[i],
		}
	}

	heights := mesh.UniqueHeightMap()
	copy(nvm.Heights[:], heights[:])
	planeType, planeHeight := mesh.PlaneMaps()
	copy(nvm.PlaneType[:], planeType[:])
	copy(nvm.PlaneHeight[:], planeHeight[:])

	if withGlobals {
		nvm.GlobalEdges = s.buildMapOnlyGlobalEdges(rx, ry, nvm, slopeThreshold)
	}
	return mapOnlyBuild{NVM: nvm, WalkableCount: walkableCount}
}

func (s *Server) computeMapOnlyWalkability(mesh *sromap.Mesh, placements []sromap.ObjectEntry, slopeThreshold float32) [sromap.NVMTotalTiles]bool {
	var walkable [sromap.NVMTotalTiles]bool
	for i := range walkable {
		walkable[i] = true
	}

	tileFlags := mesh.TileFlagMap()
	for i, f := range tileFlags {
		if f&1 != 0 {
			walkable[i] = false
		}
	}

	if slopeThreshold > 0 {
		heights := mesh.UniqueHeightMap()
		for z := 0; z < sromap.NVMTileCount; z++ {
			for x := 0; x < sromap.NVMTileCount; x++ {
				if terrainTileSlopeDegrees(heights, x, z) > slopeThreshold {
					walkable[z*sromap.NVMTileCount+x] = false
				}
			}
		}
	}

	return walkable
}

func assetHasMapOnlyCollision(asset *objectAsset) bool {
	return asset != nil && asset.HasCollision && asset.CollisionHasNavMesh
}

func (s *Server) markPlacementCollision(walkable *[sromap.NVMTotalTiles]bool, p sromap.ObjectEntry, asset *objectAsset) {
	triangles, ok := transformedCollisionTriangles(p, asset)
	if !ok {
		minX, maxX, minZ, maxZ := placementRotatedBBox(p, asset)
		markBBoxCollision(walkable, minX, maxX, minZ, maxZ)
		return
	}

	minX, maxX, minZ, maxZ := placementRotatedBBox(p, asset)
	x0 := clampTile(int(float32(math.Floor(float64(minX / sromap.NVMTileSize)))))
	x1 := clampTile(int(float32(math.Ceil(float64(maxX/sromap.NVMTileSize)))) - 1)
	z0 := clampTile(int(float32(math.Floor(float64(minZ / sromap.NVMTileSize)))))
	z1 := clampTile(int(float32(math.Ceil(float64(maxZ/sromap.NVMTileSize)))) - 1)
	for z := z0; z <= z1; z++ {
		for x := x0; x <= x1; x++ {
			rMinX := float32(x * sromap.NVMTileSize)
			rMinZ := float32(z * sromap.NVMTileSize)
			rMaxX := rMinX + sromap.NVMTileSize
			rMaxZ := rMinZ + sromap.NVMTileSize
			if !trianglesIntersectRect(triangles, rMinX, rMinZ, rMaxX, rMaxZ) {
				walkable[z*sromap.NVMTileCount+x] = false
			}
		}
	}
}

func markBBoxCollision(walkable *[sromap.NVMTotalTiles]bool, minX, maxX, minZ, maxZ float32) {
	x0 := clampTile(int(float32(math.Floor(float64(minX / sromap.NVMTileSize)))))
	x1 := clampTile(int(float32(math.Ceil(float64(maxX/sromap.NVMTileSize)))) - 1)
	z0 := clampTile(int(float32(math.Floor(float64(minZ / sromap.NVMTileSize)))))
	z1 := clampTile(int(float32(math.Ceil(float64(maxZ/sromap.NVMTileSize)))) - 1)
	for z := z0; z <= z1; z++ {
		for x := x0; x <= x1; x++ {
			walkable[z*sromap.NVMTileCount+x] = false
		}
	}
}

type outlineSegment struct {
	x0, z0 float32
	x1, z1 float32
}

type navTriangle2D struct {
	x0, z0 float32
	x1, z1 float32
	x2, z2 float32
}

func transformedCollisionTriangles(p sromap.ObjectEntry, asset *objectAsset) ([]navTriangle2D, bool) {
	if len(asset.CollisionNavVertices) < 9 || len(asset.CollisionNavIndices) < 3 {
		return nil, false
	}
	c := float32(math.Cos(float64(p.Yaw)))
	sn := float32(math.Sin(float64(p.Yaw)))
	transform := func(idx uint16) (float32, float32, bool) {
		base := int(idx) * 3
		if base+2 >= len(asset.CollisionNavVertices) {
			return 0, 0, false
		}
		lx := asset.CollisionNavVertices[base]
		lz := asset.CollisionNavVertices[base+2]
		return c*lx - sn*lz + p.X, sn*lx + c*lz + p.Z, true
	}
	triangles := make([]navTriangle2D, 0, len(asset.CollisionNavIndices)/3)
	for i := 0; i+2 < len(asset.CollisionNavIndices); i += 3 {
		x0, z0, ok0 := transform(asset.CollisionNavIndices[i])
		x1, z1, ok1 := transform(asset.CollisionNavIndices[i+1])
		x2, z2, ok2 := transform(asset.CollisionNavIndices[i+2])
		if !ok0 || !ok1 || !ok2 {
			continue
		}
		triangles = append(triangles, navTriangle2D{x0: x0, z0: z0, x1: x1, z1: z1, x2: x2, z2: z2})
	}
	return triangles, len(triangles) > 0
}

func transformedCollisionOutline(p sromap.ObjectEntry, asset *objectAsset) ([]outlineSegment, bool) {
	if len(asset.CollisionNavVertices) < 6 || len(asset.CollisionNavOutline) < 6 {
		return nil, false
	}
	c := float32(math.Cos(float64(p.Yaw)))
	sn := float32(math.Sin(float64(p.Yaw)))
	segments := make([]outlineSegment, 0, len(asset.CollisionNavOutline)/2)
	transform := func(idx uint16) (float32, float32, bool) {
		base := int(idx) * 3
		if base+2 >= len(asset.CollisionNavVertices) {
			return 0, 0, false
		}
		lx := asset.CollisionNavVertices[base]
		lz := asset.CollisionNavVertices[base+2]
		return c*lx - sn*lz + p.X, sn*lx + c*lz + p.Z, true
	}
	for i := 0; i+1 < len(asset.CollisionNavOutline); i += 2 {
		x0, z0, ok0 := transform(asset.CollisionNavOutline[i])
		x1, z1, ok1 := transform(asset.CollisionNavOutline[i+1])
		if !ok0 || !ok1 {
			continue
		}
		segments = append(segments, outlineSegment{x0: x0, z0: z0, x1: x1, z1: z1})
	}
	return segments, len(segments) >= 3
}

func trianglesIntersectRect(triangles []navTriangle2D, minX, minZ, maxX, maxZ float32) bool {
	for _, t := range triangles {
		if triangleIntersectsRect(t, minX, minZ, maxX, maxZ) {
			return true
		}
	}
	return false
}

func triangleIntersectsRect(t navTriangle2D, minX, minZ, maxX, maxZ float32) bool {
	if pointInRect(t.x0, t.z0, minX, minZ, maxX, maxZ) ||
		pointInRect(t.x1, t.z1, minX, minZ, maxX, maxZ) ||
		pointInRect(t.x2, t.z2, minX, minZ, maxX, maxZ) {
		return true
	}
	if pointInTriangle2D(minX, minZ, t) ||
		pointInTriangle2D(maxX, minZ, t) ||
		pointInTriangle2D(minX, maxZ, t) ||
		pointInTriangle2D(maxX, maxZ, t) ||
		pointInTriangle2D((minX+maxX)*0.5, (minZ+maxZ)*0.5, t) {
		return true
	}
	for _, edge := range [][4]float32{
		{t.x0, t.z0, t.x1, t.z1},
		{t.x1, t.z1, t.x2, t.z2},
		{t.x2, t.z2, t.x0, t.z0},
	} {
		if segmentIntersects(edge[0], edge[1], edge[2], edge[3], minX, minZ, maxX, minZ) ||
			segmentIntersects(edge[0], edge[1], edge[2], edge[3], maxX, minZ, maxX, maxZ) ||
			segmentIntersects(edge[0], edge[1], edge[2], edge[3], maxX, maxZ, minX, maxZ) ||
			segmentIntersects(edge[0], edge[1], edge[2], edge[3], minX, maxZ, minX, minZ) {
			return true
		}
	}
	return false
}

func pointInTriangle2D(x, z float32, t navTriangle2D) bool {
	d0 := orient2D(t.x0, t.z0, t.x1, t.z1, x, z)
	d1 := orient2D(t.x1, t.z1, t.x2, t.z2, x, z)
	d2 := orient2D(t.x2, t.z2, t.x0, t.z0, x, z)
	hasNeg := d0 < 0 || d1 < 0 || d2 < 0
	hasPos := d0 > 0 || d1 > 0 || d2 > 0
	return !(hasNeg && hasPos)
}

func outlineBounds(segments []outlineSegment) (minX, maxX, minZ, maxZ float32) {
	minX, maxX = float32(math.MaxFloat32), float32(-math.MaxFloat32)
	minZ, maxZ = float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, s := range segments {
		for _, x := range [2]float32{s.x0, s.x1} {
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
		}
		for _, z := range [2]float32{s.z0, s.z1} {
			if z < minZ {
				minZ = z
			}
			if z > maxZ {
				maxZ = z
			}
		}
	}
	return
}

func pointInOutline(x, z float32, segments []outlineSegment) bool {
	inside := false
	for _, s := range segments {
		if (s.z0 > z) == (s.z1 > z) {
			continue
		}
		crossX := (s.x1-s.x0)*(z-s.z0)/(s.z1-s.z0) + s.x0
		if x < crossX {
			inside = !inside
		}
	}
	return inside
}

func outlineIntersectsRect(segments []outlineSegment, minX, minZ, maxX, maxZ float32) bool {
	for _, s := range segments {
		if pointInRect(s.x0, s.z0, minX, minZ, maxX, maxZ) || pointInRect(s.x1, s.z1, minX, minZ, maxX, maxZ) {
			return true
		}
		if segmentIntersects(s.x0, s.z0, s.x1, s.z1, minX, minZ, maxX, minZ) ||
			segmentIntersects(s.x0, s.z0, s.x1, s.z1, maxX, minZ, maxX, maxZ) ||
			segmentIntersects(s.x0, s.z0, s.x1, s.z1, maxX, maxZ, minX, maxZ) ||
			segmentIntersects(s.x0, s.z0, s.x1, s.z1, minX, maxZ, minX, minZ) {
			return true
		}
	}
	return false
}

func pointInRect(x, z, minX, minZ, maxX, maxZ float32) bool {
	return x >= minX && x <= maxX && z >= minZ && z <= maxZ
}

func segmentIntersects(ax, az, bx, bz, cx, cz, dx, dz float32) bool {
	o1 := orient2D(ax, az, bx, bz, cx, cz)
	o2 := orient2D(ax, az, bx, bz, dx, dz)
	o3 := orient2D(cx, cz, dx, dz, ax, az)
	o4 := orient2D(cx, cz, dx, dz, bx, bz)
	return o1*o2 <= 0 && o3*o4 <= 0
}

func orient2D(ax, az, bx, bz, cx, cz float32) float32 {
	return (bx-ax)*(cz-az) - (bz-az)*(cx-ax)
}

func (s *Server) addMapOnlyObjectIndices(nvm *sromap.NVM) {
	const bridgeHandoffPad = float32(sromap.NVMTileSize * 2)
	for objIdx, obj := range nvm.Objects {
		asset, _ := s.objCache.get(obj.AssetID)
		if !assetHasMapOnlyCollision(asset) {
			continue
		}
		minX, maxX, minZ, maxZ := nvmObjectRotatedBBox(obj, asset)
		var triangles []navTriangle2D
		preciseFootprint := false
		if assetUsesPreciseMapOnlyObjectCells(asset) {
			if t, ok := transformedCollisionTriangles(nvmObjectAsPlacement(obj), asset); ok {
				triangles = t
				minX, maxX, minZ, maxZ = navTriangleBounds(t, bridgeHandoffPad)
				preciseFootprint = true
			}
		}
		for ci := range nvm.Cells {
			cell := &nvm.Cells[ci]
			if minF(cell.MaxX, maxX)-maxF(cell.MinX, minX) <= 0 ||
				minF(cell.MaxZ, maxZ)-maxF(cell.MinZ, minZ) <= 0 {
				continue
			}
			if preciseFootprint && !trianglesIntersectRect(
				triangles,
				cell.MinX-bridgeHandoffPad,
				cell.MinZ-bridgeHandoffPad,
				cell.MaxX+bridgeHandoffPad,
				cell.MaxZ+bridgeHandoffPad,
			) {
				continue
			}
			if len(cell.ObjectIndices) >= 255 {
				continue
			}
			cell.ObjectIndices = append(cell.ObjectIndices, uint16(objIdx))
		}
	}
}

func assetUsesPreciseMapOnlyObjectCells(asset *objectAsset) bool {
	if asset == nil {
		return false
	}
	name := strings.ToLower(asset.Source + " " + asset.Name)
	return strings.Contains(name, "bridge") || strings.Contains(name, "brid")
}

func nvmObjectAsPlacement(obj sromap.NVMObject) sromap.ObjectEntry {
	return sromap.ObjectEntry{
		ObjID:    obj.AssetID,
		X:        obj.X,
		Y:        obj.Y,
		Z:        obj.Z,
		Yaw:      obj.Yaw,
		UID:      obj.UID,
		RegionID: obj.RegionID,
	}
}

func navTriangleBounds(triangles []navTriangle2D, pad float32) (minX, maxX, minZ, maxZ float32) {
	minX, maxX = float32(math.MaxFloat32), float32(-math.MaxFloat32)
	minZ, maxZ = float32(math.MaxFloat32), float32(-math.MaxFloat32)
	for _, t := range triangles {
		for _, x := range [3]float32{t.x0, t.x1, t.x2} {
			if x < minX {
				minX = x
			}
			if x > maxX {
				maxX = x
			}
		}
		for _, z := range [3]float32{t.z0, t.z1, t.z2} {
			if z < minZ {
				minZ = z
			}
			if z > maxZ {
				maxZ = z
			}
		}
	}
	return minX - pad, maxX + pad, minZ - pad, maxZ + pad
}

type nvmObjectRefKey struct {
	assetID  uint32
	uid      int16
	regionID uint16
}

func (s *Server) preserveMapOnlyObjectMetadata(rx, ry int, nvm *sromap.NVM) {
	paths := sromap.ExistingNVMPaths(s.Root, rx, ry)
	if len(paths) == 0 {
		return
	}
	ref, err := sromap.LoadNVM(paths[0])
	if err != nil {
		return
	}

	refByKey, refDup := indexNVMObjectsByRefKey(ref.Objects)
	dstByKey, dstDup := indexNVMObjectsByRefKey(nvm.Objects)
	matched := make(map[int]int)
	for key, refIdx := range refByKey {
		if refDup[key] || dstDup[key] {
			continue
		}
		dstIdx, ok := dstByKey[key]
		if !ok {
			continue
		}
		if !sameNVMObjectPlacement(ref.Objects[refIdx], nvm.Objects[dstIdx]) {
			continue
		}
		src := ref.Objects[refIdx]
		dst := &nvm.Objects[dstIdx]
		dst.Type = src.Type
		dst.Short0 = src.Short0
		dst.IsBig = src.IsBig
		dst.IsStruct = src.IsStruct
		matched[refIdx] = dstIdx
	}

	for refIdx, dstIdx := range matched {
		src := ref.Objects[refIdx]
		if len(src.Links) == 0 {
			continue
		}
		links := make([]sromap.NVMObjectLink, 0, len(src.Links))
		for _, link := range src.Links {
			linkedRefIdx := int(link.LinkedObjectID)
			if linkedRefIdx < 0 || linkedRefIdx >= len(ref.Objects) {
				continue
			}
			linkedDstIdx, ok := matched[linkedRefIdx]
			if !ok {
				continue
			}
			links = append(links, sromap.NVMObjectLink{
				LinkedObjectID: int16(linkedDstIdx),
				LinkedEdgeID:   link.LinkedEdgeID,
				EdgeID:         link.EdgeID,
			})
		}
		nvm.Objects[dstIdx].Links = links
	}
}

func indexNVMObjectsByRefKey(objects []sromap.NVMObject) (map[nvmObjectRefKey]int, map[nvmObjectRefKey]bool) {
	byKey := make(map[nvmObjectRefKey]int, len(objects))
	duplicates := make(map[nvmObjectRefKey]bool)
	for i, obj := range objects {
		key := nvmObjectKey(obj)
		if _, ok := byKey[key]; ok {
			duplicates[key] = true
			continue
		}
		byKey[key] = i
	}
	return byKey, duplicates
}

func nvmObjectKey(obj sromap.NVMObject) nvmObjectRefKey {
	return nvmObjectRefKey{
		assetID:  obj.AssetID,
		uid:      obj.UID,
		regionID: obj.RegionID,
	}
}

func sameNVMObjectPlacement(a, b sromap.NVMObject) bool {
	const positionTolerance = float32(0.05)
	const heightTolerance = float32(0.25)
	const yawTolerance = float32(0.001)
	return absF(a.X-b.X) <= positionTolerance &&
		absF(a.Y-b.Y) <= heightTolerance &&
		absF(a.Z-b.Z) <= positionTolerance &&
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

func nvmObjectRotatedBBox(obj sromap.NVMObject, asset *objectAsset) (minX, maxX, minZ, maxZ float32) {
	p := sromap.ObjectEntry{
		ObjID: obj.AssetID,
		X:     obj.X,
		Y:     obj.Y,
		Z:     obj.Z,
		Yaw:   obj.Yaw,
	}
	return placementRotatedBBox(p, asset)
}

func (s *Server) buildMapOnlyGlobalEdges(rx, ry int, nvm *sromap.NVM, slopeThreshold float32) []sromap.NVMGlobalEdge {
	var edges []sromap.NVMGlobalEdge
	region0 := int16(uint16(ry)<<8 | uint16(rx))
	if north := s.loadMapOnlyNeighborNVM(rx, ry+1, slopeThreshold); north != nil {
		edges = append(edges, mapOnlyEdgesForSide(nvm, north, region0, int16(uint16(ry+1)<<8|uint16(rx)), 'N')...)
	}
	if south := s.loadMapOnlyNeighborNVM(rx, ry-1, slopeThreshold); south != nil {
		edges = append(edges, mapOnlyEdgesForSide(nvm, south, region0, int16(uint16(ry-1)<<8|uint16(rx)), 'S')...)
	}
	if east := s.loadMapOnlyNeighborNVM(rx+1, ry, slopeThreshold); east != nil {
		edges = append(edges, mapOnlyEdgesForSide(nvm, east, region0, int16(uint16(ry)<<8|uint16(rx+1)), 'E')...)
	}
	if west := s.loadMapOnlyNeighborNVM(rx-1, ry, slopeThreshold); west != nil {
		edges = append(edges, mapOnlyEdgesForSide(nvm, west, region0, int16(uint16(ry)<<8|uint16(rx-1)), 'W')...)
	}
	return edges
}

func (s *Server) loadMapOnlyNeighborNVM(rx, ry int, slopeThreshold float32) *sromap.NVM {
	if rx < 0 || rx > 255 || ry < 0 || ry > 127 {
		return nil
	}
	if mesh, err := sromap.LoadMesh(sromap.MeshPath(s.Root, rx, ry)); err == nil {
		placements := s.loadMapOnlyPlacements(rx, ry)
		return s.buildMapOnlyNVM(rx, ry, mesh, placements, slopeThreshold, false).NVM
	} else if !os.IsNotExist(err) {
		return nil
	}
	paths := sromap.ExistingNVMPaths(s.Root, rx, ry)
	if len(paths) == 0 {
		return nil
	}
	nvm, err := sromap.LoadNVM(paths[0])
	if err != nil {
		return nil
	}
	return nvm
}

func mapOnlyEdgesForSide(local, neighbor *sromap.NVM, region0, region1 int16, side byte) []sromap.NVMGlobalEdge {
	const extent = float32(sromap.RegionSize)
	var out []sromap.NVMGlobalEdge
	localOpen := int(local.OpenCellCount)
	if localOpen > len(local.Cells) {
		localOpen = len(local.Cells)
	}
	neighborOpen := int(neighbor.OpenCellCount)
	if neighborOpen > len(neighbor.Cells) {
		neighborOpen = len(neighbor.Cells)
	}
	for li := 0; li < localOpen; li++ {
		lc := local.Cells[li]
		if !cellTouchesSide(lc, side) {
			continue
		}
		l0, l1 := boundarySpan(lc, side)
		for ni := 0; ni < neighborOpen; ni++ {
			nc := neighbor.Cells[ni]
			if !cellTouchesSide(nc, oppositeSide(side)) {
				continue
			}
			n0, n1 := boundarySpan(nc, oppositeSide(side))
			lo := maxF(l0, n0)
			hi := minF(l1, n1)
			if hi <= lo {
				continue
			}
			e := sromap.NVMGlobalEdge{
				Flag:    sromap.NVMGlobalEdgeFlag,
				Cell0:   int16(li),
				Cell1:   int16(ni),
				Region0: region0,
				Region1: region1,
			}
			switch side {
			case 'N':
				e.MinX, e.MaxX = lo, hi
				e.MinZ, e.MaxZ = extent, extent
				e.Dir0, e.Dir1 = sromap.NVMDirNorth, sromap.NVMDirSouth
			case 'S':
				e.MinX, e.MaxX = lo, hi
				e.MinZ, e.MaxZ = 0, 0
				e.Dir0, e.Dir1 = sromap.NVMDirSouth, sromap.NVMDirNorth
			case 'E':
				e.MinX, e.MaxX = extent, extent
				e.MinZ, e.MaxZ = lo, hi
				e.Dir0, e.Dir1 = sromap.NVMDirEast, sromap.NVMDirWest
			case 'W':
				e.MinX, e.MaxX = 0, 0
				e.MinZ, e.MaxZ = lo, hi
				e.Dir0, e.Dir1 = sromap.NVMDirWest, sromap.NVMDirEast
			}
			out = append(out, e)
		}
	}
	return out
}

func cellTouchesSide(c sromap.NVMCell, side byte) bool {
	const extent = float32(sromap.RegionSize)
	switch side {
	case 'N':
		return c.MaxZ == extent
	case 'S':
		return c.MinZ == 0
	case 'E':
		return c.MaxX == extent
	case 'W':
		return c.MinX == 0
	default:
		return false
	}
}

func boundarySpan(c sromap.NVMCell, side byte) (float32, float32) {
	switch side {
	case 'N', 'S':
		return c.MinX, c.MaxX
	case 'E', 'W':
		return c.MinZ, c.MaxZ
	default:
		return 0, 0
	}
}

func oppositeSide(side byte) byte {
	switch side {
	case 'N':
		return 'S'
	case 'S':
		return 'N'
	case 'E':
		return 'W'
	case 'W':
		return 'E'
	default:
		return 0
	}
}
