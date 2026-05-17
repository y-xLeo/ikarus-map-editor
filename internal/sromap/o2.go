package sromap

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
)

const o2Signature = "JMXVMAPO1001"

type ObjectEntry struct {
	ObjID    uint32  `json:"objID"`
	UID      int16   `json:"uid"`
	RegionID uint16  `json:"regionID"`
	X        float32 `json:"x"`
	Y        float32 `json:"y"`
	Z        float32 `json:"z"`
	Yaw      float32 `json:"yaw"`
	Static   int16   `json:"static"`
	Short0   int16   `json:"short0"`
	Big      bool    `json:"big"`
	Struct   bool    `json:"struct"`
	XBlock   int     `json:"xBlock"`
	ZBlock   int     `json:"zBlock"`
	LODGroup int     `json:"lodGroup"`
}

type O2 struct {
	Path    string
	Entries []ObjectEntry
}

func LoadO2(path string) (*O2, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 || string(data[:12]) != o2Signature {
		return nil, fmt.Errorf("invalid O2 signature")
	}
	offset := 12
	readU16 := func() (uint16, error) {
		if offset+2 > len(data) {
			return 0, fmt.Errorf("unexpected EOF at offset %d", offset)
		}
		v := binary.LittleEndian.Uint16(data[offset : offset+2])
		offset += 2
		return v, nil
	}
	readI16 := func() (int16, error) {
		v, err := readU16()
		return int16(v), err
	}
	readU32 := func() (uint32, error) {
		if offset+4 > len(data) {
			return 0, fmt.Errorf("unexpected EOF at offset %d", offset)
		}
		v := binary.LittleEndian.Uint32(data[offset : offset+4])
		offset += 4
		return v, nil
	}
	readF32 := func() (float32, error) {
		v, err := readU32()
		return math.Float32frombits(v), err
	}
	readU8 := func() (byte, error) {
		if offset+1 > len(data) {
			return 0, fmt.Errorf("unexpected EOF at offset %d", offset)
		}
		v := data[offset]
		offset++
		return v, nil
	}

	o := &O2{Path: path}
	for zBlock := 0; zBlock < MeshBlockCount; zBlock++ {
		for xBlock := 0; xBlock < MeshBlockCount; xBlock++ {
			for lodGroup := 0; lodGroup < 4; lodGroup++ {
				count, err := readU16()
				if err != nil {
					return nil, err
				}
				for i := 0; i < int(count); i++ {
					objID, err := readU32()
					if err != nil {
						return nil, err
					}
					x, err := readF32()
					if err != nil {
						return nil, err
					}
					y, err := readF32()
					if err != nil {
						return nil, err
					}
					z, err := readF32()
					if err != nil {
						return nil, err
					}
					static, err := readI16()
					if err != nil {
						return nil, err
					}
					yaw, err := readF32()
					if err != nil {
						return nil, err
					}
					uid, err := readI16()
					if err != nil {
						return nil, err
					}
					short0, err := readI16()
					if err != nil {
						return nil, err
					}
					big, err := readU8()
					if err != nil {
						return nil, err
					}
					isStruct, err := readU8()
					if err != nil {
						return nil, err
					}
					regionID, err := readU16()
					if err != nil {
						return nil, err
					}
					o.Entries = append(o.Entries, ObjectEntry{
						ObjID: objID, UID: uid, RegionID: regionID,
						X: x, Y: y, Z: z, Yaw: yaw,
						Static: static, Short0: short0, Big: big != 0, Struct: isStruct != 0,
						XBlock: xBlock, ZBlock: zBlock, LODGroup: lodGroup,
					})
				}
			}
		}
	}
	if offset != len(data) {
		return nil, fmt.Errorf("O2 parsed %d bytes but file has %d", offset, len(data))
	}
	return o, nil
}

type ObjectEdit struct {
	ObjID    uint32  `json:"objID"`
	UID      int16   `json:"uid"`
	RegionID uint16  `json:"regionID"`
	X        float32 `json:"x"`
	Y        float32 `json:"y"`
	Z        float32 `json:"z"`
	Yaw      float32 `json:"yaw"`
}

type ObjectKey struct {
	ObjID    uint32 `json:"objID"`
	UID      int16  `json:"uid"`
	RegionID uint16 `json:"regionID"`
}

type ObjectAdd struct {
	ObjID    uint32  `json:"objID"`
	RegionID uint16  `json:"regionID"`
	X        float32 `json:"x"`
	Y        float32 `json:"y"`
	Z        float32 `json:"z"`
	Yaw      float32 `json:"yaw"`
	IsBig    bool    `json:"big"`
	IsStruct bool    `json:"struct"`
}

type AddedObject struct {
	ObjID    uint32 `json:"objID"`
	UID      int16  `json:"uid"`
	RegionID uint16 `json:"regionID"`
}

type ChangeResult struct {
	Updated int           `json:"updated"`
	Deleted int           `json:"deleted"`
	Added   []AddedObject `json:"added"`
}

// ApplyEdits updates every entry matching (RegionID, UID, ObjID). A single
// logical object can have multiple host-block entries; those entries are the
// map-side visibility/culling buckets, so moving an object must move the bucket
// footprint too. Returns the number of original entries touched.
func (o *O2) ApplyEdits(edits []ObjectEdit) int {
	updated := 0
	for _, edit := range edits {
		var matches []ObjectEntry
		kept := o.Entries[:0]
		for _, e := range o.Entries {
			if e.RegionID == edit.RegionID && e.UID == edit.UID && e.ObjID == edit.ObjID {
				matches = append(matches, e)
				continue
			}
			kept = append(kept, e)
		}
		if len(matches) == 0 {
			o.Entries = kept
			continue
		}
		oldHostX, oldHostZ := hostBlockOf(matches[0].X, matches[0].Z)
		newHostX, newHostZ := hostBlockOf(edit.X, edit.Z)
		template := matches[0]
		template.X = edit.X
		template.Y = edit.Y
		template.Z = edit.Z
		template.Yaw = edit.Yaw

		type blockPattern struct {
			dx, dz int
			lod    int
		}
		patterns := make([]blockPattern, 0, len(matches))
		seenPattern := make(map[blockPattern]bool, len(matches))
		for _, e := range matches {
			p := blockPattern{dx: e.XBlock - oldHostX, dz: e.ZBlock - oldHostZ, lod: e.LODGroup}
			if seenPattern[p] {
				continue
			}
			seenPattern[p] = true
			patterns = append(patterns, p)
		}
		if len(patterns) == 0 {
			patterns = append(patterns, blockPattern{lod: template.LODGroup})
		}

		type blockKey struct {
			x, z, lod int
		}
		seenBlock := make(map[blockKey]bool, len(patterns))
		for _, p := range patterns {
			bx := clampBlock(newHostX + p.dx)
			bz := clampBlock(newHostZ + p.dz)
			k := blockKey{x: bx, z: bz, lod: p.lod}
			if seenBlock[k] {
				continue
			}
			seenBlock[k] = true
			e := template
			e.XBlock = bx
			e.ZBlock = bz
			e.LODGroup = p.lod
			kept = append(kept, e)
		}
		o.Entries = kept
		updated += len(matches)
	}
	return updated
}

// ApplyDeletes removes every entry matching one of the supplied keys. Multiple
// host-block entries for the same logical object are all removed together.
func (o *O2) ApplyDeletes(keys []ObjectKey) int {
	if len(keys) == 0 {
		return 0
	}
	matchSet := make(map[ObjectKey]struct{}, len(keys))
	for _, k := range keys {
		matchSet[k] = struct{}{}
	}
	kept := o.Entries[:0]
	deleted := 0
	for _, e := range o.Entries {
		k := ObjectKey{ObjID: e.ObjID, UID: e.UID, RegionID: e.RegionID}
		if _, ok := matchSet[k]; ok {
			deleted++
			continue
		}
		kept = append(kept, e)
	}
	o.Entries = kept
	return deleted
}

// ApplyAdds inserts new entries. Each ObjectAdd becomes either 1 entry (regular
// objects) or 9 entries in a 3x3 host-block neighborhood (when IsBig is true),
// matching the reference editor's placement strategy. Returns the assigned
// UID for every added object so the caller can update its in-memory state.
func (o *O2) ApplyAdds(adds []ObjectAdd) []AddedObject {
	if len(adds) == 0 {
		return nil
	}
	// Cache next-uid per region so multiple adds in one call get unique IDs.
	nextUID := make(map[uint16]int16)
	for _, e := range o.Entries {
		if e.UID > nextUID[e.RegionID] {
			nextUID[e.RegionID] = e.UID
		}
	}

	added := make([]AddedObject, 0, len(adds))
	for _, a := range adds {
		nextUID[a.RegionID]++
		uid := nextUID[a.RegionID]
		hostX, hostZ := hostBlockOf(a.X, a.Z)
		base := ObjectEntry{
			ObjID: a.ObjID, UID: uid, RegionID: a.RegionID,
			X: a.X, Y: a.Y, Z: a.Z, Yaw: a.Yaw,
			Static: -1, Short0: 0,
			Big:    a.IsBig,
			Struct: a.IsStruct,
		}
		if a.IsBig {
			seenBlock := make(map[[2]int]bool, 9)
			for dz := -1; dz <= 1; dz++ {
				for dx := -1; dx <= 1; dx++ {
					bx := clampBlock(hostX + dx)
					bz := clampBlock(hostZ + dz)
					k := [2]int{bx, bz}
					if seenBlock[k] {
						continue
					}
					seenBlock[k] = true
					e := base
					e.XBlock = bx
					e.ZBlock = bz
					e.LODGroup = 2
					o.Entries = append(o.Entries, e)
				}
			}
		} else {
			e := base
			e.XBlock = hostX
			e.ZBlock = hostZ
			e.LODGroup = 2
			o.Entries = append(o.Entries, e)
		}
		added = append(added, AddedObject{ObjID: a.ObjID, UID: uid, RegionID: a.RegionID})
	}
	return added
}

// ApplyChanges combines edits, deletes, and adds into a single pass.
// Order: deletes are applied first (so an edit + delete on the same key acts
// like a delete), then edits, then adds (so new entries don't accidentally
// match a delete key).
func (o *O2) ApplyChanges(edits []ObjectEdit, deletes []ObjectKey, adds []ObjectAdd) ChangeResult {
	res := ChangeResult{}
	res.Deleted = o.ApplyDeletes(deletes)
	res.Updated = o.ApplyEdits(edits)
	res.Added = o.ApplyAdds(adds)
	return res
}

func hostBlockOf(x, z float32) (int, int) {
	return clampBlock(int(x / 320)), clampBlock(int(z / 320))
}

func clampBlock(b int) int {
	if b < 0 {
		return 0
	}
	if b > MeshBlockCount-1 {
		return MeshBlockCount - 1
	}
	return b
}

func (o *O2) Save(path string) error {
	type chunkKey struct {
		Z, X, LOD int
	}
	chunks := make(map[chunkKey][]ObjectEntry, MeshBlockCount*MeshBlockCount*4)
	for _, e := range o.Entries {
		k := chunkKey{Z: e.ZBlock, X: e.XBlock, LOD: e.LODGroup}
		chunks[k] = append(chunks[k], e)
	}

	totalEntries := len(o.Entries)
	buf := make([]byte, 0, 12+totalEntries*30+MeshBlockCount*MeshBlockCount*4*2)
	buf = append(buf, []byte(o2Signature)...)

	for zBlock := 0; zBlock < MeshBlockCount; zBlock++ {
		for xBlock := 0; xBlock < MeshBlockCount; xBlock++ {
			for lodGroup := 0; lodGroup < 4; lodGroup++ {
				entries := chunks[chunkKey{Z: zBlock, X: xBlock, LOD: lodGroup}]
				buf = appendU16LE(buf, uint16(len(entries)))
				for _, e := range entries {
					buf = appendEntry(buf, e)
				}
			}
		}
	}
	return os.WriteFile(path, buf, 0644)
}

func appendU16LE(buf []byte, v uint16) []byte {
	return append(buf, byte(v), byte(v>>8))
}

func appendU32LE(buf []byte, v uint32) []byte {
	return append(buf, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func appendF32LE(buf []byte, v float32) []byte {
	return appendU32LE(buf, math.Float32bits(v))
}

func appendEntry(buf []byte, e ObjectEntry) []byte {
	buf = appendU32LE(buf, e.ObjID)
	buf = appendF32LE(buf, e.X)
	buf = appendF32LE(buf, e.Y)
	buf = appendF32LE(buf, e.Z)
	buf = appendU16LE(buf, uint16(e.Static))
	buf = appendF32LE(buf, e.Yaw)
	buf = appendU16LE(buf, uint16(e.UID))
	buf = appendU16LE(buf, uint16(e.Short0))
	if e.Big {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	if e.Struct {
		buf = append(buf, 1)
	} else {
		buf = append(buf, 0)
	}
	buf = appendU16LE(buf, e.RegionID)
	return buf
}
