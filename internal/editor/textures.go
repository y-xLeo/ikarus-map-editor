package editor

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"strings"
	"sync"

	"sromapedit/internal/sromap"
)

const (
	compositeTileSize = 16
	compositeTiles    = 96
	compositeSize     = compositeTiles * compositeTileSize
	textureRepeat     = 8
)

type textureCache struct {
	root  string
	dir   string
	table map[uint32]sromap.Tile2DEntry
	dirIx map[string]string

	mu    sync.RWMutex
	ddj   map[uint32]*sromap.DDJImage
	tiles map[uint32][]byte
	bad   map[uint32]bool

	regionMu       sync.Mutex
	regions        map[string]*regionTexture
	regionVersions map[string]int

	lightmapMu       sync.Mutex
	lightmaps        map[string]*lightmapEntry
	lightmapVersions map[string]int

	worldMapMu sync.Mutex
	worldMaps  map[int]*regionTexture
}

type regionTexture struct {
	png    []byte
	width  int
	height int
}

type lightmapEntry struct {
	png []byte
	bad bool
}

func newTextureCache(root string, table map[uint32]sromap.Tile2DEntry) *textureCache {
	dir := sromap.Tile2DDir(root)
	index := buildTile2DIndex(dir)
	return &textureCache{
		root:             root,
		dir:              dir,
		table:            table,
		dirIx:            index,
		ddj:              make(map[uint32]*sromap.DDJImage),
		tiles:            make(map[uint32][]byte),
		bad:              make(map[uint32]bool),
		regions:          make(map[string]*regionTexture),
		regionVersions:   make(map[string]int),
		lightmaps:        make(map[string]*lightmapEntry),
		lightmapVersions: make(map[string]int),
		worldMaps:        make(map[int]*regionTexture),
	}
}

func buildTile2DIndex(dir string) map[string]string {
	index := make(map[string]string)
	entries, err := filepath.Glob(filepath.Join(dir, "*.*"))
	if err != nil {
		return index
	}
	for _, p := range entries {
		base := filepath.Base(p)
		index[strings.ToLower(base)] = p
	}
	return index
}

func (c *textureCache) decoded(id uint32) *sromap.DDJImage {
	c.mu.RLock()
	if img, ok := c.ddj[id]; ok {
		c.mu.RUnlock()
		return img
	}
	if c.bad[id] {
		c.mu.RUnlock()
		return nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	if img, ok := c.ddj[id]; ok {
		return img
	}
	if c.bad[id] {
		return nil
	}

	entry, ok := c.table[id]
	if !ok {
		c.bad[id] = true
		return nil
	}
	path := c.resolvePath(entry.Filename)
	if path == "" {
		c.bad[id] = true
		return nil
	}
	img, err := sromap.LoadDDJ(path)
	if err != nil || img == nil {
		c.bad[id] = true
		return nil
	}
	c.ddj[id] = img
	return img
}

func (c *textureCache) resolvePath(filename string) string {
	direct := filepath.Join(c.dir, filename)
	if p, ok := c.dirIx[strings.ToLower(filename)]; ok {
		return p
	}
	if _, err := filepath.Abs(direct); err == nil {
		if p, ok := c.dirIx[strings.ToLower(filepath.Base(filename))]; ok {
			return p
		}
	}
	return ""
}

func (c *textureCache) tilePNG(id uint32) ([]byte, bool) {
	c.mu.RLock()
	if data, ok := c.tiles[id]; ok {
		c.mu.RUnlock()
		return data, true
	}
	c.mu.RUnlock()

	img := c.decoded(id)
	if img == nil {
		return nil, false
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img.RGBA); err != nil {
		return nil, false
	}
	data := buf.Bytes()
	c.mu.Lock()
	c.tiles[id] = data
	c.mu.Unlock()
	return data, true
}

func (c *textureCache) regionComposite(x, y int, mesh *sromap.Mesh) (*regionTexture, error) {
	key := fmt.Sprintf("%d,%d", x, y)
	c.regionMu.Lock()
	if existing, ok := c.regions[key]; ok {
		c.regionMu.Unlock()
		return existing, nil
	}
	c.regionMu.Unlock()

	composite, err := c.buildComposite(mesh)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, composite); err != nil {
		return nil, err
	}
	rt := &regionTexture{png: buf.Bytes(), width: composite.Bounds().Dx(), height: composite.Bounds().Dy()}
	c.regionMu.Lock()
	c.regions[key] = rt
	c.regionMu.Unlock()
	return rt, nil
}

func (c *textureCache) regionCompositeFromIDs(ids []uint16) (*regionTexture, error) {
	composite, err := c.buildCompositeIDs(ids)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, composite); err != nil {
		return nil, err
	}
	return &regionTexture{png: buf.Bytes(), width: composite.Bounds().Dx(), height: composite.Bounds().Dy()}, nil
}

func (c *textureCache) regionCompositePatchFromIDs(ids []uint16, minTileX, minTileZ, maxTileX, maxTileZ int) (*regionTexture, error) {
	composite, err := c.buildCompositeIDRange(ids, minTileX, minTileZ, maxTileX, maxTileZ)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, composite); err != nil {
		return nil, err
	}
	return &regionTexture{png: buf.Bytes(), width: composite.Bounds().Dx(), height: composite.Bounds().Dy()}, nil
}

func (c *textureCache) regionCompositePatchGridFromIDs(ids []uint16, minTileX, minTileZ, maxTileX, maxTileZ int) (*regionTexture, error) {
	composite, err := c.buildCompositePatchGrid(ids, minTileX, minTileZ, maxTileX, maxTileZ)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, composite); err != nil {
		return nil, err
	}
	return &regionTexture{png: buf.Bytes(), width: composite.Bounds().Dx(), height: composite.Bounds().Dy()}, nil
}

func (c *textureCache) worldMapTexture(root string, mapInfo *sromap.MapInfo, scale int) (*regionTexture, error) {
	c.worldMapMu.Lock()
	if existing, ok := c.worldMaps[scale]; ok {
		c.worldMapMu.Unlock()
		return existing, nil
	}
	c.worldMapMu.Unlock()

	img := image.NewRGBA(image.Rect(0, 0, 256*scale, 128*scale))
	fillRGBA(img, color.RGBA{8, 11, 9, 255})

	for _, region := range mapInfo.ActiveRegions() {
		if region.IsDungeon {
			continue
		}
		mesh, err := sromap.LoadMesh(sromap.MeshPath(root, region.X, region.Y))
		if err != nil {
			fillWorldMapRegion(img, region.X, region.Y, scale, color.RGBA{45, 68, 47, 255})
			continue
		}
		ids, _, _ := mesh.UniqueTextureMap()
		c.drawWorldMapRegion(img, ids[:], region.X, region.Y, scale)
	}

	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	rt := &regionTexture{png: buf.Bytes(), width: img.Bounds().Dx(), height: img.Bounds().Dy()}
	c.worldMapMu.Lock()
	c.worldMaps[scale] = rt
	c.worldMapMu.Unlock()
	return rt, nil
}

func (c *textureCache) drawWorldMapRegion(out *image.RGBA, ids []uint16, rx, ry, scale int) {
	if len(ids) != sromap.MeshGridSize*sromap.MeshGridSize {
		fillWorldMapRegion(out, rx, ry, scale, color.RGBA{96, 84, 64, 255})
		return
	}
	used := make(map[uint16]*sromap.DDJImage)
	fallback := color.RGBA{96, 84, 64, 255}
	baseX := rx * scale
	baseY := (127 - ry) * scale
	for py := 0; py < scale; py++ {
		mapV := 1 - (float32(py)+0.5)/float32(scale)
		tileZf := mapV * float32(compositeTiles)
		tileZ := int(tileZf)
		if tileZ < 0 {
			tileZ = 0
		}
		if tileZ >= compositeTiles {
			tileZ = compositeTiles - 1
		}
		blendV := tileZf - float32(tileZ)
		if blendV < 0 {
			blendV = 0
		}
		if blendV > 1 {
			blendV = 1
		}
		for px := 0; px < scale; px++ {
			mapU := (float32(px) + 0.5) / float32(scale)
			tileXf := mapU * float32(compositeTiles)
			tileX := int(tileXf)
			if tileX < 0 {
				tileX = 0
			}
			if tileX >= compositeTiles {
				tileX = compositeTiles - 1
			}
			blendU := tileXf - float32(tileX)
			if blendU < 0 {
				blendU = 0
			}
			if blendU > 1 {
				blendU = 1
			}
			r, g, b := c.worldMapSample(ids, used, tileX, tileZ, blendU, blendV, mapU, 1-mapV, fallback)
			off := out.PixOffset(baseX+px, baseY+py)
			out.Pix[off] = r
			out.Pix[off+1] = g
			out.Pix[off+2] = b
			out.Pix[off+3] = 255
		}
	}
}

func (c *textureCache) worldMapSample(ids []uint16, used map[uint16]*sromap.DDJImage, tileX, tileZ int, blendU, blendV, texU, texV float32, fallback color.RGBA) (byte, byte, byte) {
	i00 := tileZ*sromap.MeshGridSize + tileX
	i10 := i00 + 1
	i01 := i00 + sromap.MeshGridSize
	i11 := i01 + 1
	tex00 := c.cachedWorldMapTile(used, ids[i00])
	tex10 := c.cachedWorldMapTile(used, ids[i10])
	tex01 := c.cachedWorldMapTile(used, ids[i01])
	tex11 := c.cachedWorldMapTile(used, ids[i11])
	oneU := 1 - blendU
	oneV := 1 - blendV
	w00 := oneU * oneV
	w10 := blendU * oneV
	w01 := oneU * blendV
	w11 := blendU * blendV
	r00, g00, b00 := sampleNearest(tex00, texU, texV, fallback)
	r10, g10, b10 := sampleNearest(tex10, texU, texV, fallback)
	r01, g01, b01 := sampleNearest(tex01, texU, texV, fallback)
	r11, g11, b11 := sampleNearest(tex11, texU, texV, fallback)
	r := float32(r00)*w00 + float32(r10)*w10 + float32(r01)*w01 + float32(r11)*w11
	g := float32(g00)*w00 + float32(g10)*w10 + float32(g01)*w01 + float32(g11)*w11
	b := float32(b00)*w00 + float32(b10)*w10 + float32(b01)*w01 + float32(b11)*w11
	return clampByte(r), clampByte(g), clampByte(b)
}

func (c *textureCache) cachedWorldMapTile(used map[uint16]*sromap.DDJImage, id uint16) *sromap.DDJImage {
	key := id & 0x7FF
	if img, ok := used[key]; ok {
		return img
	}
	img := c.decoded(uint32(key))
	used[key] = img
	return img
}

func fillRGBA(img *image.RGBA, c color.RGBA) {
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			off := img.PixOffset(x, y)
			img.Pix[off] = c.R
			img.Pix[off+1] = c.G
			img.Pix[off+2] = c.B
			img.Pix[off+3] = c.A
		}
	}
}

func fillWorldMapRegion(img *image.RGBA, rx, ry, scale int, c color.RGBA) {
	baseX := rx * scale
	baseY := (127 - ry) * scale
	for py := 0; py < scale; py++ {
		for px := 0; px < scale; px++ {
			off := img.PixOffset(baseX+px, baseY+py)
			img.Pix[off] = c.R
			img.Pix[off+1] = c.G
			img.Pix[off+2] = c.B
			img.Pix[off+3] = c.A
		}
	}
}

func (c *textureCache) invalidateRegion(x, y int) {
	key := fmt.Sprintf("%d,%d", x, y)
	c.regionMu.Lock()
	delete(c.regions, key)
	c.regionVersions[key]++
	c.regionMu.Unlock()
	c.invalidateWorldMap()
}

func (c *textureCache) invalidateWorldMap() {
	c.worldMapMu.Lock()
	c.worldMaps = make(map[int]*regionTexture)
	c.worldMapMu.Unlock()
}

func (c *textureCache) regionVersion(x, y int) int {
	key := fmt.Sprintf("%d,%d", x, y)
	c.regionMu.Lock()
	v := c.regionVersions[key]
	c.regionMu.Unlock()
	return v
}

func (c *textureCache) invalidateLightmap(x, y int) {
	key := fmt.Sprintf("%d,%d", x, y)
	c.lightmapMu.Lock()
	delete(c.lightmaps, key)
	c.lightmapVersions[key]++
	c.lightmapMu.Unlock()
}

func (c *textureCache) lightmapVersion(x, y int) int {
	key := fmt.Sprintf("%d,%d", x, y)
	c.lightmapMu.Lock()
	v := c.lightmapVersions[key]
	c.lightmapMu.Unlock()
	return v
}

func (c *textureCache) lightmapPNG(root string, x, y int) ([]byte, bool) {
	key := fmt.Sprintf("%d,%d", x, y)
	c.lightmapMu.Lock()
	if entry, ok := c.lightmaps[key]; ok {
		c.lightmapMu.Unlock()
		if entry.bad {
			return nil, false
		}
		return entry.png, true
	}
	c.lightmapMu.Unlock()

	path := sromap.LightmapPath(root, x, y)
	img, err := sromap.LoadLightmap(path)
	if err != nil || img == nil {
		c.lightmapMu.Lock()
		c.lightmaps[key] = &lightmapEntry{bad: true}
		c.lightmapMu.Unlock()
		return nil, false
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img.RGBA); err != nil {
		c.lightmapMu.Lock()
		c.lightmaps[key] = &lightmapEntry{bad: true}
		c.lightmapMu.Unlock()
		return nil, false
	}
	data := buf.Bytes()
	c.lightmapMu.Lock()
	c.lightmaps[key] = &lightmapEntry{png: data}
	c.lightmapMu.Unlock()
	return data, true
}

func (c *textureCache) buildComposite(mesh *sromap.Mesh) (*image.RGBA, error) {
	ids, _, _ := mesh.UniqueTextureMap()
	return c.buildCompositeIDs(ids[:])
}

func (c *textureCache) buildCompositeIDs(ids []uint16) (*image.RGBA, error) {
	return c.buildCompositeIDRange(ids, 0, 0, compositeTiles-1, compositeTiles-1)
}

func (c *textureCache) buildCompositeIDRange(ids []uint16, minTileX, minTileZ, maxTileX, maxTileZ int) (*image.RGBA, error) {
	if len(ids) != sromap.MeshGridSize*sromap.MeshGridSize {
		return nil, fmt.Errorf("texture map must contain %d values, got %d", sromap.MeshGridSize*sromap.MeshGridSize, len(ids))
	}
	if minTileX < 0 || minTileZ < 0 || maxTileX >= compositeTiles || maxTileZ >= compositeTiles ||
		minTileX > maxTileX || minTileZ > maxTileZ {
		return nil, fmt.Errorf("invalid texture range %d,%d..%d,%d", minTileX, minTileZ, maxTileX, maxTileZ)
	}

	used := make(map[uint16]*sromap.DDJImage)
	addID := func(id uint16) {
		key := id & 0x7FF
		if _, ok := used[key]; ok {
			return
		}
		used[key] = c.decoded(uint32(key))
	}
	for tz := minTileZ; tz <= maxTileZ; tz++ {
		for tx := minTileX; tx <= maxTileX; tx++ {
			i00 := tz*sromap.MeshGridSize + tx
			addID(ids[i00])
			addID(ids[i00+1])
			addID(ids[i00+sromap.MeshGridSize])
			addID(ids[i00+sromap.MeshGridSize+1])
		}
	}

	width := (maxTileX - minTileX + 1) * compositeTileSize
	height := (maxTileZ - minTileZ + 1) * compositeTileSize
	out := image.NewRGBA(image.Rect(0, 0, width, height))
	stride := out.Stride
	pix := out.Pix

	fallback := color.RGBA{96, 84, 64, 255}

	tileSize := compositeTileSize
	tileSizeF := float32(tileSize)
	tileSpan := tileSize * textureRepeat
	tileSpanF := float32(tileSpan)
	for tz := minTileZ; tz <= maxTileZ; tz++ {
		for tx := minTileX; tx <= maxTileX; tx++ {
			i00 := tz*sromap.MeshGridSize + tx
			i10 := i00 + 1
			i01 := i00 + sromap.MeshGridSize
			i11 := i01 + 1

			tex00 := used[ids[i00]&0x7FF]
			tex10 := used[ids[i10]&0x7FF]
			tex01 := used[ids[i01]&0x7FF]
			tex11 := used[ids[i11]&0x7FF]

			for py := 0; py < tileSize; py++ {
				blendV := (float32(py) + 0.5) / tileSizeF
				oneBlendV := 1 - blendV
				outY := (tz-minTileZ)*tileSize + py
				texV := (float32((tz*tileSize+py)%tileSpan) + 0.5) / tileSpanF
				for px := 0; px < tileSize; px++ {
					blendU := (float32(px) + 0.5) / tileSizeF
					oneBlendU := 1 - blendU
					w00 := oneBlendU * oneBlendV
					w10 := blendU * oneBlendV
					w01 := oneBlendU * blendV
					w11 := blendU * blendV

					texU := (float32((tx*tileSize+px)%tileSpan) + 0.5) / tileSpanF

					r00, g00, bl00 := sampleNearest(tex00, texU, texV, fallback)
					r10, g10, bl10 := sampleNearest(tex10, texU, texV, fallback)
					r01, g01, bl01 := sampleNearest(tex01, texU, texV, fallback)
					r11, g11, bl11 := sampleNearest(tex11, texU, texV, fallback)

					r := float32(r00)*w00 + float32(r10)*w10 + float32(r01)*w01 + float32(r11)*w11
					g := float32(g00)*w00 + float32(g10)*w10 + float32(g01)*w01 + float32(g11)*w11
					b := float32(bl00)*w00 + float32(bl10)*w10 + float32(bl01)*w01 + float32(bl11)*w11

					outX := (tx-minTileX)*tileSize + px
					off := outY*stride + outX*4
					pix[off] = clampByte(r)
					pix[off+1] = clampByte(g)
					pix[off+2] = clampByte(b)
					pix[off+3] = 255
				}
			}
		}
	}
	return out, nil
}

func (c *textureCache) buildCompositePatchGrid(ids []uint16, minTileX, minTileZ, maxTileX, maxTileZ int) (*image.RGBA, error) {
	if minTileX < 0 || minTileZ < 0 || maxTileX >= compositeTiles || maxTileZ >= compositeTiles ||
		minTileX > maxTileX || minTileZ > maxTileZ {
		return nil, fmt.Errorf("invalid texture range %d,%d..%d,%d", minTileX, minTileZ, maxTileX, maxTileZ)
	}
	tileW := maxTileX - minTileX + 1
	tileH := maxTileZ - minTileZ + 1
	gridW := tileW + 1
	gridH := tileH + 1
	if len(ids) != gridW*gridH {
		return nil, fmt.Errorf("texture patch must contain %d values, got %d", gridW*gridH, len(ids))
	}

	used := make(map[uint16]*sromap.DDJImage)
	addID := func(id uint16) {
		key := id & 0x7FF
		if _, ok := used[key]; ok {
			return
		}
		used[key] = c.decoded(uint32(key))
	}
	for _, id := range ids {
		addID(id)
	}

	out := image.NewRGBA(image.Rect(0, 0, tileW*compositeTileSize, tileH*compositeTileSize))
	stride := out.Stride
	pix := out.Pix

	fallback := color.RGBA{96, 84, 64, 255}

	tileSize := compositeTileSize
	tileSizeF := float32(tileSize)
	tileSpan := tileSize * textureRepeat
	tileSpanF := float32(tileSpan)
	for tz := minTileZ; tz <= maxTileZ; tz++ {
		lz := tz - minTileZ
		for tx := minTileX; tx <= maxTileX; tx++ {
			lx := tx - minTileX
			i00 := lz*gridW + lx
			i10 := i00 + 1
			i01 := i00 + gridW
			i11 := i01 + 1

			tex00 := used[ids[i00]&0x7FF]
			tex10 := used[ids[i10]&0x7FF]
			tex01 := used[ids[i01]&0x7FF]
			tex11 := used[ids[i11]&0x7FF]

			for py := 0; py < tileSize; py++ {
				blendV := (float32(py) + 0.5) / tileSizeF
				oneBlendV := 1 - blendV
				outY := lz*tileSize + py
				texV := (float32((tz*tileSize+py)%tileSpan) + 0.5) / tileSpanF
				for px := 0; px < tileSize; px++ {
					blendU := (float32(px) + 0.5) / tileSizeF
					oneBlendU := 1 - blendU
					w00 := oneBlendU * oneBlendV
					w10 := blendU * oneBlendV
					w01 := oneBlendU * blendV
					w11 := blendU * blendV

					texU := (float32((tx*tileSize+px)%tileSpan) + 0.5) / tileSpanF

					r00, g00, bl00 := sampleNearest(tex00, texU, texV, fallback)
					r10, g10, bl10 := sampleNearest(tex10, texU, texV, fallback)
					r01, g01, bl01 := sampleNearest(tex01, texU, texV, fallback)
					r11, g11, bl11 := sampleNearest(tex11, texU, texV, fallback)

					r := float32(r00)*w00 + float32(r10)*w10 + float32(r01)*w01 + float32(r11)*w11
					g := float32(g00)*w00 + float32(g10)*w10 + float32(g01)*w01 + float32(g11)*w11
					b := float32(bl00)*w00 + float32(bl10)*w10 + float32(bl01)*w01 + float32(bl11)*w11

					outX := lx*tileSize + px
					off := outY*stride + outX*4
					pix[off] = clampByte(r)
					pix[off+1] = clampByte(g)
					pix[off+2] = clampByte(b)
					pix[off+3] = 255
				}
			}
		}
	}
	return out, nil
}

func sampleNearest(img *sromap.DDJImage, u, v float32, fallback color.RGBA) (r, g, b byte) {
	if img == nil {
		return fallback.R, fallback.G, fallback.B
	}
	w := img.Width
	h := img.Height
	x := int(u * float32(w))
	y := int(v * float32(h))
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= w {
		x = w - 1
	}
	if y >= h {
		y = h - 1
	}
	off := (y*w + x) * 4
	pix := img.RGBA.Pix
	return pix[off], pix[off+1], pix[off+2]
}

func clampByte(v float32) byte {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return byte(v)
}
