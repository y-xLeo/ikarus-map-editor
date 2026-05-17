package main

import (
	"fmt"
	"os"
	"strconv"

	"sromapedit/internal/editor"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: fixo2visibility <root> <x> <y> [asset-id...]")
		os.Exit(2)
	}
	root := os.Args[1]
	rx, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad x:", err)
		os.Exit(2)
	}
	ry, err := strconv.Atoi(os.Args[3])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad y:", err)
		os.Exit(2)
	}
	var filter map[uint32]bool
	if len(os.Args) > 4 {
		filter = make(map[uint32]bool, len(os.Args)-4)
		for _, arg := range os.Args[4:] {
			v, err := strconv.ParseUint(arg, 10, 32)
			if err != nil {
				fmt.Fprintln(os.Stderr, "bad asset-id:", err)
				os.Exit(2)
			}
			filter[uint32(v)] = true
		}
	}
	srv, err := editor.NewServer(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
	changed, err := srv.RepairO2Visibility(rx, ry, filter)
	if err != nil {
		fmt.Fprintln(os.Stderr, "repair:", err)
		os.Exit(1)
	}
	fmt.Printf("%d,%d visibility entries added=%d\n", rx, ry, changed)
}
