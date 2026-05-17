package sromap

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// EncodeMinimalBMT writes a one-material BMT with a single diffuse texture.
// No normal map, no specular, no extra channels — the simplest case that
// the game's material loader is likely to accept.
//
// `materialName` must match the `material` value the corresponding BMS
// vertex section references (case-sensitive).
//
// `textureFile` is the relative resource path written into the BMT, e.g.
// "res\\custom\\japanese_house\\diffuse.ddj". Use forward slashes if the
// path is absolute; we set the isAbsolutePath flag accordingly.
func EncodeMinimalBMT(materialName, textureFile string, isAbsolute bool) ([]byte, error) {
	if materialName == "" {
		return nil, fmt.Errorf("bmt encode: materialName is required")
	}
	if textureFile == "" {
		return nil, fmt.Errorf("bmt encode: textureFile is required")
	}
	var buf bytes.Buffer
	buf.WriteString("JMXVBMT 0102") // 12 bytes
	// material count
	var u [4]byte
	binary.LittleEndian.PutUint32(u[:], 1)
	buf.Write(u[:])

	// Material body — values copied from a real diffuse-texture material
	// (cj_ferry.bmt's first material) so the engine sees a fully valid
	// material descriptor. Without these the model loads as untextured grey.
	writeLenString(&buf, materialName)
	// Diffuse RGBA = pure white (no tint over texture).
	for _, f := range [4]float32{1, 1, 1, 1} {
		binary.LittleEndian.PutUint32(u[:], floatBits(f))
		buf.Write(u[:])
	}
	// ambient/specular/emissive matching the real-asset pattern.
	ambient := [4]float32{1, 1, 1, 1}
	spec := [4]float32{0.9, 0.9, 0.9, 1}
	emit := [4]float32{0, 0, 0, 1}
	for _, c := range [][4]float32{ambient, spec, emit} {
		for _, f := range c {
			binary.LittleEndian.PutUint32(u[:], floatBits(f))
			buf.Write(u[:])
		}
	}
	// specPow
	binary.LittleEndian.PutUint32(u[:], floatBits(0))
	buf.Write(u[:])
	// flags — real BMTs use 0x140 (= 0x40 | 0x100). Exact meaning unknown but
	// likely "use diffuse texture" + something else. Zero produces untextured.
	binary.LittleEndian.PutUint32(u[:], 0x00000140)
	buf.Write(u[:])
	// texFile
	writeLenString(&buf, textureFile)
	// texParam = 1.0 (real BMTs; zero rendered black).
	binary.LittleEndian.PutUint32(u[:], floatBits(1.0))
	buf.Write(u[:])
	// unkByte01 = 0x18, unkByte02 = 0 — matched against cj_ferry.bmt.
	buf.WriteByte(0x18)
	buf.WriteByte(0)
	if isAbsolute {
		buf.WriteByte(1)
	} else {
		buf.WriteByte(0)
	}
	return buf.Bytes(), nil
}
