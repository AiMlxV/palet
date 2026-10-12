package completion

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Result holds the completion results for a given path prefix.
type Result struct {
	Prefix       string   // Original query prefix
	Dir          string   // Directory that was searched
	Candidates   []string // Matching relative paths or filenames
	CommonPrefix string   // Longest common prefix among candidates (if > 1)
}

// CompletePath returns directory and file matches based on the typed prefix.
// If onlyDirs is true, only directories are returned.
func CompletePath(input string, onlyDirs bool) Result {
	raw := strings.TrimSpace(input)
	expanded := raw

	// Handle ~ home expansion
	isHome := false
	if strings.HasPrefix(raw, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			if raw == "~" {
				expanded = home + "/"
				isHome = true
			} else if strings.HasPrefix(raw, "~/") {
				expanded = filepath.Join(home, raw[2:])
				if strings.HasSuffix(raw, "/") {
					expanded += "/"
				}
				isHome = true
			}
		}
	}

	searchDir := "."
	filterPrefix := ""

	if strings.HasSuffix(expanded, "/") || strings.HasSuffix(expanded, string(filepath.Separator)) {
		searchDir = expanded
		filterPrefix = ""
	} else if strings.Contains(expanded, "/") || strings.Contains(expanded, string(filepath.Separator)) {
		searchDir = filepath.Dir(expanded)
		filterPrefix = filepath.Base(expanded)
	} else {
		searchDir = "."
		filterPrefix = expanded
	}

	entries, err := os.ReadDir(searchDir)
	if err != nil {
		return Result{Prefix: input}
	}

	lowerFilter := strings.ToLower(filterPrefix)
	var matches []string

	for _, e := range entries {
		name := e.Name()
		// Hide hidden files unless user explicitly typed '.'
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(filterPrefix, ".") {
			continue
		}

		if filterPrefix != "" && !strings.HasPrefix(strings.ToLower(name), lowerFilter) {
			continue
		}

		isDir := e.IsDir()
		if onlyDirs && !isDir {
			continue
		}

		match := name
		if isDir {
			match += "/"
		}

		// Rebuild full relative path to replace input
		var fullMatch string
		if raw == "" {
			fullMatch = match
		} else if strings.HasSuffix(raw, "/") {
			fullMatch = raw + match
		} else if strings.Contains(raw, "/") {
			dirPart := raw[:strings.LastIndex(raw, "/")+1]
			fullMatch = dirPart + match
		} else if isHome {
			fullMatch = match
		} else {
			fullMatch = match
		}

		matches = append(matches, fullMatch)
	}

	sort.Strings(matches)

	common := ""
	if len(matches) == 1 {
		common = matches[0]
	} else if len(matches) > 1 {
		common = findCommonPrefix(matches)
	}

	return Result{
		Prefix:       input,
		Dir:          searchDir,
		Candidates:   matches,
		CommonPrefix: common,
	}
}

// IsPathArg returns true if the argument name or type suggests a file or directory path.
func IsPathArg(name, argType, currentVal string) bool {
	t := strings.ToLower(argType)
	if t == "file" || t == "dir" || t == "directory" || t == "path" {
		return true
	}

	n := strings.ToLower(name)
	switch n {
	case "file", "path", "dir", "dest", "source", "src", "target", "output", "out",
		"input", "in", "filepath", "folder", "filename", "dst", "from", "to", "location":
		return true
	}

	val := strings.TrimSpace(currentVal)
	if strings.HasPrefix(val, "./") || strings.HasPrefix(val, "../") ||
		strings.HasPrefix(val, "/") || strings.HasPrefix(val, "~") {
		return true
	}

	return false
}

func findCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			if len(prefix) == 0 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}
