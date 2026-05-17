package main
import (
    "fmt"
    "os"
    "strconv"
    "sromapedit/internal/sromap"
)
// Append ONE NVMObject (CUSTOM AssetID arg or 945 default) at house position
// to baseline. Tests if the chain (NMI exists → SNavMeshInst → collision)
// works when AssetID is one the server has NMI for.
func main(){
    src := os.Args[1]
    dst := os.Args[2]
    var assetID uint32 = 945
    if len(os.Args) >= 4 {
        v, _ := strconv.ParseUint(os.Args[3], 10, 32)
        assetID = uint32(v)
    }
    n, err := sromap.LoadNVM(src)
    if err != nil { panic(err) }
    before := len(n.Objects)
    n.Objects = append(n.Objects, sromap.NVMObject{
        AssetID: assetID,
        X: 1553.83, Y: 215.59, Z: 1612.72,
        Yaw: 0,
        Type: -1,
        UID: 29699,
        Short0: 0,
        IsBig: false,
        IsStruct: false,
        RegionID: 0x5C94,
        Links: nil,
    })
    if err := n.Save(dst); err != nil { panic(err) }
    fmt.Printf("NVMObjects: %d -> %d (added asset %d at house position)\n", before, len(n.Objects), assetID)
}
