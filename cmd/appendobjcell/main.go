package main
import (
    "fmt"
    "os"
    "strconv"
    "sromapedit/internal/sromap"
)
// Append NVMObject + closed cell + wall edges around perimeter + tile redirect.
// Lightweight version: only modifies one area, leaves rest of cell graph alone.
func main(){
    src := os.Args[1]
    dst := os.Args[2]
    var assetID uint32 = 968
    if len(os.Args) >= 4 {
        v, _ := strconv.ParseUint(os.Args[3], 10, 32)
        assetID = uint32(v)
    }
    n, err := sromap.LoadNVM(src)
    if err != nil { panic(err) }

    newObjIdx := uint16(len(n.Objects))
    n.Objects = append(n.Objects, sromap.NVMObject{
        AssetID: assetID,
        X: 1553.83, Y: 215.59, Z: 1612.72,
        Yaw: 0, Type: -1, UID: 29699, Short0: 0,
        IsBig: false, IsStruct: false,
        RegionID: 0x5C94, Links: nil,
    })

    const tile = float32(20)
    minX, minZ, maxX, maxZ := float32(1400), float32(1460), float32(1720), float32(1760)
    newCellIdx := int32(len(n.Cells))
    n.Cells = append(n.Cells, sromap.NVMCell{
        MinX: minX, MinZ: minZ, MaxX: maxX, MaxZ: maxZ,
        ObjectIndices: []uint16{newObjIdx},
    })

    iMin, iMax := int(minX/tile), int(maxX/tile)-1
    jMin, jMax := int(minZ/tile), int(maxZ/tile)-1
    for tj := jMin; tj <= jMax; tj++ {
        for ti := iMin; ti <= iMax; ti++ {
            idx := tj*sromap.NVMTileCount + ti
            n.Tiles[idx].CellID = newCellIdx
            n.Tiles[idx].Flag |= 1
        }
    }

    // Wall edges: one per perimeter tile, attached to adjacent open cell
    const wallFlag uint8 = 0x02
    addWall := func(minX, minZ, maxX, maxZ float32, dir uint8, ngTile int) {
        if ngTile < 0 || ngTile >= sromap.NVMTotalTiles { return }
        cell0 := n.Tiles[ngTile].CellID
        if cell0 < 0 || uint32(cell0) >= n.OpenCellCount { return }
        n.InternalEdges = append(n.InternalEdges, sromap.NVMInternalEdge{
            MinX: minX, MinZ: minZ, MaxX: maxX, MaxZ: maxZ,
            Flag: wallFlag, Dir0: dir, Dir1: 0xFF,
            Cell0: int16(cell0), Cell1: -1,
        })
    }
    // South side z=minZ (dir=0 from south-adj cell)
    for ti := iMin; ti <= iMax; ti++ {
        tx := float32(ti) * tile
        if jMin > 0 { addWall(tx, minZ, tx+tile, minZ, 0, (jMin-1)*sromap.NVMTileCount+ti) }
    }
    // North z=maxZ (dir=2)
    for ti := iMin; ti <= iMax; ti++ {
        tx := float32(ti) * tile
        if jMax+1 < sromap.NVMTileCount { addWall(tx, maxZ, tx+tile, maxZ, 2, (jMax+1)*sromap.NVMTileCount+ti) }
    }
    // West x=minX (dir=1 from west-adj cell)
    for tj := jMin; tj <= jMax; tj++ {
        tz := float32(tj) * tile
        if iMin > 0 { addWall(minX, tz, minX, tz+tile, 1, tj*sromap.NVMTileCount+(iMin-1)) }
    }
    // East x=maxX (dir=3)
    for tj := jMin; tj <= jMax; tj++ {
        tz := float32(tj) * tile
        if iMax+1 < sromap.NVMTileCount { addWall(maxX, tz, maxX, tz+tile, 3, tj*sromap.NVMTileCount+(iMax+1)) }
    }

    if err := n.Save(dst); err != nil { panic(err) }
    fmt.Printf("NVMObjects=%d Cells=%d IntEdges=%d (added asset %d + closed cell + wall edges)\n",
        len(n.Objects), len(n.Cells), len(n.InternalEdges), assetID)
}
