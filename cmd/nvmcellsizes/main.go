package main

import (
	"fmt"
	"os"
	"sort"

	"sromapedit/internal/sromap"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: nvmcellsizes <nvm>")
		os.Exit(2)
	}
	nvm, err := sromap.LoadNVM(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "load nvm:", err)
		os.Exit(1)
	}
	open := int(nvm.OpenCellCount)
	if open > len(nvm.Cells) {
		open = len(nvm.Cells)
	}
	dump("open", nvm.Cells[:open])
	dump("closed", nvm.Cells[open:])
}

func dump(label string, cells []sromap.NVMCell) {
	counts := map[string]int{}
	areaCounts := map[int]int{}
	for _, c := range cells {
		w := int(c.MaxX - c.MinX)
		h := int(c.MaxZ - c.MinZ)
		counts[fmt.Sprintf("%dx%d", w, h)]++
		areaCounts[w*h]++
	}
	fmt.Printf("%s cells=%d\n", label, len(cells))
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return counts[keys[i]] > counts[keys[j]]
	})
	for i, k := range keys {
		if i >= 20 {
			break
		}
		fmt.Printf("  %s: %d\n", k, counts[k])
	}
	areas := make([]int, 0, len(areaCounts))
	for a := range areaCounts {
		areas = append(areas, a)
	}
	sort.Ints(areas)
	fmt.Print("  areas:")
	for i, a := range areas {
		if i >= 20 {
			break
		}
		fmt.Printf(" %d:%d", a, areaCounts[a])
	}
	fmt.Println()
}
