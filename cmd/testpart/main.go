package main
import ("fmt";"sromapedit/internal/sromap")
func main(){
    var walkable [sromap.NVMTotalTiles]bool
    for i:=range walkable {walkable[i]=true}
    cells,_,openCount:=sromap.PartitionCells(walkable)
    fmt.Printf("All-walkable region produces %d cells (open=%d, closed=%d)\n",
        len(cells), openCount, len(cells)-openCount)
    sizes:=map[string]int{}
    for _,c:=range cells {
        w:=c.MaxTileX-c.MinTileX+1
        h:=c.MaxTileZ-c.MinTileZ+1
        k:=fmt.Sprintf("%dx%d",w*sromap.NVMTileSize,h*sromap.NVMTileSize)
        sizes[k]++
    }
    for k,v:=range sizes{fmt.Printf("  %s: %d cells\n",k,v)}
}
