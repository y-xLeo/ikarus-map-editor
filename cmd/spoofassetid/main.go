package main
import (
    "fmt"
    "os"
    "sromapedit/internal/sromap"
)
// Usage: spoofassetid <nvm-path> <nvmobject-index> <new-asset-id>
// Changes only the AssetID of a specific NVMObject. Other fields untouched.
func main(){
    if len(os.Args) < 4 {
        fmt.Println("usage: spoofassetid <nvm> <idx> <newAssetID>")
        os.Exit(2)
    }
    var idx int
    var newAID uint32
    fmt.Sscanf(os.Args[2], "%d", &idx)
    fmt.Sscanf(os.Args[3], "%d", &newAID)
    n, err := sromap.LoadNVM(os.Args[1])
    if err != nil { panic(err) }
    if idx < 0 || idx >= len(n.Objects) {
        fmt.Printf("idx %d out of range (objects=%d)\n", idx, len(n.Objects))
        os.Exit(2)
    }
    old := n.Objects[idx].AssetID
    n.Objects[idx].AssetID = newAID
    if err := n.Save(os.Args[1]); err != nil { panic(err) }
    fmt.Printf("Changed NVMObject[%d].AssetID from %d to %d in %s\n", idx, old, newAID, os.Args[1])
}
