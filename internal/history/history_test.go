package history

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordAndRecent(t *testing.T) {
	tmpDir := t.TempDir()
	historyFile := filepath.Join(tmpDir, "test-history.json")
	SetHistoryPath(historyFile)
	defer SetHistoryPath("")

	if err := Record("docker compose up -d"); err != nil {
		t.Fatalf("failed to record command: %v", err)
	}
	if err := Record("git status"); err != nil {
		t.Fatalf("failed to record command: %v", err)
	}
	// Recording "docker compose up -d" again should bring it to the top
	if err := Record("docker compose up -d"); err != nil {
		t.Fatalf("failed to record command: %v", err)
	}

	recent := Recent()
	if len(recent) != 2 {
		t.Fatalf("expected 2 items, got %d", len(recent))
	}
	if recent[0] != "docker compose up -d" {
		t.Fatalf("expected 'docker compose up -d' at top, got '%s'", recent[0])
	}
	if recent[1] != "git status" {
		t.Fatalf("expected 'git status' at index 1, got '%s'", recent[1])
	}

	if err := Clear(); err != nil {
		t.Fatalf("failed to clear history: %v", err)
	}
	if len(Recent()) != 0 {
		t.Fatalf("expected empty history after Clear, got %v", Recent())
	}
	_ = os.Remove(historyFile)
}
