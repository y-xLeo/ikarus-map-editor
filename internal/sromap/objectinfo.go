package sromap

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type ObjectInfo struct {
	ID    uint32 `json:"id"`
	Flags uint32 `json:"flags"`
	Path  string `json:"path"`
	IsCPD bool   `json:"isCpd"`
}

var objectInfoLine = regexp.MustCompile(`^(\d+)\s+(0x[0-9a-fA-F]+)\s+"(.+)"$`)

func LoadObjectInfo(path string) (map[uint32]ObjectInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	text := sanitizeASCII(data)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	nonEmpty := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			nonEmpty = append(nonEmpty, line)
		}
	}
	if len(nonEmpty) == 0 || !strings.HasPrefix(strings.TrimSpace(nonEmpty[0]), "JMXVOBJI") {
		return nil, fmt.Errorf("invalid object.ifo signature")
	}
	objects := make(map[uint32]ObjectInfo)
	for _, line := range nonEmpty[2:] {
		m := objectInfoLine.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil {
			continue
		}
		id64, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			continue
		}
		flags64, err := strconv.ParseUint(m[2][2:], 16, 32)
		if err != nil {
			continue
		}
		p := strings.ToLower(strings.ReplaceAll(m[3], "\\", "/"))
		objects[uint32(id64)] = ObjectInfo{
			ID:    uint32(id64),
			Flags: uint32(flags64),
			Path:  p,
			IsCPD: strings.HasSuffix(p, ".cpd"),
		}
	}
	return objects, nil
}

func sanitizeASCII(data []byte) string {
	var b strings.Builder
	b.Grow(len(data))
	for _, c := range data {
		if c < 128 {
			b.WriteByte(c)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
