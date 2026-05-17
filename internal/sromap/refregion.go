package sromap

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf16"
)

type RefRegionEntry struct {
	WRegionID     int    `json:"wRegionID"`
	X             int    `json:"x"`
	Z             int    `json:"z"`
	ContinentName string `json:"continentName"`
	AreaName      string `json:"areaName"`
	Climate       int    `json:"climate"`
	MaxCapacity   int    `json:"maxCapacity"`
	AssocObjID    int    `json:"assocObjID"`
	AssocServer   int    `json:"assocServer"`
	AssocFile256  string `json:"assocFile256"`
}

func LoadRefRegions(path string) (map[int]RefRegionEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := decodeText(data)
	entries := make(map[int]RefRegionEntry)
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 11 {
			cols = strings.Fields(line)
		}
		if len(cols) < 11 {
			continue
		}
		id, ok := atoi(cols[0])
		if !ok {
			continue
		}
		x, _ := atoi(cols[1])
		z, _ := atoi(cols[2])
		climate, _ := atoi(cols[6])
		maxCapacity, _ := atoi(cols[7])
		assocObjID, _ := atoi(cols[8])
		assocServer, _ := atoi(cols[9])
		entries[id] = RefRegionEntry{
			WRegionID: id, X: x, Z: z,
			ContinentName: cols[3],
			AreaName:      cols[4],
			Climate:       climate,
			MaxCapacity:   maxCapacity,
			AssocObjID:    assocObjID,
			AssocServer:   assocServer,
			AssocFile256:  cols[10],
		}
	}
	return entries, nil
}

func decodeText(data []byte) string {
	if len(data) >= 2 && data[0] == 0xff && data[1] == 0xfe {
		u16 := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			u16 = append(u16, uint16(data[i])|uint16(data[i+1])<<8)
		}
		return string(utf16.Decode(u16))
	}
	if len(data) >= 2 && data[0] == 0xfe && data[1] == 0xff {
		u16 := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			u16 = append(u16, uint16(data[i])<<8|uint16(data[i+1]))
		}
		return string(utf16.Decode(u16))
	}
	return string(bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf}))
}

func atoi(s string) (int, bool) {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	return v, err == nil
}

func RefRegionPath(root string) string {
	return filepath.Join(root, "Media", "server_dep", "silkroad", "textdata", "refregion.txt")
}
