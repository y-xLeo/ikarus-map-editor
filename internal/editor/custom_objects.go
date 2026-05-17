package editor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"sromapedit/internal/sromap"
)

// CustomObjectIDBase is the lowest object-ID assigned to user-imported OBJs.
// Game-side objIDs sit in a much lower range (object.ifo entries are usually
// 5–6 digits), so the high bit being set guarantees no overlap.
const CustomObjectIDBase uint32 = 0x80000000

// CustomObjectsDir is the directory under root that stores imported OBJs and
// their textures. Lives outside the game data tree so accidental PK2 packing
// won't include it.
const CustomObjectsDir = "CustomObjects"

// CustomObjectMeta is the on-disk JSON metadata for one imported object.
type CustomObjectMeta struct {
	ID               uint32     `json:"id"`
	Name             string     `json:"name"`
	Source           string     `json:"source"`  // path of original OBJ at import time
	BBoxMin          [3]float32 `json:"bboxMin"` // bbox after Scale is applied
	BBoxMax          [3]float32 `json:"bboxMax"`
	Scale            float32    `json:"scale"` // uniform scale applied at load time
	HasTex           bool       `json:"hasTexture"`
	TexFile          string     `json:"textureFile,omitempty"`
	MeshFile         string     `json:"meshFile,omitempty"`         // OBJ path relative to the object's dir
	GameObjID        int        `json:"gameObjID,omitempty"`        // assigned by ExportCustomObjectToGame; 0 = not yet exported
	BSRPath          string     `json:"bsrPath,omitempty"`          // resource path written to object.ifo on export
	CollisionOffsetX float32    `json:"collisionOffsetX,omitempty"` // local BMS-space NavMesh offset; visual mesh stays put
	CollisionOffsetZ float32    `json:"collisionOffsetZ,omitempty"`
	CreatedAt        int64      `json:"createdAt"`
}

// CustomObjectStore manages imported OBJ objects on disk and in memory.
type CustomObjectStore struct {
	root string

	mu     sync.Mutex
	byID   map[uint32]*CustomObjectMeta
	bySlug map[string]*CustomObjectMeta
	meshes map[uint32]*sromap.OBJMesh
	nextID uint32
}

// NewCustomObjectStore opens (or creates) the on-disk store under root.
func NewCustomObjectStore(root string) (*CustomObjectStore, error) {
	dir := filepath.Join(root, CustomObjectsDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	store := &CustomObjectStore{
		root:   root,
		byID:   make(map[uint32]*CustomObjectMeta),
		bySlug: make(map[string]*CustomObjectMeta),
		meshes: make(map[uint32]*sromap.OBJMesh),
		nextID: CustomObjectIDBase,
	}
	if err := store.scan(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *CustomObjectStore) BaseDir() string {
	return filepath.Join(s.root, CustomObjectsDir)
}

// UpdateMeta atomically rewrites the on-disk meta.json for the given id and
// the in-memory copy. Used after export so the assigned game objID sticks.
func (s *CustomObjectStore) UpdateMeta(id uint32, mutate func(*CustomObjectMeta)) error {
	s.mu.Lock()
	meta, ok := s.byID[id]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("custom object %d not found", id)
	}
	mutate(meta)
	return writeJSONFile(filepath.Join(s.BaseDir(), slugFromName(meta.Name), "meta.json"), meta)
}

// scan walks the CustomObjects directory and loads each meta.json.
func (s *CustomObjectStore) scan() error {
	entries, err := os.ReadDir(s.BaseDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(s.BaseDir(), e.Name(), "meta.json")
		raw, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta CustomObjectMeta
		if err := json.Unmarshal(raw, &meta); err != nil {
			continue
		}
		s.byID[meta.ID] = &meta
		s.bySlug[strings.ToLower(e.Name())] = &meta
		if meta.ID >= s.nextID {
			s.nextID = meta.ID + 1
		}
	}
	return nil
}

// List returns the loaded metas sorted by ID, suitable for /api/custom-objects.
func (s *CustomObjectStore) List() []CustomObjectMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CustomObjectMeta, 0, len(s.byID))
	for _, m := range s.byID {
		out = append(out, *m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Get returns the meta for the given ID along with the parsed mesh (loaded
// on first use). Returns (nil, "", error) if the ID isn't a custom object.
func (s *CustomObjectStore) Get(id uint32) (*CustomObjectMeta, *sromap.OBJMesh, error) {
	s.mu.Lock()
	meta, ok := s.byID[id]
	if !ok {
		s.mu.Unlock()
		return nil, nil, errors.New("custom object not found")
	}
	if cached, ok := s.meshes[id]; ok {
		s.mu.Unlock()
		return meta, cached, nil
	}
	s.mu.Unlock()
	mesh, err := sromap.LoadOBJ(filepath.Join(s.BaseDir(), slugFromName(meta.Name), meta.MeshFile))
	if err != nil {
		return nil, nil, err
	}
	if meta.Scale > 0 && meta.Scale != 1 {
		for i := range mesh.Vertices {
			mesh.Vertices[i].X *= meta.Scale
			mesh.Vertices[i].Y *= meta.Scale
			mesh.Vertices[i].Z *= meta.Scale
		}
		for i := 0; i < 3; i++ {
			mesh.BBoxMin[i] *= meta.Scale
			mesh.BBoxMax[i] *= meta.Scale
		}
	}
	s.mu.Lock()
	s.meshes[id] = mesh
	s.mu.Unlock()
	return meta, mesh, nil
}

// ImportRequest defines an import operation.
type ImportRequest struct {
	Name       string  // display name; slug is derived from this
	SourceObj  string  // path to OBJ file
	DiffusePng string  // optional path to PNG; empty = no texture
	TargetSize float32 // longest-axis size in Silkroad units (0 = use default 300)
}

// DefaultCustomObjectSize is the default longest-axis size for newly-
// imported OBJs, in Silkroad world units. Most generated meshes ship at
// unit-scale (~[-1, 1]) so unscaled they'd render as a speck — 300 units
// is roughly a small building. Override via ImportRequest.TargetSize.
const DefaultCustomObjectSize = float32(300.0)

// Import parses and stores a new custom object. Returns the assigned meta.
// On success, files are copied under <root>/CustomObjects/<slug>/.
func (s *CustomObjectStore) Import(req ImportRequest) (*CustomObjectMeta, error) {
	if req.SourceObj == "" {
		return nil, errors.New("sourceObj is required")
	}
	if req.Name == "" {
		req.Name = strings.TrimSuffix(filepath.Base(req.SourceObj), filepath.Ext(req.SourceObj))
	}
	slug := slugFromName(req.Name)
	if slug == "" {
		return nil, errors.New("name produced an empty slug")
	}
	s.mu.Lock()
	if existing, exists := s.bySlug[strings.ToLower(slug)]; exists {
		// Idempotent: re-importing the same name returns the existing meta
		// so the UI can find the object instead of stalling on an error.
		s.mu.Unlock()
		return existing, nil
	}
	id := s.nextID
	s.nextID++
	s.mu.Unlock()

	// Parse first so we can fail before writing anything.
	mesh, err := sromap.LoadOBJ(req.SourceObj)
	if err != nil {
		return nil, fmt.Errorf("parse obj: %w", err)
	}
	// Auto-scale to the requested target size so unit-scale OBJs from
	// meshy.ai / Blender are visible in-engine. We store the original mesh
	// on disk and record the scale in meta.json; Get() applies it at load.
	target := req.TargetSize
	if target <= 0 {
		target = DefaultCustomObjectSize
	}
	scale := computeObjAutoScale(mesh, target)
	if scale <= 0 {
		scale = 1
	}
	if scale != 1 {
		for i := range mesh.Vertices {
			mesh.Vertices[i].X *= scale
			mesh.Vertices[i].Y *= scale
			mesh.Vertices[i].Z *= scale
		}
		for i := 0; i < 3; i++ {
			mesh.BBoxMin[i] *= scale
			mesh.BBoxMax[i] *= scale
		}
	}

	dir := filepath.Join(s.BaseDir(), slug)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	meshDst := filepath.Join(dir, "base.obj")
	if err := copyFile(req.SourceObj, meshDst); err != nil {
		return nil, fmt.Errorf("copy obj: %w", err)
	}

	hasTex := false
	texFile := ""
	if req.DiffusePng != "" {
		texDst := filepath.Join(dir, "diffuse.png")
		if err := copyFile(req.DiffusePng, texDst); err != nil {
			return nil, fmt.Errorf("copy texture: %w", err)
		}
		hasTex = true
		texFile = "diffuse.png"
	}

	meta := &CustomObjectMeta{
		ID:       id,
		Name:     req.Name,
		Source:   req.SourceObj,
		BBoxMin:  mesh.BBoxMin,
		BBoxMax:  mesh.BBoxMax,
		Scale:    scale,
		HasTex:   hasTex,
		TexFile:  texFile,
		MeshFile: "base.obj",
	}
	if err := writeJSONFile(filepath.Join(dir, "meta.json"), meta); err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.byID[id] = meta
	s.bySlug[strings.ToLower(slug)] = meta
	s.meshes[id] = mesh // already scaled in memory
	s.mu.Unlock()
	return meta, nil
}

// TextureURL returns the path that /api/custom-asset should serve for this
// custom object's diffuse texture (or empty if there isn't one).
func (s *CustomObjectStore) TextureURL(id uint32) string {
	meta, ok := s.byID[id]
	if !ok || !meta.HasTex {
		return ""
	}
	rel := filepath.ToSlash(filepath.Join(CustomObjectsDir, slugFromName(meta.Name), meta.TexFile))
	return "/api/custom-asset?path=" + url.QueryEscape(rel)
}

// SafePath resolves a custom-asset path (relative to root) and verifies it
// stays under <root>/CustomObjects/. Returns the absolute disk path or "".
func (s *CustomObjectStore) SafePath(rel string) string {
	rel = filepath.Clean(strings.ReplaceAll(rel, "\\", "/"))
	if rel == "" || strings.HasPrefix(rel, "..") {
		return ""
	}
	abs, err := filepath.Abs(filepath.Join(s.root, rel))
	if err != nil {
		return ""
	}
	customRoot := filepath.Clean(s.BaseDir())
	if !strings.HasPrefix(strings.ToLower(abs), strings.ToLower(customRoot)) {
		return ""
	}
	if _, err := os.Stat(abs); err != nil {
		return ""
	}
	return abs
}

// computeObjAutoScale returns the uniform scale factor needed to make the
// longest axis of the mesh equal `target`. Returns 1 if the mesh is empty
// or already at the target.
func computeObjAutoScale(mesh *sromap.OBJMesh, target float32) float32 {
	dx := mesh.BBoxMax[0] - mesh.BBoxMin[0]
	dy := mesh.BBoxMax[1] - mesh.BBoxMin[1]
	dz := mesh.BBoxMax[2] - mesh.BBoxMin[2]
	extent := dx
	if dy > extent {
		extent = dy
	}
	if dz > extent {
		extent = dz
	}
	if extent <= 0 {
		return 1
	}
	return target / extent
}

func slugFromName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '_' || r == '-':
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func writeJSONFile(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
