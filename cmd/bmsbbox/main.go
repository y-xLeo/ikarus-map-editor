package main
import ("fmt";"math";"os";"sromapedit/internal/sromap")
func main(){
    bms,err:=sromap.LoadBMS(os.Args[1])
    if err!=nil{panic(err)}
    fmt.Printf("=== %s ===\n", os.Args[1])
    fmt.Printf("BBoxMin: %v\n", bms.BBoxMin)
    fmt.Printf("BBoxMax: %v\n", bms.BBoxMax)
    fmt.Printf("Size: %vx%vx%v\n", bms.BBoxMax[0]-bms.BBoxMin[0], bms.BBoxMax[1]-bms.BBoxMin[1], bms.BBoxMax[2]-bms.BBoxMin[2])
    
    // Compute actual vertex extents
    var minX,minY,minZ float32 = math.MaxFloat32, math.MaxFloat32, math.MaxFloat32
    var maxX,maxY,maxZ float32 = -math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32
    for _,v:=range bms.Vertices {
        if v.X<minX{minX=v.X};if v.X>maxX{maxX=v.X}
        if v.Y<minY{minY=v.Y};if v.Y>maxY{maxY=v.Y}
        if v.Z<minZ{minZ=v.Z};if v.Z>maxZ{maxZ=v.Z}
    }
    fmt.Printf("\nActual vertex extents (%d verts):\n", len(bms.Vertices))
    fmt.Printf("  X: %.2f .. %.2f (range %.2f)\n", minX, maxX, maxX-minX)
    fmt.Printf("  Y: %.2f .. %.2f (range %.2f)\n", minY, maxY, maxY-minY)
    fmt.Printf("  Z: %.2f .. %.2f (range %.2f)\n", minZ, maxZ, maxZ-minZ)
    fmt.Printf("  Center: (%.2f, %.2f, %.2f)\n", (minX+maxX)/2, (minY+maxY)/2, (minZ+maxZ)/2)
    
    // Compute where in world the collision will land
    placeX,placeZ:=float32(1752.40), float32(1477.64)
    yaw:=float32(-1.5882)
    c:=float32(math.Cos(float64(yaw)))
    s:=float32(math.Sin(float64(yaw)))
    fmt.Printf("\nAfter yaw=%.4f rotation + translate to (%.0f, %.0f):\n", yaw, placeX, placeZ)
    for _,corner:=range [][2]float32{{minX,minZ},{maxX,minZ},{minX,maxZ},{maxX,maxZ}} {
        rx:=c*corner[0]-s*corner[1]+placeX
        rz:=s*corner[0]+c*corner[1]+placeZ
        fmt.Printf("  local (%.2f, %.2f) -> world (%.2f, %.2f)\n", corner[0], corner[1], rx, rz)
    }
}
