package main
import (
    "fmt"
    "os"
    "path/filepath"
    "sromapedit/internal/sromap"
)
func main(){
    root := os.Args[1]
    slug := "test_new_obj"
    bmsAbs := filepath.Join(root, "Data", "res", "custom", slug, slug+".bms")
    bms, err := sromap.LoadBMS(bmsAbs)
    if err != nil { panic(err) }
    bmtPath := filepath.Join("prim", "mtrl", "custom", slug, slug+".bmt")
    bmsRel := filepath.Join("prim", "mesh", "custom", slug, slug+".bms")
    bsrBytes, err := sromap.EncodeMinimalBSR(slug, bmtPath, bmsRel, bmsRel, bms.BBoxMin, bms.BBoxMax)
    if err != nil { panic(err) }
    if err := os.WriteFile(filepath.Join(root, "Data", "res", "custom", slug, slug+".bsr"), bsrBytes, 0644); err != nil { panic(err) }
    dst := filepath.Join(root, "Data", "prim", "mesh", "custom", slug, slug+".bms")
    os.MkdirAll(filepath.Dir(dst), 0755)
    bmsData, _ := os.ReadFile(bmsAbs)
    os.WriteFile(dst, bmsData, 0644)
    fmt.Printf("BSR written: %d bytes (CollisionMesh -> %s)\n", len(bsrBytes), bmsRel)
    fmt.Printf("BMS copied to: %s (%d bytes)\n", dst, len(bmsData))
}
