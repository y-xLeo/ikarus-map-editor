package main
import (
    "fmt"
    "os"
    "sromapedit/internal/sromap"
)
// Replace NVMObject[idx]'s position/AssetID with the house location.
// Keeps the count at baseline (10). The original cells/edges reference this
// NVMObject by index, so they still point to it after our change. We just
// move it to the house and (optionally) change its asset.
func main(){
    n, err := sromap.LoadNVM(os.Args[1])
    if err != nil { panic(err) }
    // Pick obj[6] (one of the duplicate asset 973 entries) and reposition.
    idx := 6
    old := n.Objects[idx]
    fmt.Printf("BEFORE: obj[%d]: AssetID=%d pos=(%.0f,%.0f,%.0f) UID=%d region=0x%04x\n",
        idx, old.AssetID, old.X, old.Y, old.Z, old.UID, old.RegionID)
    n.Objects[idx].X = 1553
    n.Objects[idx].Y = 216
    n.Objects[idx].Z = 1613
    if err := n.Save(os.Args[1]); err != nil { panic(err) }
    fmt.Printf("AFTER : obj[%d]: AssetID=%d pos=(%.0f,%.0f,%.0f) UID=%d region=0x%04x\n",
        idx, n.Objects[idx].AssetID, n.Objects[idx].X, n.Objects[idx].Y, n.Objects[idx].Z, n.Objects[idx].UID, n.Objects[idx].RegionID)
}
