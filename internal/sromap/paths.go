package sromap

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func MeshPath(root string, x, y int) string {
	return filepath.Join(root, "Map", fmt.Sprint(y), fmt.Sprintf("%d.m", x))
}

func O2Path(root string, x, y int) string {
	return filepath.Join(root, "Map", fmt.Sprint(y), fmt.Sprintf("%d.o2", x))
}

func ObjectInfoPath(root string) string {
	return filepath.Join(root, "Map", "object.ifo")
}

func MapInfoPath(root string) string {
	return filepath.Join(root, "Map", "mapinfo.mfo")
}

func NVMFileName(x, y int) string {
	return fmt.Sprintf("nv_%04x.nvm", y<<8|x)
}

func CandidateNVMPaths(root string, x, y int) []string {
	name := NVMFileName(x, y)
	return []string{
		filepath.Join(root, "SR_GameServer", "Data", "navmesh", name),
		filepath.Join(root, "Data", "Navmesh", name),
		filepath.Join(root, "Data", "navmesh", name),
		filepath.Join(root, "changed", "Data", "Navmesh", name),
		filepath.Join(root, "changed", "Data", "navmesh", name),
	}
}

func ExistingNVMPaths(root string, x, y int) []string {
	var paths []string
	seen := map[string]struct{}{}
	for _, p := range CandidateNVMPaths(root, x, y) {
		key := strings.ToLower(filepath.Clean(p))
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if _, err := os.Stat(p); err == nil {
			paths = append(paths, p)
		}
	}
	return paths
}
