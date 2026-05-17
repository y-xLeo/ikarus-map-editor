package main
import ("fmt";"os";"reflect";"sromapedit/internal/sromap")
func main(){
    a,_:=sromap.LoadNVM(os.Args[1])
    b,_:=sromap.LoadNVM(os.Args[2])
    if len(a.Cells)!=len(b.Cells) {fmt.Printf("Cell count differs %d vs %d\n",len(a.Cells),len(b.Cells));return}
    for i:=range a.Cells {
        if !reflect.DeepEqual(a.Cells[i].ObjectIndices, b.Cells[i].ObjectIndices) {
            fmt.Printf("cell %d ObjIdx %v -> %v (AABB=%.0f,%.0f..%.0f,%.0f)\n",
                i, a.Cells[i].ObjectIndices, b.Cells[i].ObjectIndices,
                a.Cells[i].MinX,a.Cells[i].MinZ,a.Cells[i].MaxX,a.Cells[i].MaxZ)
        }
    }
}
