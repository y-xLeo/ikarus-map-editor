package main
import ("fmt";"os";"sromapedit/internal/sromap")
func main(){
    n,_:=sromap.LoadNVM(os.Args[1])
    fd:=map[uint16]int{}
    for _,t:=range n.Tiles {fd[t.Flag]++}
    fmt.Printf("=== %s ===\n", os.Args[1])
    for k,v:=range fd {fmt.Printf("  Flag=0x%04x: %d tiles\n", k, v)}
}
