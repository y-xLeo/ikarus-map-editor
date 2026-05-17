package main
import ("fmt";"os";"sromapedit/internal/sromap")
func main(){
    n,_:=sromap.LoadNVM(os.Args[1])
    target:=uint16(10) // asset 3308 is at index 10
    fmt.Printf("=== Cells referencing NVMObject index %d (asset 3308) ===\n", target)
    for ci,c:=range n.Cells {
        for _,idx:=range c.ObjectIndices {
            if idx==target {
                kind:="OPEN"; if uint32(ci)>=n.OpenCellCount{kind="CLOSED"}
                fmt.Printf("  cell %d (%s) AABB=(%.0f,%.0f)..(%.0f,%.0f) size=%.0fx%.0f ObjIdx=%v\n",
                    ci,kind,c.MinX,c.MinZ,c.MaxX,c.MaxZ,c.MaxX-c.MinX,c.MaxZ-c.MinZ,c.ObjectIndices)
                break
            }
        }
    }
}
