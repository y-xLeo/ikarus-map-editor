# sromapedit — NVM Cell-Split Handoff (2026-05-16)

## What we're solving

Custom OBJ-imported objects (e.g. `test_new_obj` — Japanese house at asset ID **3308**) need working **server-side collision** in Silkroad Online. They render visually (because the .o2 placement is fine) but the player walks through them without an NVM fix.

A separate "rebuild" tool (output at `D:\ExportedPK2\rebuild\Data\navmesh\`) successfully produces working collision for asset 3308 — with one quirk: the collision box is **offset** from the visible house (cell-grid snapping; not fixable from nvm generation, present in rebuild's output too). Our job is to make **sromapedit** generate equivalent nvm output.

## Quick map of where the work happens

| File | Purpose |
| --- | --- |
| `internal/sromap/nvm.go` | NVM struct definitions (NVMCell, NVMObject, edges, tiles) |
| `internal/sromap/nvm_rebuild.go` | Partition algorithm + `ApplyNVMNavRebuild` (full-repartition path) |
| `internal/editor/nvm_rebuild.go` | Custom-asset pipeline: `addCustomNVMObjectsOnly`, `addCustomBboxObjectIndices`, `computeWalkability`, `pathLooksCustom` helper |
| `internal/editor/nvm_split.go` | **NEW** — `splitOversizedCellsForCustom` + `splitOpenCell` (the cell splitter) |
| `internal/editor/nvm_split_test.go` | `TestSplitCell6Baseline` — programmatic verification |
| `internal/editor/server.go:521-561` | The HTTP handler that calls the pipeline (default vs `?full=1`) |
| `cmd/diffnvm/main.go` | Cell-by-cell diff between two nvm files within a target bbox |
| `cmd/verifyfix/main.go` | Standalone simulation of bbox-spread with the min-overlap threshold |

## File-format crash course (just what matters for this work)

An nvm file describes one 1920×1920 world region (region ID `0x5C94` etc.):

- **NVMObject[]** — per-asset placement records. Fields we care about: `AssetID`, `X/Y/Z/Yaw`, `UID`, `RegionID`. The server uses these to load each asset's collision mesh from its BSR/BMS.
- **NVMCell[]** — axis-aligned rectangles. First `OpenCellCount` entries are OPEN (walkable); the rest are CLOSED (block movement). Each cell carries `ObjectIndices: []uint16` — indices into NVMObject[] for assets whose footprint sits inside this cell.
- **NVMInternalEdge[]** — walls/passages between two cells in the same region. `Cell0` and `Cell1` are cell indices; `Flag=4` for normal passage, `0x02` for impassable wall.
- **NVMGlobalEdge[]** — region boundary crossings. `Cell0` is the local cell, `Region1`/`Cell1` is in the neighbour region.
- **Tile[96×96]** — fine-grained 20×20 grid. `Tile.CellID` points into NVMCell[]. `Tile.Flag` bit 0 = "manually blocked".

**Server collision chain** (decoded from `RTNavMeshTerrain.cpp` strings in the patched `SR_GameServer.exe`):
1. Server reads `data\navmesh\nv_<region>.nvm` at region init.
2. For every NVMObject, it lazy-loads a per-asset `SNavMeshInst` from `CMemPool` (size 14098) using AssetID → object.ifo → BSR → BMS NavMesh section.
3. Cells with `ObjectIndices=[N]` are linked to NVMObject N's collision template.
4. **If asset N has no collision template** (e.g. custom AssetID with no `data_simple_mesh\` entry), `SNavMeshInst` returns null → server hits assertion `pNMI != NULL` at line 107 of RTNavMeshTerrain.cpp.

**Binary patch already applied** at file offset `0x7DAA44` in `SR_GameServer.exe` (Rigid Egypt 2017 build): `68 b8 b0 e3 00` → `e9 38 01 00 00` (jmp past the assertion). Backup at `SR_GameServer.exe.bak`. Patch tool: `cmd/patchexe/`. **Don't undo this — the server will crash without it.**

## Cell-grid algorithm (the heart of the work)

Discovered by diffing `D:\ExportedPK2\Data\Navmesh\nv_5c94.nvm` (broken) vs `D:\ExportedPK2\rebuild\Data\navmesh\nv_5c94.nvm` (working):

- **Base cell grid is 240×240 world units = 12×12 tiles** (cap enforced by `NVMMaxCellTile = 12` in `sromap/nvm_rebuild.go`).
- Open areas get one big 240×240 cell each; smaller cells appear around stock object footprints.
- For an asset at world (X, Z) with rotated bbox (xMin..xMax, zMin..zMax), the working pattern is:
  1. NVMObject record with `AssetID, X, Y, Z, Yaw, UID, RegionID` from `.o2`.
  2. Find every 240-grid cell whose AABB **meaningfully** intersects the bbox.
  3. Append the new NVMObject's index to each intersecting cell's `ObjectIndices`.
  4. **Do NOT touch `Tile.Flag`** (rebuild tool keeps it 0x00 — stamping Flag=1 broke slope walkability in earlier attempts).
- **"Meaningfully" = overlap ≥ 20 units (one tile) on BOTH axes.** Without this threshold, a cell that clips just a 10-unit Z sliver of the bbox gets the ObjectIndex and confuses the engine into treating the whole cell as the asset's footprint, breaking navigation when the player approaches.

## What the rebuild tool does that we have to match

Concrete comparison of baseline vs rebuild around asset 3308:

```
Baseline 5c94: 310 cells, 41 global edges, 673 internal edges, 10 NVMObjects
Rebuild  5c94: 352 cells, 47 global edges, 704 internal edges, 10 NVMObjects (+1 custom, dedupe -1 stock)
```

Asset 3308 rotated world bbox: **X=(1607..1897), Z=(1325..1630)** (~290×305 units).

| | BASELINE cells in bbox | REBUILD cells in bbox |
| --- | --- | --- |
| | cell 6:  (1440,1140)..(1920,1620) **480×480** ObjIdx=[9] | cell 171: (1440,1200)..(1680,1440) 240×240 ObjIdx=[9] (NW) |
| | cell 24: (1600,1620)..(1920,1760) 320×140 ObjIdx=[] | cell 172: (1680,1200)..(1920,1440) 240×240 ObjIdx=**[7, 9]** (NE) |
| | | cell 179: (1440,1440)..(1680,1680) 240×240 ObjIdx=[9] (SW) |
| | | cell 180: (1680,1440)..(1920,1680) 240×240 ObjIdx=[9] (SE) |

Key observation: the rebuild tool **splits the 480×480 cell into 4 separate 240×240 sub-cells** and re-assigns each NVMObject to whichever sub-cell its center falls into. Asset 971 (at (1703, 1239)) only ends up in the NE sub-cell — its true location. Our asset 3308 ends up in all 4 sub-cells via bbox spread.

Note the **NVMObject array is also reordered** in rebuild (asset 971 baseline[9] → rebuild[7], asset 3308 appended as rebuild[9], one stock 946 entry dedupe-d out). We have not replicated this reordering — it may or may not matter; the test object's collision-area cells were rewritten correctly regardless.

## Pipeline as it stands NOW (2026-05-16 19:29)

Default rebuild path in `internal/editor/server.go` (the one users actually hit — no `?full=1`):

```go
s.addCustomNVMObjectsOnly(nvm, placements, x, y)   // 1. Sync NVMObjects from .o2
s.splitOversizedCellsForCustom(nvm)                 // 2. Split oversized cells (NEW)
s.addCustomBboxObjectIndices(nvm)                   // 3. Bbox-spread ObjectIndices
```

### Step 1 — addCustomNVMObjectsOnly

- Indexes existing NVMObjects by `(UID, AssetID)`.
- For each `.o2` custom-asset placement: if found, **UPDATE** X/Y/Z/Yaw in place. If not, append.
- Stock NVMObjects are untouched.
- **Bug we fixed:** previously only appended, never updated, leaving the asset at the stale baked position even after the user moved it in the editor.

### Step 2 — splitOversizedCellsForCustom (the new code in `nvm_split.go`)

- Walks NVMObjects, computes rotated world bbox for each custom-asset one.
- For each OPEN cell larger than 240 on either axis that meaningfully overlaps any custom bbox:
  - Partitions into 12×12-tile sub-cells (parent's tile origin, row-major).
  - **First sub-cell takes the parent's array index ci**; remaining ones are inserted just before the closed-cell section. All references (`Tile.CellID`, `InternalEdge.Cell0/Cell1`, `GlobalEdge.Cell0/Cell1`) >= insertion point shift up by N.
  - Inherits `ObjectIndices` only on the sub-cell containing each NVMObject's center (point-in-AABB test).
  - Edge case handled: if an NVMObject's (X,Z) falls outside all sub-cells due to rounding (very rare; only if cell bounds aren't tile-aligned), it gets parked on the closest sub-cell so the index isn't dropped.
  - Re-points every tile inside the parent footprint to the matching sub-cell.
  - Walks existing internal edges referencing the parent cell, determines which side (W/E/S/N) by matching edge MinX/MaxX/MinZ/MaxZ against parent bounds, and **splits each edge into per-sub-cell pieces**.
  - Adds NEW internal edges between adjacent sub-cells (4 for a 2×2 grid).
  - Applies the SAME boundary-edge surgery to **global edges** (added in the second iteration after the user reported "glitches when walking from 5c94 to another nvm"). Cross-region links (Region0/Region1) stay identical on every piece.

### Step 3 — addCustomBboxObjectIndices

- For each custom-asset NVMObject: compute rotated world bbox, walk all cells, append the NVMObject's index to cells with overlap ≥ 20 units on BOTH axes. Skips duplicates.

## What's verified

`internal/editor/nvm_split_test.go` `TestSplitCell6Baseline` (`go test -v -run TestSplitCell6Baseline ./internal/editor/`):

```
sub-cell 0 (array 6):   (1440,1140)..(1680,1380) objIdx=[]
sub-cell 1 (array 171): (1680,1140)..(1920,1380) objIdx=[9]  ← asset 971 lands here only
sub-cell 2 (array 172): (1440,1380)..(1680,1620) objIdx=[]
sub-cell 3 (array 173): (1680,1380)..(1920,1620) objIdx=[]
tile counts per sub-cell: map[6:144 171:144 172:144 173:144]
edges before pointing at ci6: 5, after split: 4
new edges between sub-cells: 4
east-boundary global edge Z=(1140..1380) -> ref cell 171
east-boundary global edge Z=(1380..1440) -> ref cell 173
east-boundary global edge Z=(1440..1620) -> ref cell 173
```

`cmd/verifyfix/` previously verified the min-overlap threshold: cell 24 (Z-overlap=10, below 20) correctly skipped while cell 6 (Z-overlap=295) correctly added.

`cmd/diffnvm/` is set up for any future side-by-side cell comparison.

## Current status — what's tested in-game

| What user tested | Result |
| --- | --- |
| Tile.Flag bit 0 only | walks through |
| Tile.Flag + CellID = -1 | server boot overlap exception |
| New NVMObject only | walks through |
| Full repartition (uniform 64-cell 240-grid) | client crash on region load (loses baseline closed cells) |
| Default mode, no cell split (only bbox-spread on baseline 480×480 cell) | navmesh breaks when player walks toward collision |
| Default mode + min-overlap threshold only | "same issue" — navmesh still breaks |
| Default mode + cell split (no global-edge split yet) | "pretty fucked up, mostly issue is with edges, glitches out when walking from 5c94 to another nvm" |
| Default mode + cell split + global-edge split (latest build, **untested in-game**) | ?? |

The collision OFFSET (~72 west / 38 south of visual house) is engine-level cell-grid snapping — present in the rebuild tool's output too. NOT a bug in our generation; fixing it would require pre-shifting BMS NavMesh vertices, which has a chicken-and-egg problem with cell-grid placement.

## Open question — cross-region edges from neighbours

After splitting cell 6 in 5c94, **our** 5c94's edges are correctly re-routed to the right sub-cells. But the **neighbour region nvms** (5d94, 5b94, etc.) have their own copies of the same global edges, with `Cell1` referencing cells in 5c94. Those references are stale:

- Baseline neighbour edge: `Cell1=6 in region 5c94`, expected cell 6 = 480×480 covering Z=(1140..1620) on east boundary.
- After our split: cell 6 in 5c94 is the **NW sub-cell**, 240×240 at (1440,1140)..(1680,1380) — does NOT reach the east boundary.

If the engine spatially matches global edges (uses MinX/MaxX/MinZ/MaxZ), this is fine. If it trusts cell indices, the neighbour's edge dangles and the player glitches on crossing.

**Test sequence to narrow this down**: use the latest sromapedit.exe to rebuild 5c94, replace **only** 5c94's nvm in client+server (keep baseline for neighbours). Walk into 5c94 from a neighbour. If it still glitches at the boundary, the neighbour's stale `Cell1` references are the cause and the splitter needs to **also open each neighbour nvm and update its `Cell1` references to point at the right sub-cell**.

**Don't import rebuild tool's neighbour nvms** — those expect rebuild's 5c94 (with rebuild's cell numbering), not ours. Mixing them creates a different mismatch.

## Things NOT to redo from scratch

The path is littered with dead ends; don't re-walk them:

- **Tile.Flag stamping for non-walkable areas:** breaks slopes (tiles under stock object AABBs cover ramps the player needs). Rebuild tool keeps `tile.Flag=0x00` around custom assets; collision comes from `cell→ObjectIndices→BSR/BMS` chain, not tile bits.
- **CellID = -1 to detach blocked tiles:** server boot overlap exception. Don't go there.
- **Spoofing a stock AssetID at a custom position:** doesn't give collision. Collision is bound to baked data outside the .nvm (probably the missing `data_simple_mesh\` blob), not the AssetID or the cells.
- **New closed cell with `ObjectIndices=[]` (terrain-style):** walks through.
- **Adding NEW global edges from scratch:** they're paired across regions — you can't add unilaterally. Split existing ones; don't fabricate.
- **Setting NVMObject count higher than original bake:** client refuses to load with `Load Fail(NavMesh Obj) res\bldg\oasis\tarim\blackrobber\oas_tarim_rob_tent01_01.bsr`. Workaround unknown; this is why our pipeline UPDATES existing NVMObjects when possible.

## Build / test commands

```powershell
# Build
cd D:\ExportedPK2\tools\sromapedit
go build -o sromapedit.exe ./cmd/sromapedit/

# Run the splitter test (uses baseline 5c94 at C:\Silkroad Stuff\Mapeditor\Data\navmesh\)
go test -v -run TestSplitCell6Baseline ./internal/editor/

# Diff two nvm files
go run ./cmd/diffnvm/ 'C:\Silkroad Stuff\Mapeditor\Data\navmesh\nv_5c94.nvm' 'D:\ExportedPK2\rebuild\Data\navmesh\nv_5c94.nvm'

# Min-overlap threshold verification
go run ./cmd/verifyfix/ 'C:\Silkroad Stuff\Mapeditor\Data\navmesh\nv_5c94.nvm'

# Patch SR_GameServer.exe (only if the binary patch was reverted — backup at SR_GameServer.exe.bak)
go run ./cmd/patchexe/
```

In-game test loop:

1. Open sromapedit, navigate to region 5c94.
2. Hit "Rebuild" (default mode — **do NOT tick "Full repartition"**).
3. Inject `nv_5c94.nvm` into:
   - Client: `Data.pk2 → navmesh/nv_5c94.nvm` (use `Replacer.exe` first time, `cmd/pk2inject/` REPLACE mode after).
   - Server: `D:\ExportedPK2\Data\Navmesh\nv_5c94.nvm` (+ `D:\ExportedPK2\export\Data\Navmesh\` mirror if VPS reads from there).
4. Launch client, port to 5c94, walk to the test house.

What to look for:
- [ ] Collision blocks you walking into the house (size matches model, position offset ~72 west + 38 south — expected, not a regression).
- [ ] Walking toward the boundary of the collision doesn't break the surrounding navmesh.
- [ ] Slopes still walkable.
- [ ] Character doesn't glitch out / fall through.
- [ ] Client doesn't crash on region load.
- [ ] **Walking across region boundaries (5c94 ↔ 5d94 etc.) is smooth, not glitchy.** ← this is the gate for the latest fix.

## Key data points to keep in your head

- Region 5c94 = world (0,0)..(1920,1920) tile coordinates. `NVMTileSize=20`, `NVMTileCount=96`.
- Test object: asset ID **3308** ("test_new_obj", Japanese house). UID **29698**. Pos (1752.40, 206.21, 1477.64), Yaw -1.5882 rad (≈ -91°). Rotated world bbox **X=(1607..1897), Z=(1325..1630)**.
- Baseline cell 6 in 5c94: index 6, OPEN, AABB (1440,1140)..(1920,1620), 480×480, ObjectIndices=[9].
- Baseline NVMObject 9: asset 971 (cj_pillar_dom06?) at (1703,1239) — the asset that needs to land in NE sub-cell after split.
- After our split: cell 6 = NW (1440,1140..1680,1380), cell 171 = NE (1680,1140..1920,1380), cell 172 = SW, cell 173 = SE.
- Rebuild tool's equivalents: cells 171/172/179/180 (different numbering because rebuild does a full re-partition; topologically identical though).

## Memory file index

User auto-memory at `C:\Users\Mace\.claude\projects\D--ExportedPK2\memory\`:

- `silkroad-binary-format-notes.md` — the long-form version of the BSR/BMS/BMT/DDJ/nvm field details, including the cell-grid algorithm section.
- `sromapedit-build-rule.md` — always `go build` after edits (static files are embedded).
- `nvm-rebuild-strategy.md` — defaults to heights+tileflags only; full repartition is destructive and opt-in.
- `user-keyboard-layout.md` — German QWERTZ; use `e.key` not `e.code`.
- `shadow-bake-calibration.md` — bake tool defaults.

## 2026-05-16 update: BMS collision-offset calibration

`EncodeMinimalBMSWithOptions` now supports a local X/Z offset for the generated BMS NavMesh section only. The visual OBJ vertices stay unchanged; the BMS/BSR bbox expands to include the shifted nav footprint so broadphase and NVM cell linking can still see it.

`CustomObjectMeta` has optional:

```json
"collisionOffsetX": -39.25,
"collisionOffsetZ": 71.33
```

The export endpoint accepts the same values as query params. The helper can set and export them:

```powershell
go run .\cmd\exportcheck D:\ExportedPK2 2147483648 -39.25 71.33
```

That candidate was exported for `test_new_obj` based on the earlier observed drift of roughly 72 west / 38 south at yaw -1.5882. If in-game testing shows the direction is wrong, set the offsets back to `0 0` or convert the measured desired world shift into local space:

```text
localX = cos(yaw)*worldX + sin(yaw)*worldZ
localZ = -sin(yaw)*worldX + cos(yaw)*worldZ
```

## Where to go next

1. **Verify the latest build (cell-split + global-edge-split) fixes the boundary glitch** with baseline neighbours kept untouched.
2. **If still glitches at boundaries**, implement neighbour-nvm patching: when `splitOpenCell` modifies global edges referencing a cell in our region, walk the 8 surrounding region nvms, find global edges with `Region1 == ourRegion` and `Cell1 == ci`, and apply the same split/re-point surgery from the other side. This means `splitOversizedCellsForCustom` needs access to the file paths of neighbour nvms — easiest via passing the region (x,y) coords from `server.go` and resolving paths via `sromap.NVMFileName(x±1, y±1)`.
3. **If neighbour patching works**, consider whether to also replicate the rebuild tool's NVMObject-array reordering. Probably not necessary for correctness, but it would make our output byte-closer to the rebuild reference.
4. **Collision offset:** likely unfixable from nvm side. If the user cares enough, the BMS NavMesh-section vertices could be pre-shifted at encode time by ~+72 X, +38 Z to compensate — but this has a chicken-and-egg with cell-grid snapping (the shift amount depends on the cell layout which depends on the asset position which depends on the shift). May need an iterative encode.
