// disasm: disassemble a region of an x86 PE binary either by file offset or VA.
// usage: disasm <exe> <hexOffset> <hexCount> [va]
// If 4th arg "va" is given, the offset is treated as a virtual address.
package main

import (
	"debug/pe"
	"fmt"
	"os"
	"strconv"

	"golang.org/x/arch/x86/x86asm"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("usage: disasm <exe> <hexOffset> <hexCount> [va]")
		os.Exit(2)
	}
	addr, _ := strconv.ParseUint(os.Args[2], 16, 64)
	count, _ := strconv.ParseUint(os.Args[3], 16, 64)
	asVA := len(os.Args) >= 5 && os.Args[4] == "va"

	raw, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}

	f, _ := pe.Open(os.Args[1])
	defer f.Close()
	var imgBase uint64
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		imgBase = uint64(oh.ImageBase)
	case *pe.OptionalHeader64:
		imgBase = oh.ImageBase
	}
	var textRVA, textOff uint32
	for _, s := range f.Sections {
		if s.Name == ".text" {
			textRVA = s.VirtualAddress
			textOff = s.Offset
		}
	}

	vaToFile := func(va uint64) uint64 {
		return va - imgBase - uint64(textRVA) + uint64(textOff)
	}
	fileToVA := func(off uint64) uint64 {
		return off + imgBase + uint64(textRVA) - uint64(textOff)
	}

	var fileOff uint64
	if asVA {
		fileOff = vaToFile(addr)
	} else {
		fileOff = addr
	}

	pos := fileOff
	end := fileOff + count
	for pos < end && pos < uint64(len(raw)) {
		inst, err := x86asm.Decode(raw[pos:], 32)
		if err != nil {
			fmt.Printf("  %08x [%08x]  %02x  <decode error: %v>\n", pos, fileToVA(pos), raw[pos], err)
			pos++
			continue
		}
		bytes := make([]string, inst.Len)
		for i := 0; i < inst.Len; i++ {
			bytes[i] = fmt.Sprintf("%02x", raw[pos+uint64(i)])
		}
		hexCol := ""
		for _, b := range bytes {
			hexCol += b + " "
		}
		for len(hexCol) < 24 {
			hexCol += " "
		}
		fmt.Printf("  %08x [%08x]  %s %s\n", pos, fileToVA(pos), hexCol, x86asm.GNUSyntax(inst, fileToVA(pos), nil))
		pos += uint64(inst.Len)
	}
}
