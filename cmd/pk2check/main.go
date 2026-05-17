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

var pk2Signature = []byte{0x4a, 0x6f, 0x79, 0x4d, 0x61, 0x78, 0x20, 0x46, 0x69, 0x6c, 0x65, 0x20, 0x4d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x21, 0x0a}
var pk2Salt = []byte{0x03, 0xF8, 0xE4, 0x44, 0x88, 0x99, 0x3F, 0x64, 0xFE, 0x35}

type E struct {
	Type byte
	Name string
	Pos  uint64
	Size uint32
	Next uint64
}

func parseE(r []byte) E {
	n := r[1:82]
	if e := bytes.IndexByte(n, 0); e >= 0 {
		n = n[:e]
	}
	return E{
		Type: r[0],
		Name: string(n),
		Pos:  binary.LittleEndian.Uint64(r[106:114]),
		Size: binary.LittleEndian.Uint32(r[114:118]),
		Next: binary.LittleEndian.Uint64(r[118:126]),
	}
}

func rev(b []byte) { b[0], b[3] = b[3], b[0]; b[1], b[2] = b[2], b[1] }

type Archive struct {
	f *os.File
	b cipher.Block
}

func (a *Archive) dec(buf []byte) {
	if a.b == nil {
		return
	}
	blk := make([]byte, 8)
	for i := 0; i < len(buf); i += 8 {
		copy(blk, buf[i:i+8])
		rev(blk[:4])
		rev(blk[4:])
		a.b.Decrypt(blk, blk)
		rev(blk[:4])
		rev(blk[4:])
		copy(buf[i:i+8], blk)
	}
}

func (a *Archive) readBlk(off int64) []byte {
	buf := make([]byte, pk2BlockSize)
	a.f.ReadAt(buf, off)
	a.dec(buf)
	return buf
}

func (a *Archive) findInChain(chain int64, name string, want byte) *E {
	for b := chain; b != 0; {
		buf := a.readBlk(b)
		for i := 0; i < pk2EntriesPerBlock; i++ {
			e := parseE(buf[i*pk2EntrySize : (i+1)*pk2EntrySize])
			if e.Type == want && strings.EqualFold(e.Name, name) {
				return &e
			}
		}
		last := parseE(buf[(pk2EntriesPerBlock-1)*pk2EntrySize : pk2EntriesPerBlock*pk2EntrySize])
		b = int64(last.Next)
	}
	return nil
}

func (a *Archive) findFile(path string) *E {
	parts := strings.FieldsFunc(strings.ToLower(path), func(r rune) bool { return r == '/' || r == '\\' })
	chain := int64(pk2RootBlockOffset)
	for _, p := range parts[:len(parts)-1] {
		f := a.findInChain(chain, p, pk2EntryTypeFolder)
		if f == nil {
			return nil
		}
		chain = int64(f.Pos)
	}
	return a.findInChain(chain, parts[len(parts)-1], pk2EntryTypeFile)
}

func main() {
	f, _ := os.OpenFile(os.Args[1], os.O_RDONLY, 0)
	h := make([]byte, pk2HeaderSize)
	io.ReadFull(f, h)
	if !bytes.HasPrefix(h[:30], pk2Signature) {
		panic(errors.New("bad sig"))
	}
	a := &Archive{f: f}
	if h[34] != 0 {
		key := []byte("169841")
		d := make([]byte, len(key))
		copy(d, key)
		base := make([]byte, 56)
		copy(base, pk2Salt)
		for i := range d {
			d[i] ^= base[i]
		}
		blk, _ := blowfish.NewCipher(d)
		a.b = blk
	}
	for _, path := range os.Args[2:] {
		// Try both folder and file lookup
		eFile := a.findFile(path)
		// findFolder: same logic but want folder
		parts := strings.FieldsFunc(strings.ToLower(path), func(r rune) bool { return r == '/' || r == '\\' })
		chain := int64(pk2RootBlockOffset)
		var eFolder *E
		for i, p := range parts {
			want := byte(pk2EntryTypeFolder)
			f := a.findInChain(chain, p, want)
			if f == nil {
				eFolder = nil
				break
			}
			if i == len(parts)-1 {
				eFolder = f
				break
			}
			chain = int64(f.Pos)
		}
		if eFile != nil {
			fmt.Printf("FILE FOUND  : %s (size=%d)\n", path, eFile.Size)
		} else if eFolder != nil {
			fmt.Printf("FOLDER FOUND: %s\n", path)
		} else {
			fmt.Printf("MISSING     : %s\n", path)
		}
	}
}
