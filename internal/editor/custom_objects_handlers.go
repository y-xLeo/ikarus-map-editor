package editor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// customObjectMesh mirrors the shape the frontend already consumes for game
// objects (see internal/editor/objects.go::objectMesh). Vertices interleaved
// as XYZUV (5 floats), uint16 indices, optional texture URL, AABB.
type customObjectMesh struct {
	Vertices   []float32  `json:"vertices"`
	Indices    []uint16   `json:"indices"`
	TextureURL string     `json:"textureUrl,omitempty"`
	BBoxMin    [3]float32 `json:"bboxMin"`
	BBoxMax    [3]float32 `json:"bboxMax"`
}

type customObjectAsset struct {
	ObjID                uint32             `json:"objID"`
	Name                 string             `json:"name"`
	Source               string             `json:"source"`
	Meshes               []customObjectMesh `json:"meshes"`
	BBoxMin              [3]float32         `json:"bboxMin"`
	BBoxMax              [3]float32         `json:"bboxMax"`
	IsCustom             bool               `json:"isCustom"`
	HasCollision         bool               `json:"hasCollision"`
	CollisionBBoxMin     [3]float32         `json:"collisionBBoxMin"`
	CollisionBBoxMax     [3]float32         `json:"collisionBBoxMax"`
	CollisionNavVertices []float32          `json:"collisionNavVertices,omitempty"`
	CollisionNavIndices  []uint16           `json:"collisionNavIndices,omitempty"`
	CollisionNavOutline  []uint16           `json:"collisionNavOutlineIndices,omitempty"`
	CollisionOffsetX     float32            `json:"collisionOffsetX,omitempty"`
	CollisionOffsetZ     float32            `json:"collisionOffsetZ,omitempty"`
}

func (s *Server) handleCustomObjectsList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	metas := s.customObjects.List()
	out := make([]map[string]any, 0, len(metas))
	for _, m := range metas {
		out = append(out, map[string]any{
			"id":               m.ID,
			"name":             m.Name,
			"bboxMin":          m.BBoxMin,
			"bboxMax":          m.BBoxMax,
			"hasTexture":       m.HasTex,
			"source":           m.Source,
			"gameObjID":        m.GameObjID,
			"bsrPath":          m.BSRPath,
			"collisionOffsetX": m.CollisionOffsetX,
			"collisionOffsetZ": m.CollisionOffsetZ,
		})
	}
	writeJSON(w, map[string]any{"count": len(out), "entries": out})
}

func (s *Server) handleCustomObject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	idStr := r.URL.Query().Get("id")
	id64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	id := uint32(id64)
	meta, mesh, err := s.customObjects.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Convert the OBJ mesh to the same interleaved-XYZUV + uint16 indices
	// shape the existing object renderer eats. Index expansion is required
	// because uint16 caps at 65535; bail with a useful error if exceeded.
	if len(mesh.Vertices) > 65535 {
		writeError(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("OBJ has %d vertices, max 65535", len(mesh.Vertices)))
		return
	}
	verts := make([]float32, 0, len(mesh.Vertices)*5)
	for _, v := range mesh.Vertices {
		verts = append(verts, v.X, v.Y, v.Z, v.U, v.V)
	}
	idx := make([]uint16, len(mesh.Indices))
	for i, v := range mesh.Indices {
		idx[i] = uint16(v)
	}
	collisionMin, collisionMax, collisionVerts, collisionIdx, collisionOutline := customObjectCollisionNav(meta, mesh)
	asset := customObjectAsset{
		ObjID:                meta.ID,
		Name:                 meta.Name,
		Source:               meta.Source,
		BBoxMin:              meta.BBoxMin,
		BBoxMax:              meta.BBoxMax,
		IsCustom:             true,
		HasCollision:         true,
		CollisionBBoxMin:     collisionMin,
		CollisionBBoxMax:     collisionMax,
		CollisionNavVertices: collisionVerts,
		CollisionNavIndices:  collisionIdx,
		CollisionNavOutline:  collisionOutline,
		CollisionOffsetX:     meta.CollisionOffsetX,
		CollisionOffsetZ:     meta.CollisionOffsetZ,
		Meshes: []customObjectMesh{{
			Vertices:   verts,
			Indices:    idx,
			TextureURL: s.customObjects.TextureURL(meta.ID),
			BBoxMin:    meta.BBoxMin,
			BBoxMax:    meta.BBoxMax,
		}},
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	writeJSON(w, asset)
}

func customObjectCollisionBounds(meta *CustomObjectMeta) ([3]float32, [3]float32) {
	min := meta.BBoxMin
	max := meta.BBoxMax
	min[0] += meta.CollisionOffsetX
	max[0] += meta.CollisionOffsetX
	min[2] += meta.CollisionOffsetZ
	max[2] += meta.CollisionOffsetZ
	return min, max
}

func (s *Server) handleCustomObjectImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
	defer r.Body.Close()
	var req struct {
		Name         string  `json:"name"`
		SourceFolder string  `json:"sourceFolder"`
		SourceObj    string  `json:"sourceObj"`
		DiffusePng   string  `json:"diffusePng"`
		TargetSize   float32 `json:"targetSize"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode: "+err.Error())
		return
	}
	// If a folder is supplied, look for the conventional meshy.ai output:
	// base.obj + texture_diffuse.png. Either explicit fields override.
	if req.SourceFolder != "" {
		if req.SourceObj == "" {
			obj := findFirstMatching(req.SourceFolder, []string{"base.obj", "model.obj", "mesh.obj"}, ".obj")
			if obj == "" {
				writeError(w, http.StatusBadRequest, "no .obj file found in sourceFolder")
				return
			}
			req.SourceObj = obj
		}
		if req.DiffusePng == "" {
			diff := findFirstMatching(req.SourceFolder,
				[]string{"texture_diffuse.png", "diffuse.png", "albedo.png", "base_color.png"}, "")
			if diff != "" {
				req.DiffusePng = diff
			}
		}
	}
	if req.SourceObj == "" {
		writeError(w, http.StatusBadRequest, "sourceObj or sourceFolder is required")
		return
	}
	if _, err := os.Stat(req.SourceObj); err != nil {
		writeError(w, http.StatusBadRequest, "sourceObj not found: "+err.Error())
		return
	}
	// If the user gave us a folder but no name, prefer the folder name to
	// the inner file name (folder is more recognizable than "base").
	if req.Name == "" && req.SourceFolder != "" {
		req.Name = filepath.Base(strings.TrimRight(req.SourceFolder, `/\`))
	}
	if req.DiffusePng != "" {
		if _, err := os.Stat(req.DiffusePng); err != nil {
			req.DiffusePng = ""
		}
	}
	meta, err := s.customObjects.Import(ImportRequest{
		Name:       req.Name,
		SourceObj:  req.SourceObj,
		DiffusePng: req.DiffusePng,
		TargetSize: req.TargetSize,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{
		"id":         meta.ID,
		"name":       meta.Name,
		"bboxMin":    meta.BBoxMin,
		"bboxMax":    meta.BBoxMax,
		"hasTexture": meta.HasTex,
	})
}

func (s *Server) handleCustomObjectExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	idStr := r.URL.Query().Get("id")
	id64, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	id := uint32(id64)
	if r.URL.Query().Has("collisionOffsetX") || r.URL.Query().Has("collisionOffsetZ") {
		ox, err := strconv.ParseFloat(r.URL.Query().Get("collisionOffsetX"), 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid collisionOffsetX")
			return
		}
		oz, err := strconv.ParseFloat(r.URL.Query().Get("collisionOffsetZ"), 32)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid collisionOffsetZ")
			return
		}
		if err := s.customObjects.UpdateMeta(id, func(m *CustomObjectMeta) {
			m.CollisionOffsetX = float32(ox)
			m.CollisionOffsetZ = float32(oz)
		}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	res, err := s.ExportCustomObjectToGame(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, res)
}

func (s *Server) handleCustomAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rel, err := url.QueryUnescape(r.URL.Query().Get("path"))
	if err != nil || rel == "" {
		writeError(w, http.StatusBadRequest, "missing path")
		return
	}
	abs := s.customObjects.SafePath(rel)
	if abs == "" {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	// Serve PNG directly — much simpler than the DDJ pipeline.
	http.ServeFile(w, r, abs)
}

// ------- Custom placements (sidecar, per region) -------

type customPlacement struct {
	ObjID uint32  `json:"objID"`
	UID   int32   `json:"uid"`
	X     float32 `json:"x"`
	Y     float32 `json:"y"`
	Z     float32 `json:"z"`
	Yaw   float32 `json:"yaw"`
}

func customPlacementsPath(root string, x, y int) string {
	return filepath.Join(root, CustomObjectsDir, "placements", fmt.Sprint(y), fmt.Sprintf("%d.json", x))
}

func (s *Server) handleRegionCustomPlacements(w http.ResponseWriter, r *http.Request) {
	x, y, err := parseRegionQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	switch r.Method {
	case http.MethodGet:
		path := customPlacementsPath(s.Root, x, y)
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				writeJSON(w, map[string]any{"placements": []customPlacement{}})
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		var p []customPlacement
		if err := json.Unmarshal(data, &p); err != nil {
			writeError(w, http.StatusInternalServerError, "decode placements: "+err.Error())
			return
		}
		writeJSON(w, map[string]any{"placements": p})
	case http.MethodPost:
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		defer r.Body.Close()
		var body struct {
			Placements []customPlacement `json:"placements"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "decode: "+err.Error())
			return
		}
		// Preserve client-supplied order. UID matching after save relies on
		// the response coming back in the same index order as the request.
		// Auto-assign UIDs for entries that don't have one yet (UID <= 0).
		// We allocate the next free integer per regional ObjID so undo/redo
		// keeps stable references.
		used := make(map[uint32]map[int32]bool)
		for _, p := range body.Placements {
			if p.UID <= 0 {
				continue
			}
			if used[p.ObjID] == nil {
				used[p.ObjID] = make(map[int32]bool)
			}
			used[p.ObjID][p.UID] = true
		}
		nextUID := func(id uint32) int32 {
			candidates := used[id]
			if candidates == nil {
				candidates = make(map[int32]bool)
				used[id] = candidates
			}
			for u := int32(1); ; u++ {
				if !candidates[u] {
					candidates[u] = true
					return u
				}
			}
		}
		for i := range body.Placements {
			if body.Placements[i].UID <= 0 {
				body.Placements[i].UID = nextUID(body.Placements[i].ObjID)
			}
		}
		path := customPlacementsPath(s.Root, x, y)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "mkdir: "+err.Error())
			return
		}
		if len(body.Placements) == 0 {
			// Clean the file rather than leaving an empty array on disk.
			_ = os.Remove(path)
			writeJSON(w, map[string]any{"saved": 0, "placements": []customPlacement{}})
			return
		}
		data, _ := json.MarshalIndent(body.Placements, "", "  ")
		if err := os.WriteFile(path, data, 0644); err != nil {
			writeError(w, http.StatusInternalServerError, "write: "+err.Error())
			return
		}
		writeJSON(w, map[string]any{"saved": len(body.Placements), "placements": body.Placements})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// findFirstMatching looks for the first file in `folder` matching one of
// `preferred` (case-insensitive). Falls back to any file with `fallbackExt`.
func findFirstMatching(folder string, preferred []string, fallbackExt string) string {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return ""
	}
	lowerToName := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lowerToName[strings.ToLower(e.Name())] = e.Name()
	}
	for _, p := range preferred {
		if real, ok := lowerToName[strings.ToLower(p)]; ok {
			return filepath.Join(folder, real)
		}
	}
	if fallbackExt != "" {
		ext := strings.ToLower(fallbackExt)
		for low, real := range lowerToName {
			if strings.HasSuffix(low, ext) {
				return filepath.Join(folder, real)
			}
		}
	}
	return ""
}
