package project

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectGoProject(t *testing.T) {
	tmp := t.TempDir()
	goMod := "module github.com/test/my-go-tool\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(tmp, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	info := Detect(tmp)
	if !contains(stringsFields(info.Type), "go") {
		t.Errorf("expected go in type, got %q", info.Type)
	}
	if !hasPrefixInSlice(info.Suggestions, "go test") {
		t.Errorf("expected 'go test' in suggestions, got %v", info.Suggestions)
	}
}

func TestDetectDockerComposeProject(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "docker-compose.yml"), []byte("version: '3'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	info := Detect(tmp)
	if !contains(stringsFields(info.Type), "docker-compose") {
		t.Errorf("expected docker-compose in type, got %q", info.Type)
	}
	if !hasPrefixInSlice(info.Suggestions, "docker compose up") {
		t.Errorf("expected 'docker compose up' in suggestions, got %v", info.Suggestions)
	}
}

func TestDetectNodeProject(t *testing.T) {
	tmp := t.TempDir()
	pkgJSON := `{"name": "test-app", "scripts": {"dev": "vite", "build": "vite build"}}`
	if err := os.WriteFile(filepath.Join(tmp, "package.json"), []byte(pkgJSON), 0644); err != nil {
		t.Fatal(err)
	}

	info := Detect(tmp)
	if !contains(stringsFields(info.Type), "node") {
		t.Errorf("expected node in type, got %q", info.Type)
	}
	if !hasPrefixInSlice(info.Suggestions, "npm run dev") {
		t.Errorf("expected 'npm run dev' in suggestions, got %v", info.Suggestions)
	}
}

func stringsFields(s string) []string {
	var out []string
	for _, it := range filepath.SplitList(s) {
		out = append(out, it)
	}
	return out
}

func hasPrefixInSlice(slice []string, prefix string) bool {
	for _, it := range slice {
		if it == prefix {
			return true
		}
	}
	return false
}
