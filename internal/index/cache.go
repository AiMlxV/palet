package index

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Thitipong-PP/palet/internal/config"
)

// cachedEntry stores one custom config file's hash + parsed result.
type cachedEntry struct {
	Hash  string       `json:"hash"`
	Group config.Group `json:"group"`
}

// cacheFile maps absolute filepath → cachedEntry.
// Each file is tracked independently so only changed files get re-parsed.
type cacheFile struct {
	Files map[string]cachedEntry `json:"files"`
}

// LoadCached loads custom configs file-by-file, using the cached version for any
// file whose hash hasn't changed, and automatically indexes all system commands.
func LoadCached() []Entry {
	path := cachePath()
	cf := loadCacheFile(path)

	var groups []config.Group
	dirty := false

	for _, absPath := range config.Files() {
		hash := hashFile(absPath)

		// cache hit — reuse without parsing
		if cached, ok := cf.Files[absPath]; ok && cached.Hash == hash {
			groups = append(groups, cached.Group)
			continue
		}

		// cache miss — parse and update cache entry
		g, err := config.ParseFile(absPath)
		if err != nil {
			continue
		}
		cf.Files[absPath] = cachedEntry{Hash: hash, Group: g}
		groups = append(groups, g)
		dirty = true
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

	var entries []Entry
	// Automatically map and search all system commands
	if !strings.EqualFold(os.Getenv("PALET_SYSTEM"), "off") {
		entries = append(entries, systemEntries()...)
	}
	// Append user custom commands
	entries = append(entries, Build(groups)...)
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
