package completion

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/Thitipong-PP/palet/internal/config"
	"gopkg.in/yaml.v3"
)

// DetectCommandArg returns a semantic argument definition if the command expects one,
// or nil if the command takes no positional target by default.
func DetectCommandArg(commandStr string) *config.Arg {
	fields := strings.Fields(strings.TrimSpace(commandStr))
	if len(fields) == 0 {
		return nil
	}

	mainCmd := fields[0]
	subCmd := ""
	if len(fields) > 1 {
		subCmd = fields[1]
	}

	// 1. Git commands
	if mainCmd == "git" {
		switch subCmd {
		case "branch", "checkout", "switch", "merge", "rebase":
			return &config.Arg{
				Name:        "branch",
				Description: "Branch name (Tab to autocomplete branches)",
				Type:        "branch",
				Required:    false,
			}
		case "add", "restore", "rm", "mv":
			return &config.Arg{
				Name:        "path",
				Description: "File or directory path (Tab to autocomplete paths)",
				Type:        "path",
				Required:    false,
			}
		case "diff", "show":
			return &config.Arg{
				Name:        "target",
				Description: "File, commit, or branch (Tab to autocomplete)",
				Type:        "path",
				Required:    false,
			}
		case "commit":
			return nil // git commit uses -m flag or opens editor
		case "status", "log", "pull", "push", "fetch", "stash":
			return nil // No positional argument needed
		}
	}

	// 2. Docker compose commands
	if (mainCmd == "docker" && subCmd == "compose") || mainCmd == "docker-compose" {
		csub := ""
		if mainCmd == "docker" && len(fields) > 2 {
			csub = fields[2]
		} else if len(fields) > 1 {
			csub = fields[1]
		}
		switch csub {
		case "up", "logs", "restart", "stop", "build", "kill", "rm":
			return &config.Arg{
				Name:        "service",
				Description: "Service name (Tab to autocomplete services)",
				Type:        "service",
				Required:    false,
			}
		case "down", "ps", "top", "version":
			return nil
		}
	}

	// 3. Node script runners
	if (mainCmd == "npm" || mainCmd == "pnpm" || mainCmd == "yarn" || mainCmd == "bun") && subCmd == "run" {
		return &config.Arg{
			Name:        "script",
			Description: "Package script (Tab to autocomplete scripts)",
			Type:        "script",
			Required:    false,
		}
	}

	// 4. File-centric unix utilities
	switch mainCmd {
	case "cat", "tar", "rm", "cp", "mv", "touch", "nano", "vim", "vi", "code", "less", "more", "head", "tail":
		return &config.Arg{
			Name:        "path",
			Description: "File or directory path (Tab to autocomplete paths)",
			Type:        "path",
			Required:    false,
		}
	case "cd", "ls", "find", "mkdir", "rmdir":
		return &config.Arg{
			Name:        "dir",
			Description: "Directory path (Tab to autocomplete paths)",
			Type:        "dir",
			Required:    false,
		}
	}

	// 5. Zero-argument commands (status, version, info)
	switch mainCmd {
	case "pwd", "whoami", "date", "uptime", "top", "htop", "clear", "env", "df", "free":
		return nil
	}

	return nil
}

// CompleteSemantic returns autocompletions based on the semantic type (branch, service, script, path).
func CompleteSemantic(argType, input string) Result {
	switch argType {
	case "branch":
		branches := GitBranches()
		return filterCandidates(input, branches)
	case "service":
		services := ComposeServices()
		return filterCandidates(input, services)
	case "script":
		scripts := NpmScripts()
		return filterCandidates(input, scripts)
	case "dir":
		return CompletePath(input, true)
	default:
		return CompletePath(input, false)
	}
}

// GitBranches lists local git branches in the current repository.
func GitBranches() []string {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "branch", "--list", "--format=%(refname:short)")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	lines := strings.Split(string(out), "\n")
	var branches []string
	for _, l := range lines {
		trim := strings.TrimSpace(l)
		if trim != "" {
			branches = append(branches, trim)
		}
	}
	sort.Strings(branches)
	return branches
}

// ComposeServices inspects docker-compose.yml or compose.yaml in the working directory.
func ComposeServices() []string {
	for _, name := range []string{"docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"} {
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var compose struct {
			Services map[string]interface{} `yaml:"services"`
		}
		if err := yaml.Unmarshal(data, &compose); err == nil && len(compose.Services) > 0 {
			var services []string
			for s := range compose.Services {
				services = append(services, s)
			}
			sort.Strings(services)
			return services
		}
	}
	return nil
}

// NpmScripts inspects package.json in the working directory.
func NpmScripts() []string {
	data, err := os.ReadFile("package.json")
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	var scripts []string
	for s := range pkg.Scripts {
		scripts = append(scripts, s)
	}
	sort.Strings(scripts)
	return scripts
}

func filterCandidates(input string, candidates []string) Result {
	raw := strings.TrimSpace(input)
	lower := strings.ToLower(raw)

	var matches []string
	for _, c := range candidates {
		if raw == "" || strings.HasPrefix(strings.ToLower(c), lower) {
			matches = append(matches, c)
		}
	}

	common := ""
	if len(matches) == 1 {
		common = matches[0]
	} else if len(matches) > 1 {
		common = findCommonPrefix(matches)
	}

	return Result{
		Prefix:       input,
		Candidates:   matches,
		CommonPrefix: common,
	}
}
