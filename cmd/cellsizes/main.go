package main
import ("fmt";"os";"sort";"sromapedit/internal/sromap")
func main(){
    n,_:=sromap.LoadNVM(os.Args[1])
    fmt.Printf("=== %s: %d cells ===\n", os.Args[1], len(n.Cells))
    sizes:=map[string]int{}
    for _,c:=range n.Cells {
        w:=c.MaxX-c.MinX; h:=c.MaxZ-c.MinZ
        k:=fmt.Sprintf("%.0fx%.0f",w,h); sizes[k]++
    }
    type kv struct{k string;v int}
    var e []kv
    for k,v:=range sizes{e=append(e,kv{k,v})}
    sort.Slice(e,func(i,j int)bool{return e[i].v>e[j].v})
    for i:=0;i<10&&i<len(e);i++{fmt.Printf("  %s: %d cells\n",e[i].k,e[i].v)}
}
