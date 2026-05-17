package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: nvmseams <navmesh-dir> [focus-region...]")
		os.Exit(2)
	}
	focus := parseFocus(os.Args[2:])
	files := filesToLoad(os.Args[1], focus)
	regions := map[[2]int]*sromap.NVM{}
	for _, p := range files {
		r, ok := parseRegion(p)
		if !ok {
			continue
		}
		nvm, err := sromap.LoadNVM(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "load %s: %v\n", p, err)
			os.Exit(1)
		}
		regions[r] = nvm
	}
	keys := make([][2]int, 0, len(regions))
	for k := range regions {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][1] != keys[j][1] {
			return keys[i][1] < keys[j][1]
		}
		return keys[i][0] < keys[j][0]
	})
	totalPairs := 0
	edgeMismatch := 0
	heightMismatch := 0
	planeMismatch := 0
	for _, r := range keys {
		n := regions[r]
		for _, d := range []struct {
			name byte
			dx   int
			dy   int
		}{
			{name: 'E', dx: 1, dy: 0},
			{name: 'N', dx: 0, dy: 1},
		} {
			nr := [2]int{r[0] + d.dx, r[1] + d.dy}
			other := regions[nr]
			if other == nil {
				continue
			}
			totalPairs++
			edgeOK, missingA, missingB := reciprocalEdges(n, other, d.name)
			heightMax := maxHeightDelta(n, other, d.name)
			planeMax, planeTypeDiffs := planeDelta(n, other, d.name)
			if !edgeOK {
				edgeMismatch++
			}
			if heightMax > 0.001 {
				heightMismatch++
			}
			if planeMax > 0.001 || planeTypeDiffs > 0 {
				planeMismatch++
			}
			if len(focus) > 0 && !focus[r] && !focus[nr] {
				continue
			}
			status := "OK"
			if !edgeOK || heightMax > 0.001 || planeMax > 0.001 || planeTypeDiffs > 0 {
				status = "DIFF"
			}
			fmt.Printf("%s nv_%02x%02x-%c-nv_%02x%02x missingLocal=%d missingNeighbor=%d maxHeightDelta=%.4f planeTypeDiffs=%d maxPlaneDelta=%.4f\n",
				status, r[1], r[0], d.name, nr[1], nr[0], missingA, missingB, heightMax, planeTypeDiffs, planeMax)
		}
	}
	fmt.Printf("pairs=%d edgeMismatch=%d heightMismatch=%d planeMismatch=%d\n", totalPairs, edgeMismatch, heightMismatch, planeMismatch)
}

func filesToLoad(dir string, focus map[[2]int]bool) []string {
	if len(focus) == 0 {
		files, err := filepath.Glob(filepath.Join(dir, "nv_*.nvm"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "glob:", err)
			os.Exit(1)
		}
		return files
	}
	load := map[[2]int]bool{}
	for r := range focus {
		load[r] = true
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nr := [2]int{r[0] + d[0], r[1] + d[1]}
			if nr[0] >= 0 && nr[0] <= 255 && nr[1] >= 0 && nr[1] <= 127 {
				load[nr] = true
			}
		}
	}
	files := make([]string, 0, len(load))
	for r := range load {
		p := filepath.Join(dir, sromap.NVMFileName(r[0], r[1]))
		if _, err := os.Stat(p); err == nil {
			files = append(files, p)
		}
	}
	return files
}

func parseFocus(args []string) map[[2]int]bool {
	if len(args) == 0 {
		return nil
	}
	out := map[[2]int]bool{}
	for _, arg := range args {
		name := strings.ToLower(filepath.Base(arg))
		name = strings.TrimPrefix(name, "nv_")
		name = strings.TrimSuffix(name, ".nvm")
		if len(name) != 4 {
			continue
		}
		v, err := strconv.ParseUint(name, 16, 16)
		if err != nil {
			continue
		}
		out[[2]int{int(v & 0xff), int(v >> 8)}] = true
	}
	return out
}

func parseRegion(p string) ([2]int, bool) {
	name := strings.ToLower(filepath.Base(p))
	name = strings.TrimPrefix(name, "nv_")
	name = strings.TrimSuffix(name, ".nvm")
	if len(name) != 4 {
		return [2]int{}, false
	}
	v, err := strconv.ParseUint(name, 16, 16)
	if err != nil {
		return [2]int{}, false
	}
	return [2]int{int(v & 0xff), int(v >> 8)}, true
}

func reciprocalEdges(local, neighbor *sromap.NVM, side byte) (bool, int, int) {
	localSet := seamEdgeSet(local.GlobalEdges, side, false)
	neighborSet := seamEdgeSet(neighbor.GlobalEdges, oppositeSide(side), true)
	missingLocal := countMissing(localSet, neighborSet)
	missingNeighbor := countMissing(neighborSet, localSet)
	return missingLocal == 0 && missingNeighbor == 0, missingLocal, missingNeighbor
}

func seamEdgeSet(edges []sromap.NVMGlobalEdge, side byte, reverseCells bool) map[string]bool {
	out := map[string]bool{}
	for _, e := range edges {
		if !globalEdgeOnSide(e, side) {
			continue
		}
		out[seamKey(e, side, reverseCells)] = true
	}
	return out
}

func globalEdgeOnSide(e sromap.NVMGlobalEdge, side byte) bool {
	const extent = float32(sromap.RegionSize)
	switch side {
	case 'N':
		return e.MinZ == extent && e.MaxZ == extent && e.Dir0 == sromap.NVMDirNorth
	case 'S':
		return e.MinZ == 0 && e.MaxZ == 0 && e.Dir0 == sromap.NVMDirSouth
	case 'E':
		return e.MinX == extent && e.MaxX == extent && e.Dir0 == sromap.NVMDirEast
	case 'W':
		return e.MinX == 0 && e.MaxX == 0 && e.Dir0 == sromap.NVMDirWest
	default:
		return false
	}
}

func seamKey(e sromap.NVMGlobalEdge, side byte, reverseCells bool) string {
	c0, c1 := e.Cell0, e.Cell1
	if reverseCells {
		c0, c1 = c1, c0
	}
	switch side {
	case 'N', 'S':
		return fmt.Sprintf("%.0f..%.0f|%d|%d", e.MinX, e.MaxX, c0, c1)
	case 'E', 'W':
		return fmt.Sprintf("%.0f..%.0f|%d|%d", e.MinZ, e.MaxZ, c0, c1)
	default:
		return ""
	}
}

func countMissing(a, b map[string]bool) int {
	count := 0
	for k := range a {
		if !b[k] {
			count++
		}
	}
	return count
}

func maxHeightDelta(local, neighbor *sromap.NVM, side byte) float64 {
	maxDelta := 0.0
	for i := 0; i < sromap.MeshGridSize; i++ {
		var a, b float32
		switch side {
		case 'N':
			a = local.Heights[(sromap.MeshGridSize-1)*sromap.MeshGridSize+i]
			b = neighbor.Heights[i]
		case 'E':
			a = local.Heights[i*sromap.MeshGridSize+sromap.MeshGridSize-1]
			b = neighbor.Heights[i*sromap.MeshGridSize]
		}
		d := math.Abs(float64(a - b))
		if d > maxDelta {
			maxDelta = d
		}
	}
	return maxDelta
}

func planeDelta(local, neighbor *sromap.NVM, side byte) (float64, int) {
	maxDelta := 0.0
	typeDiffs := 0
	for i := 0; i < sromap.MeshBlockCount; i++ {
		var ai, bi int
		switch side {
		case 'N':
			ai = (sromap.MeshBlockCount-1)*sromap.MeshBlockCount + i
			bi = i
		case 'E':
			ai = i*sromap.MeshBlockCount + sromap.MeshBlockCount - 1
			bi = i * sromap.MeshBlockCount
		default:
			continue
		}
		if local.PlaneType[ai] != neighbor.PlaneType[bi] {
			typeDiffs++
		}
		d := math.Abs(float64(local.PlaneHeight[ai] - neighbor.PlaneHeight[bi]))
		if d > maxDelta {
			maxDelta = d
		}
	}
	return maxDelta, typeDiffs
}

func oppositeSide(side byte) byte {
	switch side {
	case 'N':
		return 'S'
	case 'S':
		return 'N'
	case 'E':
		return 'W'
	case 'W':
		return 'E'
	default:
		return 0
	}
}
