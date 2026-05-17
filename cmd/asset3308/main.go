package main
import ("fmt";"math";"os";"sromapedit/internal/sromap")
func main(){
    n,_:=sromap.LoadNVM(os.Args[1])
    if len(n.Objects)<10 {fmt.Println("no obj 9");return}
    o:=n.Objects[9]
    fmt.Printf("=== %s : NVMObject[9] ===\n", os.Args[1])
    fmt.Printf("  AssetID=%d  pos=(%.2f, %.2f, %.2f)  yaw=%.4f (=%.2f°)  UID=%d  Type=%d\n",
        o.AssetID, o.X, o.Y, o.Z, o.Yaw, o.Yaw*180/math.Pi, o.UID, o.Type)
    fmt.Printf("  IsBig=%v IsStruct=%v  RegionID=0x%04x\n", o.IsBig, o.IsStruct, o.RegionID)

    fmt.Println("\n=== All cells with ObjIdx referencing 9 ===")
    for ci,c:=range n.Cells {
        for _,idx:=range c.ObjectIndices {
            if idx==9 {
                kind:="OPEN"; if uint32(ci)>=n.OpenCellCount{kind="CLOSED"}
                fmt.Printf("  cell %d (%s) AABB=(%.0f,%.0f)..(%.0f,%.0f) size=%.0fx%.0f\n",
                    ci, kind, c.MinX, c.MinZ, c.MaxX, c.MaxZ, c.MaxX-c.MinX, c.MaxZ-c.MinZ)
                break
            }
        }
    }

    // Tile flags around asset 3308's footprint
    fmt.Println("\n=== Tile flags around (1752, 1478) ±5 tiles ===")
    cgx:=87; cgz:=73  // tile coords of asset position (1752/20=87, 1478/20=73)
    for dz:=-5;dz<=5;dz++ {
        for dx:=-5;dx<=5;dx++ {
            gx:=cgx+dx; gz:=cgz+dz
            if gx<0||gx>=96||gz<0||gz>=96 {fmt.Printf("  .  ");continue}
            t:=n.Tiles[gz*96+gx]
            fmt.Printf(" %02x:%-3d", t.Flag, t.CellID)
        }
        fmt.Println()
    }
}
