package editor

import (
	"bytes"
	"fmt"
	"image/png"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"sromapedit/internal/sromap"
)

type objectMesh struct {
	Vertices   []float32  `json:"vertices"`
	Indices    []uint16   `json:"indices"`
	TextureURL string     `json:"textureUrl,omitempty"`
	Material   string     `json:"material,omitempty"`
	BBoxMin    [3]float32 `json:"bboxMin"`
	BBoxMax    [3]float32 `json:"bboxMax"`
}

type objectAsset struct {
	ObjID                uint32       `json:"objID"`
	Name                 string       `json:"name"`
	Source               string       `json:"source"`
	Meshes               []objectMesh `json:"meshes"`
	BBoxMin              [3]float32   `json:"bboxMin"`
	BBoxMax              [3]float32   `json:"bboxMax"`
	CollisionBBoxMin     [3]float32   `json:"collisionBBoxMin,omitempty"`
	CollisionBBoxMax     [3]float32   `json:"collisionBBoxMax,omitempty"`
	CollisionNavVertices []float32    `json:"collisionNavVertices,omitempty"`
	CollisionNavIndices  []uint16     `json:"collisionNavIndices,omitempty"`
	CollisionNavOutline  []uint16     `json:"collisionNavOutlineIndices,omitempty"`
	HasCollision         bool         `json:"hasCollision,omitempty"`
	CollisionHasNavMesh  bool         `json:"collisionHasNavMesh,omitempty"`
}

type objectCache struct {
	root   string
	infos  map[uint32]sromap.ObjectInfo
	assets *sromap.AssetIndex

	mu    sync.Mutex
	cache map[uint32]*objectAsset
	bad   map[uint32]string

	textureMu sync.Mutex
	textures  map[string][]byte // asset path → PNG bytes
	texMiss   map[string]bool
}

func newObjectCache(root string, infos map[uint32]sromap.ObjectInfo) *objectCache {
	return &objectCache{
		root:     root,
		infos:    infos,
		assets:   sromap.NewAssetIndex(root),
		cache:    make(map[uint32]*objectAsset),
		bad:      make(map[uint32]string),
		textures: make(map[string][]byte),
		texMiss:  make(map[string]bool),
	}
}

// Invalidate drops cached assets/errors and refreshes the file index. Use
// after writing new asset files (custom-object export) so they're picked up.
func (c *objectCache) Invalidate() {
	c.mu.Lock()
	c.cache = make(map[uint32]*objectAsset)
	c.bad = make(map[uint32]string)
	c.mu.Unlock()
	c.assets.Refresh()
}

func (c *objectCache) get(id uint32) (*objectAsset, string) {
	c.mu.Lock()
	if a, ok := c.cache[id]; ok {
		c.mu.Unlock()
		return a, ""
	}
	if reason, ok := c.bad[id]; ok {
		c.mu.Unlock()
		return nil, reason
	}
	c.mu.Unlock()

	asset, err := c.build(id)
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.bad[id] = err.Error()
		return nil, err.Error()
	}
	c.cache[id] = asset
	return asset, ""
}

func (c *objectCache) build(id uint32) (*objectAsset, error) {
	info, ok := c.infos[id]
	if !ok {
		return nil, fmt.Errorf("objID %d not in object.ifo", id)
	}
	resolvedRoot := c.assets.Resolve(info.Path)
	if resolvedRoot == "" {
		return nil, fmt.Errorf("could not resolve %q", info.Path)
	}

	asset := &objectAsset{ObjID: id, Source: info.Path}
	if info.IsCPD {
		cpd, err := sromap.LoadCPD(resolvedRoot)
		if err != nil {
			return nil, fmt.Errorf("CPD: %w", err)
		}
		asset.Name = cpd.Name
		for _, res := range cpd.Resources {
			if !strings.HasSuffix(strings.ToLower(res), ".bsr") {
				continue
			}
			if p := c.assets.Resolve(res); p != "" {
				if err := c.appendBSR(asset, p); err != nil {
					// keep going; partial geometry is better than none
					continue
				}
			}
		}
	} else {
		if err := c.appendBSR(asset, resolvedRoot); err != nil {
			return nil, err
		}
	}

	computeAssetBounds(asset)
	if len(asset.Meshes) == 0 {
		return nil, fmt.Errorf("no meshes loaded for %q", info.Path)
	}
	return asset, nil
}

func (c *objectCache) appendBSR(asset *objectAsset, bsrPath string) error {
	bsr, err := sromap.LoadBSR(bsrPath)
	if err != nil {
		return err
	}
	if asset.Name == "" {
		asset.Name = bsr.Name
	}

	// Try to load the BSR's collision mesh — its bbox is much tighter than
	// the visual mesh (e.g. tree trunk vs. canopy) and gives a cleaner
	// navmesh rebuild.
	if bsr.CollisionMesh != "" && !asset.HasCollision {
		asset.CollisionBBoxMin = bsr.CollisionBBox0Min
		asset.CollisionBBoxMax = bsr.CollisionBBox0Max
		if cp := c.assets.Resolve(bsr.CollisionMesh); cp != "" {
			if bms, err := sromap.LoadBMS(cp); err == nil {
				if bms.HasNavMesh {
					asset.CollisionNavVertices, asset.CollisionNavIndices, asset.CollisionNavOutline = bmsCollisionNavTriangles(bms)
					if len(asset.CollisionNavIndices) == 0 {
						asset.CollisionNavVertices, asset.CollisionNavIndices, asset.CollisionNavOutline = collisionQuadNav(asset.CollisionBBoxMin, asset.CollisionBBoxMax)
					}
					asset.CollisionHasNavMesh = true
				} else {
					asset.CollisionNavVertices, asset.CollisionNavIndices, asset.CollisionNavOutline = collisionQuadNav(asset.CollisionBBoxMin, asset.CollisionBBoxMax)
				}
				asset.HasCollision = true
			}
		}
	}

	// Load any BMT files referenced by this BSR and index materials by name.
	bmtMaterials := map[string]sromap.BMTMaterial{}
	bmtDirs := map[string]string{}
	for _, mat := range bsr.Materials {
		bmtPath := c.assets.Resolve(mat.Path)
		if bmtPath == "" {
			continue
		}
		bmt, err := sromap.LoadBMT(bmtPath)
		if err != nil {
			continue
		}
		dir := filepath.Dir(bmtPath)
		for _, m := range bmt.Materials {
			key := strings.ToLower(strings.TrimSpace(m.Name))
			if _, exists := bmtMaterials[key]; !exists {
				bmtMaterials[key] = m
				bmtDirs[key] = dir
			}
		}
	}

	for _, meshRel := range bsr.Meshes {
		meshPath := c.assets.Resolve(meshRel)
		if meshPath == "" {
			continue
		}
		bms, err := sromap.LoadBMS(meshPath)
		if err != nil {
			continue
		}
		mesh := objectMesh{
			Material: bms.Material,
		}
		mesh.Vertices = make([]float32, 0, len(bms.Vertices)*5)
		for _, v := range bms.Vertices {
			mesh.Vertices = append(mesh.Vertices, v.X, v.Y, v.Z, v.U, v.V)
		}
		mesh.Indices = bms.Indices
		mesh.BBoxMin, mesh.BBoxMax = bmsVertexBounds(bms.Vertices, bms.BBoxMin, bms.BBoxMax)

		matKey := strings.ToLower(strings.TrimSpace(bms.Material))
		if m, ok := bmtMaterials[matKey]; ok && m.TextureFile != "" {
			tex := resolveTexturePath(c.assets, bmtDirs[matKey], m)
			if tex != "" {
				mesh.TextureURL = "/api/asset?path=" + url.QueryEscape(tex)
			}
		}
		asset.Meshes = append(asset.Meshes, mesh)
	}
	return nil
}

func bmsVertexBounds(vertices []sromap.BMSVertex, fallbackMin, fallbackMax [3]float32) ([3]float32, [3]float32) {
	if len(vertices) == 0 {
		return fallbackMin, fallbackMax
	}
	min := [3]float32{vertices[0].X, vertices[0].Y, vertices[0].Z}
	max := min
	for _, v := range vertices[1:] {
		p := [3]float32{v.X, v.Y, v.Z}
		for i := 0; i < 3; i++ {
			if p[i] < min[i] {
				min[i] = p[i]
			}
			if p[i] > max[i] {
				max[i] = p[i]
			}
		}
	}
	return min, max
}

func bmsCollisionNavTriangles(bms *sromap.BMS) ([]float32, []uint16, []uint16) {
	if bms == nil || !bms.HasNavMesh || len(bms.NavVertices) == 0 || len(bms.NavCells) == 0 {
		return nil, nil, nil
	}
	vertices := make([]float32, 0, len(bms.NavVertices)*3)
	for _, v := range bms.NavVertices {
		vertices = append(vertices, v.X, v.Y, v.Z)
	}
	indices := make([]uint16, 0, len(bms.NavCells)*3)
	for _, c := range bms.NavCells {
		if int(c.V0) >= len(bms.NavVertices) || int(c.V1) >= len(bms.NavVertices) || int(c.V2) >= len(bms.NavVertices) {
			continue
		}
		indices = append(indices, c.V0, c.V1, c.V2)
	}
	if len(indices) == 0 {
		return nil, nil, nil
	}
	outline := make([]uint16, 0, len(bms.NavOutlineEdges)*2)
	for _, e := range bms.NavOutlineEdges {
		if int(e.SrcVertex) >= len(bms.NavVertices) || int(e.DstVertex) >= len(bms.NavVertices) {
			continue
		}
		outline = append(outline, e.SrcVertex, e.DstVertex)
	}
	return vertices, indices, outline
}

func collisionQuadNav(min, max [3]float32) ([]float32, []uint16, []uint16) {
	return []float32{
		min[0], min[1], min[2],
		max[0], min[1], min[2],
		max[0], min[1], max[2],
		min[0], min[1], max[2],
	}, []uint16{0, 1, 2, 0, 2, 3}, []uint16{0, 1, 1, 2, 2, 3, 3, 0}
}

func resolveTexturePath(idx *sromap.AssetIndex, bmtDir string, mat sromap.BMTMaterial) string {
	if mat.IsAbsolutePath {
		if p := idx.Resolve(mat.TextureFile); p != "" {
			return p
		}
	}
	candidate := filepath.Join(bmtDir, mat.TextureFile)
	if p := idx.Resolve(candidate); p != "" {
		return p
	}
	return idx.Resolve(mat.TextureFile)
}

func (c *objectCache) texturePNG(diskPath string) ([]byte, bool) {
	key := strings.ToLower(filepath.Clean(diskPath))
	c.textureMu.Lock()
	if data, ok := c.textures[key]; ok {
		c.textureMu.Unlock()
		return data, true
	}
	if c.texMiss[key] {
		c.textureMu.Unlock()
		return nil, false
	}
	c.textureMu.Unlock()

	img, err := sromap.LoadDDJ(diskPath)
	if err != nil || img == nil {
		c.textureMu.Lock()
		c.texMiss[key] = true
		c.textureMu.Unlock()
		return nil, false
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img.RGBA); err != nil {
		c.textureMu.Lock()
		c.texMiss[key] = true
		c.textureMu.Unlock()
		return nil, false
	}
	data := buf.Bytes()
	c.textureMu.Lock()
	c.textures[key] = data
	c.textureMu.Unlock()
	return data, true
}

func computeAssetBounds(asset *objectAsset) {
	if len(asset.Meshes) == 0 {
		return
	}
	asset.BBoxMin = asset.Meshes[0].BBoxMin
	asset.BBoxMax = asset.Meshes[0].BBoxMax
	for _, m := range asset.Meshes[1:] {
		for i := 0; i < 3; i++ {
			if m.BBoxMin[i] < asset.BBoxMin[i] {
				asset.BBoxMin[i] = m.BBoxMin[i]
			}
			if m.BBoxMax[i] > asset.BBoxMax[i] {
				asset.BBoxMax[i] = m.BBoxMax[i]
			}
		}
	}
}

// safeAssetPath ensures a path requested via /api/asset stays under the root.
func (c *objectCache) safeAssetPath(p string) string {
	if p == "" {
		return ""
	}
	root := filepath.Clean(c.root)
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(abs), strings.ToLower(root)) {
		return ""
	}
	return abs
}

// resolveAndServe combines AssetIndex.Resolve with the safety check.
func (c *objectCache) resolveAndServe(rel string) (string, bool) {
	rel = path.Clean(strings.ReplaceAll(rel, "\\", "/"))
	disk := c.assets.Resolve(rel)
	if disk == "" {
		return "", false
	}
	return c.safeAssetPath(disk), c.safeAssetPath(disk) != ""
}
