package sromap

import (
	"encoding/binary"
	"fmt"
	"os"
)

// SaveLightmap writes a .t file with the supplied 512x512 RGBA lightmap as
// the high-resolution DXT1 payload. The tile-lightmap (96x96 bytes) and the
// DDS header are rebuilt to known-good values matching the original game
// format. Returns os.WriteFile errors as-is.
func SaveLightmap(path string, rgba []byte, width, height int, tileLightmap []byte) error {
	if width != 512 || height != 512 {
		return fmt.Errorf("lightmap: only 512x512 supported, got %dx%d", width, height)
	}
	if len(rgba) != width*height*4 {
		return fmt.Errorf("lightmap: rgba buffer must be %d bytes", width*height*4)
	}
	if tileLightmap == nil {
		tileLightmap = make([]byte, 96*96)
		for i := range tileLightmap {
			tileLightmap[i] = 0xFF
		}
	}
	if len(tileLightmap) != 96*96 {
		return fmt.Errorf("lightmap: tile lightmap must be %d bytes", 96*96)
	}

	dxt1, err := EncodeDXT1(rgba, width, height)
	if err != nil {
		return err
	}
	if len(dxt1) != 131072 {
		return fmt.Errorf("lightmap: unexpected DXT1 payload size %d", len(dxt1))
	}

	buf := make([]byte, 0, 140436)
	buf = append(buf, []byte(lightmapSignature)...)
	buf = append(buf, tileLightmap...)

	// uint32 size (matches reference: 131208 = 128 DDS header + 131072 payload + 8)
	buf = appendU32LE(buf, 131208)
	// uint32 mip-count marker (reference writes 3)
	buf = appendU32LE(buf, 3)

	buf = append(buf, 'D', 'D', 'S', ' ')

	header := make([]byte, 124)
	binary.LittleEndian.PutUint32(header[0:4], 124)
	binary.LittleEndian.PutUint32(header[4:8], 0x00021007) // CAPS|HEIGHT|WIDTH|PIXELFORMAT|LINEARSIZE
	binary.LittleEndian.PutUint32(header[8:12], uint32(height))
	binary.LittleEndian.PutUint32(header[12:16], uint32(width))
	binary.LittleEndian.PutUint32(header[16:20], 131072) // pitchOrLinearSize for mip0
	binary.LittleEndian.PutUint32(header[24:28], 10)     // mipMapCount – matches reference even though we only emit mip0
	binary.LittleEndian.PutUint32(header[72:76], 32)     // DDS_PIXELFORMAT.size
	binary.LittleEndian.PutUint32(header[76:80], 4)      // DDPF_FOURCC
	copy(header[80:84], "DXT1")
	binary.LittleEndian.PutUint32(header[104:108], 0x1000) // DDSCAPS_TEXTURE
	buf = append(buf, header...)
	buf = append(buf, dxt1...)

	return os.WriteFile(path, buf, 0644)
}
