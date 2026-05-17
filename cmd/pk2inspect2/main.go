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

	"golang.org/x/crypto/blowfish"
)

const (
	pk2HeaderSize      = 256
	pk2BlockSize       = 2560
	pk2EntrySize       = 128
	pk2EntriesPerBlock = 20
	pk2RootBlockOffset = 256
	pk2EntryTypeFolder = 1
	pk2EntryTypeFile   = 2
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

func parse(raw []byte) Entry {
	n := raw[1:82]
	if e := bytes.IndexByte(n, 0); e >= 0 {
		n = n[:e]
	}
	return Entry{
		Type: raw[0],
		Name: string(n),
		Pos:  binary.LittleEndian.Uint64(raw[106:114]),
		Size: binary.LittleEndian.Uint32(raw[114:118]),
		Next: binary.LittleEndian.Uint64(raw[118:126]),
	}
}

func rev(b []byte) { b[0], b[3] = b[3], b[0]; b[1], b[2] = b[2], b[1] }

type Archive struct {
	f *os.File
	b cipher.Block
}

func (a *Archive) decrypt(buf []byte) {
	if a.b == nil {
		return
	}
	block := make([]byte, 8)
	for i := 0; i < len(buf); i += 8 {
		copy(block, buf[i:i+8])
		rev(block[:4])
		rev(block[4:])
		a.b.Decrypt(block, block)
		rev(block[:4])
		rev(block[4:])
		copy(buf[i:i+8], block)
	}
}

func (a *Archive) readBlk(off int64) []byte {
	buf := make([]byte, pk2BlockSize)
	a.f.ReadAt(buf, off)
	a.decrypt(buf)
	return buf
}

func (a *Archive) findInChain(chain int64, name string, wantType byte) *Entry {
	for blk := chain; blk != 0; {
		buf := a.readBlk(blk)
		for i := 0; i < pk2EntriesPerBlock; i++ {
			e := parse(buf[i*pk2EntrySize : (i+1)*pk2EntrySize])
			if e.Type == wantType && strings.EqualFold(e.Name, name) {
				return &e
			}
		}
		last := parse(buf[(pk2EntriesPerBlock-1)*pk2EntrySize : pk2EntriesPerBlock*pk2EntrySize])
		blk = int64(last.Next)
	}
	return nil
}

func (a *Archive) findFile(p string) *Entry {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(p)), func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return nil
	}
	chain := int64(pk2RootBlockOffset)
	for _, x := range parts[:len(parts)-1] {
		f := a.findInChain(chain, x, pk2EntryTypeFolder)
		if f == nil {
			return nil
		}
		chain = int64(f.Pos)
	}
	return a.findInChain(chain, parts[len(parts)-1], pk2EntryTypeFile)
}

func (a *Archive) read(p string) []byte {
	e := a.findFile(p)
	if e == nil {
		return nil
	}
	d := make([]byte, e.Size)
	a.f.ReadAt(d, int64(e.Pos))
	return d
}

func openA(path string) *Archive {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		panic(err)
	}
	h := make([]byte, pk2HeaderSize)
	io.ReadFull(f, h)
	if !bytes.HasPrefix(h[:30], pk2Signature) {
		panic(errors.New("bad sig"))
	}
	a := &Archive{f: f}
	if h[34] != 0 {
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
	a := openA(os.Args[1])
	defer a.f.Close()
	paths := []string{
		"navmesh/nv_5c94.nvm",
		"navmesh/object.ifo",
		"navmesh/objext.ifo",
		"res/custom/test_new_obj/test_new_obj.bsr",
		"res/custom/test_new_obj/test_new_obj.bms",
		"prim/mtrl/custom/test_new_obj/test_new_obj.bmt",
		"res/bldg/oasis/tarim/blackrobber/oas_tarim_rob_smallfire01.bsr",
	}
	for _, p := range paths {
		e := a.findFile(p)
		if e == nil {
			fmt.Printf("MISSING: %s\n", p)
			continue
		}
		fmt.Printf("FOUND  : %s (size=%d)\n", p, e.Size)
		d := a.read(p)
		if strings.HasSuffix(strings.ToLower(p), ".nvm") && len(d) >= 14 {
			cnt := binary.LittleEndian.Uint16(d[12:14])
			fmt.Printf("         NVMObjects=%d\n", cnt)
		}
		if strings.HasSuffix(strings.ToLower(p), "object.ifo") {
			lines := strings.Split(string(d), "\n")
			if len(lines) >= 2 {
				fmt.Printf("         count: %s\n", strings.TrimSpace(lines[1]))
			}
			if len(lines) > 0 {
				last := ""
				for i := len(lines) - 1; i >= 0; i-- {
					if strings.TrimSpace(lines[i]) != "" {
						last = strings.TrimSpace(lines[i])
						break
					}
				}
				fmt.Printf("         last : %s\n", last)
			}
		}
	}
}
