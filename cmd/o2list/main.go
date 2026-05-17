package main

import (
	"fmt"
	"os"
	"strconv"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: o2list <root> [x y [asset-id]]")
		return
	}
	x, y := 148, 92
	if len(os.Args) >= 4 {
		var err error
		x, err = strconv.Atoi(os.Args[2])
		if err != nil {
			fmt.Println("bad x:", err)
			return
		}
		y, err = strconv.Atoi(os.Args[3])
		if err != nil {
			fmt.Println("bad y:", err)
			return
		}
	}
	var filter uint64
	if len(os.Args) >= 5 {
		v, err := strconv.ParseUint(os.Args[4], 10, 32)
		if err != nil {
			fmt.Println("bad asset-id:", err)
			return
		}
		filter = v
	}

	p := sromap.O2Path(os.Args[1], x, y)
	o2, err := sromap.LoadO2(p)
	if err != nil {
		fmt.Println("ERR:", err)
		return
	}

	fmt.Printf("%s entries=%d\n", p, len(o2.Entries))
	matches := 0
	for i, e := range o2.Entries {
		if filter != 0 && uint64(e.ObjID) != filter {
			continue
		}
		matches++
		fmt.Printf("  [%d] asset=%d uid=%d region=0x%04x pos=(%.2f, %.2f, %.2f) yaw=%.4f big=%v struct=%v block=(%d,%d) lod=%d\n",
			i, e.ObjID, e.UID, e.RegionID, e.X, e.Y, e.Z, e.Yaw, e.Big, e.Struct, e.XBlock, e.ZBlock, e.LODGroup)
	}
	fmt.Printf("matches=%d\n", matches)
}
