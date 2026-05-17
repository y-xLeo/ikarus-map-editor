package sromap

import (
	"encoding/binary"
	"fmt"
	"os"
)

type Region struct {
	ID        uint16
	X         int
	Y         int
	IsDungeon bool
}

type MapInfo struct {
	Path       string
	Width      uint16
	Height     uint16
	Unknowns   [4]int16
	RegionData []byte
}

func LoadMapInfo(path string) (*MapInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) != MFOSize {
		return nil, fmt.Errorf("invalid mapinfo size: got %d, want %d", len(data), MFOSize)
	}
	if string(data[:12]) != mfoSignature {
		return nil, fmt.Errorf("invalid mapinfo signature %q", string(data[:12]))
	}

	return &MapInfo{
		Path:   path,
		Width:  binary.LittleEndian.Uint16(data[12:14]),
		Height: binary.LittleEndian.Uint16(data[14:16]),
		Unknowns: [4]int16{
			int16(binary.LittleEndian.Uint16(data[16:18])),
			int16(binary.LittleEndian.Uint16(data[18:20])),
			int16(binary.LittleEndian.Uint16(data[20:22])),
			int16(binary.LittleEndian.Uint16(data[22:24])),
		},
		RegionData: append([]byte(nil), data[24:24+8192]...),
	}, nil
}

func (m *MapInfo) HasRegion(x, y int) bool {
	if x < 0 || x > 255 || y < 0 || y > 255 {
		return false
	}
	id := y<<8 | x
	b := m.RegionData[id/8]
	mask := byte(1 << (7 - (id % 8)))
	return b&mask != 0
}

func (m *MapInfo) ActiveRegions() []Region {
	regions := make([]Region, 0, 1024)
	for id := 0; id < 65536; id++ {
		if m.RegionData[id/8]&(1<<(7-(id%8))) == 0 {
			continue
		}
		regions = append(regions, Region{
			ID:        uint16(id),
			X:         id & 0xff,
			Y:         (id >> 8) & 0x7f,
			IsDungeon: ((id >> 15) & 1) != 0,
		})
	}
	return regions
}

// SetRegion flips the bit for region (x, y) in RegionData. Returns true if
// the bit changed (i.e. the region was newly activated / deactivated).
func (m *MapInfo) SetRegion(x, y int, active bool) bool {
	if x < 0 || x > 255 || y < 0 || y > 255 {
		return false
	}
	id := y<<8 | x
	idx := id / 8
	mask := byte(1 << (7 - (id % 8)))
	cur := m.RegionData[idx]&mask != 0
	if cur == active {
		return false
	}
	if active {
		m.RegionData[idx] |= mask
	} else {
		m.RegionData[idx] &^= mask
	}
	return true
}

// Save rewrites the .mfo on disk with the current header + RegionData. The
// remaining bytes (after the 8192-byte region bitmap) are preserved verbatim
// from the original file.
func (m *MapInfo) Save(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if len(data) != MFOSize {
		return fmt.Errorf("mapinfo on disk has wrong size %d, want %d", len(data), MFOSize)
	}
	copy(data[24:24+8192], m.RegionData)
	return os.WriteFile(path, data, 0644)
}

func (m *MapInfo) Bounds() (minX, maxX, minY, maxY int, ok bool) {
	minX, minY = 1<<30, 1<<30
	maxX, maxY = -1, -1
	for _, r := range m.ActiveRegions() {
		if r.IsDungeon {
			continue
		}
		if r.X < minX {
			minX = r.X
		}
		if r.X > maxX {
			maxX = r.X
		}
		if r.Y < minY {
			minY = r.Y
		}
		if r.Y > maxY {
			maxY = r.Y
		}
		ok = true
	}
	return minX, maxX, minY, maxY, ok
}
