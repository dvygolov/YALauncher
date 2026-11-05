package index

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Cache is the on-disk copy of the index. Flow Launcher keeps a binary cache
// of its program list and only rescans when the cache is empty or older than
// 30 hours; the same idea applies here, with the file watched folders in
// between: a JSON snapshot of a few thousand short records loads in a couple
// of milliseconds and a scan is only needed when the cache is missing.
type Cache struct {
	Version int    `json:"version"`
	BuiltAt int64  `json:"builtAt"`
	Items   []Item `json:"items"`
}

const cacheVersion = 1

// Save writes the snapshot atomically (write to .tmp, then rename).
func (ix *Index) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	c := Cache{Version: cacheVersion, BuiltAt: time.Now().Unix()}
	c.Items = make([]Item, 0, len(ix.items))
	for _, it := range ix.items {
		c.Items = append(c.Items, *it)
	}
	raw, err := json.Marshal(&c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// LoadCache reads a snapshot. It returns nil when the file is missing, older
// than maxAge or written by another version of the format.
func LoadCache(path string, maxAge time.Duration) *Index {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c Cache
	if err := json.Unmarshal(raw, &c); err != nil || c.Version != cacheVersion {
		return nil
	}
	if time.Since(time.Unix(c.BuiltAt, 0)) > maxAge {
		return nil
	}
	ix := New()
	ix.BuiltAt = c.BuiltAt
	ix.items = make([]*Item, 0, len(c.Items))
	for i := range c.Items {
		it := c.Items[i]
		ix.items = append(ix.items, &it)
	}
	ix.byName = ix.items
	return ix
}
