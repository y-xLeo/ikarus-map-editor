// bmshdr prints the raw header of a BMS file so we can compare what real
// game assets put in the fields our parser skips ("unknown" / mystery bytes).
package main

import (
	"encoding/binary"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: bmshdr <path.bms>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		die("read:", err)
	}
	if len(data) < 96 {
		die("file too small")
	}
	fmt.Printf("signature: %q\n", string(data[:12]))
	off := 12
	fmt.Println("file offsets (7):")
	for i := 0; i < 7; i++ {
		v := binary.LittleEndian.Uint32(data[off : off+4])
		fmt.Printf("  [%d] = 0x%X (%d)\n", i, v, v)
		off += 4
	}
	navMesh := binary.LittleEndian.Uint32(data[off : off+4])
	fmt.Printf("navMesh offset: 0x%X\n", navMesh)
	off += 4
	skNav := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	unk9 := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	fmt.Printf("skinnedNavMesh: 0x%X, unknown09: 0x%X\n", skNav, unk9)
	unk0 := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	navFlag := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	subPrim := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	vFlag := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	unk2 := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	fmt.Printf("unkUInt0=0x%X, navFlag=0x%X, subPrimCount=%d\n", unk0, navFlag, subPrim)
	fmt.Printf("vertexFlag=0x%X (binary %032b)\n", vFlag, vFlag)
	fmt.Printf("unkUInt2=0x%X\n", unk2)

	// LenString for name
	nameLen := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	fmt.Printf("name(%d) = %q\n", nameLen, string(data[off:off+int(nameLen)]))
	off += int(nameLen)
	matLen := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	fmt.Printf("material(%d) = %q\n", matLen, string(data[off:off+int(matLen)]))
	off += int(matLen)
	postMat := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	fmt.Printf("post-material u32 = 0x%X (likely texturePathFlag or similar)\n", postMat)
	vertexCount := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	fmt.Printf("vertexCount = %d\n", vertexCount)
	fmt.Printf("first vertex bytes start at 0x%X\n", off)
	// Dump first vertex worth of bytes
	stride := 44
	if vFlag&0x400 != 0 {
		stride += 8
	}
	if vFlag&0x800 != 0 {
		stride += 40 + 8 - 12
	}
	if off+stride > len(data) {
		stride = len(data) - off
	}
	fmt.Printf("first %d bytes (one vertex at this flag): ", stride)
	for i := 0; i < stride; i++ {
		fmt.Printf("%02x", data[off+i])
		if i%4 == 3 {
			fmt.Print(" ")
		}
	}
	fmt.Println()
}

func die(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
