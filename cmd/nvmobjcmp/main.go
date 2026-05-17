package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sromapedit/internal/sromap"
)

type objKey struct {
	region uint16
	uid    int16
	asset  uint32
}

type objInfo struct {
	idx    int
	obj    sromap.NVMObject
	open   int
	closed int
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: nvmobjcmp <base-nvm-or-dir> <test-nvm-or-dir>")
		os.Exit(2)
	}
	pairs, err := collectPairs(os.Args[1], os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, pair := range pairs {
		compare(pair[0], pair[1])
	}
}

func collectPairs(a, b string) ([][2]string, error) {
	ai, err := os.Stat(a)
	if err != nil {
		return nil, err
	}
	bi, err := os.Stat(b)
	if err != nil {
		return nil, err
	}
	if !ai.IsDir() && !bi.IsDir() {
		return [][2]string{{a, b}}, nil
	}
	if !ai.IsDir() || !bi.IsDir() {
		return nil, fmt.Errorf("both args must both be files or both be dirs")
	}
	var pairs [][2]string
	err = filepath.WalkDir(a, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(path), ".nvm") {
			return err
		}
		other := filepath.Join(b, filepath.Base(path))
		if _, err := os.Stat(other); err == nil {
			pairs = append(pairs, [2]string{path, other})
		}
		return nil
	})
	sort.Slice(pairs, func(i, j int) bool { return filepath.Base(pairs[i][0]) < filepath.Base(pairs[j][0]) })
	return pairs, err
}

func compare(basePath, testPath string) {
	base, err := sromap.LoadNVM(basePath)
	if err != nil {
		fmt.Printf("%s: load base: %v\n", filepath.Base(basePath), err)
		return
	}
	test, err := sromap.LoadNVM(testPath)
	if err != nil {
		fmt.Printf("%s: load test: %v\n", filepath.Base(testPath), err)
		return
	}
	bm := objectMap(base)
	tm := objectMap(test)
	keys := make([]objKey, 0, len(bm)+len(tm))
	seen := make(map[objKey]bool, len(bm)+len(tm))
	for k := range bm {
		keys = append(keys, k)
		seen[k] = true
	}
	for k := range tm {
		if !seen[k] {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].region != keys[j].region {
			return keys[i].region < keys[j].region
		}
		if keys[i].uid != keys[j].uid {
			return keys[i].uid < keys[j].uid
		}
		return keys[i].asset < keys[j].asset
	})

	fieldDiffs, cellDiffs, missing, added := 0, 0, 0, 0
	var lines []string
	for _, k := range keys {
		bo, bok := bm[k]
		to, tok := tm[k]
		if !bok {
			added++
			lines = append(lines, fmt.Sprintf("  + test idx=%d asset=%d uid=%d region=0x%04x cells=%d/%d type=%d big=%t struct=%t",
				to.idx, k.asset, k.uid, k.region, to.open, to.closed, to.obj.Type, to.obj.IsBig, to.obj.IsStruct))
			continue
		}
		if !tok {
			missing++
			lines = append(lines, fmt.Sprintf("  - base idx=%d asset=%d uid=%d region=0x%04x cells=%d/%d type=%d big=%t struct=%t",
				bo.idx, k.asset, k.uid, k.region, bo.open, bo.closed, bo.obj.Type, bo.obj.IsBig, bo.obj.IsStruct))
			continue
		}
		field := bo.obj.Type != to.obj.Type || bo.obj.Short0 != to.obj.Short0 || bo.obj.IsBig != to.obj.IsBig ||
			bo.obj.IsStruct != to.obj.IsStruct || len(bo.obj.Links) != len(to.obj.Links)
		cell := bo.open != to.open || bo.closed != to.closed
		if field {
			fieldDiffs++
		}
		if cell {
			cellDiffs++
		}
		if field || cell {
			lines = append(lines, fmt.Sprintf("  * asset=%d uid=%d region=0x%04x baseIdx=%d testIdx=%d type %d/%d short %d/%d big %t/%t struct %t/%t links %d/%d cells open %d/%d closed %d/%d",
				k.asset, k.uid, k.region, bo.idx, to.idx,
				bo.obj.Type, to.obj.Type, bo.obj.Short0, to.obj.Short0,
				bo.obj.IsBig, to.obj.IsBig, bo.obj.IsStruct, to.obj.IsStruct,
				len(bo.obj.Links), len(to.obj.Links), bo.open, to.open, bo.closed, to.closed))
		}
	}
	if fieldDiffs == 0 && cellDiffs == 0 && missing == 0 && added == 0 {
		fmt.Printf("%s: object metadata/cell refs match (%d objects)\n", filepath.Base(basePath), len(base.Objects))
		return
	}
	fmt.Printf("%s: base obj=%d cells=%d/%d test obj=%d cells=%d/%d fieldDiff=%d cellDiff=%d missing=%d added=%d\n",
		filepath.Base(basePath), len(base.Objects), len(base.Cells), base.OpenCellCount,
		len(test.Objects), len(test.Cells), test.OpenCellCount, fieldDiffs, cellDiffs, missing, added)
	for i, line := range lines {
		if i >= 16 {
			fmt.Printf("  ... %d more\n", len(lines)-i)
			break
		}
		fmt.Println(line)
	}
}

func objectMap(nvm *sromap.NVM) map[objKey]objInfo {
	out := make(map[objKey]objInfo, len(nvm.Objects))
	for i, o := range nvm.Objects {
		open, closed := objectCellCounts(nvm, i)
		out[objKey{region: o.RegionID, uid: o.UID, asset: o.AssetID}] = objInfo{
			idx: i, obj: o, open: open, closed: closed,
		}
	}
	return out
}

func objectCellCounts(nvm *sromap.NVM, objIdx int) (open, closed int) {
	for ci, cell := range nvm.Cells {
		for _, idx := range cell.ObjectIndices {
			if int(idx) != objIdx {
				continue
			}
			if uint32(ci) < nvm.OpenCellCount {
				open++
			} else {
				closed++
			}
			break
		}
	}
	return open, closed
}
