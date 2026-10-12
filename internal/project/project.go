package project

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Info contains detected project metadata from the working directory.
type Info struct {
	Type        string   // e.g. "go", "node", "rust", "docker", "git"
	Summary     string   // e.g. "Go module (palet)", "Node.js (npm)", "Git repo (dirty)"
	Suggestions []string // exact command names or prefixes to boost
}

// Detect analyzes the given directory (or current working directory if empty).
func Detect(dir string) Info {
	if dir == "" {
		if wd, err := os.Getwd(); err == nil {
			dir = wd
		} else {
			dir = "."
		}
	}

	var suggestions []string
	var types []string
	var summaries []string

	// 1. Git repository detection
	if isGitRepo(dir) {
		dirty := isGitDirty(dir)
		types = append(types, "git")
		if dirty {
			summaries = append(summaries, "Git (uncommitted changes)")
			suggestions = append(suggestions, "git status", "git diff", "git add", "git commit")
		} else {
			summaries = append(summaries, "Git repo")
			suggestions = append(suggestions, "git status", "git pull", "git log", "git checkout")
		}
	}

	// 2. Docker & Docker Compose
	hasCompose := fileExists(filepath.Join(dir, "docker-compose.yml")) ||
		fileExists(filepath.Join(dir, "docker-compose.yaml")) ||
		fileExists(filepath.Join(dir, "compose.yml")) ||
		fileExists(filepath.Join(dir, "compose.yaml"))
	hasDockerfile := fileExists(filepath.Join(dir, "Dockerfile"))

	if hasCompose {
		types = append(types, "docker-compose")
		summaries = append(summaries, "Docker Compose")
		suggestions = append(suggestions, "docker compose up", "docker compose ps", "docker compose logs", "docker compose down", "docker compose build")
	} else if hasDockerfile {
		types = append(types, "docker")
		summaries = append(summaries, "Docker")
		suggestions = append(suggestions, "docker build", "docker run")
	}

	// 3. Go
	if fileExists(filepath.Join(dir, "go.mod")) {
		types = append(types, "go")
		modName := readGoModName(filepath.Join(dir, "go.mod"))
		if modName != "" {
			summaries = append(summaries, "Go ("+filepath.Base(modName)+")")
		} else {
			summaries = append(summaries, "Go project")
		}
		suggestions = append(suggestions, "go test", "go build", "go run", "go mod tidy")
	}

	// 4. Node.js / JavaScript / TypeScript
	pkgPath := filepath.Join(dir, "package.json")
	if fileExists(pkgPath) {
		types = append(types, "node")
		pm := detectNodePackageManager(dir)
		summaries = append(summaries, "Node.js ("+pm+")")

		scripts := extractPackageScripts(pkgPath)
		for _, s := range []string{"dev", "start", "test", "build"} {
			if contains(scripts, s) {
				suggestions = append(suggestions, pm+" run "+s)
			}
		}
		suggestions = append(suggestions, pm+" test", pm+" run build", pm+" install")
	}

	// 5. Rust
	if fileExists(filepath.Join(dir, "Cargo.toml")) {
		types = append(types, "rust")
		summaries = append(summaries, "Rust (Cargo)")
		suggestions = append(suggestions, "cargo test", "cargo build", "cargo run", "cargo check")
	}

	// 6. Python
	if fileExists(filepath.Join(dir, "pyproject.toml")) ||
		fileExists(filepath.Join(dir, "requirements.txt")) ||
		fileExists(filepath.Join(dir, "Pipfile")) {
		types = append(types, "python")
		summaries = append(summaries, "Python")
		suggestions = append(suggestions, "pytest", "python", "pip install")
	}

	// 7. Makefile
	if fileExists(filepath.Join(dir, "Makefile")) || fileExists(filepath.Join(dir, "makefile")) {
		if !contains(types, "make") {
			types = append(types, "make")
			suggestions = append(suggestions, "make test", "make build", "make")
		}
	}

	return Info{
		Type:        strings.Join(types, ", "),
		Summary:     strings.Join(summaries, " • "),
		Suggestions: uniqueStrings(suggestions),
	}
}

func isGitRepo(dir string) bool {
	gitDir := filepath.Join(dir, ".git")
	info, err := os.Stat(gitDir)
	return err == nil && (info.IsDir() || info.Mode().IsRegular())
}

func isGitDirty(dir string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", dir, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

func readGoModName(p string) string {
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func detectNodePackageManager(dir string) string {
	if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) {
		return "pnpm"
	}
	if fileExists(filepath.Join(dir, "yarn.lock")) {
		return "yarn"
	}
	if fileExists(filepath.Join(dir, "bun.lockb")) || fileExists(filepath.Join(dir, "bun.lock")) {
		return "bun"
	}
	return "npm"
}

func extractPackageScripts(pkgPath string) []string {
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	var out []string
	for k := range pkg.Scripts {
		out = append(out, k)
	}
	return out
}

func contains(slice []string, s string) bool {
	for _, it := range slice {
		if it == s {
			return true
		}
	}
	return false
}

func uniqueStrings(slice []string) []string {
	seen := make(map[string]struct{}, len(slice))
	var out []string
	for _, s := range slice {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}
