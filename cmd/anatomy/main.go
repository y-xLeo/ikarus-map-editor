package main
import (
    "fmt"
    "os"
    "sromapedit/internal/sromap"
)
func main(){
    n, _ := sromap.LoadNVM(os.Args[1])
    fmt.Printf("=== %s ===\n", os.Args[1])
    fmt.Printf("NVMObjects=%d Cells=%d (open=%d closed=%d) IntEdges=%d\n",
        len(n.Objects), len(n.Cells), n.OpenCellCount, uint32(len(n.Cells))-n.OpenCellCount,
        len(n.InternalEdges))

    fmt.Println("\n=== All cells referencing NVMObject 4 (asset 968 at 141,290,460) ===")
    cellsForObj4 := []int{}
    for ci, c := range n.Cells {
        for _, idx := range c.ObjectIndices {
            if idx == 4 {
                kind := "OPEN"
                if uint32(ci) >= n.OpenCellCount { kind = "CLOSED" }
                fmt.Printf("  cell %d (%s): AABB=(%.0f,%.0f)..(%.0f,%.0f) ObjIdx=%v\n",
                    ci, kind, c.MinX, c.MinZ, c.MaxX, c.MaxZ, c.ObjectIndices)
                cellsForObj4 = append(cellsForObj4, ci)
                break
            }
        }
    }
    fmt.Printf("Total cells: %d\n", len(cellsForObj4))

    fmt.Println("\n=== Internal edges touching any of those cells ===")
    edgesFor := 0
    for _, e := range n.InternalEdges {
        for _, ci := range cellsForObj4 {
            if int(e.Cell0) == ci || int(e.Cell1) == ci {
                edgesFor++
                break
            }
        }
    }
    fmt.Printf("Internal edges touching obj 4's cells: %d\n", edgesFor)
}
