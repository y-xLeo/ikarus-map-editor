package sromap

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type Tile2DEntry struct {
	ID       uint32 `json:"id"`
	TileType uint32 `json:"tileType"`
	Folder   string `json:"folder"`
	Filename string `json:"filename"`
	Grass    string `json:"grass,omitempty"`
}

var tile2dLine = regexp.MustCompile(`^(\d+)\s+(0x[0-9a-fA-F]+)\s+"([^"]*)"\s+"([^"]*)"\s*(.*)$`)

func Tile2DInfoPath(root string) string {
	return filepath.Join(root, "Map", "tile2d.ifo")
}

func Tile2DDir(root string) string {
	return filepath.Join(root, "Map", "Tile2D")
}

func LoadTile2DInfo(path string) (map[uint32]Tile2DEntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1<<14), 1<<20)
	entries := make(map[uint32]Tile2DEntry)
	headerSeen := false
	countSeen := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !headerSeen {
			if !strings.HasPrefix(line, "JMXV2DTI") {
				return nil, fmt.Errorf("invalid tile2d.ifo signature %q", line)
			}
			headerSeen = true
			continue
		}
		if !countSeen {
			countSeen = true
			if _, err := strconv.Atoi(line); err == nil {
				continue
			}
		}
		m := tile2dLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		id, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			continue
		}
		tileType, err := strconv.ParseUint(strings.TrimPrefix(m[2], "0x"), 16, 32)
		if err != nil {
			continue
		}
		entries[uint32(id)] = Tile2DEntry{
			ID:       uint32(id),
			TileType: uint32(tileType),
			Folder:   m[3],
			Filename: m[4],
			Grass:    strings.TrimSpace(m[5]),
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if !headerSeen {
		return nil, fmt.Errorf("empty tile2d.ifo")
	}
	return entries, nil
}

func (e Tile2DEntry) DDJPath(root string) string {
	return filepath.Join(Tile2DDir(root), e.Filename)
}
