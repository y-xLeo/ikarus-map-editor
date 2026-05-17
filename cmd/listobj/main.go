package main
import ("fmt";"os";"sromapedit/internal/sromap")
func main(){
    n,_:=sromap.LoadNVM(os.Args[1])
    fmt.Printf("=== %s: %d NVMObjects ===\n", os.Args[1], len(n.Objects))
    for i,o:=range n.Objects {
        fmt.Printf("  [%d] aid=%d pos=(%.0f,%.0f,%.0f) uid=%d yaw=%.2f type=%d region=0x%04x\n",
            i,o.AssetID,o.X,o.Y,o.Z,o.UID,o.Yaw,o.Type,o.RegionID)
    }
}
