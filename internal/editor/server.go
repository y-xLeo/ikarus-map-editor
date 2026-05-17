package editor

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"io/fs"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sromapedit/internal/sromap"
)

//go:embed static/*
var staticFiles embed.FS

type Server struct {
	Root          string
	ExportDir     string
	mapInfo       *sromap.MapInfo
	refRegions    map[int]sromap.RefRegionEntry
	objects       map[uint32]sromap.ObjectInfo
	tiles         map[uint32]sromap.Tile2DEntry
	textures      *textureCache
	objCache      *objectCache
	customObjects *CustomObjectStore
}

type boundsResponse struct {
	MinX    int `json:"minX"`
	MaxX    int `json:"maxX"`
	MinY    int `json:"minY"`
	MaxY    int `json:"maxY"`
	CenterX int `json:"centerX"`
	CenterY int `json:"centerY"`
}

type regionSummary struct {
	ID        uint16 `json:"id"`
	X         int    `json:"x"`
	Y         int    `json:"y"`
	IsDungeon bool   `json:"isDungeon"`
	HasRef    bool   `json:"hasRef"`
}

type infoResponse struct {
	Root           string          `json:"root"`
	ExportDir      string          `json:"exportDir"`
	Bounds         boundsResponse  `json:"bounds"`
	ActiveCount    int             `json:"activeCount"`
	RefRegionCount int             `json:"refRegionCount"`
	Regions        []regionSummary `json:"regions"`
}

type meshStatsResponse struct {
	MinHeight float32 `json:"minHeight"`
	MaxHeight float32 `json:"maxHeight"`
}

type regionResponse struct {
	X            int                    `json:"x"`
	Y            int                    `json:"y"`
	Active       bool                   `json:"active"`
	HasMesh      bool                   `json:"hasMesh"`
	HasNVM       bool                   `json:"hasNVM"`
	Stats        meshStatsResponse      `json:"stats"`
	Heights      []float32              `json:"heights"`
	TextureIDs   []uint16               `json:"textureIDs,omitempty"`
	RefRegion    *sromap.RefRegionEntry `json:"refRegion,omitempty"`
	Objects      []objectPlacement      `json:"objects"`
	TextureURL   string                 `json:"textureUrl,omitempty"`
	TextureReady bool                   `json:"textureReady"`
	Warnings     []string               `json:"warnings,omitempty"`
}

type objectPlacement struct {
	ObjID      uint32   `json:"objID"`
	UID        int16    `json:"uid"`
	RegionID   uint16   `json:"regionID"`
	RegionX    int      `json:"regionX"`
	RegionY    int      `json:"regionY"`
	X          float32  `json:"x"`
	Y          float32  `json:"y"`
	Z          float32  `json:"z"`
	Yaw        float32  `json:"yaw"`
	Static     int16    `json:"static"`
	Short0     int16    `json:"short0"`
	Big        bool     `json:"big"`
	Struct     bool     `json:"struct"`
	ObjectPath string   `json:"objectPath,omitempty"`
	IsCPD      bool     `json:"isCpd,omitempty"`
	HostBlocks []string `json:"hostBlocks"`
	LODGroups  []int    `json:"lodGroups"`
}

type saveRegionRequest struct {
	X          int       `json:"x"`
	Y          int       `json:"y"`
	Heights    []float32 `json:"heights"`
	TextureIDs []uint16  `json:"textureIDs,omitempty"`
	SyncNVM    bool      `json:"syncNvm"`
}

type texturePreviewRequest struct {
	X          int                  `json:"x"`
	Y          int                  `json:"y"`
	TextureIDs []uint16             `json:"textureIDs"`
	Patch      *texturePreviewPatch `json:"patch,omitempty"`
}

type texturePreviewPatch struct {
	MinTileX int `json:"minTileX"`
	MinTileZ int `json:"minTileZ"`
	MaxTileX int `json:"maxTileX"`
	MaxTileZ int `json:"maxTileZ"`
}

type saveRegionResponse struct {
	X          int      `json:"x"`
	Y          int      `json:"y"`
	MeshPath   string   `json:"meshPath"`
	NVMPaths   []string `json:"nvmPaths"`
	SavedNVMs  int      `json:"savedNvms"`
	BackupNote string   `json:"backupNote"`
}

func NewServer(root string) (*Server, error) {
	if root == "" {
		root = "."
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	mfo, err := sromap.LoadMapInfo(sromap.MapInfoPath(abs))
	if err != nil {
		return nil, fmt.Errorf("load mapinfo: %w", err)
	}
	refRegions := map[int]sromap.RefRegionEntry{}
	if refs, err := sromap.LoadRefRegions(sromap.RefRegionPath(abs)); err == nil {
		refRegions = refs
	}
	objects := map[uint32]sromap.ObjectInfo{}
	if infos, err := sromap.LoadObjectInfo(sromap.ObjectInfoPath(abs)); err == nil {
		objects = infos
	}
	tiles := map[uint32]sromap.Tile2DEntry{}
	if t, err := sromap.LoadTile2DInfo(sromap.Tile2DInfoPath(abs)); err == nil {
		tiles = t
	}
	customStore, err := NewCustomObjectStore(abs)
	if err != nil {
		return nil, fmt.Errorf("init custom objects: %w", err)
	}
	return &Server{
		Root:          abs,
		ExportDir:     filepath.Join(abs, "export"),
		mapInfo:       mfo,
		refRegions:    refRegions,
		objects:       objects,
		tiles:         tiles,
		textures:      newTextureCache(abs, tiles),
		objCache:      newObjectCache(abs, objects),
		customObjects: customStore,
	}, nil
}

// mirrorDeleteFromExport removes the corresponding export-folder copy of a
// file that was just deleted from Root, so the user can re-import without the
// stale region lingering. Silent if there's no export dir or the path is
// outside Root.
func (s *Server) mirrorDeleteFromExport(srcPath string) error {
	if s.ExportDir == "" {
		return nil
	}
	rel, err := filepath.Rel(s.Root, srcPath)
	if err != nil {
		return err
	}
	if strings.HasPrefix(rel, "..") {
		return nil
	}
	dst := filepath.Join(s.ExportDir, rel)
	if err := os.Remove(dst); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// mirrorToExport copies a file that was just saved under Root to the same
// relative path inside ExportDir. Silent if ExportDir is empty or the path is
// outside Root.
func (s *Server) mirrorToExport(srcPath string) error {
	if s.ExportDir == "" {
		return nil
	}
	rel, err := filepath.Rel(s.Root, srcPath)
	if err != nil {
		return err
	}
	if strings.HasPrefix(rel, "..") {
		return nil
	}
	dst := filepath.Join(s.ExportDir, rel)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/info", s.handleInfo)
	mux.HandleFunc("/api/region", s.handleRegion)
	mux.HandleFunc("/api/region/save", s.handleSaveRegion)
	mux.HandleFunc("/api/region/texture", s.handleRegionTexture)
	mux.HandleFunc("/api/region/texture/preview", s.handleRegionTexturePreview)
	mux.HandleFunc("/api/region/lightmap", s.handleRegionLightmap)
	mux.HandleFunc("/api/region/bake-shadows", s.handleBakeShadows)
	mux.HandleFunc("/api/region/restore-lightmap", s.handleRestoreLightmap)
	mux.HandleFunc("/api/region/rebuild-nvm", s.handleRebuildNVM)
	mux.HandleFunc("/api/region/nvm-cells", s.handleRegionNVMCells)
	mux.HandleFunc("/api/region/restore-nvm", s.handleRestoreNVM)
	mux.HandleFunc("/api/region/objects/save", s.handleSaveRegionObjects)
	mux.HandleFunc("/api/region/create", s.handleCreateRegions)
	mux.HandleFunc("/api/region/delete", s.handleDeleteRegions)
	mux.HandleFunc("/api/region/duplicate", s.handleDuplicateRegions)
	mux.HandleFunc("/api/tile", s.handleTile)
	mux.HandleFunc("/api/tiles", s.handleTiles)
	mux.HandleFunc("/api/object", s.handleObject)
	mux.HandleFunc("/api/objects", s.handleObjectInfoList)
	mux.HandleFunc("/api/asset", s.handleAsset)
	mux.HandleFunc("/api/custom-object", s.handleCustomObject)
	mux.HandleFunc("/api/custom-objects", s.handleCustomObjectsList)
	mux.HandleFunc("/api/custom-object/import", s.handleCustomObjectImport)
	mux.HandleFunc("/api/custom-asset", s.handleCustomAsset)
	mux.HandleFunc("/api/region/custom-placements", s.handleRegionCustomPlacements)
	mux.HandleFunc("/api/custom-object/export", s.handleCustomObjectExport)

	static, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(static)))
	return mux
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	minX, maxX, minY, maxY, ok := s.mapInfo.Bounds()
	if !ok {
		minX, maxX, minY, maxY = 0, 255, 0, 127
	}
	regions := s.mapInfo.ActiveRegions()
	out := infoResponse{
		Root:           s.Root,
		ExportDir:      s.ExportDir,
		Bounds:         boundsResponse{MinX: minX, MaxX: maxX, MinY: minY, MaxY: maxY, CenterX: (minX + maxX) / 2, CenterY: (minY + maxY) / 2},
		ActiveCount:    len(regions),
		RefRegionCount: len(s.refRegions),
		Regions:        make([]regionSummary, 0, len(regions)),
	}
	for _, region := range regions {
		_, hasRef := s.refRegions[int(region.ID)]
		out.Regions = append(out.Regions, regionSummary{
			ID: region.ID, X: region.X, Y: region.Y, IsDungeon: region.IsDungeon, HasRef: hasRef,
		})
	}
	writeJSON(w, out)
}

func (s *Server) handleRegion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	x, y, err := parseRegionQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	out := regionResponse{
		X:       x,
		Y:       y,
		Active:  s.mapInfo.HasRegion(x, y),
		HasNVM:  len(sromap.ExistingNVMPaths(s.Root, x, y)) > 0,
		Objects: []objectPlacement{},
	}
	if ref, ok := s.refRegions[y<<8|x]; ok {
		refCopy := ref
		out.RefRegion = &refCopy
	}

	mesh, err := sromap.LoadMesh(sromap.MeshPath(s.Root, x, y))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeJSON(w, out)
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out.HasMesh = true
	stats := mesh.Stats()
	out.Stats = meshStatsResponse{MinHeight: stats.MinHeight, MaxHeight: stats.MaxHeight}
	heightMap := mesh.UniqueHeightMap()
	out.Heights = make([]float32, len(heightMap))
	copy(out.Heights, heightMap[:])
	tileIDs, _, _ := mesh.UniqueTextureMap()
	out.TextureIDs = make([]uint16, len(tileIDs))
	copy(out.TextureIDs, tileIDs[:])

	if o2, err := sromap.LoadO2(sromap.O2Path(s.Root, x, y)); err == nil {
		out.Objects = s.uniqueObjectPlacements(o2.Entries)
	} else if !errors.Is(err, os.ErrNotExist) {
		out.Warnings = append(out.Warnings, "O2: "+err.Error())
	}

	if len(s.tiles) > 0 {
		v := s.textures.regionVersion(x, y)
		out.TextureURL = fmt.Sprintf("/api/region/texture?x=%d&y=%d&v=%d", x, y, v)
		out.TextureReady = true
	}

	writeJSON(w, out)
}

func (s *Server) handleRegionNVMCells(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	x, y, err := parseRegionQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	paths := sromap.ExistingNVMPaths(s.Root, x, y)
	if len(paths) == 0 {
		writeError(w, http.StatusNotFound, "no NVM")
		return
	}
	nvm, err := sromap.LoadNVM(paths[0])
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	type cell struct {
		Index         int      `json:"index"`
		MinX          float32  `json:"minX"`
		MinZ          float32  `json:"minZ"`
		MaxX          float32  `json:"maxX"`
		MaxZ          float32  `json:"maxZ"`
		Open          bool     `json:"open"`
		ObjectIndices []uint16 `json:"objectIndices,omitempty"`
	}
	type object struct {
		Index    int     `json:"index"`
		AssetID  uint32  `json:"assetID"`
		UID      int16   `json:"uid"`
		RegionID uint16  `json:"regionID"`
		X        float32 `json:"x"`
		Y        float32 `json:"y"`
		Z        float32 `json:"z"`
		Yaw      float32 `json:"yaw"`
		Links    int     `json:"links,omitempty"`
	}
	cells := make([]cell, len(nvm.Cells))
	for i, c := range nvm.Cells {
		objectIndices := append([]uint16(nil), c.ObjectIndices...)
		cells[i] = cell{
			Index: i,
			MinX:  c.MinX, MinZ: c.MinZ, MaxX: c.MaxX, MaxZ: c.MaxZ,
			Open:          uint32(i) < nvm.OpenCellCount,
			ObjectIndices: objectIndices,
		}
	}
	objects := make([]object, len(nvm.Objects))
	for i, obj := range nvm.Objects {
		objects[i] = object{
			Index:    i,
			AssetID:  obj.AssetID,
			UID:      obj.UID,
			RegionID: obj.RegionID,
			X:        obj.X,
			Y:        obj.Y,
			Z:        obj.Z,
			Yaw:      obj.Yaw,
			Links:    len(obj.Links),
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, map[string]any{
		"cells":     cells,
		"objects":   objects,
		"openCount": nvm.OpenCellCount,
	})
}

func (s *Server) handleRestoreNVM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	x, y, err := parseRegionQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	paths := sromap.ExistingNVMPaths(s.Root, x, y)
	restored := 0
	skipped := 0
	for _, p := range paths {
		bak := p + ".bak"
		if _, err := os.Stat(bak); errors.Is(err, os.ErrNotExist) {
			skipped++
			continue
		} else if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		in, err := os.Open(bak)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		out, err := os.Create(p)
		if err != nil {
			in.Close()
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if _, err := io.Copy(out, in); err != nil {
			in.Close()
			out.Close()
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		in.Close()
		if err := out.Close(); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		restored++
	}
	writeJSON(w, map[string]any{
		"restored": restored,
		"skipped":  skipped,
		"paths":    paths,
	})
}

func (s *Server) handleRebuildNVM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	x, y, err := parseRegionQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	slope := float32(DefaultNVMSlopeThreshold)
	if v := r.URL.Query().Get("slope"); v != "" {
		if f, err := strconv.ParseFloat(v, 32); err == nil && f > 0 && f < 1000 {
			slope = float32(f)
		}
	}

	fullRebuild := r.URL.Query().Get("full") == "1"
	nvmPaths := sromap.ExistingNVMPaths(s.Root, x, y)
	createdNew := false
	allowCreate := r.URL.Query().Get("create") == "1"
	if len(nvmPaths) == 0 {
		if !allowCreate {
			writeError(w, http.StatusNotFound, "no NVM for region (export base-game NVM first, or pass create=1)")
			return
		}
		if !fullRebuild {
			// Creating a fresh NVM only makes sense with a full rebuild —
			// otherwise we'd write an empty file.
			writeError(w, http.StatusBadRequest, "create=1 requires full=1")
			return
		}
		defaultPath := filepath.Join(s.Root, "Data", "Navmesh", sromap.NVMFileName(x, y))
		if err := os.MkdirAll(filepath.Dir(defaultPath), 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "mkdir "+filepath.Dir(defaultPath)+": "+err.Error())
			return
		}
		nvmPaths = []string{defaultPath}
		createdNew = true
	}
	mesh, err := sromap.LoadMesh(sromap.MeshPath(s.Root, x, y))
	if err != nil {
		writeError(w, http.StatusNotFound, "mesh not found")
		return
	}

	placements := s.loadNavRebuildPlacements(x, y)

	walkable := s.computeWalkability(mesh, placements, slope)
	walkableCount := 0
	for _, w := range walkable {
		if w {
			walkableCount++
		}
	}

	heights := mesh.UniqueHeightMap()
	var lastCells, lastEdges int
	var lastOpenCells uint32
	neighborEdgePatches := 0
	for _, path := range nvmPaths {
		var nvm *sromap.NVM
		if fullRebuild {
			if !createdNew {
				if err := backupOnce(path); err != nil {
					writeError(w, http.StatusInternalServerError, "backup "+path+": "+err.Error())
					return
				}
			}
			mapPlacements := s.loadMapOnlyPlacements(x, y)
			built := s.buildMapOnlyNVM(x, y, mesh, mapPlacements, slope, true)
			nvm = built.NVM
			nvm.Path = path
			walkableCount = built.WalkableCount
			if err := nvm.Save(path); err != nil {
				writeError(w, http.StatusInternalServerError, "save NVM "+path+": "+err.Error())
				return
			}
			_ = s.mirrorToExport(path)
			patched, err := s.syncReciprocalGlobalEdges(x, y, nvm)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "sync neighbor NVM edges: "+err.Error())
				return
			}
			neighborEdgePatches += patched
			lastCells = len(nvm.Cells)
			lastEdges = len(nvm.InternalEdges)
			lastOpenCells = nvm.OpenCellCount
			continue
		}
		if createdNew {
			nvm = &sromap.NVM{Path: path}
		} else {
			loaded, err := sromap.LoadNVM(path)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "load NVM "+path+": "+err.Error())
				return
			}
			nvm = loaded
			if err := backupOnce(path); err != nil {
				writeError(w, http.StatusInternalServerError, "backup "+path+": "+err.Error())
				return
			}
		}
		if err := nvm.SetHeightMap(heights[:]); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if fullRebuild {
			// FULL REPARTITION — destructive. Regenerates the entire
			// cell graph as a uniform 240×240 grid. The rebuild tool
			// we benchmarked against does NOT treat placements as
			// non-walkable for the partition pass (its open cells
			// run right through object footprints; collision comes
			// from cell→ObjectIndices→BSR/BMS, not from carving the
			// cell graph around bboxes). Use all-walkable here so
			// we produce clean 240×240 cells, not thin strips between
			// objects.
			var allWalkable [sromap.NVMTotalTiles]bool
			for i := range allWalkable {
				allWalkable[i] = true
			}
			s.addCustomNVMObjectsOnly(nvm, placements)
			sromap.ApplyNVMNavRebuild(nvm, allWalkable)
			s.addCustomBboxObjectIndices(nvm)
		} else {
			// DEFAULT — matches the working "rebuild tool" output.
			// Preserves the baked cell graph and DOES NOT touch tile
			// flags. The rebuild tool we reverse-engineered leaves
			// tile.Flag=0x00 around custom assets; collision comes
			// from the cell→ObjectIndices→BSR/BMS chain, not tile
			// bits. Calling ApplyNVMTileFlags here was breaking slope
			// walkability (it stamped Flag=1 on every tile under
			// every placement's AABB, including stock objects whose
			// AABBs cover ramps/stairs/walkable surfaces).
			//
			// What we DO is:
			//   1. Sync custom-asset NVMObjects with the latest .o2
			//      (append if new, UPDATE position+yaw if existing
			//      drifted — fixes the staleness bug where moving a
			//      custom placement left the NVMObject pointing at
			//      the old location).
			//   2. For each CUSTOM NVMObject's rotated world bbox,
			//      find OPEN cells that meaningfully overlap AND are
			//      bigger than one 240-grid cell on either axis. Split
			//      those cells into 240×240 sub-cells, re-routing tiles
			//      and edges. (Without this, a baseline 480×480 open
			//      cell catching the custom asset's bbox would receive
			//      a single ObjectIndex that the engine treats as "the
			//      whole 480×480 region is this asset", breaking the
			//      navmesh around it. The working rebuild tool produces
			//      4 separate 240-grid cells in exactly this case.)
			//   3. For each CUSTOM NVMObject, compute its rotated
			//      world bbox and append its index to every cell
			//      whose AABB intersects (with min-overlap threshold).
			s.addCustomNVMObjectsOnly(nvm, placements)
			s.splitOversizedCellsForCustom(nvm, uint16(y)<<8|uint16(x))
			s.addCustomBboxObjectIndices(nvm)
		}
		if err := nvm.Save(path); err != nil {
			writeError(w, http.StatusInternalServerError, "save NVM "+path+": "+err.Error())
			return
		}
		_ = s.mirrorToExport(path)
		patched, err := s.syncReciprocalGlobalEdges(x, y, nvm)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "sync neighbor NVM edges: "+err.Error())
			return
		}
		neighborEdgePatches += patched
		lastCells = len(nvm.Cells)
		lastEdges = len(nvm.InternalEdges)
		lastOpenCells = nvm.OpenCellCount
	}

	mode := "tileflags"
	if fullRebuild {
		mode = "map-only"
	}
	writeJSON(w, map[string]any{
		"paths":         nvmPaths,
		"mode":          mode,
		"createdNew":    createdNew,
		"placements":    len(placements),
		"walkableTiles": walkableCount,
		"totalTiles":    sromap.NVMTotalTiles,
		"cells":         lastCells,
		"openCells":     lastOpenCells,
		"internalEdges": lastEdges,
		"neighborEdges": neighborEdgePatches,
		"slope":         slope,
	})
}

func (s *Server) handleRestoreLightmap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	x, y, err := parseRegionQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	tPath := sromap.LightmapPath(s.Root, x, y)
	bakPath := tPath + ".bak"
	if _, err := os.Stat(bakPath); errors.Is(err, os.ErrNotExist) {
		writeJSON(w, map[string]any{
			"restored": false,
			"reason":   "no backup",
			"path":     tPath,
		})
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	in, err := os.Open(bakPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "open bak: "+err.Error())
		return
	}
	defer in.Close()
	out, err := os.Create(tPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create .t: "+err.Error())
		return
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		writeError(w, http.StatusInternalServerError, "copy: "+err.Error())
		return
	}
	if err := out.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "close: "+err.Error())
		return
	}
	s.textures.invalidateLightmap(x, y)
	writeJSON(w, map[string]any{
		"restored": true,
		"path":     tPath,
		"source":   bakPath,
	})
}

func (s *Server) handleBakeShadows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	x, y, err := parseRegionQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mesh, err := sromap.LoadMesh(sromap.MeshPath(s.Root, x, y))
	if err != nil {
		writeError(w, http.StatusNotFound, "mesh not found")
		return
	}
	heights := mesh.UniqueHeightMap()

	o2, err := sromap.LoadO2(sromap.O2Path(s.Root, x, y))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load .o2: "+err.Error())
		return
	}
	targetRegionID := uint16(y)<<8 | uint16(x)

	// Collect unique placements local to this region.
	type pkey struct {
		uid   int16
		objID uint32
	}
	seen := make(map[pkey]bool)
	type placement struct {
		x, y, z, yaw float32
		objID        uint32
	}
	var placements []placement
	for _, e := range o2.Entries {
		if e.RegionID != targetRegionID {
			continue
		}
		k := pkey{e.UID, e.ObjID}
		if seen[k] {
			continue
		}
		seen[k] = true
		placements = append(placements, placement{x: e.X, y: e.Y, z: e.Z, yaw: e.Yaw, objID: e.ObjID})
	}

	// Build the triangle list in region-local coordinates.
	var tris []sromap.Triangle
	for _, p := range placements {
		asset, _ := s.objCache.get(p.objID)
		if asset == nil {
			continue
		}
		c := float32(math.Cos(float64(p.yaw)))
		sn := float32(math.Sin(float64(p.yaw)))
		for _, m := range asset.Meshes {
			for i := 0; i+2 < len(m.Indices); i += 3 {
				a := transformVertex(m, m.Indices[i], p.x, p.y, p.z, c, sn)
				b := transformVertex(m, m.Indices[i+1], p.x, p.y, p.z, c, sn)
				cc := transformVertex(m, m.Indices[i+2], p.x, p.y, p.z, c, sn)
				tris = append(tris, sromap.Triangle{A: a, B: b, C: cc})
			}
		}
	}

	bvh := sromap.BuildBVH(tris)

	params := sromap.DefaultShadowParams()
	if v := r.URL.Query().Get("azimuth"); v != "" {
		if az, err := strconv.ParseFloat(v, 32); err == nil {
			el := 60.0
			if v2 := r.URL.Query().Get("elevation"); v2 != "" {
				if e, err := strconv.ParseFloat(v2, 32); err == nil {
					el = e
				}
			}
			params.SunDir = sromap.SunFromAngles(float32(az), float32(el))
		}
	}
	if v := r.URL.Query().Get("samples"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 64 {
			params.Samples = n
		}
	}
	if v := r.URL.Query().Get("softness"); v != "" {
		if s, err := strconv.ParseFloat(v, 32); err == nil && s >= 0 && s <= 0.5 {
			params.SunRadius = float32(s)
		}
	}
	const lmSize = 512
	rgba := sromap.BakeShadows(heights, bvh, lmSize, lmSize, params)

	tPath := sromap.LightmapPath(s.Root, x, y)
	if _, err := os.Stat(tPath); err == nil {
		_ = backupOnce(tPath)
	}
	var tileLightmap []byte
	if existing, err := os.ReadFile(tPath); err == nil && len(existing) >= 12+96*96 {
		tileLightmap = make([]byte, 96*96)
		copy(tileLightmap, existing[12:12+96*96])
	}
	if err := sromap.SaveLightmap(tPath, rgba, lmSize, lmSize, tileLightmap); err != nil {
		writeError(w, http.StatusInternalServerError, "save .t: "+err.Error())
		return
	}
	_ = s.mirrorToExport(tPath)
	s.textures.invalidateLightmap(x, y)

	writeJSON(w, map[string]any{
		"path":       tPath,
		"placements": len(placements),
		"triangles":  len(tris),
	})
}

func transformVertex(m objectMesh, idx uint16, px, py, pz, cosY, sinY float32) [3]float32 {
	off := int(idx) * 5
	bx := m.Vertices[off]
	by := m.Vertices[off+1]
	bz := m.Vertices[off+2]
	return [3]float32{
		px + cosY*bx - sinY*bz,
		py + by,
		pz + sinY*bx + cosY*bz,
	}
}

func (s *Server) handleRegionLightmap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	x, y, err := parseRegionQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	data, ok := s.textures.lightmapPNG(s.Root, x, y)
	if !ok {
		writeError(w, http.StatusNotFound, "no lightmap")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=600")
	_, _ = w.Write(data)
}

func (s *Server) handleRegionTexture(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	x, y, err := parseRegionQuery(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(s.tiles) == 0 {
		writeError(w, http.StatusNotFound, "tile2d.ifo not loaded")
		return
	}
	mesh, err := sromap.LoadMesh(sromap.MeshPath(s.Root, x, y))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, "mesh not found")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rt, err := s.textures.regionComposite(x, y, mesh)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=600")
	w.Header().Set("X-Texture-Size", fmt.Sprintf("%dx%d", rt.width, rt.height))
	_, _ = w.Write(rt.png)
}

func (s *Server) handleRegionTexturePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if len(s.tiles) == 0 {
		writeError(w, http.StatusNotFound, "tile2d.ifo not loaded")
		return
	}
	var req texturePreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode request: "+err.Error())
		return
	}
	if err := validateRegion(req.X, req.Y); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	var (
		rt  *regionTexture
		err error
	)
	if req.Patch != nil {
		p := req.Patch
		if p.MinTileX < 0 || p.MinTileZ < 0 || p.MaxTileX >= sromap.NVMTileCount || p.MaxTileZ >= sromap.NVMTileCount ||
			p.MinTileX > p.MaxTileX || p.MinTileZ > p.MaxTileZ {
			writeError(w, http.StatusBadRequest, "invalid texture patch bounds")
			return
		}
		expectedPatchIDs := (p.MaxTileX - p.MinTileX + 2) * (p.MaxTileZ - p.MinTileZ + 2)
		if r.URL.Query().Get("raw") == "1" {
			var img *image.RGBA
			switch len(req.TextureIDs) {
			case sromap.MeshGridSize * sromap.MeshGridSize:
				img, err = s.textures.buildCompositeIDRange(req.TextureIDs, p.MinTileX, p.MinTileZ, p.MaxTileX, p.MaxTileZ)
			case expectedPatchIDs:
				img, err = s.textures.buildCompositePatchGrid(req.TextureIDs, p.MinTileX, p.MinTileZ, p.MaxTileX, p.MaxTileZ)
			default:
				writeError(w, http.StatusBadRequest, fmt.Sprintf("texture patch must contain %d or %d values, got %d", sromap.MeshGridSize*sromap.MeshGridSize, expectedPatchIDs, len(req.TextureIDs)))
				return
			}
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Texture-Format", "rgba8")
			w.Header().Set("X-Texture-Size", fmt.Sprintf("%dx%d", img.Rect.Dx(), img.Rect.Dy()))
			w.Header().Set("X-Texture-Offset", fmt.Sprintf("%d,%d", p.MinTileX*compositeTileSize, p.MinTileZ*compositeTileSize))
			_, _ = w.Write(img.Pix[:img.Rect.Dy()*img.Stride])
			return
		}
		switch len(req.TextureIDs) {
		case sromap.MeshGridSize * sromap.MeshGridSize:
			rt, err = s.textures.regionCompositePatchFromIDs(req.TextureIDs, p.MinTileX, p.MinTileZ, p.MaxTileX, p.MaxTileZ)
		case expectedPatchIDs:
			rt, err = s.textures.regionCompositePatchGridFromIDs(req.TextureIDs, p.MinTileX, p.MinTileZ, p.MaxTileX, p.MaxTileZ)
		default:
			writeError(w, http.StatusBadRequest, fmt.Sprintf("texture patch must contain %d or %d values, got %d", sromap.MeshGridSize*sromap.MeshGridSize, expectedPatchIDs, len(req.TextureIDs)))
			return
		}
		w.Header().Set("X-Texture-Offset", fmt.Sprintf("%d,%d", p.MinTileX*compositeTileSize, p.MinTileZ*compositeTileSize))
	} else {
		if len(req.TextureIDs) != sromap.MeshGridSize*sromap.MeshGridSize {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("texture map must contain %d values", sromap.MeshGridSize*sromap.MeshGridSize))
			return
		}
		rt, err = s.textures.regionCompositeFromIDs(req.TextureIDs)
		w.Header().Set("X-Texture-Offset", "0,0")
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Texture-Size", fmt.Sprintf("%dx%d", rt.width, rt.height))
	_, _ = w.Write(rt.png)
}

func (s *Server) handleTile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	idStr := r.URL.Query().Get("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	data, ok := s.textures.tilePNG(uint32(id))
	if !ok {
		writeError(w, http.StatusNotFound, "tile not available")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(data)
}

func (s *Server) handleTiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	entries := make([]sromap.Tile2DEntry, 0, len(s.tiles))
	for _, e := range s.tiles {
		entries = append(entries, e)
	}
	writeJSON(w, map[string]any{"count": len(entries), "entries": entries})
}

func (s *Server) handleObject(w http.ResponseWriter, r *http.Request) {
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
	asset, reason := s.objCache.get(uint32(id64))
	if asset == nil {
		writeError(w, http.StatusNotFound, reason)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, asset)
}

func (s *Server) handleCreateRegions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var req struct {
		Regions []struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"regions"`
		Height        float32 `json:"height"`
		DefaultTileID uint16  `json:"defaultTileID"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode: "+err.Error())
		return
	}
	if len(req.Regions) == 0 {
		writeError(w, http.StatusBadRequest, "no regions")
		return
	}
	if len(req.Regions) > 4096 {
		writeError(w, http.StatusBadRequest, "too many regions in one request")
		return
	}
	template, templatePath, err := s.findTemplateMesh()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "find template mesh: "+err.Error())
		return
	}

	var created []map[string]int
	var skipped []map[string]any
	mapInfoChanged := false
	for _, rg := range req.Regions {
		if err := validateRegion(rg.X, rg.Y); err != nil {
			skipped = append(skipped, map[string]any{"x": rg.X, "y": rg.Y, "reason": err.Error()})
			continue
		}
		meshPath := sromap.MeshPath(s.Root, rg.X, rg.Y)
		if _, err := os.Stat(meshPath); err == nil {
			skipped = append(skipped, map[string]any{"x": rg.X, "y": rg.Y, "reason": "mesh already exists"})
			continue
		}
		flat, err := sromap.NewFlatMesh(template, req.Height, req.DefaultTileID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "build flat mesh: "+err.Error())
			return
		}
		if err := os.MkdirAll(filepath.Dir(meshPath), 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "mkdir: "+err.Error())
			return
		}
		if err := flat.Save(meshPath); err != nil {
			writeError(w, http.StatusInternalServerError, "write mesh "+meshPath+": "+err.Error())
			return
		}
		_ = s.mirrorToExport(meshPath)

		// Generate a clean minimal NVM next to the mesh so the server can
		// pathfind into the new region. Without this the region is visible
		// in the client but the server refuses to spawn players there.
		nvm := sromap.NewFlatNVM(req.Height)
		nvmPath := filepath.Join(s.Root, "Data", "Navmesh", sromap.NVMFileName(rg.X, rg.Y))
		if err := os.MkdirAll(filepath.Dir(nvmPath), 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "mkdir navmesh: "+err.Error())
			return
		}
		if err := nvm.Save(nvmPath); err != nil {
			writeError(w, http.StatusInternalServerError, "write nvm "+nvmPath+": "+err.Error())
			return
		}
		_ = s.mirrorToExport(nvmPath)

		if s.mapInfo.SetRegion(rg.X, rg.Y, true) {
			mapInfoChanged = true
		}
		created = append(created, map[string]int{"x": rg.X, "y": rg.Y})
	}
	if mapInfoChanged {
		if err := s.mapInfo.Save(sromap.MapInfoPath(s.Root)); err != nil {
			writeError(w, http.StatusInternalServerError, "save mapinfo: "+err.Error())
			return
		}
		_ = s.mirrorToExport(sromap.MapInfoPath(s.Root))
	}
	writeJSON(w, map[string]any{
		"created":      created,
		"skipped":      skipped,
		"template":     templatePath,
		"mapInfoSaved": mapInfoChanged,
	})
}

func (s *Server) handleDeleteRegions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	var req struct {
		Regions []struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"regions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode: "+err.Error())
		return
	}
	if len(req.Regions) == 0 {
		writeError(w, http.StatusBadRequest, "no regions")
		return
	}
	if len(req.Regions) > 4096 {
		writeError(w, http.StatusBadRequest, "too many regions in one request")
		return
	}

	type result struct {
		X, Y         int      `json:"x" json:",inline"`
		FilesRemoved []string `json:"files"`
	}
	var deleted []map[string]any
	var skipped []map[string]any
	mapInfoChanged := false
	for _, rg := range req.Regions {
		if err := validateRegion(rg.X, rg.Y); err != nil {
			skipped = append(skipped, map[string]any{"x": rg.X, "y": rg.Y, "reason": err.Error()})
			continue
		}
		var removed []string
		removeIfExists := func(path string) {
			if path == "" {
				return
			}
			if _, err := os.Stat(path); err != nil {
				return
			}
			if err := os.Remove(path); err == nil {
				removed = append(removed, path)
				_ = s.mirrorDeleteFromExport(path)
			}
		}
		removeIfExists(sromap.MeshPath(s.Root, rg.X, rg.Y))
		removeIfExists(sromap.O2Path(s.Root, rg.X, rg.Y))
		for _, p := range sromap.ExistingNVMPaths(s.Root, rg.X, rg.Y) {
			removeIfExists(p)
		}
		if s.mapInfo.SetRegion(rg.X, rg.Y, false) {
			mapInfoChanged = true
		}
		// Invalidate cached terrain composite so a re-create doesn't show stale tiles.
		s.textures.invalidateRegion(rg.X, rg.Y)
		if len(removed) == 0 && !mapInfoChanged {
			skipped = append(skipped, map[string]any{"x": rg.X, "y": rg.Y, "reason": "nothing to delete"})
			continue
		}
		deleted = append(deleted, map[string]any{"x": rg.X, "y": rg.Y, "files": removed})
	}
	if mapInfoChanged {
		if err := s.mapInfo.Save(sromap.MapInfoPath(s.Root)); err != nil {
			writeError(w, http.StatusInternalServerError, "save mapinfo: "+err.Error())
			return
		}
		_ = s.mirrorToExport(sromap.MapInfoPath(s.Root))
	}
	writeJSON(w, map[string]any{
		"deleted":      deleted,
		"skipped":      skipped,
		"mapInfoSaved": mapInfoChanged,
	})
}

func (s *Server) handleDuplicateRegions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()
	type copyOp struct {
		SrcX int `json:"srcX"`
		SrcY int `json:"srcY"`
		DstX int `json:"dstX"`
		DstY int `json:"dstY"`
	}
	var req struct {
		Copies       []copyOp `json:"copies"`
		Overwrite    bool     `json:"overwrite"`
		TargetFloorY *float32 `json:"targetFloorY"`
		Rotation     int      `json:"rotation"`
		FlipX        bool     `json:"flipX"`
		FlipZ        bool     `json:"flipZ"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode: "+err.Error())
		return
	}
	if len(req.Copies) == 0 {
		writeError(w, http.StatusBadRequest, "no copies")
		return
	}
	if len(req.Copies) > 4096 {
		writeError(w, http.StatusBadRequest, "too many copies in one request")
		return
	}
	switch req.Rotation {
	case 0, 90, 180, 270:
	default:
		writeError(w, http.StatusBadRequest, "rotation must be 0, 90, 180 or 270")
		return
	}
	transform := sromap.RegionTransform{
		Rotation: req.Rotation,
		FlipX:    req.FlipX,
		FlipZ:    req.FlipZ,
	}

	// If a target floor height is given, compute a single delta from the
	// global minimum across all source meshes so the bundle's vertical
	// relationships (cliffs, slopes between regions) are preserved.
	var heightDelta float32
	sourceFloorY := float32(math.Inf(1))
	if req.TargetFloorY != nil {
		for _, c := range req.Copies {
			meshPath := sromap.MeshPath(s.Root, c.SrcX, c.SrcY)
			mesh, err := sromap.LoadMesh(meshPath)
			if err != nil {
				continue
			}
			if st := mesh.Stats(); st.MinHeight < sourceFloorY {
				sourceFloorY = st.MinHeight
			}
		}
		if !math.IsInf(float64(sourceFloorY), 1) {
			heightDelta = *req.TargetFloorY - sourceFloorY
		}
	}

	var done []map[string]any
	var skipped []map[string]any
	mapInfoChanged := false
	for _, c := range req.Copies {
		if err := validateRegion(c.SrcX, c.SrcY); err != nil {
			skipped = append(skipped, map[string]any{"srcX": c.SrcX, "srcY": c.SrcY, "reason": "src " + err.Error()})
			continue
		}
		if err := validateRegion(c.DstX, c.DstY); err != nil {
			skipped = append(skipped, map[string]any{"dstX": c.DstX, "dstY": c.DstY, "reason": "dst " + err.Error()})
			continue
		}
		if c.SrcX == c.DstX && c.SrcY == c.DstY {
			skipped = append(skipped, map[string]any{"srcX": c.SrcX, "srcY": c.SrcY, "reason": "src == dst"})
			continue
		}
		srcMesh := sromap.MeshPath(s.Root, c.SrcX, c.SrcY)
		if _, err := os.Stat(srcMesh); err != nil {
			skipped = append(skipped, map[string]any{"srcX": c.SrcX, "srcY": c.SrcY, "reason": "no source mesh"})
			continue
		}
		dstMesh := sromap.MeshPath(s.Root, c.DstX, c.DstY)
		if !req.Overwrite {
			if _, err := os.Stat(dstMesh); err == nil {
				skipped = append(skipped, map[string]any{"dstX": c.DstX, "dstY": c.DstY, "reason": "destination not empty"})
				continue
			}
		}

		if err := os.MkdirAll(filepath.Dir(dstMesh), 0755); err != nil {
			writeError(w, http.StatusInternalServerError, "mkdir: "+err.Error())
			return
		}
		needsRewrite := heightDelta != 0 || !transform.IsIdentity()
		if !needsRewrite {
			if data, err := os.ReadFile(srcMesh); err == nil {
				if req.Overwrite {
					_ = backupOnce(dstMesh)
				}
				if err := os.WriteFile(dstMesh, data, 0644); err != nil {
					writeError(w, http.StatusInternalServerError, "write dst mesh: "+err.Error())
					return
				}
				_ = s.mirrorToExport(dstMesh)
			}
		} else {
			mesh, err := sromap.LoadMesh(srcMesh)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "load src mesh for rewrite: "+err.Error())
				return
			}
			heights := mesh.UniqueHeightMap()
			tileIDs, _, _ := mesh.UniqueTextureMap()
			// Apply spatial transform first, then height shift.
			heightsT := transform.PermuteFloat32Grid(heights[:], sromap.MeshGridSize)
			for i := range heightsT {
				heightsT[i] += heightDelta
			}
			tileIDsT := transform.PermuteUint16Grid(tileIDs[:], sromap.MeshGridSize)
			if err := mesh.SetUniqueHeightMap(heightsT); err != nil {
				writeError(w, http.StatusInternalServerError, "set mesh heights: "+err.Error())
				return
			}
			if err := mesh.SetUniqueTextureIDs(tileIDsT); err != nil {
				writeError(w, http.StatusInternalServerError, "set mesh tile IDs: "+err.Error())
				return
			}
			if req.Overwrite {
				_ = backupOnce(dstMesh)
			}
			if err := mesh.Save(dstMesh); err != nil {
				writeError(w, http.StatusInternalServerError, "write transformed mesh: "+err.Error())
				return
			}
			_ = s.mirrorToExport(dstMesh)
		}

		srcRegionID := uint16(c.SrcY)<<8 | uint16(c.SrcX)
		dstRegionID := uint16(c.DstY)<<8 | uint16(c.DstX)

		// Objects: copy entries that belong to the source region, remap regionID,
		// optionally rotate/flip position+yaw and shift Y.
		srcO2 := sromap.O2Path(s.Root, c.SrcX, c.SrcY)
		if o2, err := sromap.LoadO2(srcO2); err == nil {
			out := &sromap.O2{Entries: make([]sromap.ObjectEntry, 0, len(o2.Entries))}
			for _, e := range o2.Entries {
				if e.RegionID != srcRegionID {
					continue
				}
				ne := e
				ne.RegionID = dstRegionID
				ne.X, ne.Z = transform.TransformPos(ne.X, ne.Z, float32(sromap.RegionSize))
				ne.Yaw = transform.TransformYaw(ne.Yaw)
				ne.Y += heightDelta
				out.Entries = append(out.Entries, ne)
			}
			dstO2 := sromap.O2Path(s.Root, c.DstX, c.DstY)
			if req.Overwrite {
				_ = backupOnce(dstO2)
			}
			if err := out.Save(dstO2); err != nil {
				writeError(w, http.StatusInternalServerError, "write dst o2: "+err.Error())
				return
			}
			_ = s.mirrorToExport(dstO2)
		}

		// NVM: copy first matching source NVM, remap NVM-object region IDs,
		// optionally rotate/flip its spatial data, strip global edges.
		if srcPaths := sromap.ExistingNVMPaths(s.Root, c.SrcX, c.SrcY); len(srcPaths) > 0 {
			nvm, err := sromap.LoadNVM(srcPaths[0])
			if err == nil {
				regionSize := float32(sromap.RegionSize)
				for i := range nvm.Objects {
					o := &nvm.Objects[i]
					if o.RegionID == srcRegionID {
						o.RegionID = dstRegionID
					}
					o.X, o.Z = transform.TransformPos(o.X, o.Z, regionSize)
					o.Yaw = transform.TransformYaw(o.Yaw)
					o.Y += heightDelta
				}
				if !transform.IsIdentity() {
					// Heights and plane data are 97×97 and 6×6 respectively.
					hs := transform.PermuteFloat32Grid(nvm.Heights[:], sromap.MeshGridSize)
					copy(nvm.Heights[:], hs)
					pt := transform.PermuteByteGrid(nvm.PlaneType[:], sromap.MeshBlockCount)
					copy(nvm.PlaneType[:], pt)
					ph := transform.PermuteFloat32Grid(nvm.PlaneHeight[:], sromap.MeshBlockCount)
					copy(nvm.PlaneHeight[:], ph)
					// Tile grid (96×96): walk every tile to a new position.
					var newTiles [sromap.NVMTotalTiles]sromap.NVMTile
					for gz := 0; gz < sromap.NVMTileCount; gz++ {
						for gx := 0; gx < sromap.NVMTileCount; gx++ {
							nx, nz := transform.TransformGrid(gx, gz, sromap.NVMTileCount)
							newTiles[nz*sromap.NVMTileCount+nx] = nvm.Tiles[gz*sromap.NVMTileCount+gx]
						}
					}
					nvm.Tiles = newTiles
					// Cells: rotate each AABB. CellID inside each tile still
					// points into the same cell slice, so we don't reorder cells.
					for i := range nvm.Cells {
						c := &nvm.Cells[i]
						c.MinX, c.MinZ, c.MaxX, c.MaxZ =
							transform.TransformBounds(c.MinX, c.MinZ, c.MaxX, c.MaxZ, regionSize)
					}
					for i := range nvm.InternalEdges {
						e := &nvm.InternalEdges[i]
						e.MinX, e.MinZ, e.MaxX, e.MaxZ =
							transform.TransformBounds(e.MinX, e.MinZ, e.MaxX, e.MaxZ, regionSize)
					}
				}
				if heightDelta != 0 {
					for i := range nvm.Heights {
						nvm.Heights[i] += heightDelta
					}
					for i := range nvm.PlaneHeight {
						nvm.PlaneHeight[i] += heightDelta
					}
				}
				nvm.GlobalEdges = nil
				dstName := sromap.NVMFileName(c.DstX, c.DstY)
				dstDir := filepath.Join(s.Root, "Data", "Navmesh")
				if err := os.MkdirAll(dstDir, 0755); err == nil {
					dstNVM := filepath.Join(dstDir, dstName)
					if req.Overwrite {
						_ = backupOnce(dstNVM)
					}
					if err := nvm.Save(dstNVM); err == nil {
						_ = s.mirrorToExport(dstNVM)
					}
				}
			}
		}

		// Lightmap (.t): byte-copy from source so the destination's prebaked
		// shadows match the new terrain. If the source has no .t, remove any
		// stale .t at the destination so it won't bleed through.
		srcT := sromap.LightmapPath(s.Root, c.SrcX, c.SrcY)
		dstT := sromap.LightmapPath(s.Root, c.DstX, c.DstY)
		if data, err := os.ReadFile(srcT); err == nil {
			if req.Overwrite {
				_ = backupOnce(dstT)
			}
			if err := os.WriteFile(dstT, data, 0644); err != nil {
				writeError(w, http.StatusInternalServerError, "write dst lightmap: "+err.Error())
				return
			}
			_ = s.mirrorToExport(dstT)
		} else if errors.Is(err, os.ErrNotExist) {
			if _, statErr := os.Stat(dstT); statErr == nil {
				_ = os.Remove(dstT)
				_ = s.mirrorDeleteFromExport(dstT)
			}
		}

		if s.mapInfo.SetRegion(c.DstX, c.DstY, true) {
			mapInfoChanged = true
		}
		s.textures.invalidateRegion(c.DstX, c.DstY)
		s.textures.invalidateLightmap(c.DstX, c.DstY)
		done = append(done, map[string]any{
			"srcX": c.SrcX, "srcY": c.SrcY,
			"dstX": c.DstX, "dstY": c.DstY,
		})
	}
	if mapInfoChanged {
		if err := s.mapInfo.Save(sromap.MapInfoPath(s.Root)); err != nil {
			writeError(w, http.StatusInternalServerError, "save mapinfo: "+err.Error())
			return
		}
		_ = s.mirrorToExport(sromap.MapInfoPath(s.Root))
	}
	resp := map[string]any{
		"duplicated":   done,
		"skipped":      skipped,
		"mapInfoSaved": mapInfoChanged,
		"heightDelta":  heightDelta,
	}
	if req.TargetFloorY != nil && !math.IsInf(float64(sourceFloorY), 1) {
		resp["sourceFloorY"] = sourceFloorY
	}
	writeJSON(w, resp)
}

// findTemplateMesh returns the first .m file we can load — used as a
// structural template for newly-created regions.
func (s *Server) findTemplateMesh() (*sromap.Mesh, string, error) {
	for _, r := range s.mapInfo.ActiveRegions() {
		if r.IsDungeon {
			continue
		}
		p := sromap.MeshPath(s.Root, r.X, r.Y)
		if m, err := sromap.LoadMesh(p); err == nil {
			return m, p, nil
		}
	}
	return nil, "", fmt.Errorf("no template mesh found in %s", s.Root)
}

func (s *Server) handleObjectInfoList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	type entry struct {
		ID    uint32 `json:"id"`
		Path  string `json:"path"`
		IsCPD bool   `json:"isCpd,omitempty"`
	}
	entries := make([]entry, 0, len(s.objects))
	for _, o := range s.objects {
		entries = append(entries, entry{ID: o.ID, Path: o.Path, IsCPD: o.IsCPD})
	}
	w.Header().Set("Cache-Control", "private, max-age=300")
	writeJSON(w, map[string]any{"count": len(entries), "entries": entries})
}

func (s *Server) handleSaveRegionObjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<21)
	defer r.Body.Close()
	var req struct {
		X       int                 `json:"x"`
		Y       int                 `json:"y"`
		Edits   []sromap.ObjectEdit `json:"edits"`
		Deletes []sromap.ObjectKey  `json:"deletes"`
		Adds    []sromap.ObjectAdd  `json:"adds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode: "+err.Error())
		return
	}
	if err := validateRegion(req.X, req.Y); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Edits) == 0 && len(req.Deletes) == 0 && len(req.Adds) == 0 {
		writeError(w, http.StatusBadRequest, "no changes")
		return
	}
	path := sromap.O2Path(s.Root, req.X, req.Y)
	o2, err := sromap.LoadO2(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, ".o2 not found")
		} else {
			writeError(w, http.StatusInternalServerError, "load .o2: "+err.Error())
		}
		return
	}
	if err := backupOnce(path); err != nil {
		writeError(w, http.StatusInternalServerError, "backup: "+err.Error())
		return
	}
	res := o2.ApplyChanges(req.Edits, req.Deletes, req.Adds)
	visibilityKeys := make(map[uint64]bool)
	for _, edit := range req.Edits {
		visibilityKeys[placementKey(edit.RegionID, edit.UID, edit.ObjID)] = true
	}
	for _, added := range res.Added {
		visibilityKeys[placementKey(added.RegionID, added.UID, added.ObjID)] = true
	}
	visibilityRepaired := s.normalizeChangedO2Visibility(o2, req.X, req.Y, visibilityKeys)
	if err := o2.Save(path); err != nil {
		writeError(w, http.StatusInternalServerError, "save: "+err.Error())
		return
	}
	_ = s.mirrorToExport(path)
	affectedObjIDs := make(map[uint32]bool)
	for _, edit := range req.Edits {
		affectedObjIDs[edit.ObjID] = true
	}
	for _, del := range req.Deletes {
		affectedObjIDs[del.ObjID] = true
	}
	for _, add := range req.Adds {
		affectedObjIDs[add.ObjID] = true
	}
	for objID := range affectedObjIDs {
		s.mirrorMapPlacementFiles(objID, nil)
	}
	writeJSON(w, map[string]any{
		"path":               path,
		"updated":            res.Updated,
		"deleted":            res.Deleted,
		"added":              res.Added,
		"visibilityRepaired": visibilityRepaired,
	})
}

func (s *Server) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	rel := r.URL.Query().Get("path")
	if rel == "" {
		writeError(w, http.StatusBadRequest, "missing path")
		return
	}
	disk := s.objCache.safeAssetPath(rel)
	if disk == "" {
		writeError(w, http.StatusNotFound, "asset not found")
		return
	}
	data, ok := s.objCache.texturePNG(disk)
	if !ok {
		writeError(w, http.StatusInternalServerError, "decode failed")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write(data)
}

func (s *Server) uniqueObjectPlacements(entries []sromap.ObjectEntry) []objectPlacement {
	placements := make([]objectPlacement, 0, len(entries))
	byKey := make(map[string]int, len(entries))
	for _, entry := range entries {
		key := fmt.Sprintf("%d_%d_%d", entry.RegionID, entry.UID, entry.ObjID)
		index, ok := byKey[key]
		if !ok {
			regionX := int(entry.RegionID & 0xff)
			regionY := int((entry.RegionID >> 8) & 0xff)
			placement := objectPlacement{
				ObjID: entry.ObjID, UID: entry.UID, RegionID: entry.RegionID,
				RegionX: regionX, RegionY: regionY,
				X: entry.X, Y: entry.Y, Z: entry.Z, Yaw: entry.Yaw,
				Static: entry.Static, Short0: entry.Short0, Big: entry.Big, Struct: entry.Struct,
				HostBlocks: []string{}, LODGroups: []int{},
			}
			if info, ok := s.objects[entry.ObjID]; ok {
				placement.ObjectPath = info.Path
				placement.IsCPD = info.IsCPD
			}
			placements = append(placements, placement)
			index = len(placements) - 1
			byKey[key] = index
		}
		block := fmt.Sprintf("%d,%d", entry.XBlock, entry.ZBlock)
		if !containsString(placements[index].HostBlocks, block) {
			placements[index].HostBlocks = append(placements[index].HostBlocks, block)
		}
		if !containsInt(placements[index].LODGroups, entry.LODGroup) {
			placements[index].LODGroups = append(placements[index].LODGroups, entry.LODGroup)
		}
	}
	return placements
}

func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func containsInt(values []int, value int) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func (s *Server) handleSaveRegion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<22)
	defer r.Body.Close()
	var req saveRegionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode request: "+err.Error())
		return
	}
	if err := validateRegion(req.X, req.Y); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Heights) != sromap.MeshGridSize*sromap.MeshGridSize {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("height map must contain %d values", sromap.MeshGridSize*sromap.MeshGridSize))
		return
	}

	meshPath := sromap.MeshPath(s.Root, req.X, req.Y)
	mesh, err := sromap.LoadMesh(meshPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := backupOnce(meshPath); err != nil {
		writeError(w, http.StatusInternalServerError, "backup mesh: "+err.Error())
		return
	}
	if err := mesh.SetUniqueHeightMap(req.Heights); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.TextureIDs) == sromap.MeshGridSize*sromap.MeshGridSize {
		if err := mesh.SetUniqueTextureIDs(req.TextureIDs); err != nil {
			writeError(w, http.StatusBadRequest, "set tiles: "+err.Error())
			return
		}
		s.textures.invalidateRegion(req.X, req.Y)
	}
	if err := mesh.Save(meshPath); err != nil {
		writeError(w, http.StatusInternalServerError, "write mesh: "+err.Error())
		return
	}
	_ = s.mirrorToExport(meshPath)

	out := saveRegionResponse{
		X:          req.X,
		Y:          req.Y,
		MeshPath:   meshPath,
		BackupNote: ".bak files are created once beside edited source files",
	}
	if req.SyncNVM {
		nvmPaths := sromap.ExistingNVMPaths(s.Root, req.X, req.Y)
		var walkable [sromap.NVMTotalTiles]bool
		var haveWalkable bool
		if len(nvmPaths) > 0 {
			placements := s.loadNavRebuildPlacements(req.X, req.Y)
			walkable = s.computeWalkability(mesh, placements, float32(DefaultNVMSlopeThreshold))
			haveWalkable = true
		}
		for _, path := range nvmPaths {
			nvm, err := sromap.LoadNVM(path)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "load NVM: "+err.Error())
				return
			}
			if err := backupOnce(path); err != nil {
				writeError(w, http.StatusInternalServerError, "backup NVM: "+err.Error())
				return
			}
			if err := nvm.SetHeightMap(req.Heights); err != nil {
				writeError(w, http.StatusInternalServerError, "set NVM heights: "+err.Error())
				return
			}
			if haveWalkable {
				sromap.ApplyNVMTileFlags(nvm, walkable)
			}
			if err := nvm.Save(path); err != nil {
				writeError(w, http.StatusInternalServerError, "write NVM: "+err.Error())
				return
			}
			_ = s.mirrorToExport(path)
			out.NVMPaths = append(out.NVMPaths, path)
			out.SavedNVMs++
		}
	}
	writeJSON(w, out)
}

func parseRegionQuery(r *http.Request) (int, int, error) {
	q := r.URL.Query()
	x, err := strconv.Atoi(q.Get("x"))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid x")
	}
	y, err := strconv.Atoi(q.Get("y"))
	if err != nil {
		return 0, 0, fmt.Errorf("invalid y")
	}
	return x, y, validateRegion(x, y)
}

func validateRegion(x, y int) error {
	if x < 0 || x > 255 || y < 0 || y > 127 {
		return fmt.Errorf("region out of range: %d,%d", x, y)
	}
	return nil
}

func backupOnce(path string) error {
	backup := path + ".bak"
	if _, err := os.Stat(backup); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": strings.TrimSpace(msg)})
}
