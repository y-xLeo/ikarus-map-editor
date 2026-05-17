package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"sromapedit/internal/editor"
	"sromapedit/internal/sromap"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		usage(stdout)
		return nil
	}
	switch args[0] {
	case "info":
		return cmdInfo(args[1:], stdout)
	case "inspect":
		return cmdInspect(args[1:], stdout)
	case "raise":
		return cmdRaise(args[1:], stdout)
	case "serve":
		return cmdServe(args[1:], stdout)
	case "help", "-h", "--help":
		usage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func cmdInfo(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "exported PK2 root")
	if err := fs.Parse(args); err != nil {
		return err
	}

	mfo, err := sromap.LoadMapInfo(sromap.MapInfoPath(*root))
	if err != nil {
		return err
	}
	regions := mfo.ActiveRegions()
	minX, maxX, minY, maxY, ok := mfo.Bounds()
	fmt.Fprintf(out, "mapinfo: %s\n", mfo.Path)
	fmt.Fprintf(out, "size: %dx%d active=%d\n", mfo.Width, mfo.Height, len(regions))
	if ok {
		fmt.Fprintf(out, "field bounds: x=%d..%d y=%d..%d center=%d,%d\n", minX, maxX, minY, maxY, (minX+maxX)/2, (minY+maxY)/2)
	}

	serverNavmesh := filepath.Join(*root, "SR_GameServer", "Data", "navmesh")
	if count, err := countFiles(serverNavmesh, ".nvm"); err == nil {
		fmt.Fprintf(out, "server navmesh: %s (%d .nvm files)\n", serverNavmesh, count)
	}
	return nil
}

func cmdServe(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "exported PK2 root")
	addr := fs.String("addr", "127.0.0.1:18080", "HTTP listen address")
	if err := fs.Parse(args); err != nil {
		return err
	}
	server, err := editor.NewServer(*root)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "SRO map editor serving %s\n", server.Root)
	fmt.Fprintf(out, "Open http://%s/\n", *addr)
	return http.ListenAndServe(*addr, server.Handler())
}

func cmdInspect(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "exported PK2 root")
	region := fs.String("region", "", "region as x,y")
	meshFile := fs.String("mesh", "", "direct .m path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	x, y, hasRegion, err := regionOrPath(*region, *meshFile)
	if err != nil {
		return err
	}
	path := *meshFile
	if path == "" {
		path = sromap.MeshPath(*root, x, y)
	}
	mesh, err := sromap.LoadMesh(path)
	if err != nil {
		return err
	}
	stats := mesh.Stats()
	fmt.Fprintf(out, "mesh: %s\n", path)
	fmt.Fprintf(out, "height range: %.3f..%.3f\n", stats.MinHeight, stats.MaxHeight)
	fmt.Fprintf(out, "vertices: stored=%d unique=%d extraBytes=%d\n", stats.StoredVertices, stats.UniqueVertices, stats.ExtraBytes)

	if hasRegion {
		paths := sromap.ExistingNVMPaths(*root, x, y)
		if len(paths) == 0 {
			fmt.Fprintf(out, "nvm: not found for region %d,%d (%s)\n", x, y, sromap.NVMFileName(x, y))
		}
		for _, p := range paths {
			nvm, err := sromap.LoadNVM(p)
			if err != nil {
				fmt.Fprintf(out, "nvm: %s (%v)\n", p, err)
				continue
			}
			ns := nvm.Stats()
			fmt.Fprintf(out, "nvm: %s height range %.3f..%.3f cells=%d openCells=%d edges=%d/%d\n",
				p, ns.MinHeight, ns.MaxHeight, len(nvm.Cells), nvm.OpenCellCount,
				len(nvm.InternalEdges), len(nvm.GlobalEdges))
		}
	}
	return nil
}

func cmdRaise(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("raise", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	root := fs.String("root", ".", "exported PK2 root")
	region := fs.String("region", "", "region as x,y")
	meshFile := fs.String("mesh", "", "direct .m path")
	outFile := fs.String("out", "", "write edited mesh to this path instead of source")
	delta := fs.Float64("delta", 0, "height delta; negative lowers terrain")
	all := fs.Bool("all", false, "edit the whole region")
	cx := fs.Float64("cx", sromap.RegionSize/2, "brush center X in local region units")
	cz := fs.Float64("cz", sromap.RegionSize/2, "brush center Z in local region units")
	radius := fs.Float64("radius", 160, "brush radius in local region units")
	falloff := fs.String("falloff", "smooth", "none, linear, or smooth")
	write := fs.Bool("write", false, "overwrite the source mesh when -out is not set")
	backup := fs.Bool("backup", true, "create .bak before overwriting source mesh/NVM")
	syncNVM := fs.Bool("sync-nvm", false, "also apply the same height brush to existing NVM heightmaps")
	if err := fs.Parse(args); err != nil {
		return err
	}

	x, y, hasRegion, err := regionOrPath(*region, *meshFile)
	if err != nil {
		return err
	}
	path := *meshFile
	if path == "" {
		path = sromap.MeshPath(*root, x, y)
	}
	brush := sromap.Brush{
		All:     *all,
		CenterX: *cx,
		CenterZ: *cz,
		Radius:  *radius,
		Delta:   float32(*delta),
		Falloff: *falloff,
	}

	mesh, err := sromap.LoadMesh(path)
	if err != nil {
		return err
	}
	report, err := mesh.ApplyBrush(brush)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "mesh: %s\n", path)
	fmt.Fprintf(out, "changed vertices: stored=%d unique=%d\n", report.StoredVerticesChanged, report.UniqueVerticesChanged)
	fmt.Fprintf(out, "height range: %.3f..%.3f -> %.3f..%.3f\n", report.MinHeightBefore, report.MaxHeightBefore, report.MinHeightAfter, report.MaxHeightAfter)

	writePath := *outFile
	if writePath == "" && *write {
		writePath = path
	}
	if writePath == "" {
		fmt.Fprintln(out, "dry-run: pass -write to overwrite or -out <path> to write a copy")
		return nil
	}
	if *syncNVM && !samePath(writePath, path) {
		return fmt.Errorf("-sync-nvm is only allowed when overwriting the source mesh with -write")
	}
	if samePath(writePath, path) && *backup {
		if err := copyFile(path, path+".bak"); err != nil {
			return fmt.Errorf("backup mesh: %w", err)
		}
		fmt.Fprintf(out, "backup: %s\n", path+".bak")
	}
	if err := mesh.Save(writePath); err != nil {
		return err
	}
	fmt.Fprintf(out, "wrote mesh: %s\n", writePath)

	if *syncNVM {
		if !hasRegion {
			return fmt.Errorf("-sync-nvm requires -region x,y")
		}
		for _, nvmPath := range sromap.ExistingNVMPaths(*root, x, y) {
			nvm, err := sromap.LoadNVM(nvmPath)
			if err != nil {
				return fmt.Errorf("load NVM %s: %w", nvmPath, err)
			}
			changed, err := nvm.ApplyBrush(brush)
			if err != nil {
				return err
			}
			if *backup {
				if err := copyFile(nvmPath, nvmPath+".bak"); err != nil {
					return fmt.Errorf("backup NVM: %w", err)
				}
			}
			if err := nvm.Save(nvmPath); err != nil {
				return fmt.Errorf("write NVM %s: %w", nvmPath, err)
			}
			fmt.Fprintf(out, "wrote NVM heightmap: %s (%d vertices)\n", nvmPath, changed)
		}
	}
	return nil
}

func regionOrPath(region, meshPath string) (int, int, bool, error) {
	if region == "" {
		if meshPath != "" {
			return 0, 0, false, nil
		}
		return 0, 0, false, fmt.Errorf("pass -region x,y or -mesh path")
	}
	parts := strings.Split(region, ",")
	if len(parts) != 2 {
		return 0, 0, false, fmt.Errorf("region must be x,y")
	}
	x, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid region x: %w", err)
	}
	y, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid region y: %w", err)
	}
	if x < 0 || x > 255 || y < 0 || y > 127 {
		return 0, 0, false, fmt.Errorf("region out of range: %d,%d", x, y)
	}
	return x, y, true, nil
}

func countFiles(dir, ext string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ext) {
			count++
		}
	}
	return count, nil
}

func samePath(a, b string) bool {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return strings.EqualFold(filepath.Clean(aa), filepath.Clean(bb))
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func usage(out io.Writer) {
	fmt.Fprintln(out, "sromapedit - Silkroad map terrain editor")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Commands:")
	fmt.Fprintln(out, "  info    -root <ExportedPK2>")
	fmt.Fprintln(out, "  inspect -root <ExportedPK2> -region <x,y>")
	fmt.Fprintln(out, "  raise   -root <ExportedPK2> -region <x,y> -delta <height> [-all | -cx 960 -cz 960 -radius 160] [-write]")
	fmt.Fprintln(out, "  serve   -root <ExportedPK2> [-addr 127.0.0.1:18080]")
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Examples:")
	fmt.Fprintln(out, "  sromapedit info -root D:\\ExportedPK2")
	fmt.Fprintln(out, "  sromapedit inspect -root D:\\ExportedPK2 -region 100,100")
	fmt.Fprintln(out, "  sromapedit raise -root D:\\ExportedPK2 -region 100,100 -delta 5 -cx 960 -cz 960 -radius 200")
	fmt.Fprintln(out, "  sromapedit raise -root D:\\ExportedPK2 -region 100,100 -delta -2 -all -write -sync-nvm")
	fmt.Fprintln(out, "  sromapedit serve -root D:\\ExportedPK2")
}
