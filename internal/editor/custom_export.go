package editor

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sromapedit/internal/sromap"
)

// fillMissingNormals computes face-averaged unit normals for any vertex
// whose normal is zero. Zero normals in BMS data have been observed to
// crash the Silkroad client when an object enters draw range — likely a
// division-by-length in the renderer's lighting path.
func fillMissingNormals(verts []sromap.BMSVertex, indices []uint32) {
	missing := make([]bool, len(verts))
	any := false
	for i, v := range verts {
		if v.NX*v.NX+v.NY*v.NY+v.NZ*v.NZ < 1e-6 {
			missing[i] = true
			any = true
		}
	}
	if !any {
		return
	}
	accX := make([]float32, len(verts))
	accY := make([]float32, len(verts))
	accZ := make([]float32, len(verts))
	for i := 0; i+2 < len(indices); i += 3 {
		a, b, c := indices[i], indices[i+1], indices[i+2]
		va, vb, vc := verts[a], verts[b], verts[c]
		ex, ey, ez := vb.X-va.X, vb.Y-va.Y, vb.Z-va.Z
		fx, fy, fz := vc.X-va.X, vc.Y-va.Y, vc.Z-va.Z
		nx := ey*fz - ez*fy
		ny := ez*fx - ex*fz
		nz := ex*fy - ey*fx
		// Use the cross-product magnitude as a face-area weight so big
		// triangles influence the averaged normal more than slivers.
		accX[a] += nx
		accY[a] += ny
		accZ[a] += nz
		accX[b] += nx
		accY[b] += ny
		accZ[b] += nz
		accX[c] += nx
		accY[c] += ny
		accZ[c] += nz
	}
	for i := range verts {
		if !missing[i] {
			continue
		}
		nx, ny, nz := accX[i], accY[i], accZ[i]
		l := float32(math.Sqrt(float64(nx*nx + ny*ny + nz*nz)))
		if l < 1e-6 {
			verts[i].NX, verts[i].NY, verts[i].NZ = 0, 1, 0
			continue
		}
		verts[i].NX = nx / l
		verts[i].NY = ny / l
		verts[i].NZ = nz / l
	}
}

// CustomGamePaths is the set of resource paths for one exported custom object.
// All paths are relative to the PK2 root (root), so a Silkroad client/server
// resolves them via its own asset index.
type CustomGamePaths struct {
	BMS      string // prim\mesh\custom\<slug>\<slug>.bms — meshes live under prim/mesh/, matches stock asset layout
	BMT      string // prim\mtrl\custom\<slug>\<slug>.bmt
	BSR      string // res\custom\<slug>\<slug>.bsr  (also referenced from object.ifo)
	DDJ      string // res\custom\<slug>\diffuse.ddj
	Material string // material name inside BMT/BMS — kept simple as the slug.
}

// ExportResult summarises a successful export.
type ExportResult struct {
	ObjectID         uint32          `json:"objectID"`
	Slug             string          `json:"slug"`
	GameObjID        int             `json:"gameObjID"`
	Paths            CustomGamePaths `json:"paths"`
	BytesWritten     map[string]int  `json:"bytesWritten"`
	CollisionOffsetX float32         `json:"collisionOffsetX,omitempty"`
	CollisionOffsetZ float32         `json:"collisionOffsetZ,omitempty"`
}

// ExportCustomObjectToGame produces the four game files (.bsr/.bms/.bmt/.ddj)
// for the given custom object and appends an entry to object.ifo so the game
// can find the new asset by ID. Files are written under root AND mirrored to
// the export dir if one is configured.
//
// On success, the returned ExportResult contains the in-game objID that was
// allocated for this asset; callers should switch placement persistence to
// .o2 using that ID.
//
// This is best-effort: the encoders produce files our own parsers round-trip,
// but the game's loader is stricter. Expect to iterate against client
// crashes / "object not visible" / missing collision until each unknown
// field is tuned.
func (s *Server) ExportCustomObjectToGame(id uint32) (*ExportResult, error) {
	meta, mesh, err := s.customObjects.Get(id)
	if err != nil {
		return nil, fmt.Errorf("custom object %d: %w", id, err)
	}
	slug := slugFromName(meta.Name)
	if slug == "" {
		return nil, fmt.Errorf("invalid slug for %q", meta.Name)
	}
	paths := CustomGamePaths{
		// BMS goes under prim/mesh/custom/ — matches stock asset layout (e.g.,
		// stock oas_tarim_rob_smallfire01.bms is at prim/mesh/bldg/oasis/...).
		// The client's collision-mesh loader appears to only resolve paths
		// under prim/mesh/, not res/custom/.
		BMS: filepath.ToSlash(filepath.Join("prim", "mesh", "custom", slug, slug+".bms")),
		BMT: filepath.ToSlash(filepath.Join("prim", "mtrl", "custom", slug, slug+".bmt")),
		BSR: filepath.ToSlash(filepath.Join("res", "custom", slug, slug+".bsr")),
		// DDJ sits next to the BMT so the engine's material loader can find
		// it with the relative filename — matches the cj_ferry.bmt layout.
		DDJ:      filepath.ToSlash(filepath.Join("prim", "mtrl", "custom", slug, "diffuse.ddj")),
		Material: slug,
	}

	written := map[string]int{}

	// --- BMS (the mesh) ---
	bmsVerts := make([]sromap.BMSVertex, len(mesh.Vertices))
	for i, v := range mesh.Vertices {
		bmsVerts[i] = sromap.BMSVertex{X: v.X, Y: v.Y, Z: v.Z, U: v.U, V: v.V, NX: v.NX, NY: v.NY, NZ: v.NZ}
	}
	fillMissingNormals(bmsVerts, mesh.Indices)
	bmsIdx := make([]uint16, len(mesh.Indices))
	for i, v := range mesh.Indices {
		bmsIdx[i] = uint16(v)
	}
	footprint := customObjectCollisionFootprint(mesh)
	navOpts := sromap.BMSNavOptions{
		OffsetX:   meta.CollisionOffsetX,
		OffsetZ:   meta.CollisionOffsetZ,
		Footprint: footprint,
	}
	bmsBytes, err := sromap.EncodeMinimalBMSWithOptions(slug, paths.Material, bmsVerts, bmsIdx, mesh.BBoxMin, mesh.BBoxMax, navOpts)
	if err != nil {
		return nil, fmt.Errorf("encode bms: %w", err)
	}
	bmsAbs := filepath.Join(s.Root, "Data", filepath.FromSlash(paths.BMS))
	if err := s.writeAndMirror(bmsAbs, bmsBytes); err != nil {
		return nil, err
	}
	written[paths.BMS] = len(bmsBytes)
	// Early test exports wrote the BMS next to the BSR under res/custom.
	// Keep that compatibility copy current so stale BSRs cannot keep loading
	// an older collision mesh during in-game tests.
	legacyBMS := filepath.ToSlash(filepath.Join("res", "custom", slug, slug+".bms"))
	legacyBMSAbs := filepath.Join(s.Root, "Data", filepath.FromSlash(legacyBMS))
	if err := s.writeAndMirror(legacyBMSAbs, bmsBytes); err != nil {
		return nil, err
	}
	written[legacyBMS] = len(bmsBytes)

	// --- collision BMS ---
	// Keep the rendered mesh and collision mesh separate. The BSR collision
	// path should resolve to a compact ground-footprint mesh, so server paths
	// that inspect BMS faces and paths that inspect the BMS NavMesh section
	// both see the same collision footprint.
	collisionBMSPath := filepath.ToSlash(filepath.Join("prim", "mesh", "custom", slug, slug+"_collision.bms"))
	collisionFootprint := make([][2]float32, len(footprint))
	for i, p := range footprint {
		collisionFootprint[i] = [2]float32{p[0] + meta.CollisionOffsetX, p[1] + meta.CollisionOffsetZ}
	}
	collisionVerts, collisionIdx, collisionBBoxMin, collisionBBoxMax := customObjectCollisionBMSMesh(mesh, footprint, meta.CollisionOffsetX, meta.CollisionOffsetZ)
	collisionBytes, err := sromap.EncodeMinimalBMSWithOptions(
		slug+"_collision", paths.Material,
		collisionVerts, collisionIdx,
		collisionBBoxMin, collisionBBoxMax,
		sromap.BMSNavOptions{Footprint: collisionFootprint},
	)
	if err != nil {
		return nil, fmt.Errorf("encode collision bms: %w", err)
	}
	collisionBMSAbs := filepath.Join(s.Root, "Data", filepath.FromSlash(collisionBMSPath))
	if err := s.writeAndMirror(collisionBMSAbs, collisionBytes); err != nil {
		return nil, err
	}
	written[collisionBMSPath] = len(collisionBytes)

	// --- BMT (the material) ---
	// Reference the DDJ by filename only (BMT lives in the same directory),
	// matching the real-asset pattern where cj_ferry.bmt points at
	// "cj_ferry_buil1_door.ddj" alongside it.
	bmtBytes, err := sromap.EncodeMinimalBMT(paths.Material, "diffuse.ddj", false)
	if err != nil {
		return nil, fmt.Errorf("encode bmt: %w", err)
	}
	bmtAbs := filepath.Join(s.Root, "Data", filepath.FromSlash(paths.BMT))
	if err := s.writeAndMirror(bmtAbs, bmtBytes); err != nil {
		return nil, err
	}
	written[paths.BMT] = len(bmtBytes)

	// --- BSR (the wrapper) ---
	bsrBytes, err := sromap.EncodeMinimalBSR(slug,
		filepath.FromSlash(paths.BMT), filepath.FromSlash(paths.BMS), filepath.FromSlash(collisionBMSPath),
		collisionBBoxMin, collisionBBoxMax)
	if err != nil {
		return nil, fmt.Errorf("encode bsr: %w", err)
	}
	bsrAbs := filepath.Join(s.Root, "Data", filepath.FromSlash(paths.BSR))
	if err := s.writeAndMirror(bsrAbs, bsrBytes); err != nil {
		return nil, err
	}
	written[paths.BSR] = len(bsrBytes)

	// --- DDJ (the texture) ---
	// Source PNG lives inside the custom-objects store.
	srcDir := filepath.Join(s.customObjects.BaseDir(), slug)
	pngPath := filepath.Join(srcDir, meta.TexFile)
	if !meta.HasTex || meta.TexFile == "" {
		return nil, fmt.Errorf("custom object %s has no texture — export requires a diffuse map", slug)
	}
	pngFile, err := os.Open(pngPath)
	if err != nil {
		return nil, fmt.Errorf("open texture %s: %w", pngPath, err)
	}
	pngImg, err := png.Decode(pngFile)
	_ = pngFile.Close()
	if err != nil {
		return nil, fmt.Errorf("decode texture %s: %w", pngPath, err)
	}
	// 512px max — Silkroad's stock textures are typically 256–512, anything
	// larger is wasted on the 2005 engine.
	ddjBytes, err := sromap.EncodeDDJFromImage(pngImg, 512)
	if err != nil {
		return nil, fmt.Errorf("encode ddj: %w", err)
	}
	ddjAbs := filepath.Join(s.Root, "Data", filepath.FromSlash(paths.DDJ))
	if err := s.writeAndMirror(ddjAbs, ddjBytes); err != nil {
		return nil, err
	}
	written[paths.DDJ] = len(ddjBytes)

	// --- object.ifo append (skipped on re-export) ---
	gameObjID := meta.GameObjID
	if gameObjID == 0 {
		newID, err := s.appendObjectIfoEntry(paths.BSR)
		if err != nil {
			return nil, fmt.Errorf("update object.ifo: %w", err)
		}
		gameObjID = newID
		_ = s.customObjects.UpdateMeta(id, func(m *CustomObjectMeta) {
			m.GameObjID = gameObjID
			m.BSRPath = paths.BSR
		})
	}
	s.mirrorMapPlacementFiles(uint32(gameObjID), written)

	// Make our editor pipeline find the newly-written assets when it next
	// renders this object via /api/object (since the asset index is built
	// once on startup and would otherwise miss new files).
	s.objCache.Invalidate()

	return &ExportResult{
		ObjectID:         id,
		Slug:             slug,
		GameObjID:        gameObjID,
		Paths:            paths,
		BytesWritten:     written,
		CollisionOffsetX: meta.CollisionOffsetX,
		CollisionOffsetZ: meta.CollisionOffsetZ,
	}, nil
}

func (s *Server) writeAndMirror(abs string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(abs, data, 0644); err != nil {
		return err
	}
	_ = s.mirrorToExport(abs)
	return nil
}

func (s *Server) mirrorMapPlacementFiles(gameObjID uint32, written map[string]int) {
	if s.ExportDir == "" || gameObjID == 0 {
		return
	}
	mapObjectIfo := sromap.ObjectInfoPath(s.Root)
	if st, err := os.Stat(mapObjectIfo); err == nil {
		_ = s.mirrorToExport(mapObjectIfo)
		if written != nil {
			if rel, err := filepath.Rel(s.Root, mapObjectIfo); err == nil && !strings.HasPrefix(rel, "..") {
				written[filepath.ToSlash(rel)] = int(st.Size())
			}
		}
	}

	mapRoot := filepath.Join(s.Root, "Map")
	toMirror := make(map[string]string)
	collect := func(scanRoot string) {
		entries, err := os.ReadDir(scanRoot)
		if err != nil {
			return
		}
		for _, yDir := range entries {
			if !yDir.IsDir() {
				continue
			}
			yPath := filepath.Join(scanRoot, yDir.Name())
			o2Files, err := os.ReadDir(yPath)
			if err != nil {
				continue
			}
			for _, file := range o2Files {
				if file.IsDir() || !strings.EqualFold(filepath.Ext(file.Name()), ".o2") {
					continue
				}
				o2Path := filepath.Join(yPath, file.Name())
				if !fileMaybeContainsObjID(o2Path, gameObjID) {
					continue
				}
				o2, err := sromap.LoadO2(o2Path)
				if err != nil {
					continue
				}
				found := false
				for _, e := range o2.Entries {
					if e.ObjID == gameObjID {
						found = true
						break
					}
				}
				if !found {
					continue
				}
				rel, err := filepath.Rel(scanRoot, o2Path)
				if err != nil || strings.HasPrefix(rel, "..") {
					continue
				}
				src := filepath.Join(mapRoot, rel)
				if _, err := os.Stat(src); err == nil {
					toMirror[filepath.Clean(src)] = src
				}
			}
		}
	}
	collect(mapRoot)
	if s.ExportDir != "" {
		collect(filepath.Join(s.ExportDir, "Map"))
	}
	for _, o2Path := range toMirror {
		_ = s.mirrorToExport(o2Path)
		if written != nil {
			if st, err := os.Stat(o2Path); err == nil {
				if rel, err := filepath.Rel(s.Root, o2Path); err == nil && !strings.HasPrefix(rel, "..") {
					written[filepath.ToSlash(rel)] = int(st.Size())
				}
			}
		}
	}
}

func fileMaybeContainsObjID(path string, gameObjID uint32) bool {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	needle := []byte{
		byte(gameObjID),
		byte(gameObjID >> 8),
		byte(gameObjID >> 16),
		byte(gameObjID >> 24),
	}
	return bytes.Contains(raw, needle)
}

// ----------------------------------------------------------------------------
// object.ifo modification (plain ASCII, LF line endings)
//
//   JMXVOBJI1000
//   3308
//   00000 0x00000001 "res\bldg\china\cj_ferry\cj_ferry_buil.bsr"
//   00001 0x00000000 "res\bldg\china\cj_ferry\cj_ferry_warehou.bsr"
//   ...
// ----------------------------------------------------------------------------

func (s *Server) appendObjectIfoEntry(bsrRelPath string) (int, error) {
	// The client reads TWO separate object catalogs (verified via strings
	// in sro_client.exe):
	//   1. Map.pk2's "object.ifo"        — for visual placement (.o2 → BSR)
	//   2. Data.pk2's "data\navmesh\object.ifo" — for NavMesh objects (NVMObject → BSR)
	//
	// When we add an NVMObject pointing at a custom AssetID, the client looks
	// it up in catalog (2). If our id isn't there, it falls through to
	// garbage and pops "Load Fail(NavMesh Obj)" pointing at whatever asset
	// the garbage path happened to read. So both files must contain our
	// new entry, or NVMObject-based collision can't work for custom assets.
	primary := sromap.ObjectInfoPath(s.Root) // Map/object.ifo (editor source of truth)
	navPath := filepath.Join(s.Root, "Data", "Navmesh", "object.ifo")

	newID, err := appendToObjectIfo(primary, bsrRelPath, true)
	if err != nil {
		return 0, fmt.Errorf("append %s: %w", primary, err)
	}
	_ = s.mirrorToExport(primary)
	if updated, err := sromap.LoadObjectInfo(primary); err == nil {
		s.objects = updated
	}

	if _, err := os.Stat(navPath); err == nil {
		if _, err := appendToObjectIfo(navPath, bsrRelPath, false); err != nil {
			return newID, fmt.Errorf("append nav %s: %w (primary written)", navPath, err)
		}
		_ = s.mirrorToExport(navPath)
	}
	return newID, nil
}

// appendToObjectIfo appends a single entry to a JMXVOBJI catalog, bumping
// the count line and assigning the next free ID. `forceID` ignored — IDs
// always come from max-in-file+1, so the same call produces the same ID
// against any catalog with the same head.
func appendToObjectIfo(ifoPath, bsrRelPath string, takeBackup bool) (int, error) {
	raw, err := os.ReadFile(ifoPath)
	if err != nil {
		return 0, err
	}
	text := string(raw)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[0], "JMXVOBJI") {
		return 0, fmt.Errorf("object.ifo: bad header %q", lines[0])
	}
	declaredCount, _ := strconv.Atoi(strings.TrimSpace(lines[1]))
	maxFound := 0
	for _, line := range lines[2:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		id, err := strconv.Atoi(fields[0])
		if err == nil && id > maxFound {
			maxFound = id
		}
	}
	newID := maxFound + 1
	gamePath := strings.ReplaceAll(bsrRelPath, "/", `\`)
	newLine := fmt.Sprintf("%05d 0x%08X \"%s\"", newID, uint32(0), gamePath)

	var out strings.Builder
	out.WriteString(lines[0])
	out.WriteByte('\n')
	newCount := newID + 1
	if declaredCount > newCount {
		newCount = declaredCount
	}
	out.WriteString(strconv.Itoa(newCount))
	out.WriteByte('\n')
	for _, l := range lines[2:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		out.WriteString(l)
		out.WriteByte('\n')
	}
	out.WriteString(newLine)
	out.WriteByte('\n')

	if takeBackup {
		if err := backupOnce(ifoPath); err != nil {
			return 0, fmt.Errorf("backup: %w", err)
		}
	}
	if err := os.WriteFile(ifoPath, []byte(out.String()), 0644); err != nil {
		return 0, err
	}
	return newID, nil
}
