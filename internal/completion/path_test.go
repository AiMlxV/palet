package completion

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompletePath(t *testing.T) {
	tmp := t.TempDir()

	// Create test files and directories
	_ = os.WriteFile(filepath.Join(tmp, "app.go"), []byte("package main"), 0644)
	_ = os.WriteFile(filepath.Join(tmp, "app_test.go"), []byte("package main"), 0644)
	_ = os.WriteFile(filepath.Join(tmp, "main.go"), []byte("package main"), 0644)
	_ = os.Mkdir(filepath.Join(tmp, "assets"), 0755)
	_ = os.Mkdir(filepath.Join(tmp, "internal"), 0755)

	// Test prefix matching inside temp dir
	res := CompletePath(filepath.Join(tmp, "ap"), false)
	if len(res.Candidates) != 2 {
		t.Fatalf("expected 2 candidates for 'ap', got %v", res.Candidates)
	}

	// Test directory completion
	dirRes := CompletePath(filepath.Join(tmp, "ass"), true)
	if len(dirRes.Candidates) != 1 || !filepath.IsAbs(dirRes.Candidates[0]) {
		// Should match assets/
		if len(dirRes.Candidates) != 1 {
			t.Fatalf("expected 1 dir candidate, got %v", dirRes.Candidates)
		}
	}
}

func TestIsPathArg(t *testing.T) {
	tests := []struct {
		name     string
		argType  string
		val      string
		expected bool
	}{
		{"file", "string", "", true},
		{"path", "string", "", true},
		{"anything", "file", "", true},
		{"anything", "dir", "", true},
		{"target", "string", "", true},
		{"something", "string", "./foo", true},
		{"something", "string", "/etc/nginx", true},
		{"pkg", "string", "curl", false},
		{"env", "string", "staging", false},
	}

	for _, tt := range tests {
		got := IsPathArg(tt.name, tt.argType, tt.val)
		if got != tt.expected {
			t.Errorf("IsPathArg(%q, %q, %q) = %v, want %v", tt.name, tt.argType, tt.val, got, tt.expected)
		}
	}
}
