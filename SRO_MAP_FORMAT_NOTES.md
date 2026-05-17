# Silkroad Map File Format Notes

This is a working knowledge dump for the Silkroad map editor work in this
folder. It combines three sources:

- Confirmed behavior from the Go parser/editor in `tools/sromapedit`.
- Structure recovered from the bundled/minified source in
  `D:\ExportedPK2\srcofothermapeditor`.
- Empirical checks against the extracted files in `D:\ExportedPK2`.

When a section says "current editor", it describes what the Go tool already
implements. When it says "reference editor", it describes behavior seen in the
other map editor source and should be treated as strong guidance, but not yet a
fully tested Go implementation.

## Global Constants

The normal outdoor map is region based.

| Name | Value | Notes |
| --- | ---: | --- |
| Region size | 1920 | World units per region on X and Z |
| Cell size | 20 | Terrain vertex spacing |
| Unique terrain grid | 97 x 97 | 9409 unique height samples |
| Blocks per region | 6 x 6 | 36 terrain blocks |
| Tiles per block | 16 x 16 | 256 tiles |
| Stored vertices per block | 17 x 17 | 289 stored vertices |
| Block size in world units | 320 | 16 tiles * 20 units |
| Stored vertices per `.m` | 10404 | 36 * 17 * 17, with duplicate borders |
| Unique vertices per `.m` | 9409 | 97 * 97 |

Region IDs are normally encoded as:

```text
regionID = y << 8 | x
x = regionID & 0xff
y = (regionID >> 8) & 0xff
```

For active regions in `mapinfo.mfo`, the high bit can mark dungeon regions.
The Go parser exposes:

```text
isDungeon = ((id >> 15) & 1) != 0
fieldY = (id >> 8) & 0x7f
```

The current extracted map had these field bounds when inspected:

```text
active regions: 4180
field x range: 45..252
field y range: 57..127
field center: 148,92
```

## Important Paths

Paths are relative to the extracted PK2 root unless noted.

| File | Path pattern |
| --- | --- |
| Map info | `Map\mapinfo.mfo` |
| Terrain mesh | `Map\<y>\<x>.m` |
| Object placement | `Map\<y>\<x>.o2` |
| Terrain lightmap | `Map\<y>\<x>.t` |
| Object table | `Map\object.ifo` |
| Tile table | `Map\tile2d.ifo` |
| Tile textures | `Map\Tile2D\...` |
| RefRegion textdata | `Media\server_dep\silkroad\textdata\refregion.txt` |
| Region code textdata | `Media\server_dep\silkroad\textdata\regioncode.txt` |
| Environment sounds | `Media\server_dep\silkroad\textdata\effectenvsnd.txt` |
| Region info | `Data\regioninfo.txt` |
| NVM navmesh | usually `Data\navmesh\nv_%04x.nvm` |
| Server NVM mirror | often `SR_GameServer\Data\navmesh\nv_%04x.nvm` |

For NVM names, `%04x` is the hex form of `y << 8 | x`.
Example:

```text
region 182,74
regionID = 74 << 8 | 182 = 0x4ab6
NVM = nv_4ab6.nvm
```

The current Go tool searches these NVM candidates:

```text
SR_GameServer\Data\navmesh\nv_%04x.nvm
Data\Navmesh\nv_%04x.nvm
Data\navmesh\nv_%04x.nvm
```

## Coordinate System

There are two coordinate spaces to keep separate:

- Local region coordinates: `0..1920` in X and Z.
- Render/world coordinates: centered around a chosen map center or selected
  base region.

The reference editor computes region world offsets from the active map center:

```text
worldOffsetX = -(1920 * (regionX - centerX))
worldOffsetZ =  (1920 * (regionY - centerY))
```

The current WebGL editor uses the selected loaded region as the local base:

```text
regionOffsetX = -(regionX - baseX) * 1920
regionOffsetZ =  (regionY - baseY) * 1920
```

Terrain and object X coordinates are flipped for display. This matches the
reference editor, which used a plane scaled by `-1` on X.

Terrain vertex render transform:

```text
worldX = regionOffsetX + 960 - gridX * 20
worldY = height
worldZ = regionOffsetZ - 960 + gridZ * 20
```

Object render transform:

```text
worldX = regionOffsetX + 960 - objectLocalX
worldY = objectLocalY
worldZ = regionOffsetZ - 960 + objectLocalZ
rotationY = yaw
scaleX = -1
```

If an `.o2` entry belongs to a different `regionID` than the file being loaded,
the reference editor offsets the object by the difference between the file
region and entry region:

```text
entryRegionX = regionID & 0xff
entryRegionY = (regionID >> 8) & 0xff

entryWorldOffsetX = fileWorldOffsetX - (entryRegionX - fileRegionX) * 1920
entryWorldOffsetZ = fileWorldOffsetZ + (entryRegionY - fileRegionY) * 1920
```

## `mapinfo.mfo`

Signature:

```text
JMXVMFO 1000
```

Expected size:

```text
8216 bytes
```

Layout:

| Offset | Size | Type | Meaning |
| ---: | ---: | --- | --- |
| 0 | 12 | ASCII | Signature `JMXVMFO 1000` |
| 12 | 2 | uint16 LE | Map width |
| 14 | 2 | uint16 LE | Map height |
| 16 | 2 | int16 LE | Unknown 0 |
| 18 | 2 | int16 LE | Unknown 1 |
| 20 | 2 | int16 LE | Unknown 2 |
| 22 | 2 | int16 LE | Unknown 3 |
| 24 | 8192 | bitset | 65536 active-region bits |

Active region bit mapping:

```text
id = y << 8 | x
byteIndex = id / 8
bitMask = 1 << (7 - (id % 8))
active = (regionData[byteIndex] & bitMask) != 0
```

Current editor support:

- Reads signature, dimensions, unknowns, and active region bitset.
- Reports active field bounds while ignoring dungeon-marked regions.
- Uses this to decide whether a region is active.

Reference editor support:

- Can set region active/inactive.
- Mirrors `mapinfo.mfo` to `SR_GameServer\Data\navmesh\mapinfo.mfo` during save.
- Marks nearby NVMs dirty when region activation changes.

## `.m` Terrain Mesh

Signature:

```text
JMXVMAPM1000
```

Expected fixed size:

```text
92712 bytes
```

The Go parser accepts larger files and preserves trailing bytes. This is
intentional because some variants may append unknown data.

High-level layout:

| Offset | Size | Meaning |
| ---: | ---: | --- |
| 0 | 12 | Signature |
| 12 | 36 * 2575 | Terrain blocks |

Block order:

```text
for zBlock = 0..5
  for xBlock = 0..5
```

Block offset:

```text
blockOffset = 12 + (zBlock * 6 + xBlock) * 2575
```

Block layout:

| Relative offset | Size | Type | Meaning |
| ---: | ---: | --- | --- |
| 0 | 4 | uint32 LE | Block flag |
| 4 | 2 | uint16 LE | Environment/profile ID |
| 6 | 2023 | 289 vertices * 7 bytes |
| 2029 | 1 | int8 | Water type |
| 2030 | 1 | uint8 | Water wave type |
| 2031 | 4 | float32 LE | Water height |
| 2035 | 512 | 256 * uint16 LE | Tile flags |
| 2547 | 4 | float32 LE | Height max |
| 2551 | 4 | float32 LE | Height min |
| 2555 | 20 | bytes | Reserved/unknown |

Each block stores `17 * 17 = 289` vertices. Each vertex is 7 bytes:

| Relative offset | Size | Type | Meaning |
| ---: | ---: | --- | --- |
| 0 | 4 | float32 LE | Height |
| 4 | 2 | uint16 LE | Packed texture ID and scale |
| 6 | 1 | uint8 | Brightness |

Packed texture value:

```text
textureID = packed & 1023
textureScale = (packed >> 10) & 63
```

Vertex mapping to the unique 97 x 97 grid:

```text
gridX = xBlock * 16 + localVertexX
gridZ = zBlock * 16 + localVertexZ
heightIndex = gridZ * 97 + gridX
```

Because each block has 17 vertices over 16 tiles, vertices along block borders
are duplicated in adjacent blocks. Any editor that changes terrain heights must
write all stored duplicates for the same unique grid point.

Current editor support:

- Reads and writes `.m`.
- Exposes a 97 x 97 unique heightmap.
- Writes edits back to all duplicate stored vertices.
- Recomputes `heightMax` and `heightMin` for every block.
- Preserves unknown bytes and trailing bytes.
- Creates a `.bak` backup before overwriting the original file.

Current brush behavior:

```text
none:   weight = 1 inside radius
linear: weight = 1 - distance / radius
smooth: weight = 1 - t*t*(3 - 2*t), where t = distance / radius
```

The command-line brush uses local region coordinates `0..1920`. The web editor
sends the whole 97 x 97 height array to the backend for saving.

## `.nvm` Navmesh

Signature:

```text
JMXVNVM 1000
```

This is variable length. The current editor walks the file to find the
heightmap offset and can update that heightmap. It does not regenerate cells,
edges, or object navigation.

Top-level layout from the reference parser:

| Section | Layout |
| --- | --- |
| Header | 12-byte signature |
| Object count | int16 |
| NVM objects | variable |
| Cell count | int32 |
| Open cell count | int32 |
| Cells | variable |
| Global edges | variable |
| Internal edges | variable |
| Tile map | 96 * 96 entries, 8 bytes each |
| Height map | 97 * 97 float32 values |
| Plane type map | 36 bytes |
| Plane height map | 36 float32 values |

NVM object layout:

| Size | Type | Meaning |
| ---: | --- | --- |
| 4 | uint32 LE | Asset/object ID |
| 4 | float32 LE | X |
| 4 | float32 LE | Y |
| 4 | float32 LE | Z |
| 2 | int16 LE | Type |
| 4 | float32 LE | Yaw |
| 2 | int16 LE | UID |
| 2 | int16 LE | short0 |
| 1 | uint8 | isBig |
| 1 | uint8 | isStruct |
| 2 | uint16 LE | regionID |
| 2 | uint16 LE | link count |
| 6 * link count | int16 triples | linked object ID, linked edge ID, edge ID |

The current Go parser skips 30 bytes before `linkCount`; that corresponds to
the fields before the link count.

Cell layout:

| Size | Type | Meaning |
| ---: | --- | --- |
| 4 | float32 LE | minX |
| 4 | float32 LE | minZ |
| 4 | float32 LE | maxX |
| 4 | float32 LE | maxZ |
| 1 | uint8 | object index count |
| 2 * count | uint16 LE | object indices |

Global edge layout is 27 bytes:

| Size | Type | Meaning |
| ---: | --- | --- |
| 16 | 4 * float32 LE | minX, minZ, maxX, maxZ |
| 1 | uint8 | flag |
| 1 | uint8 | dir0 |
| 1 | uint8 | dir1 |
| 2 | int16 LE | cell0 |
| 2 | int16 LE | cell1 |
| 2 | int16 LE | region0 |
| 2 | int16 LE | region1 |

Internal edge layout is 23 bytes:

| Size | Type | Meaning |
| ---: | --- | --- |
| 16 | 4 * float32 LE | minX, minZ, maxX, maxZ |
| 1 | uint8 | flag |
| 1 | uint8 | dir0 |
| 1 | uint8 | dir1 |
| 2 | int16 LE | cell0 |
| 2 | int16 LE | cell1 |

Tile map entry layout:

| Size | Type | Meaning |
| ---: | --- | --- |
| 4 | int32 LE | cellID |
| 2 | uint16 LE | tile flag |
| 2 | uint16 LE | texture ID |

Walkability inferred by the reference editor:

```text
forcedBlocked = (tileFlags[tile] & 2) != 0
walkable = cellIDs[tile] < openCellCount
```

Current editor support:

- Finds the tile map, height map, plane type map, and plane height map offsets.
- Can apply the same height changes to the 97 x 97 NVM heightmap.
- Can set the NVM heightmap to match the `.m` heightmap.

Important limitation:

Updating only the NVM heightmap keeps terrain heights visually and partially
navmesh-height aligned, but it does not rebuild navmesh topology. Large terrain
shape changes, deleted regions, new walls, or object collision changes need a
real NVM regeneration step.

The reference editor used a server-side NVM regeneration session and expanded
the dirty set around changed regions, often by a 3-region ring.

## `.o2` Object Placement

Signature:

```text
JMXVMAPO1001
```

High-level layout:

```text
12-byte signature
for zBlock = 0..5
  for xBlock = 0..5
    for lodGroup = 0..3
      uint16 count
      count * 30-byte entries
```

Entry layout:

| Offset | Size | Type | Meaning |
| ---: | ---: | --- | --- |
| 0 | 4 | uint32 LE | objID |
| 4 | 4 | float32 LE | local X |
| 8 | 4 | float32 LE | local Y |
| 12 | 4 | float32 LE | local Z |
| 16 | 2 | int16 LE | isStatic |
| 18 | 4 | float32 LE | yaw |
| 22 | 2 | int16 LE | UID |
| 24 | 2 | int16 LE | short0 |
| 26 | 1 | uint8 | isBig |
| 27 | 1 | uint8 | isStruct |
| 28 | 2 | uint16 LE | regionID |

The file stores entries under spatial host block and LOD groups. The same
logical object can appear multiple times, especially for large objects or LOD
coverage. The reference editor deduplicates logical objects with:

```text
key = regionID + "_" + uid + "_" + objID
```

It also records:

```text
hostBlocks = unique "xBlock,zBlock" values
lodGroups = unique lod group values
```

Reference object insertion behavior:

```text
hostXBlock = clamp(floor(localX / 320), 0, 5)
hostZBlock = clamp(floor(localZ / 320), 0, 5)
default lodGroup = 2
isStatic = -1
short0 = 0
regionID = y << 8 | x
```

If `isBig` is true, the reference editor writes the object into the 3 x 3 block
neighborhood around its host block. Smaller objects use a radius-based coverage
calculation, but still end up duplicated into neighboring host blocks when
needed.

Current editor support:

- Parses `.o2`.
- Attaches `xBlock`, `zBlock`, and `lodGroup` to each parsed entry.
- Deduplicates objects for API/display.
- Shows objects as markers/proxies in WebGL.
- Looks up object paths from `object.ifo`.

Current editor does not yet serialize modified `.o2` files, place new objects,
or render full BSR/BMS object meshes.

## `object.ifo`

Path:

```text
Map\object.ifo
```

The file is text-like. The current parser sanitizes bytes above ASCII to `_`,
because the useful object table lines are ASCII enough for parsing.

Signature:

```text
JMXVOBJI...
```

The second non-empty line is the entry count. Entry lines match:

```text
<decimal id> <hex flags> "<resource path>"
```

Current parser regex:

```text
^(\d+)\s+(0x[0-9a-fA-F]+)\s+"(.+)"$
```

Current parser output:

| Field | Meaning |
| --- | --- |
| ID | decimal object ID |
| Flags | hex flags parsed as uint32 |
| Path | lower-case resource path, `\` normalized to `/` |
| IsCPD | true if path ends with `.cpd` |

Example from inspected data:

```text
res/nature/europe/east eurpoe/tree/euro_esteuro_tree07_l03.bsr
```

The reference editor uses this table as the starting point for loading CPD or
BSR object resources.

## BSR, BMS, BMT, DDJ, EFP, CPD Object Resources

Status: BSR + BMS + BMT + CPD parsers are now in `internal/sromap/` and
exposed via `/api/object?id=N`. Textures are served as PNG via
`/api/asset?path=...`. The WebGL editor renders all placed objects as real
triangle meshes (textured) instead of point markers.

### Axis convention (THE one that actually works)

After several rounds of fighting this, the correct interpretation is:

- **Read X, Y, Z straight** from the BMS vertex block: 1st float → X,
  2nd → Y, 3rd → Z. No swap and no negation.
- **Y is up, origin is at the base of the model.** vertex.y = 0 is the
  base; vertex.y = bbox.maxY is the top.
- **Texture V is flipped on read** (`V = 1 - v`) to convert the
  DirectX/Blender V-down convention to OpenGL's V-up.
- **Object placement**: world = `T(world) * R_y(yaw) * S(-1, 1, 1)`, with
  `world = (offX + 960 - obj.x, obj.y, offZ - 960 + obj.z)`. The
  `scale.x = -1` mirrors the mesh to match the terrain's X flip; the
  Y axis is preserved (no `m[5] = -1`).

**Pitfall avoided**: the Blender plugin (`JellyBMS.py`) names variables
`x, z, y` when reading the 3 floats and stores `[x, y, z]`. That naming is
*for Blender's Z-up convention* (it puts the 2nd file float into Blender's
Z = up slot). For WebGL/Y-up, this is identity — read straight, no swap.
Don't copy the Blender plugin's variable order verbatim.

### Browser-cache gotcha

When iterating on `/api/object`, set `Cache-Control: no-store` (not
`max-age=600` like we used initially). Otherwise the browser will keep
serving an old, possibly Y-flipped version of the mesh data and you'll
think you're testing the new code when you're not.

### Original general flow (unchanged)

General object loading flow:

```text
object.ifo objID -> resource path
if path ends with .cpd:
  parse CPD and resolve its collision/resource path
else:
  parse BSR
BSR -> mesh/material/resource references
BMS -> geometry
BMT -> material and texture data
DDJ -> texture pixels
EFP -> optional particle/effect attachments
```

Observed BMS concepts:

- Vertex positions.
- Normals.
- UVs.
- Indices.
- Vertex count.
- Face count.
- A navmesh/collision offset check.

Observed BMT concepts:

- Materials.
- Texture references.
- Texture animation values such as scroll U, scroll V, and speed scale.

Observed DDJ usage:

- Object material textures.
- Tile textures.
- Minimap textures.

Observed EFP usage:

- Particle effect files can reference textures and particle meshes.
- Full object rendering should include effect attachments for objects that use
  them, but that is a later feature.

Open work for full object rendering:

- Implement BSR parser.
- Implement BMS mesh parser in Go or port enough of the reference parser.
- Implement BMT material parser.
- Implement DDJ decode path for browser or server-side conversion.
- Resolve Data/ and Particles/ assets through a case-insensitive file index.
- Add mesh/material caching.
- Add collision/bounds support for brush/object interaction.

## `refregion.txt`

Path:

```text
Media\server_dep\silkroad\textdata\refregion.txt
```

Encoding:

- UTF-16 LE with BOM.
- UTF-16 BE with BOM.
- UTF-8 or ASCII-compatible text.

The reference editor also tries EUC-KR for some other textdata files.

Known columns:

| Column | Name |
| ---: | --- |
| 0 | wRegionID |
| 1 | X |
| 2 | Z |
| 3 | ContinentName |
| 4 | AreaName |
| 5 | IsBattleField |
| 6 | Climate |
| 7 | MaxCapacity |
| 8 | AssocObjID |
| 9 | AssocServer |
| 10 | AssocFile256 |
| 11 | LinkedRegion_1 |
| 12 | LinkedRegion_2 |
| 13 | LinkedRegion_3 |
| 14 | LinkedRegion_4 |
| 15 | LinkedRegion_5 |
| 16 | LinkedRegion_6 |
| 17 | LinkedRegion_7 |
| 18 | LinkedRegion_8 |
| 19 | LinkedRegion_9 |
| 20 | LinkedRegion_10 |

Current editor support:

- Loads entries by region ID.
- Parses the main metadata columns.
- Displays the entry for a loaded region through the API.
- Does not yet preserve/write linked-region columns.

Reference editor behavior:

- Creates default entries when needed.
- Removes entries when regions are deleted.
- Recomputes linked regions before serialization.
- Can generate SQL upserts for server tables.

Default entry from reference source:

```text
wRegionID = z << 8 | x
X = x
Z = z
ContinentName = TOREPLACE
AreaName = field
IsBattleField = 1
Climate = 1001
MaxCapacity = 200
AssocObjID = 0
AssocServer = 0
AssocFile256 = ""
LinkedRegion_1..10 = 0
```

Linked-region directions from the reference editor:

| Link | Delta X | Delta Z |
| ---: | ---: | ---: |
| 1 | 0 | 1 |
| 2 | 1 | 1 |
| 3 | 1 | 0 |
| 4 | 1 | -1 |
| 5 | 0 | -1 |
| 6 | -1 | -1 |
| 7 | -1 | 0 |
| 8 | -1 | 1 |
| 9 | 0 | 0 |
| 10 | 0 | 0 |

Links 1 through 8 are set to the neighbor region ID only if that neighbor
exists in the refregion entry map. Out-of-bounds or missing neighbors become 0.
Links 9 and 10 are always 0 in the reference serializer.

Reference SQL generator touched:

```text
[SRO_VT_SHARD].[dbo].[_RefRegion]
[SRO_VT_SHARD].[dbo].[_RefInstance_World_Region]
[SRO_VT_SHARD].[dbo].[_RefRegionBindAssocServer]
```

## `regioncode.txt`

Path:

```text
Media\server_dep\silkroad\textdata\regioncode.txt
```

Reference parser behavior:

- Skips empty lines and `//` comments.
- Parses each remaining line into:

```text
service
regionID
codeName
soundRef
```

The pasted/minified source was partly damaged around split logic, so this needs
verification against real source or real files before writing a Go serializer.

## `effectenvsnd.txt`

Path:

```text
Media\server_dep\silkroad\textdata\effectenvsnd.txt
```

Reference parser markers:

```text
<1> environment/profile name
"music file"
<2> period name
<3>"ambient wav" range
```

Parsed structure:

```text
profile:
  name
  music
  periods:
    name
    ambients:
      wav
      range
```

Reference serializer can append cloned/new profiles to the original raw file
when given a list of names to add.

## `regioninfo.txt`

Path:

```text
Data\regioninfo.txt
```

Reference parser behavior:

- Zone headers start with `#TOWN` or `#FIELD`.
- Header contains type, name, and optional folder.
- Region lines start with numbers.
- Region line layout:

```text
rx ry shape
```

For rectangular subareas:

```text
rx ry RECT x1 y1 x2 y2
```

Most whole-region entries use:

```text
rx ry ALL
```

Reference editor adds pasted/copied regions to matching zones.

## `tile2d.ifo`

Path:

```text
Map\tile2d.ifo
```

Signature prefix:

```text
JMXV2DTI
```

The Go editor now ships its own parser at `internal/sromap/tile2d.go`. It
accepts both `\r\n` and `\n` line endings, skips the count line, and exposes
the entries as a `map[uint32]Tile2DEntry` keyed by ID. Files are looked up
in `Map\Tile2D\<filename>` directly because the on-disk folder is flat — the
`folder` column in `tile2d.ifo` is a logical region label, not a subdirectory.

Reference parser behavior:

- First non-empty line starts with the signature.
- Second line is entry count.
- Entry line regex:

```text
^(\d+)\s+(0x[0-9a-fA-F]+)\s+"([^"]*)"\s+"([^"]*)"
```

Parsed fields:

```text
id
tileType
folder
filename
grass/raw trailing data
```

Known tile type labels from the reference source:

| Value | Label |
| ---: | --- |
| 0 | Dirt |
| 1 | Sand |
| 2 | Ashfield |
| 3 | Stone |
| 4 | Metal |
| 5 | Wood |
| 6 | Mud |
| 7 | Water |
| 8 | DeepWater |
| 9 | Snow |
| 10 | Grass |
| 11 | LongGrass |
| 12 | Forest |
| 13 | Cloud |

The reference terrain renderer extracts texture IDs from `.m` vertices, loads
DDJ tile textures from `Map\Tile2D`, and either:

- Builds a CPU composite terrain texture of 768 x 768 pixels.
- Builds GPU data textures: texture array, 97 x 97 index map, 97 x 97
  brightness map, and optional `.t` lightmap.

Current editor support:

- Parses `tile2d.ifo` and `Map/Tile2D/*.ddj` (DXT1/DXT3/DXT5) in pure Go.
- Builds a 1536x1536 per-region terrain composite. Each tile bilinearly blends
  its 4 corner textures sampled at a tiled UV that **repeats every 8 tiles**
  (matching the reference shader: `tiledUV = gridPos / 8.0`). Served from
  `/api/region/texture?x=&y=`.
- WebGL terrain shader samples that composite via per-vertex UV
  `(gx/96, gz/96)` and falls back to height-shaded vertex color until the
  PNG finishes loading.
- Terrain is rendered double-sided (`gl.CULL_FACE` is disabled for the
  terrain pass) because back-facing slopes were dropping out as black
  silhouettes.
- Per-vertex normals are computed from heightmap derivatives client-side and
  fed into a simple directional NdotL term for relief shading.
- Texture scale (the upper 6 bits of the packed `uint16`) is not yet applied.

### Pitfalls

- **Do NOT use the per-vertex brightness byte as a direct color multiplier.**
  In the real `.m` files a huge fraction of vertices have `brightness=0`
  (region 100,100 averaged ~70 with many zeros). Multiplying texture color
  by `brightness/128` produces large pure-black patches that look like
  see-through terrain. The reference engine ignores this byte for color
  shading and uses the `.t` lightmap (sun shadows) + an NdotL term instead.
- Texture sampling at per-tile UV (one source-texture cycle per tile) is too
  coarse — the source detail vanishes. Use the reference's tiled UV where
  the source texture cycles once every 8 tiles across the region.

## `.t` Terrain Lightmap

Signature:

```text
JMXVMAPT1001
```

Reference parser constants:

```text
96 x 96 tile lightmap = 9216 bytes
512 x 512 high-resolution lightmap
DXT1 payload = 131072 bytes
serialized .t size = 140436 bytes
```

Layout:

| Offset | Size | Meaning |
| ---: | ---: | --- |
| 0 | 12 | Signature `JMXVMAPT1001` |
| 12 | 9216 | 96 x 96 lightmap bytes |
| 9228 | 4 | uint32 LE, reference writes 131208 |
| 9232 | 4 | uint32 LE, reference writes 3 |
| 9236 | 4 | DDS magic `DDS ` |
| 9240 | 124 | DDS header |
| 9364 | 131072 | DXT1 texture data |

DDS fields written by the reference serializer:

| DDS-relative offset | Value |
| ---: | --- |
| 4 | 124 |
| 8 | 4103 |
| 12 | 512 height |
| 16 | 512 width |
| 76 | pixel format size 32 |
| 80 | pixel flags 4 |
| 84 | `DXT1` |
| 108 | caps 4096 |

Reference behavior:

- Decodes DXT1 to 512 x 512 RGBA for rendering.
- Can encode DXT1 and serialize `.t`.
- Can create a white default lightmap.
- Can bake a terrain/object shadow lightmap.

Current editor support:

- Parses `.t` in `internal/sromap/lightmap.go`. The 512x512 DXT1 high-res
  lightmap is extracted from offset `12 + 9216 + 8 = 9236` (signature +
  96x96 tile lightmap + DDS size/mip-count) and decoded via the existing
  DDJ pipeline (which handles a pure `DDS ` payload too).
- Served as PNG via `/api/region/lightmap?x=&y=` with a per-region cache.
- Terrain shader multiplies it as a baked-shadow factor with an adjustable
  strength slider (`Sun shadow` in the left panel).
- **UV gotcha**: image row 0 of the lightmap corresponds to `gridZ = 0`.
  In a terrain-mesh shader where `vUV.y = gz/96`, sample with
  `texture2D(uLightmap, vUV)` — no V flip. The reference editor flips V
  because their `vUV` convention is `(1 - gz/96)`; if you copy their
  shader verbatim you'll get a vertically-mirrored shadow map.
- Does not yet write `.t`. Editing/baking shadows is a separate effort.

## `environment.ifo`

Signature:

```text
JMXVENVI1003
```

Reference parser behavior:

- Reads binary little-endian values.
- Strings are stored as:

```text
uint32 byteLength
byteLength bytes of ASCII/text data
```

Top-level structure:

```text
signature
uint16 profileCount
string environmentSetName
profile[profileCount]
recursive environment tree
```

Profile fields observed:

```text
id
name
dayBGM
nightBGM
sunColor
skyTopColor
diffuseColor
objectAmbientColor
graph4
terrainAmbientColor
terrainShadowColor
fogNearPlane
fogFarPlane
fogColor
graph10
graph11
graph12
skyBottomColor
waterColor
graph15
```

Graph types:

```text
color graph entry = float32 r, float32 g, float32 b, float32 time
value graph entry = float32 value, float32 time
```

The reference editor samples color graphs by linear interpolation over time.

Current editor support:

- Does not yet parse or use `environment.ifo`.

## Minimap Textures

Reference editor path:

```text
Media\minimap\<x>x<y>.ddj
```

Reference behavior:

- Scans DDJ minimap tiles in the requested region range.
- Draws each 16 x 16 tile into a composite canvas.
- Inverts vertical placement by using:

```text
drawX = (rx - minX) * 16
drawY = (maxY - ry) * 16
```

Current editor support:

- Does not yet render minimap DDJ tiles.

## Current Go/Web Editor API

The `serve` command starts an HTTP server:

```powershell
.\sromapedit.exe serve -root D:\ExportedPK2 -addr 127.0.0.1:18080
```

Endpoints:

| Endpoint | Method | Purpose |
| --- | --- | --- |
| `/` | GET | WebGL editor UI |
| `/app.js` | GET | Embedded editor JavaScript |
| `/style.css` | GET | Embedded editor CSS |
| `/api/info` | GET | Root path, bounds, active count, refregion count |
| `/api/region?x=&y=` | GET | Region terrain, NVM, refregion, object marker data |
| `/api/region/save` | POST | Save a 97 x 97 heightmap to `.m`, optional NVM sync |

`/api/region` returns:

- Whether the region is active in `mapinfo.mfo`.
- Whether `.m` exists.
- Whether NVM exists.
- Terrain stats.
- 9409 height samples.
- RefRegion entry if present.
- Deduplicated object markers if `.o2` and `object.ifo` are available.
- Warnings for partial/missing files.

`/api/region/save` accepts:

```json
{
  "x": 100,
  "y": 100,
  "heights": [9409 numbers],
  "syncNvm": true
}
```

Save behavior:

- Loads existing `.m`.
- Writes the unique heightmap to all duplicate stored vertices.
- Recalculates block min/max heights.
- Creates `.bak` once if no backup exists.
- Writes `.m`.
- If `syncNvm` is true and an NVM exists, overwrites its 97 x 97 heightmap.

## Verified Current Behavior

These checks were run during development:

```text
go test ./...
node --check internal\editor\static\app.js
```

The Go tests passed, but Go printed a telemetry warning because it could not
write to `C:\Users\Mace\AppData\Roaming\go\telemetry\local\upload.token`.
That warning is unrelated to parser behavior.

API checks:

```text
/api/info returned activeCount=4180 and refRegionCount=9
/api/region?x=182&y=74 returned hasMesh=true, hasNVM=true, 9409 heights
/api/region?x=100&y=100 returned 61 deduplicated objects
```

## Known Gaps For A Full Map Editor

Terrain:

- Current terrain editing is height-only.
- Tile texture painting is not implemented.
- Water editing is not implemented.
- `.t` lightmap rendering/writing is not implemented.

Objects:

- `.o2` parsing works, but `.o2` writing/object placement does not.
- Full BSR/BMS/BMT/DDJ object rendering is not implemented.
- Collision mesh loading is not implemented.
- Object delete/move/rotate/save is not implemented.

Region metadata:

- `refregion.txt` parsing is partial.
- Linked regions are not yet preserved or regenerated in Go.
- Region creation/deletion does not yet update `mapinfo.mfo`, `refregion.txt`,
  `regioninfo.txt`, `regioncode.txt`, or SQL output.

Navigation:

- NVM heightmap sync works.
- Full NVM rebuild is not implemented.
- Cell/edge/object navigation updates require a real generator.

Rendering:

- Current WebGL is functional for flying around, terrain height editing, and
  object markers.
- It does not yet load real object meshes, minimap, DDJ terrain textures,
  lightmaps, particles, or environment profiles.

## Practical Implementation Order

For turning this into a full editor, the safest order is:

1. Keep `.m` read/write robust and covered by tests.
2. Add `tile2d.ifo` and DDJ decode so terrain looks like the real map.
3. Add `.t` lightmap read/render/write.
4. Add `.o2` serializer with round-trip tests before object placement.
5. Add object resource loading: `object.ifo` -> CPD/BSR -> BMS/BMT/DDJ.
6. Add object move/rotate/delete/place and verify `.o2` duplicate host blocks.
7. Add `mapinfo.mfo` region activation writes.
8. Add full `refregion.txt` parse/serialize with linked-region regeneration.
9. Add `regioninfo.txt`, `regioncode.txt`, and environment/sound metadata.
10. Integrate or build an NVM regeneration path instead of heightmap-only sync.

