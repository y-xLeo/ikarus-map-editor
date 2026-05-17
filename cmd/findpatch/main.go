package main

import (
	"debug/pe"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

func main() {
	f, err := pe.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer f.Close()
	var imgBase uint64
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		imgBase = uint64(oh.ImageBase)
	case *pe.OptionalHeader64:
		imgBase = oh.ImageBase
	}
	fmt.Printf("ImageBase: 0x%x\n", imgBase)
	raw, _ := os.ReadFile(os.Args[1])
	var pNMIFileOff int = -1
	for off := 0; off+5 <= len(raw); off++ {
		if string(raw[off:off+5]) == "pNMI\x00" {
			pNMIFileOff = off
			break
		}
	}
	if pNMIFileOff < 0 {
		fmt.Println("pNMI not found")
		return
	}
	var pNMIVA uint32
	for _, s := range f.Sections {
		if uint32(pNMIFileOff) >= s.Offset && uint32(pNMIFileOff) < s.Offset+s.Size {
			pNMIVA = uint32(imgBase) + uint32(pNMIFileOff) - s.Offset + s.VirtualAddress
			fmt.Printf("pNMI: fileOff=0x%x section=%s VA=0x%x\n", pNMIFileOff, s.Name, pNMIVA)
			break
		}
	}
	needle := []byte{0x2e, 0x5c, 0x52, 0x54, 0x4e, 0x61, 0x76, 0x4d, 0x65, 0x73, 0x68, 0x54, 0x65, 0x72, 0x72, 0x61, 0x69, 0x6e, 0x2e, 0x63, 0x70, 0x70}
	var fileVA uint32
	for off := 0; off+len(needle) <= len(raw); off++ {
		match := true
		for k := range needle {
			if raw[off+k] != needle[k] {
				match = false
				break
			}
		}
		if match && raw[off+len(needle)] == 0 {
			for _, s := range f.Sections {
				if uint32(off) >= s.Offset && uint32(off) < s.Offset+s.Size {
					fileVA = uint32(imgBase) + uint32(off) - s.Offset + s.VirtualAddress
					fmt.Printf("RTNavMeshTerrain.cpp: fileOff=0x%x VA=0x%x\n", off, fileVA)
					break
				}
			}
			break
		}
	}
	var text *pe.Section
	for _, ts := range f.Sections {
		if ts.Name == ".text" {
			text = ts
			break
		}
	}
	tdata, _ := io.ReadAll(text.Open())
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], pNMIVA)
	fmt.Printf("Searching .text for push pNMIVA (68 % x)\n", buf[:])
	for i := 0; i+5 <= len(tdata); i++ {
		if tdata[i] == 0x68 && tdata[i+1] == buf[0] && tdata[i+2] == buf[1] && tdata[i+3] == buf[2] && tdata[i+4] == buf[3] {
			fileOff := int(text.Offset) + i
			fmt.Printf("FOUND push pNMIVA at .text+0x%x (fileOff=0x%x)\n", i, fileOff)
			start := fileOff - 80
			if start < 0 {
				start = 0
			}
			end := fileOff + 48
			if end > len(raw) {
				end = len(raw)
			}
			for j := 0; j < end-start; j += 16 {
				k := start + j
				e := k + 16
				if e > end {
					e = end
				}
				fmt.Printf("  %08x: % x\n", k, raw[k:e])
			}
		}
	}
}
