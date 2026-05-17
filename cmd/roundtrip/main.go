package main
import (
    "fmt"
    "os"
    "sromapedit/internal/sromap"
)
func main(){
    n, err := sromap.LoadNVM(os.Args[1])
    if err != nil { panic(err) }
    if err := n.Save(os.Args[2]); err != nil { panic(err) }
    fmt.Printf("Roundtripped %s -> %s\n", os.Args[1], os.Args[2])
}
