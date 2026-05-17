package main

import (
	"fmt"
	"os"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: planeseam <local.nvm> <neighbor.nvm> <E|N>")
		os.Exit(2)
	}
	a, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load local:", err)
		os.Exit(1)
	}
	b, err := sromap.LoadNVM(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load neighbor:", err)
		os.Exit(1)
	}
	side := os.Args[3][0]
	for i := 0; i < sromap.MeshBlockCount; i++ {
		var ai, bi int
		switch side {
		case 'N', 'n':
			ai = (sromap.MeshBlockCount-1)*sromap.MeshBlockCount + i
			bi = i
		case 'E', 'e':
			ai = i*sromap.MeshBlockCount + sromap.MeshBlockCount - 1
			bi = i * sromap.MeshBlockCount
		default:
			fmt.Fprintln(os.Stderr, "side must be E or N")
			os.Exit(2)
		}
		fmt.Printf("block %d local type=%d height=%.3f neighbor type=%d height=%.3f\n",
			i, a.PlaneType[ai], a.PlaneHeight[ai], b.PlaneType[bi], b.PlaneHeight[bi])
	}
}
