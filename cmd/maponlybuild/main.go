package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sromapedit/internal/editor"
	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: maponlybuild [--terrain-only] <root> <outdir> <region|rx ry> [<region|rx ry>...]")
		os.Exit(2)
	}
	args := os.Args[1:]
	terrainOnly := false
	slope := float32(editor.DefaultNVMSlopeThreshold)
	for len(args) > 0 {
		if args[0] == "--terrain-only" {
			terrainOnly = true
			args = args[1:]
			continue
		}
		if strings.HasPrefix(args[0], "--slope=") {
			v, err := strconv.ParseFloat(strings.TrimPrefix(args[0], "--slope="), 32)
			if err != nil {
				fmt.Fprintln(os.Stderr, "bad slope:", err)
				os.Exit(2)
			}
			slope = float32(v)
			args = args[1:]
			continue
		}
		break
	}
	if len(args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: maponlybuild [--terrain-only] <root> <outdir> <region|rx ry> [<region|rx ry>...]")
		os.Exit(2)
	}
	root := args[0]
	outDir := args[1]
	regions, err := parseRegions(args[2:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintln(os.Stderr, "mkdir:", err)
		os.Exit(1)
	}
	srv, err := editor.NewServer(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
	for _, r := range regions {
		var nvm *sromap.NVM
		var walkable int
		if terrainOnly {
			nvm, walkable, err = srv.BuildTerrainOnlyNVMFromMaps(r.x, r.y, slope, false)
		} else {
			nvm, walkable, err = srv.BuildMapOnlyNVMFromMaps(r.x, r.y, slope, true)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "build %s: %v\n", sromap.NVMFileName(r.x, r.y), err)
			os.Exit(1)
		}
		outPath := filepath.Join(outDir, sromap.NVMFileName(r.x, r.y))
		if err := nvm.Save(outPath); err != nil {
			fmt.Fprintf(os.Stderr, "save %s: %v\n", outPath, err)
			os.Exit(1)
		}
		fmt.Printf("%s walkable=%d objects=%d cells=%d open=%d globals=%d internals=%d\n",
			sromap.NVMFileName(r.x, r.y), walkable, len(nvm.Objects), len(nvm.Cells), nvm.OpenCellCount, len(nvm.GlobalEdges), len(nvm.InternalEdges))
	}
}

type region struct {
	x int
	y int
}

func parseRegions(args []string) ([]region, error) {
	var out []region
	for i := 0; i < len(args); {
		if r, ok := parseRegionToken(args[i]); ok {
			out = append(out, r)
			i++
			continue
		}
		if i+1 >= len(args) {
			return nil, fmt.Errorf("missing y coordinate after %q", args[i])
		}
		x, err := strconv.Atoi(args[i])
		if err != nil {
			return nil, fmt.Errorf("bad x coordinate %q", args[i])
		}
		y, err := strconv.Atoi(args[i+1])
		if err != nil {
			return nil, fmt.Errorf("bad y coordinate %q", args[i+1])
		}
		if x < 0 || x > 255 || y < 0 || y > 255 {
			return nil, fmt.Errorf("region out of byte range: %d %d", x, y)
		}
		out = append(out, region{x: x, y: y})
		i += 2
	}
	return out, nil
}

func parseRegionToken(arg string) (region, bool) {
	name := strings.ToLower(filepath.Base(arg))
	name = strings.TrimPrefix(name, "nv_")
	name = strings.TrimSuffix(name, ".nvm")
	if len(name) != 4 {
		return region{}, false
	}
	v, err := strconv.ParseUint(name, 16, 16)
	if err != nil {
		return region{}, false
	}
	return region{x: int(v & 0xff), y: int(v >> 8)}, true
}
