// nvmcollisionprobe dumps which tiles in a region's NVM are currently
// marked blocked (bit 1 of Tile.Flag) and prints the placements + their
// AABBs side-by-side. Lets us verify "house is at X, blocked tiles cover X".
package main

import (
	"fmt"
	"os"

	"sromapedit/internal/editor"
	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: nvmcollisionprobe <root> <regionX> <regionY>")
		os.Exit(2)
	}
	root := os.Args[1]
	rx := atoi(os.Args[2])
	ry := atoi(os.Args[3])

	srv, err := editor.NewServer(root)
	if err != nil {
		die("server:", err)
	}
	_ = srv

	// Load placements from .o2.
	o2, err := sromap.LoadO2(sromap.O2Path(root, rx, ry))
	if err != nil {
		fmt.Fprintln(os.Stderr, "load o2:", err)
		os.Exit(1)
	}
	regionID := uint16(ry)<<8 | uint16(rx)
	var placements []sromap.ObjectEntry
	for _, e := range o2.Entries {
		if e.RegionID == regionID {
			placements = append(placements, e)
		}
	}
	fmt.Printf("Placements in region %d,%d (%d):\n", rx, ry, len(placements))
	for _, p := range placements {
		fmt.Printf("  objID=%d uid=%d  local (%.1f, %.1f, %.1f)  yaw=%.2f\n",
			p.ObjID, p.UID, p.X, p.Y, p.Z, p.Yaw)
	}

	// Load NVM and dump blocked tiles.
	nvmPaths := sromap.ExistingNVMPaths(root, rx, ry)
	if len(nvmPaths) == 0 {
		die("no NVM for region")
	}
	n, err := sromap.LoadNVM(nvmPaths[0])
	if err != nil {
		die("load nvm:", err)
	}
	fmt.Printf("\nNVM %s\n", nvmPaths[0])
	blockedCount := 0
	for i := range n.Tiles {
		if n.Tiles[i].Flag&2 != 0 {
			blockedCount++
		}
	}
	fmt.Printf("Blocked tiles: %d / %d\n", blockedCount, len(n.Tiles))

	// 96x96 ASCII heat-map of blocked tiles (1 char per tile, so 96 wide).
	// "#" = blocked, "." = walkable. Z increases downward in the printout,
	// X increases rightward — same orientation as you look at the world map.
	fmt.Println("\nTile map (X right, Z down, # = blocked):")
	for j := sromap.NVMTileCount - 1; j >= 0; j-- {
		for i := 0; i < sromap.NVMTileCount; i++ {
			t := n.Tiles[j*sromap.NVMTileCount+i]
			if t.Flag&2 != 0 {
				fmt.Print("#")
			} else {
				fmt.Print(".")
			}
		}
		fmt.Println()
	}
}

func atoi(s string) int {
	v := 0
	for _, c := range s {
		v = v*10 + int(c-'0')
	}
	return v
}

func die(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
