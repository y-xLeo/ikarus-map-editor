// fixupandinject: full clean-up and injection workflow.
//   1. Restore baseline nv_5c94.nvm everywhere
//   2. Re-encode the BSR with prim/mesh path (instead of res/custom)
//   3. Copy the BMS to the new prim/mesh path
//   4. Inject all the right files into local Data.pk2
//
// usage: fixupandinject <root> <pk2-data>
package main

import (
	"bytes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/blowfish"
	"sromapedit/internal/sromap"
)

const (
	pk2HeaderSize         = 256
	pk2BlockSize          = 2560
	pk2EntrySize          = 128
	pk2EntriesPerBlock    = 20
	pk2RootBlockOffset    = 256
	pk2HeaderSignatureLen = 30
	pk2HeaderEncryptedAt  = 34
	pk2AllocationUnit     = 4096
	pk2EntryTypeFolder    = 1
	pk2EntryTypeFile      = 2
)

var (
	pk2Signature = []byte{0x4a, 0x6f, 0x79, 0x4d, 0x61, 0x78, 0x20, 0x46, 0x69, 0x6c, 0x65, 0x20, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x21, 0x0a}
	pk2Salt      = []byte{0x03, 0xF8, 0xE4, 0x44, 0x88, 0x99, 0x3F, 0x64, 0xFE, 0x35}
)

type Entry struct {
	Type byte
	Name string
	Pos  uint64
	Size uint32
	Next uint64
}

type EntryRef struct {
	BlockOff int64
	Idx      int
	Ent      Entry
}

type Archive struct {
	f *os.File
	b cipher.Block
}

func parseE(raw []byte) Entry {
	n := raw[1:82]
	if e := bytes.IndexByte(n, 0); e >= 0 {
		n = n[:e]
	}
	return Entry{
		Type: raw[0], Name: string(n),
		Pos:  binary.LittleEndian.Uint64(raw[106:114]),
		Size: binary.LittleEndian.Uint32(raw[114:118]),
		Next: binary.LittleEndian.Uint64(raw[118:126]),
	}
}

func (e Entry) writeTo(raw []byte) {
	for i := range raw {
		raw[i] = 0
	}
	raw[0] = e.Type
	nm := []byte(e.Name)
	if len(nm) > 80 {
		nm = nm[:80]
	}
	copy(raw[1:1+len(nm)], nm)
	now := uint64(time.Now().UTC().Unix()+11644473600) * 10000000
	binary.LittleEndian.PutUint64(raw[82:90], now)
	binary.LittleEndian.PutUint64(raw[90:98], now)
	binary.LittleEndian.PutUint64(raw[98:106], now)
	binary.LittleEndian.PutUint64(raw[106:114], e.Pos)
	binary.LittleEndian.PutUint32(raw[114:118], e.Size)
	binary.LittleEndian.PutUint64(raw[118:126], e.Next)
}

func rev(b []byte) { b[0], b[3] = b[3], b[0]; b[1], b[2] = b[2], b[1] }

func (a *Archive) crypt(buf []byte, enc bool) {
	if a.b == nil {
		return
	}
	blk := make([]byte, 8)
	for i := 0; i < len(buf); i += 8 {
		copy(blk, buf[i:i+8])
		rev(blk[:4])
		rev(blk[4:])
		if enc {
			a.b.Encrypt(blk, blk)
		} else {
			a.b.Decrypt(blk, blk)
		}
		rev(blk[:4])
		rev(blk[4:])
		copy(buf[i:i+8], blk)
	}
}

func (a *Archive) readBlk(off int64) []byte {
	buf := make([]byte, pk2BlockSize)
	a.f.ReadAt(buf, off)
	a.crypt(buf, false)
	return buf
}

func (a *Archive) writeBlk(buf []byte, off int64) {
	if a.b != nil {
		enc := make([]byte, len(buf))
		copy(enc, buf)
		a.crypt(enc, true)
		a.f.WriteAt(enc, off)
	} else {
		a.f.WriteAt(buf, off)
	}
}

func (a *Archive) findInChain(chain int64, name string, wantType byte) *EntryRef {
	for blk := chain; blk != 0; {
		buf := a.readBlk(blk)
		for i := 0; i < pk2EntriesPerBlock; i++ {
			e := parseE(buf[i*pk2EntrySize : (i+1)*pk2EntrySize])
			if e.Type == wantType && strings.EqualFold(e.Name, name) {
				return &EntryRef{BlockOff: blk, Idx: i, Ent: e}
			}
		}
		last := parseE(buf[(pk2EntriesPerBlock-1)*pk2EntrySize : pk2EntriesPerBlock*pk2EntrySize])
		blk = int64(last.Next)
	}
	return nil
}

func (a *Archive) findOrCreateEmpty(chainOff int64) *EntryRef {
	for blk := chainOff; ; {
		buf := a.readBlk(blk)
		for i := 0; i < pk2EntriesPerBlock; i++ {
			e := parseE(buf[i*pk2EntrySize : (i+1)*pk2EntrySize])
			if e.Type == 0 {
				return &EntryRef{BlockOff: blk, Idx: i, Ent: e}
			}
		}
		last := parseE(buf[(pk2EntriesPerBlock-1)*pk2EntrySize : pk2EntriesPerBlock*pk2EntrySize])
		if last.Next != 0 {
			blk = int64(last.Next)
			continue
		}
		newOff := a.allocEOF(pk2BlockSize)
		last.Next = uint64(newOff)
		lastBuf := buf
		last.writeTo(lastBuf[(pk2EntriesPerBlock-1)*pk2EntrySize : pk2EntriesPerBlock*pk2EntrySize])
		a.writeBlk(lastBuf, blk)
		newBuf := make([]byte, pk2BlockSize)
		a.writeBlk(newBuf, newOff)
		return &EntryRef{BlockOff: newOff, Idx: 0, Ent: Entry{}}
	}
}

func (a *Archive) ensureFolder(chainOff int64, name string) int64 {
	if f := a.findInChain(chainOff, name, pk2EntryTypeFolder); f != nil {
		return int64(f.Ent.Pos)
	}
	// Create new folder
	newBlockOff := a.allocEOF(pk2BlockSize)
	// Initialize empty block
	emptyBlock := make([]byte, pk2BlockSize)
	a.writeBlk(emptyBlock, newBlockOff)
	// Add entry to parent chain
	slot := a.findOrCreateEmpty(chainOff)
	slot.Ent = Entry{Type: pk2EntryTypeFolder, Name: name, Pos: uint64(newBlockOff), Next: slot.Ent.Next}
	buf := a.readBlk(slot.BlockOff)
	slot.Ent.writeTo(buf[slot.Idx*pk2EntrySize : (slot.Idx+1)*pk2EntrySize])
	a.writeBlk(buf, slot.BlockOff)
	return newBlockOff
}

func (a *Archive) allocEOF(size int64) int64 {
	info, _ := a.f.Stat()
	off := info.Size()
	allocated := ((size + pk2AllocationUnit - 1) / pk2AllocationUnit) * pk2AllocationUnit
	a.f.Truncate(off + allocated)
	return off
}

func (a *Archive) injectAt(path string, data []byte) error {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(path)), func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return errors.New("empty path")
	}
	chain := int64(pk2RootBlockOffset)
	for _, p := range parts[:len(parts)-1] {
		chain = a.ensureFolder(chain, p)
	}
	// Find or create file slot
	fileName := parts[len(parts)-1]
	if existing := a.findInChain(chain, fileName, pk2EntryTypeFile); existing != nil {
		writeOff := int64(existing.Ent.Pos)
		if len(data) > int(existing.Ent.Size) {
			writeOff = a.allocEOF(int64(len(data)))
		}
		a.f.WriteAt(data, writeOff)
		existing.Ent.Pos = uint64(writeOff)
		existing.Ent.Size = uint32(len(data))
		buf := a.readBlk(existing.BlockOff)
		existing.Ent.writeTo(buf[existing.Idx*pk2EntrySize : (existing.Idx+1)*pk2EntrySize])
		a.writeBlk(buf, existing.BlockOff)
		return nil
	}
	// New file
	slot := a.findOrCreateEmpty(chain)
	writeOff := a.allocEOF(int64(len(data)))
	a.f.WriteAt(data, writeOff)
	slot.Ent = Entry{Type: pk2EntryTypeFile, Name: fileName, Pos: uint64(writeOff), Size: uint32(len(data)), Next: slot.Ent.Next}
	buf := a.readBlk(slot.BlockOff)
	slot.Ent.writeTo(buf[slot.Idx*pk2EntrySize : (slot.Idx+1)*pk2EntrySize])
	a.writeBlk(buf, slot.BlockOff)
	return nil
}

func openA(path string) *Archive {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	h := make([]byte, pk2HeaderSize)
	io.ReadFull(f, h)
	if !bytes.HasPrefix(h[:pk2HeaderSignatureLen], pk2Signature) {
		panic("bad sig")
	}
	a := &Archive{f: f}
	if h[pk2HeaderEncryptedAt] != 0 {
		key := []byte("169841")
		derived := make([]byte, len(key))
		copy(derived, key)
		base := make([]byte, 56)
		copy(base, pk2Salt)
		for i := range derived {
			derived[i] ^= base[i]
		}
		blk, _ := blowfish.NewCipher(derived)
		a.b = blk
	}
	return a
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("usage: fixupandinject <root-dir> <data.pk2>")
		os.Exit(2)
	}
	root := os.Args[1]
	pk2Path := os.Args[2]

	// 1. Read existing custom BMS, compute bbox
	bmsOldPath := filepath.Join(root, "Data", "res", "custom", "test_new_obj", "test_new_obj.bms")
	bmsData, err := os.ReadFile(bmsOldPath)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Loaded existing BMS: %d bytes\n", len(bmsData))

	// Parse BMS to get bbox (BMS has a known offset for bbox)
	bms, err := sromap.LoadBMS(bmsOldPath)
	if err != nil {
		panic(err)
	}
	fmt.Printf("BMS bbox: %v..%v\n", bms.BBoxMin, bms.BBoxMax)

	// 2. Re-encode BSR with prim/mesh path
	slug := "test_new_obj"
	bmtPath := filepath.Join("prim", "mtrl", "custom", slug, slug+".bmt")
	bmsNewPath := filepath.Join("prim", "mesh", "custom", slug, slug+".bms")
	bsrBytes, err := sromap.EncodeMinimalBSR(slug, bmtPath, bmsNewPath, bmsNewPath, bms.BBoxMin, bms.BBoxMax)
	if err != nil {
		panic(err)
	}
	bsrOutPath := filepath.Join(root, "Data", "res", "custom", slug, slug+".bsr")
	if err := os.WriteFile(bsrOutPath, bsrBytes, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("New BSR written: %d bytes (CollisionMesh -> %s)\n", len(bsrBytes), bmsNewPath)

	// 3. Copy BMS to new location
	bmsNewAbs := filepath.Join(root, "Data", "prim", "mesh", "custom", slug, slug+".bms")
	if err := os.MkdirAll(filepath.Dir(bmsNewAbs), 0755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(bmsNewAbs, bmsData, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("BMS copied to: %s\n", bmsNewAbs)

	// 4. Restore baseline nvm
	bakNvm, err := os.ReadFile(filepath.Join(root, "Data", "Navmesh", "nv_5c94.nvm.bak"))
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Data", "Navmesh", "nv_5c94.nvm"), bakNvm, 0644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(root, "export", "Data", "Navmesh", "nv_5c94.nvm"), bakNvm, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Baseline nv_5c94.nvm restored (%d bytes)\n", len(bakNvm))

	// 5. Open Data.pk2 and inject everything
	a := openA(pk2Path)
	defer a.f.Close()

	bmtData, _ := os.ReadFile(filepath.Join(root, "Data", "prim", "mtrl", "custom", slug, slug+".bmt"))
	ddjData, _ := os.ReadFile(filepath.Join(root, "Data", "prim", "mtrl", "custom", slug, "diffuse.ddj"))
	objIfoData, _ := os.ReadFile(filepath.Join(root, "Data", "Navmesh", "object.ifo"))

	injects := []struct {
		path string
		data []byte
	}{
		{"navmesh/nv_5c94.nvm", bakNvm},
		{"navmesh/object.ifo", objIfoData},
		{"res/custom/" + slug + "/" + slug + ".bsr", bsrBytes},
		{"prim/mesh/custom/" + slug + "/" + slug + ".bms", bmsData},
		{"prim/mtrl/custom/" + slug + "/" + slug + ".bmt", bmtData},
		{"prim/mtrl/custom/" + slug + "/diffuse.ddj", ddjData},
	}
	for _, inj := range injects {
		if len(inj.data) == 0 {
			fmt.Printf("  SKIP (empty): %s\n", inj.path)
			continue
		}
		if err := a.injectAt(inj.path, inj.data); err != nil {
			fmt.Printf("  FAIL: %s -> %v\n", inj.path, err)
			continue
		}
		fmt.Printf("  Injected: %s (%d bytes)\n", inj.path, len(inj.data))
	}
}
