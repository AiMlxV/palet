package index

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Thitipong-PP/palet/internal/plugin"
)

// cachedEntry stores one plugin file's hash + parsed result.
type cachedEntry struct {
	Hash   string        `json:"hash"`
	Plugin plugin.Plugin `json:"plugin"`
}

// cacheFile maps absolute filepath → cachedEntry.
// Each file is tracked independently so only changed files get re-parsed.
type cacheFile struct {
	Files map[string]cachedEntry `json:"files"`
}

// LoadCached loads plugins file-by-file, using the cached version for any
// file whose hash hasn't changed and re-parsing only the ones that have.
func LoadCached() []Entry {
	path := cachePath()
	cf := loadCacheFile(path)

	var plugins []plugin.Plugin
	dirty := false

	for _, dir := range plugin.Dirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			ext := filepath.Ext(e.Name())
			if ext != ".yaml" && ext != ".yml" {
				continue
			}

			absPath := filepath.Join(dir, e.Name())
			hash := hashFile(absPath)

			// cache hit — reuse without parsing
			if cached, ok := cf.Files[absPath]; ok && cached.Hash == hash {
				plugins = append(plugins, cached.Plugin)
				continue
			}

			// cache miss — parse and update cache entry
			p, err := plugin.ParseFile(absPath)
			if err != nil {
				continue
			}
			cf.Files[absPath] = cachedEntry{Hash: hash, Plugin: p}
			plugins = append(plugins, p)
			dirty = true
		}
	}

	// prune entries for files that no longer exist
	for absPath := range cf.Files {
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			delete(cf.Files, absPath)
			dirty = true
		}
	}

	if dirty {
		_ = writeCacheFile(path, cf) // best-effort
	}

	entries := Build(plugins)
	if strings.EqualFold(os.Getenv("PALET_SYSTEM"), "off") {
		return entries
	}
	entries = append(entries, systemEntries()...)
	return entries
}

// hashFile returns an md5 hex string based on file size + mtime.
// Reading content would be more accurate but size+mtime is fast and
// sufficient for a CLI tool where files are edited by hand.
func hashFile(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	h := md5.New()
	fmt.Fprintf(h, "%d%d", info.Size(), info.ModTime().UnixNano())
	return fmt.Sprintf("%x", h.Sum(nil))
}

func cachePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "palet", "index.json")
	}
	return filepath.Join(os.TempDir(), "palet-index.json")
}

func loadCacheFile(path string) cacheFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheFile{Files: make(map[string]cachedEntry)}
	}
	var cf cacheFile
	if err := json.Unmarshal(data, &cf); err != nil || cf.Files == nil {
		return cacheFile{Files: make(map[string]cachedEntry)}
	}
	return cf
}

func writeCacheFile(path string, cf cacheFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(cf)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}