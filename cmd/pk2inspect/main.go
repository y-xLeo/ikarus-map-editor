// pk2inspect: read specific files out of a Silkroad Data.pk2 archive and
// report sizes/headers so we can verify the local client has the right
// modified files.
//
// usage: pk2inspect <pk2-path>
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
	defaultKey            = "169841"
	pk2HeaderSize         = 256
	pk2BlockSize          = 2560
	pk2EntrySize          = 128
	pk2EntriesPerBlock    = 20
	pk2RootBlockOffset    = 256
	pk2HeaderSignatureLen = 30
	pk2HeaderEncryptedAt  = 34
	pk2HeaderChecksumAt   = 35
	pk2EntryTypeEmpty     = 0
	pk2EntryTypeFolder    = 1
	pk2EntryTypeFile      = 2
)

var (
	pk2Signature = []byte("JoyMax File Manager!\n")
	pk2Checksum  = []byte("Joymax Pack File")
	pk2Salt      = []byte{0x03, 0xF8, 0xE4, 0x44, 0x88, 0x99, 0x3F, 0x64, 0xFE, 0x35}
)

type pk2Entry struct {
	Type     byte
	Name     string
	Position uint64
	Size     uint32
	Next     uint64
}

type pk2Archive struct {
	f   *os.File
	enc bool
	b   cipher.Block
}

func parseEntry(raw []byte) pk2Entry {
	n := raw[1:82]
	if e := bytes.IndexByte(n, 0); e >= 0 {
		n = n[:e]
	}
	return pk2Entry{
		Type:     raw[0],
		Name:     string(n),
		Position: binary.LittleEndian.Uint64(raw[106:114]),
		Size:     binary.LittleEndian.Uint32(raw[114:118]),
		Next:     binary.LittleEndian.Uint64(raw[118:126]),
	}
}

func reverseDword(b []byte) {
	b[0], b[3] = b[3], b[0]
	b[1], b[2] = b[2], b[1]
}

func (a *pk2Archive) decrypt(buf []byte) {
	if !a.enc {
		return
	}
	block := make([]byte, 8)
	for i := 0; i < len(buf); i += 8 {
		copy(block, buf[i:i+8])
		reverseDword(block[:4])
		reverseDword(block[4:])
		a.b.Decrypt(block, block)
		reverseDword(block[:4])
		reverseDword(block[4:])
		copy(buf[i:i+8], block)
	}
}

func (a *pk2Archive) readBlock(off int64) []byte {
	buf := make([]byte, pk2BlockSize)
	a.f.ReadAt(buf, off)
	a.decrypt(buf)
	return buf
}

func (a *pk2Archive) findInChain(chain int64, name string, wantType byte) *pk2Entry {
	for blk := chain; blk != 0; {
		buf := a.readBlock(blk)
		for i := 0; i < pk2EntriesPerBlock; i++ {
			e := parseEntry(buf[i*pk2EntrySize : (i+1)*pk2EntrySize])
			if e.Type == wantType && strings.EqualFold(e.Name, name) {
				return &e
			}
		}
		last := parseEntry(buf[(pk2EntriesPerBlock-1)*pk2EntrySize : pk2EntriesPerBlock*pk2EntrySize])
		blk = int64(last.Next)
	}
	return nil
}

func (a *pk2Archive) findFile(path string) *pk2Entry {
	parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(path)), func(r rune) bool {
		return r == '/' || r == '\\'
	})
	if len(parts) == 0 {
		return nil
	}
	chain := int64(pk2RootBlockOffset)
	for _, p := range parts[:len(parts)-1] {
		f := a.findInChain(chain, p, pk2EntryTypeFolder)
		if f == nil {
			return nil
		}
		chain = int64(f.Position)
	}
	return a.findInChain(chain, parts[len(parts)-1], pk2EntryTypeFile)
}

func (a *pk2Archive) readFile(path string) ([]byte, error) {
	e := a.findFile(path)
	if e == nil {
		return nil, errors.New("not found")
	}
	d := make([]byte, e.Size)
	if _, err := a.f.ReadAt(d, int64(e.Position)); err != nil {
		return nil, err
	}
	return d, nil
}

func openPK2(path string) (*pk2Archive, error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	h := make([]byte, pk2HeaderSize)
	if _, err := io.ReadFull(f, h); err != nil {
		f.Close()
		return nil, err
	}
	if !bytes.HasPrefix(h[:pk2HeaderSignatureLen], pk2Signature) {
		f.Close()
		return nil, errors.New("invalid signature")
	}
	a := &pk2Archive{f: f, enc: h[pk2HeaderEncryptedAt] != 0}
	if a.enc {
		key := []byte(defaultKey)
		if len(key) > 56 {
			key = key[:56]
		}
		derived := make([]byte, len(key))
		copy(derived, key)
		base := make([]byte, 56)
		copy(base, pk2Salt)
		for i := range derived {
			derived[i] ^= base[i]
		}
		blk, err := blowfish.NewCipher(derived)
		if err != nil {
			f.Close()
			return nil, err
		}
		a.b = blk
	}
	return a, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: pk2inspect <pk2>")
		os.Exit(2)
	}
	a, err := openPK2(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer a.f.Close()

	check := func(path string) {
		e := a.findFile(path)
		if e == nil {
			fmt.Printf("  MISSING: %s\n", path)
			return
		}
		fmt.Printf("  FOUND  : %s (size=%d)\n", path, e.Size)
		data, _ := a.readFile(path)
		if len(data) >= 16 {
			fmt.Printf("           first 16 bytes: % x\n", data[:16])
		}
		// For .nvm files report NVMObject count
		if strings.HasSuffix(strings.ToLower(path), ".nvm") && len(data) >= 14 {
			cnt := binary.LittleEndian.Uint16(data[12:14])
			fmt.Printf("           NVMObjects: %d\n", cnt)
		}
		// For object.ifo, report count line + last entry
		if strings.HasSuffix(strings.ToLower(path), "object.ifo") {
			lines := strings.Split(string(data), "\n")
			if len(lines) >= 2 {
				fmt.Printf("           count line: %s\n", strings.TrimSpace(lines[1]))
			}
			if len(lines) >= 3 {
				lastNonEmpty := ""
				for i := len(lines) - 1; i >= 0; i-- {
					l := strings.TrimSpace(lines[i])
					if l != "" {
						lastNonEmpty = l
						break
					}
				}
				fmt.Printf("           last entry: %s\n", lastNonEmpty)
			}
		}
	}

	fmt.Println("=== Local Data.pk2 inspection ===")
	check("data/navmesh/nv_5c94.nvm")
	check("data/navmesh/object.ifo")
	check("data/navmesh/objext.ifo")
	check("data/res/custom/test_new_obj/test_new_obj.bsr")
	check("data/res/custom/test_new_obj/test_new_obj.bms")
	check("data/prim/mtrl/custom/test_new_obj/test_new_obj.bmt")
	check("data/res/bldg/oasis/tarim/blackrobber/oas_tarim_rob_smallfire01.bsr")
}
