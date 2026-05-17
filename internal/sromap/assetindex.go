package sromap

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AssetIndex builds a case-insensitive lookup table for files under a root.
// BSR/BMS/BMT files often reference paths with different casing than the
// extracted PK2 files on disk, so we normalize for lookup.
type AssetIndex struct {
	Root  string
	once  sync.Once
	mu    sync.RWMutex
	files map[string]string
}

func NewAssetIndex(root string) *AssetIndex {
	return &AssetIndex{Root: root, files: make(map[string]string)}
}

func (a *AssetIndex) build() {
	_ = filepath.WalkDir(a.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(a.Root, path)
		if err != nil {
			return nil
		}
		key := normalizeAssetKey(rel)
		a.files[key] = path
		return nil
	})
}

func (a *AssetIndex) ensure() {
	a.once.Do(func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.build()
	})
}

// Refresh discards the existing index and rebuilds from disk. Call after
// writing new asset files (e.g. custom-object export) so subsequent
// Resolve() calls can find them.
func (a *AssetIndex) Refresh() {
	a.mu.Lock()
	a.files = make(map[string]string)
	a.build()
	a.once = sync.Once{}
	a.mu.Unlock()
}

// Resolve takes a path as it appears in a BSR/BMT/CPD/object.ifo entry
// (often using backslashes and arbitrary case) and returns the absolute
// path of the file on disk, or empty if not found.
func (a *AssetIndex) Resolve(p string) string {
	a.ensure()
	key := normalizeAssetKey(p)
	a.mu.RLock()
	defer a.mu.RUnlock()
	if v, ok := a.files[key]; ok {
		return v
	}
	if v, ok := a.files["data/"+key]; ok {
		return v
	}
	// fallback: lookup by basename if path didn't match exactly
	base := filepath.Base(key)
	for k, v := range a.files {
		if strings.HasSuffix(k, "/"+base) || k == base {
			return v
		}
	}
	return ""
}

func normalizeAssetKey(p string) string {
	s := strings.ReplaceAll(p, "\\", "/")
	s = strings.TrimPrefix(s, "./")
	s = strings.TrimPrefix(s, "/")
	return strings.ToLower(s)
}
