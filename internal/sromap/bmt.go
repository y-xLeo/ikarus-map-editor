package sromap

import (
	"fmt"
	"os"
	"strings"
)

type BMTMaterial struct {
	Name           string
	Diffuse        [4]float32
	Flags          uint32
	TextureFile    string
	IsAbsolutePath bool
	NormalMapPath  string
}

type BMT struct {
	Path      string
	Materials []BMTMaterial
}

func LoadBMT(path string) (*BMT, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b, err := DecodeBMT(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	b.Path = path
	return b, nil
}

func DecodeBMT(data []byte) (*BMT, error) {
	if len(data) < 12 || !strings.HasPrefix(string(data[:12]), "JMXVBMT") {
		return nil, fmt.Errorf("bmt: bad signature")
	}
	r := NewBinReader(data)
	if err := r.Skip(12); err != nil {
		return nil, err
	}
	count, err := r.U32()
	if err != nil {
		return nil, err
	}
	if count > 1024 {
		return nil, fmt.Errorf("bmt: implausible material count %d", count)
	}
	mats := make([]BMTMaterial, 0, count)
	for i := uint32(0); i < count; i++ {
		name, err := r.LenString()
		if err != nil {
			return nil, err
		}
		var diffuse [4]float32
		for j := 0; j < 4; j++ {
			f, err := r.F32()
			if err != nil {
				return nil, err
			}
			diffuse[j] = f
		}
		// ambient(4) + specular(4) + emissive(4) = 12 × float32
		if err := r.Skip(48); err != nil {
			return nil, err
		}
		if _, err := r.F32(); err != nil { // specPow
			return nil, err
		}
		flags, err := r.U32()
		if err != nil {
			return nil, err
		}
		texFile, err := r.LenString()
		if err != nil {
			return nil, err
		}
		if _, err := r.F32(); err != nil { // texParam
			return nil, err
		}
		if _, err := r.U8(); err != nil { // unkByte01
			return nil, err
		}
		if _, err := r.U8(); err != nil { // unkByte02
			return nil, err
		}
		isAbs, err := r.U8()
		if err != nil {
			return nil, err
		}
		normalMap := ""
		if flags&0x2000 != 0 {
			normalMap, err = r.LenString()
			if err != nil {
				return nil, err
			}
			if _, err := r.U32(); err != nil {
				return nil, err
			}
		}
		mats = append(mats, BMTMaterial{
			Name:           name,
			Diffuse:        diffuse,
			Flags:          flags,
			TextureFile:    texFile,
			IsAbsolutePath: isAbs != 0,
			NormalMapPath:  normalMap,
		})
	}
	return &BMT{Materials: mats}, nil
}
