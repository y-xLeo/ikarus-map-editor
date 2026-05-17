// patchexe: patch SR_GameServer.exe to skip pNMI-null NVMObject iterations
// instead of asserting and crashing.
//
// At file offset 0x7DAA44, replaces the 5-byte sequence that starts the
// assertion (`push pNMI_string_addr`) with a 5-byte `jmp 0xc5ab81` (the
// loop-continue point reached when other validation conditions fail).
//
// Verifies the old bytes before patching so we don't corrupt an unexpected
// binary. Writes the patched file alongside the original with a .patched
// suffix; the user can then rename / replace as they wish.
package main

import (
	"bytes"
	"fmt"
	"os"
)

const (
	patchOffset = 0x7DAA44
	oldBytesHex = "68 b8 b0 e3 00" // push 0xe3b0b8 (start of assertion)
	newBytesHex = "e9 38 01 00 00" // jmp 0xc5ab81 (loop continue)
)

func parseHex(s string) []byte {
	var out []byte
	var b byte
	half := 0
	for _, c := range s {
		switch {
		case c == ' ':
			continue
		case c >= '0' && c <= '9':
			b = b<<4 | byte(c-'0')
		case c >= 'a' && c <= 'f':
			b = b<<4 | byte(c-'a'+10)
		case c >= 'A' && c <= 'F':
			b = b<<4 | byte(c-'A'+10)
		}
		half++
		if half == 2 {
			out = append(out, b)
			b = 0
			half = 0
		}
	}
	return out
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: patchexe <SR_GameServer.exe>")
		os.Exit(2)
	}
	path := os.Args[1]
	raw, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	oldB := parseHex(oldBytesHex)
	newB := parseHex(newBytesHex)
	if len(oldB) != len(newB) {
		panic("old/new byte length mismatch")
	}

	if patchOffset+len(oldB) > len(raw) {
		panic("patch offset past end of file")
	}

	got := raw[patchOffset : patchOffset+len(oldB)]
	if !bytes.Equal(got, oldB) {
		fmt.Printf("ABORT: bytes at 0x%x don't match expected.\n", patchOffset)
		fmt.Printf("  expected: % x\n", oldB)
		fmt.Printf("  found   : % x\n", got)
		os.Exit(1)
	}

	// Backup
	backupPath := path + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := os.WriteFile(backupPath, raw, 0644); err != nil {
			panic(err)
		}
		fmt.Printf("Backup created: %s\n", backupPath)
	}

	// Apply patch
	patched := make([]byte, len(raw))
	copy(patched, raw)
	copy(patched[patchOffset:], newB)

	outPath := path + ".patched"
	if err := os.WriteFile(outPath, patched, 0644); err != nil {
		panic(err)
	}

	fmt.Printf("Patched file written: %s\n", outPath)
	fmt.Printf("  offset 0x%x: % x -> % x\n", patchOffset, oldB, newB)
	fmt.Println()
	fmt.Println("To apply: rename .patched to .exe (after backing up the original).")
}
