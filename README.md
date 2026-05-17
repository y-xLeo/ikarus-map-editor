# sromapedit

Small Go CLI for Silkroad map region terrain editing.

Current scope:

- Reads `Map/mapinfo.mfo` (`JMXVMFO 1000`) and reports active field bounds.
- Reads fixed-layout map mesh regions (`JMXVMAPM1000`) from `Map/<y>/<x>.m`.
- Raises or lowers terrain heights with whole-region or circular brush edits.
- Recomputes each block's `HeightMax` and `HeightMin`.
- Preserves unknown bytes and trailing bytes in larger `.m` variants.
- Optionally applies the same height brush to existing `JMXVNVM 1000` heightmaps.
- Parses `Map/tile2d.ifo` (`JMXV2DTI`) for the 720+ tile texture entries.
- Decodes `Map/Tile2D/*.ddj` (JMXVDDJ-wrapped DDS, DXT1/DXT3/DXT5) to RGBA in pure Go.
- Builds per-region 768x768 terrain composites from the `.m` per-vertex texture
  IDs and brightness, served as PNG and applied to the WebGL terrain.

`serve` HTTP endpoints:

| Endpoint | Description |
| --- | --- |
| `/api/info` | Map bounds, active region list, refregion count |
| `/api/region?x=&y=` | Heights, objects, refregion, `textureUrl` |
| `/api/region/save` | Save edited heights (with optional NVM sync) |
| `/api/region/texture?x=&y=` | PNG composite of the region terrain texture |
| `/api/tile?id=N` | PNG of a single tile2d texture by ID |
| `/api/tiles` | JSON listing of every tile2d.ifo entry |

Build:

```powershell
go build -o .\sromapedit.exe .\cmd\sromapedit
```

Examples:

```powershell
.\sromapedit.exe info -root D:\ExportedPK2
.\sromapedit.exe inspect -root D:\ExportedPK2 -region 100,100
.\sromapedit.exe raise -root D:\ExportedPK2 -region 100,100 -delta 5 -cx 960 -cz 960 -radius 200
.\sromapedit.exe raise -root D:\ExportedPK2 -region 100,100 -delta -2 -all -write -sync-nvm
```

`raise` is a dry run unless `-write` or `-out <path>` is passed. When overwriting, `.bak` files are created by default.

Notes:

- Local region coordinates are `0..1920` in X and Z.
- The `.m` terrain grid is `97x97` vertices at 20-unit spacing. The file stores duplicate vertices on block borders; the editor updates all duplicates.
- `-sync-nvm` only updates the NVM `HeightMap`. It does not regenerate cells, edges, or object navigation.
