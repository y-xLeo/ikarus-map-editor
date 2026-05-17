package main
import ("fmt";"os";"sromapedit/internal/sromap")
func main(){
    n,_:=sromap.LoadNVM(os.Args[1])
    fmt.Printf("=== %s ===\n", os.Args[1])
    fmt.Printf("Objects=%d Cells=%d (open=%d closed=%d) IntE=%d GlobE=%d\n",
        len(n.Objects), len(n.Cells), n.OpenCellCount, uint32(len(n.Cells))-n.OpenCellCount,
        len(n.InternalEdges), len(n.GlobalEdges))
    // Show cells that reference asset 3308 (index 9)
    fmt.Println("\nCells referencing NVMObject 9 (asset 3308):")
    for ci,c:=range n.Cells {
        for _,idx:=range c.ObjectIndices {
            if idx==9 {
                kind:="OPEN"; if uint32(ci)>=n.OpenCellCount{kind="CLOSED"}
                fmt.Printf("  cell %d (%s): AABB=(%.0f,%.0f)..(%.0f,%.0f) ObjIdx=%v\n",
                    ci, kind, c.MinX, c.MinZ, c.MaxX, c.MaxZ, c.ObjectIndices)
                break
            }
        }
    }
}
