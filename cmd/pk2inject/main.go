// pk2inject: replace/insert a file into a Silkroad Data.pk2 archive.
// usage: pk2inject <pk2> <pk2-target-path> <local-file>
package main

import (
	"bytes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/blowfish"
)

const (
	pk2HeaderSize         = 256
	pk2BlockSize          = 2560
	pk2EntrySize          = 128
	pk2EntriesPerBlock    = 20
	pk2RootBlockOffset    = 256
	pk2HeaderSignatureLen = 30
	pk2HeaderEncryptedAt  = 34
	pk2HeaderChecksumAt   = 35
	pk2AllocationUnit     = 4096
	pk2EntryTypeEmpty     = 0
	pk2EntryTypeFolder    = 1
	pk2EntryTypeFile      = 2
)

var (
	pk2Signature = []byte{0x4a, 0x6f, 0x79, 0x4d, 0x61, 0x78, 0x20, 0x46, 0x69, 0x6c, 0x65, 0x20, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x21, 0x0a}
	pk2Checksum  = []byte{0x4a, 0x6f, 0x79, 0x6d, 0x61, 0x78, 0x20, 0x50, 0x61, 0x63, 0x6b, 0x20, 0x46, 0x69, 0x6c, 0x65}
	pk2Salt      = []byte{0x03, 0xF8, 0xE4, 0x44, 0x88, 0x99, 0x3F, 0x64, 0xFE, 0x35}
)

type Entry struct {
	Type     byte
	Name     string
	Position uint64
	Size     uint32
	Next     uint64
	Atime    uint64
	Ctime    uint64
	Mtime    uint64
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

func parse(raw []byte) Entry {
	n := raw[1:82]
	if e := bytes.IndexByte(n, 0); e >= 0 {
		n = n[:e]
	}
	return Entry{
		Type:     raw[0],
		Name:     string(n),
		Atime:    binary.LittleEndian.Uint64(raw[82:90]),
		Ctime:    binary.LittleEndian.Uint64(raw[90:98]),
		Mtime:    binary.LittleEndian.Uint64(raw[98:106]),
		Position: binary.LittleEndian.Uint64(raw[106:114]),
		Size:     binary.LittleEndian.Uint32(raw[114:118]),
		Next:     binary.LittleEndian.Uint64(raw[118:126]),
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
	binary.LittleEndian.PutUint64(raw[82:90], e.Atime)
	binary.LittleEndian.PutUint64(raw[90:98], e.Ctime)
	binary.LittleEndian.PutUint64(raw[98:106], e.Mtime)
	binary.LittleEndian.PutUint64(raw[106:114], e.Position)
	binary.LittleEndian.PutUint32(raw[114:118], e.Size)
	binary.LittleEndian.PutUint64(raw[118:126], e.Next)
}

func rev(b []byte) { b[0], b[3] = b[3], b[0]; b[1], b[2] = b[2], b[1] }

func (a *Archive) crypt(buf []byte, enc bool) {
	if a.b == nil {
		return
	}
	block := make([]byte, 8)
	for i := 0; i < len(buf); i += 8 {
		copy(block, buf[i:i+8])
		rev(block[:4])
		rev(block[4:])
		if enc {
			a.b.Encrypt(block, block)
		} else {
			a.b.Decrypt(block, block)
		}
		rev(block[:4])
		rev(block[4:])
		copy(buf[i:i+8], block)
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
			e := parse(buf[i*pk2EntrySize : (i+1)*pk2EntrySize])
			if e.Type == wantType && strings.EqualFold(e.Name, name) {
				return &EntryRef{BlockOff: blk, Idx: i, Ent: e}
			}
		}
		last := parse(buf[(pk2EntriesPerBlock-1)*pk2EntrySize : pk2EntriesPerBlock*pk2EntrySize])
		blk = int64(last.Next)
	}
	return nil
}

func (a *Archive) findFile(path string) *EntryRef {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(path)), func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return nil
	}
	chain := int64(pk2RootBlockOffset)
	for _, p := range parts[:len(parts)-1] {
		f := a.findInChain(chain, p, pk2EntryTypeFolder)
		if f == nil {
			return nil
		}
		chain = int64(f.Ent.Position)
	}
	return a.findInChain(chain, parts[len(parts)-1], pk2EntryTypeFile)
}

func (a *Archive) allocEOF(size int64) int64 {
	info, _ := a.f.Stat()
	off := info.Size()
	if size <= 0 {
		return off
	}
	allocated := ((size + pk2AllocationUnit - 1) / pk2AllocationUnit) * pk2AllocationUnit
	a.f.Truncate(off + allocated)
	return off
}

func (a *Archive) replace(target string, data []byte) error {
	ref := a.findFile(target)
	if ref == nil {
		return a.createFile(target, data)
	}
	writeOff := int64(ref.Ent.Position)
	if len(data) > int(ref.Ent.Size) {
		writeOff = a.allocEOF(int64(len(data)))
	}
	if len(data) > 0 {
		a.f.WriteAt(data, writeOff)
	}
	ref.Ent.Position = uint64(writeOff)
	ref.Ent.Size = uint32(len(data))
	ref.Ent.Mtime = filetime()
	// Update the entry
	buf := a.readBlk(ref.BlockOff)
	ref.Ent.writeTo(buf[ref.Idx*pk2EntrySize : (ref.Idx+1)*pk2EntrySize])
	a.writeBlk(buf, ref.BlockOff)
	return nil
}

// findOrCreateEmptySlot mirrors pk2.go's findOrCreateEmptyEntry — searches a
// folder's block chain for an empty entry slot; if none, allocates a new
// block at EOF and links it into the chain.
func (a *Archive) findOrCreateEmptySlot(chainOff int64) (*EntryRef, error) {
	for blk := chainOff; ; {
		buf := a.readBlk(blk)
		for i := 0; i < pk2EntriesPerBlock; i++ {
			e := parse(buf[i*pk2EntrySize : (i+1)*pk2EntrySize])
			if e.Type == pk2EntryTypeEmpty {
				return &EntryRef{BlockOff: blk, Idx: i, Ent: e}, nil
			}
		}
		lastStart := (pk2EntriesPerBlock - 1) * pk2EntrySize
		lastEntry := parse(buf[lastStart : lastStart+pk2EntrySize])
		if lastEntry.Next != 0 {
			blk = int64(lastEntry.Next)
			continue
		}
		// Allocate a new block and link it from the current block's tail entry.
		newBlockOff := a.allocEOF(pk2BlockSize)
		lastEntry.Next = uint64(newBlockOff)
		lastEntry.writeTo(buf[lastStart : lastStart+pk2EntrySize])
		a.writeBlk(buf, blk)
		newBlock := make([]byte, pk2BlockSize)
		a.writeBlk(newBlock, newBlockOff)
		return &EntryRef{BlockOff: newBlockOff, Idx: 0, Ent: Entry{}}, nil
	}
}

// createFolder adds a Type=Folder entry under the chain rooted at chainOff
// and allocates a fresh block to hold its children. Returns the new block
// offset, which is also the new folder's child chain.
func (a *Archive) createFolder(chainOff int64, name string) (int64, error) {
	slot, err := a.findOrCreateEmptySlot(chainOff)
	if err != nil {
		return 0, err
	}
	newBlockOff := a.allocEOF(pk2BlockSize)
	emptyBlock := make([]byte, pk2BlockSize)
	a.writeBlk(emptyBlock, newBlockOff)

	now := filetime()
	newEntry := Entry{
		Type:     pk2EntryTypeFolder,
		Name:     name,
		Atime:    now,
		Ctime:    now,
		Mtime:    now,
		Position: uint64(newBlockOff),
		Size:     0,
		Next:     slot.Ent.Next,
	}
	buf := a.readBlk(slot.BlockOff)
	newEntry.writeTo(buf[slot.Idx*pk2EntrySize : (slot.Idx+1)*pk2EntrySize])
	a.writeBlk(buf, slot.BlockOff)
	return newBlockOff, nil
}

func (a *Archive) ensureFolderPath(parts []string) (int64, error) {
	chain := int64(pk2RootBlockOffset)
	for _, p := range parts {
		if f := a.findInChain(chain, p, pk2EntryTypeFolder); f != nil {
			chain = int64(f.Ent.Position)
			continue
		}
		newChain, err := a.createFolder(chain, p)
		if err != nil {
			return 0, err
		}
		chain = newChain
	}
	return chain, nil
}

func (a *Archive) createFile(target string, data []byte) error {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(target)), func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return errors.New("empty path")
	}
	chain, err := a.ensureFolderPath(parts[:len(parts)-1])
	if err != nil {
		return err
	}
	slot, err := a.findOrCreateEmptySlot(chain)
	if err != nil {
		return err
	}
	writeOff := a.allocEOF(int64(len(data)))
	if len(data) > 0 {
		a.f.WriteAt(data, writeOff)
	}
	now := filetime()
	newEntry := Entry{
		Type:     pk2EntryTypeFile,
		Name:     parts[len(parts)-1],
		Atime:    now,
		Ctime:    now,
		Mtime:    now,
		Position: uint64(writeOff),
		Size:     uint32(len(data)),
		Next:     slot.Ent.Next,
	}
	buf := a.readBlk(slot.BlockOff)
	newEntry.writeTo(buf[slot.Idx*pk2EntrySize : (slot.Idx+1)*pk2EntrySize])
	a.writeBlk(buf, slot.BlockOff)
	return nil
}

func filetime() uint64 {
	t := time.Now().UTC()
	return uint64(t.Unix()+11644473600)*10000000 + uint64(t.Nanosecond()/100)
}

func openA(path string) *Archive {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	h := make([]byte, pk2HeaderSize)
	if _, err := io.ReadFull(f, h); err != nil {
		panic(err)
	}
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
		// Validate
		probe := make([]byte, 8)
		copy(probe, pk2Checksum)
		probe2 := make([]byte, 8)
		copy(probe2, probe)
		rev(probe2[:4])
		rev(probe2[4:])
		a.b.Encrypt(probe2, probe2)
		rev(probe2[:4])
		rev(probe2[4:])
		if !bytes.Equal(h[pk2HeaderChecksumAt:pk2HeaderChecksumAt+3], probe2[:3]) {
			panic("bad key")
		}
	}
	return a
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("usage: pk2inject <pk2> <target-path-in-pk2> <local-file>")
		os.Exit(2)
	}
	a := openA(os.Args[1])
	defer a.f.Close()
	data, err := os.ReadFile(os.Args[3])
	if err != nil {
		panic(err)
	}
	if err := a.replace(os.Args[2], data); err != nil {
		panic(err)
	}
	fmt.Printf("Injected %s (%d bytes) into %s at %s\n", os.Args[3], len(data), os.Args[1], os.Args[2])
}
