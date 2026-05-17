package sromap

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	lightmapSignature = "JMXVMAPT1001"
	// Tile lightmap (96x96 bytes) + 4-byte DDS size + 4-byte mip-count header
	// before the embedded DDS payload starts.
	lightmapDDSOffset = 12 + 96*96 + 4 + 4 // = 9236
)

func LightmapPath(root string, x, y int) string {
	return filepath.Join(root, "Map", fmt.Sprint(y), fmt.Sprintf("%d.t", x))
}

// LoadLightmap returns the high-resolution lightmap (typically 512x512 RGBA)
// embedded in a .t file. Returns os.ErrNotExist if the file is missing.
func LoadLightmap(path string) (*DDJImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < lightmapDDSOffset+ddsHeaderSize {
		return nil, fmt.Errorf("lightmap: file truncated (%d bytes)", len(data))
	}
	if string(data[:12]) != lightmapSignature {
		return nil, fmt.Errorf("lightmap: bad signature %q", string(data[:12]))
	}
	return DecodeDDJ(data[lightmapDDSOffset:])
}
