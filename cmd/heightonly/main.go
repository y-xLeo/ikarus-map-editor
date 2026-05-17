package main
import (
    "fmt"
    "os"
    "sromapedit/internal/sromap"
)
// Modify ONLY one tile's height in the .nvm. NVMObjects, cells, edges, tile
// flags all untouched. If server crashes on this too, it has a full-file
// hash check. If it doesn't crash, only the NVMObject section is validated.
func main(){
    n, err := sromap.LoadNVM(os.Args[1])
    if err != nil { panic(err) }
    fmt.Printf("BEFORE: height[0]=%.2f\n", n.Heights[0])
    n.Heights[0] += 0.001  // tiny change
    fmt.Printf("AFTER : height[0]=%.2f\n", n.Heights[0])
    if err := n.Save(os.Args[1]); err != nil { panic(err) }
}
