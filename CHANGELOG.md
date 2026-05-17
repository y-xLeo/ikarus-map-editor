# Changelog

## v1.0.0 - 2026-05-17

First stable working checkpoint for the map editor.

- Fixed custom object collision alignment through correct BSR/BMS collision metadata and map-only NVM object references.
- Added stable map-only NVM rebuild from map files, including clean reciprocal global edge synchronization.
- Fixed bridge/terrain handoff by assigning bridge object cells from the BMS navmesh footprint instead of an oversized bbox.
- Added selected-object handoff cell visualization for inspecting NVM object ownership in the editor.
- Fixed bridge render culling by repairing `.o2` visibility block buckets from each object's rotated visual bbox.
- Added `fixo2visibility` for patching existing map files with missing object visibility buckets.
- Patched the current bridge test case in `D:\ExportedPK2\Map\92\148.o2` and mirrored it to `D:\ExportedPK2\export\Map\92\148.o2`.

Verification:

- `go test ./...`
- `node --check internal/editor/static/app.js`
- In-game validation for custom collision, bridge handoff, and bridge render visibility.

## working-bridge-handoff-2026-05-17

Bridge/object handoff fix for map-only NVM rebuilds.

- Added a selected-object handoff overlay in the editor so bridge NVM cell ownership can be inspected visually.
- Extended the NVM cell API to return each cell's `ObjectIndices` and each region's NVM object metadata.
- Changed map-only rebuilds for bridge-like objects to attach cells using the BMS navmesh triangle footprint with a small handoff pad instead of the full rotated collision bbox.
- Kept the custom test object collision path unchanged, preserving the known-good custom object behavior.
- Rebuilt and verified the bridge cluster around `nv_5b94` / `nv_5c94`; seam checks stayed clean with zero edge, height, or plane mismatches.

Verification:

- `go test ./...`
- `node --check internal/editor/static/app.js`
- `go run ./cmd/nvmseams D:\ExportedPK2\Data\Navmesh nv_5b94.nvm nv_5c94.nvm`
