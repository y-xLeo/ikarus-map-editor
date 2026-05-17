package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: nvmfindasset <navmesh-dir> <asset-id> [asset-id...]")
		os.Exit(2)
	}
	want := map[uint32]bool{}
	for _, arg := range os.Args[2:] {
		v, err := strconv.ParseUint(arg, 10, 32)
		if err != nil {
			fmt.Fprintln(os.Stderr, "bad asset id:", arg)
			os.Exit(2)
		}
		want[uint32(v)] = true
	}
	err := filepath.WalkDir(os.Args[1], func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".nvm") {
			return err
		}
		nvm, err := sromap.LoadNVM(path)
		if err != nil {
			fmt.Printf("%s: load: %v\n", filepath.Base(path), err)
			return nil
		}
		for i, obj := range nvm.Objects {
			if want[obj.AssetID] {
				fmt.Printf("%s object[%d] asset=%d uid=%d region=0x%04x pos=(%.2f,%.2f,%.2f)\n",
					filepath.Base(path), i, obj.AssetID, obj.UID, obj.RegionID, obj.X, obj.Y, obj.Z)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
