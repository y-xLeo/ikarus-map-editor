// nvmverify is the definitive correctness check for the default-mode NVM
// rebuild. It loads a real NVM, applies a no-op tile-flag pass (every tile's
// walkability matches its current flag), saves, and byte-diffs against the
// original. Default mode promises "only flips per-tile blocked bits"; any
// other byte changing is a bug.
package main

import (
	"fmt"
	"os"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: nvmverify <real-nvm>")
		os.Exit(2)
	}
	path := os.Args[1]
	orig, err := os.ReadFile(path)
	if err != nil {
		die("read:", err)
	}
	n, err := sromap.LoadNVM(path)
	if err != nil {
		die("load:", err)
	}

	// Build a walkability array that matches the CURRENT tile flags exactly.
	// The blocked bit is bit 0 (value 1) of Tile.Flag; walkable = !blocked.
	var walk [sromap.NVMTotalTiles]bool
	for i := range walk {
		walk[i] = (n.Tiles[i].Flag & 1) == 0
	}
	sromap.ApplyNVMTileFlags(n, walk)

	tmp := path + ".roundtrip"
	if err := n.Save(tmp); err != nil {
		die("save:", err)
	}
	defer os.Remove(tmp)
	out, err := os.ReadFile(tmp)
	if err != nil {
		die("read tmp:", err)
	}

	fmt.Printf("orig: %d bytes\n", len(orig))
	fmt.Printf("ours: %d bytes\n", len(out))
	if len(orig) != len(out) {
		fmt.Printf("SIZE MISMATCH: %d bytes off\n", len(out)-len(orig))
	}
	maxLen := len(orig)
	if len(out) > maxLen {
		maxLen = len(out)
	}
	diffs := 0
	firstDiff := -1
	for i := 0; i < maxLen; i++ {
		var a, b byte
		if i < len(orig) {
			a = orig[i]
		}
		if i < len(out) {
			b = out[i]
		}
		if a != b {
			if firstDiff < 0 {
				firstDiff = i
			}
			diffs++
			if diffs <= 20 {
				fmt.Printf("  0x%X: orig=%02x ours=%02x\n", i, a, b)
			}
		}
	}
	fmt.Printf("total diffs: %d (first at 0x%X)\n", diffs, firstDiff)
}

func die(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
	os.Exit(1)
}
