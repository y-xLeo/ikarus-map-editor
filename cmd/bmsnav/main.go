package main
import (
    "encoding/binary"
    "fmt"
    "math"
    "os"
)
func main(){
    raw,_:=os.ReadFile(os.Args[1])
    if string(raw[:12])!="JMXVBMS 0110" { panic("bad sig") }
    
    // BMS header has section offsets at fixed positions
    // Per memory: vertex normals, color FFFFFFFF, etc.
    // We need to find the navmesh section
    
    // Section offsets (per our encoder layout) typically:
    // 12 + 8 offsets (32 bytes), then header fields
    fmt.Printf("=== %s (%d bytes) ===\n", os.Args[1], len(raw))
    fmt.Printf("First 64 bytes:\n")
    for i:=0; i<64; i+=16 {
        fmt.Printf("  %04x: % x\n", i, raw[i:i+16])
    }
    fmt.Println()
    // Offsets at file offset 12+:
    var offsets [9]uint32
    for i:=0; i<9; i++ {
        offsets[i]=binary.LittleEndian.Uint32(raw[12+i*4:16+i*4])
    }
    fmt.Println("Section offsets (BMS header):")
    for i,o := range offsets {
        fmt.Printf("  [%d] = 0x%x (%d)\n", i, o, o)
    }
    
    // Search for navmesh marker — typically near end of file
    // Look for our marker bytes (4 verts of quad at bbox corners)
    // Our BMS encoder wrote the corners (-150, 0, -142.38), (150, 0, -142.38), etc.
    needle:=float32(-150)
    needleBytes:=make([]byte,4)
    binary.LittleEndian.PutUint32(needleBytes, math.Float32bits(needle))
    fmt.Printf("\nSearching for vertex with x=-150 (bytes %x)...\n", needleBytes)
    found:=0
    for i:=0; i+4<len(raw); i++ {
        if raw[i]==needleBytes[0] && raw[i+1]==needleBytes[1] && raw[i+2]==needleBytes[2] && raw[i+3]==needleBytes[3] {
            // Read next vertex (12 bytes for xyz)
            if i+12<=len(raw) {
                y:=math.Float32frombits(binary.LittleEndian.Uint32(raw[i+4:i+8]))
                z:=math.Float32frombits(binary.LittleEndian.Uint32(raw[i+8:i+12]))
                fmt.Printf("  offset 0x%x: x=-150 y=%.2f z=%.2f\n", i, y, z)
            }
            found++
            if found > 6 { break }
        }
    }
}
