# Changelog

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
