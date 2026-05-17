package main
import ("fmt";"os";"sromapedit/internal/sromap")
func main(){
    n,_:=sromap.LoadNVM(os.Args[1])
    // Bbox bounds (with margin)
    minX,maxX:=float32(1400),float32(1920)
    minZ,maxZ:=float32(1100),float32(1700)
    fmt.Printf("Cells overlapping (%g..%g, %g..%g):\n",minX,maxX,minZ,maxZ)
    for ci,c:=range n.Cells {
        if c.MinX>=maxX||c.MaxX<=minX||c.MinZ>=maxZ||c.MaxZ<=minZ {continue}
        kind:="OPEN";if uint32(ci)>=n.OpenCellCount{kind="CLOSED"}
        fmt.Printf("  cell %d (%s): AABB=(%.0f,%.0f)..(%.0f,%.0f) size=%.0fx%.0f ObjIdx=%v\n",
            ci,kind,c.MinX,c.MinZ,c.MaxX,c.MaxZ,c.MaxX-c.MinX,c.MaxZ-c.MinZ,c.ObjectIndices)
    }
}
