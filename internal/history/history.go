package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const maxHistory = 25

var (
	mu         sync.Mutex
	customPath string
)

func SetHistoryPath(path string) {
	mu.Lock()
	defer mu.Unlock()
	customPath = path
}

func historyFilePath() string {
	if customPath != "" {
		return customPath
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "palet", "history.json")
	}
	return filepath.Join(os.TempDir(), "palet-history.json")
}

// Record saves a command to the history list, moving it to the top.
func Record(commandStr string) error {
	cmd := strings.TrimSpace(commandStr)
	if cmd == "" {
		return nil
	}

	mu.Lock()
	defer mu.Unlock()

	items := loadHistoryLocked()
	var newItems []string
	newItems = append(newItems, cmd)
	for _, it := range items {
		if it != cmd {
			newItems = append(newItems, it)
		}
		if len(newItems) >= maxHistory {
			break
		}
	}

	path := historyFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(newItems)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Recent returns the recent command strings (most recent first).
func Recent() []string {
	mu.Lock()
	defer mu.Unlock()
	return loadHistoryLocked()
}

// Clear clears the history.
func Clear() error {
	mu.Lock()
	defer mu.Unlock()
	path := historyFilePath()
	return os.Remove(path)
}

func loadHistoryLocked() []string {
	path := historyFilePath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var items []string
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	return items
}
