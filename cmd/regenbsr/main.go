package main
import (
    "fmt"; "os"; "path/filepath"
    "sromapedit/internal/sromap"
)
func main(){
    root := os.Args[1]
    slug := "test_new_obj"
    bmsAbs := filepath.Join(root, "Data", "res", "custom", slug, slug+".bms")
    bms, _ := sromap.LoadBMS(bmsAbs)
    bmtPath := filepath.Join("prim", "mtrl", "custom", slug, slug+".bmt")
    bmsRel := filepath.Join("res", "custom", slug, slug+".bms")
    bsrBytes, err := sromap.EncodeMinimalBSR(slug, bmtPath, bmsRel, bmsRel, bms.BBoxMin, bms.BBoxMax)
    if err != nil { panic(err) }
    bsrOut := filepath.Join(root, "Data", "res", "custom", slug, slug+".bsr")
    os.WriteFile(bsrOut, bsrBytes, 0644)
    fmt.Printf("BSR regenerated with res/custom path: %d bytes\n", len(bsrBytes))
}
