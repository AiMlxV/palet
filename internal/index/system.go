package index

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Thitipong-PP/palet/internal/plugin"
)

type pathCommand struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type systemCacheEntry struct {
	Hash        string             `json:"hash"`
	Subcommands []systemSubcommand `json:"subcommands"`
	Flags       []string           `json:"flags"`
}

type systemSubcommand struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type systemCache struct {
	PathHash     string                      `json:"pathHash"`
	PathCommands []pathCommand               `json:"pathCommands"`
	Commands     map[string]systemCacheEntry `json:"commands"`
}

type ScanProgress struct {
	Current int
	Total   int
	Name    string
	Done    bool
}

var scanProgressCh = make(chan ScanProgress, 32)
var scanMu sync.Mutex
var scanLast ScanProgress
var scanHasLast bool

func ScanProgressChan() <-chan ScanProgress {
	return scanProgressCh
}

func CurrentScanProgress() (ScanProgress, bool) {
	scanMu.Lock()
	defer scanMu.Unlock()
	return scanLast, scanHasLast
}

func sendScanProgress(p ScanProgress) {
	scanMu.Lock()
	scanLast = p
	scanHasLast = true
	scanMu.Unlock()
	select {
	case scanProgressCh <- p:
	default:
	}
}

func systemEntries() []Entry {
	if strings.EqualFold(os.Getenv("PALET_SYSTEM"), "off") {
		return nil
	}
	verbose := scanVerbose()
	startAll := time.Now()
	if verbose {
		fmt.Fprintln(os.Stderr, "palet: systemEntries start")
	}
	cache := loadSystemCache()
	dirty := false
	pathStart := time.Now()
	pathHash := hashPathEnv()
	if verbose {
		fmt.Fprintf(os.Stderr, "palet: hashPathEnv %s\n", time.Since(pathStart))
	}
	cmds := cache.PathCommands
	if cache.PathHash != pathHash || len(cmds) == 0 {
		discoStart := time.Now()
		cmds = discoverPathCommands()
		if verbose {
			fmt.Fprintf(os.Stderr, "palet: discoverPathCommands %s\n", time.Since(discoStart))
		}
		cache.PathHash = pathHash
		cache.PathCommands = cmds
		dirty = true
	}
	if len(cmds) == 0 {
		if verbose {
			fmt.Fprintln(os.Stderr, "palet: no PATH commands")
		}
		return nil
	}

	allowAll := strings.EqualFold(os.Getenv("PALET_SUBCOMMANDS"), "all")
	helpBudget := 300 * time.Millisecond
	helpMax := 20
	if allowAll {
		helpBudget = 0
		helpMax = 0
	}
	start := time.Now()
	helpCount := 0
	plugins := make([]plugin.Plugin, 0, len(cmds)+1)
	systemPlugin := plugin.Plugin{
		Name:        "system",
		Description: "commands from PATH",
	}
	current := make(map[string]struct{}, len(cmds))

	for _, cmd := range cmds {
		current[cmd.Path] = struct{}{}
		hash := fileHash(cmd.Path)
		cached, ok := cache.Commands[cmd.Path]
		var subcommands []systemSubcommand
		var flags []string
		if ok && cached.Hash == hash {
			subcommands = cached.Subcommands
			flags = cached.Flags
		} else {
			scanned := false
			if (allowAll || allowlisted(cmd.Name)) && helpCount < helpMax && time.Since(start) < helpBudget {
				var sc []systemSubcommand
				var fl []string
				sc, fl, scanned = discoverHelp(cmd.Name)
				helpCount++
				if scanned {
					subcommands = sc
					flags = fl
					cache.Commands[cmd.Path] = systemCacheEntry{
						Hash:        hash,
						Subcommands: subcommands,
						Flags:       flags,
					}
					dirty = true
				}
			}
			if !scanned && ok {
				subcommands = cached.Subcommands
				flags = cached.Flags
			}
		}

		if len(subcommands) > 0 {
			p := plugin.Plugin{
				Name:        cmd.Name,
				Description: "system command",
			}
			p.Commands = append(p.Commands, plugin.Command{
				Name:        cmd.Name,
				Description: "",
				Template:    cmd.Name,
			})
			for _, sc := range subcommands {
				p.Commands = append(p.Commands, plugin.Command{
					Name:        sc.Name,
					Description: sc.Description,
					Template:    cmd.Name + " " + sc.Name,
				})
			}
			plugins = append(plugins, p)
			continue
		}

		systemPlugin.Commands = append(systemPlugin.Commands, plugin.Command{
			Name:        cmd.Name,
			Description: "",
			Template:    cmd.Name,
		})
	}

	if len(systemPlugin.Commands) > 0 {
		plugins = append(plugins, systemPlugin)
	}

	for path := range cache.Commands {
		if _, ok := current[path]; !ok {
			delete(cache.Commands, path)
			dirty = true
		}
	}

	if dirty {
		_ = writeSystemCache(cache)
	}
	startBackgroundScan(cmds, cache, allowAll)
	if verbose {
		fmt.Fprintf(os.Stderr, "palet: systemEntries done %s\n", time.Since(startAll))
	}

	return Build(plugins)
}

func discoverPathCommands() []pathCommand {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var out []pathCommand
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		entries, err := readDirWithTimeout(dir, pathDirTimeout())
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if _, ok := seen[name]; ok {
				continue
			}
			info, err := e.Info()
			if err != nil || !isExecutable(info.Mode(), name) {
				continue
			}
			seen[name] = struct{}{}
			out = append(out, pathCommand{Name: name, Path: filepath.Join(dir, name)})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func isExecutable(mode os.FileMode, name string) bool {
	if runtime.GOOS == "windows" {
		switch strings.ToLower(filepath.Ext(name)) {
		case ".exe", ".cmd", ".bat", ".com":
			return true
		default:
			return false
		}
	}
	return mode.IsRegular() && mode&0111 != 0
}

func discoverHelp(cmd string) ([]systemSubcommand, []string, bool) {
	attempted := true
	if cmd == "git" {
		cmdOut := runHelpTimeout(800*time.Millisecond, cmd, "help", "-a")
		flagOut := runHelpTimeout(500*time.Millisecond, cmd, "-h")
		if cmdOut == "" && flagOut == "" {
			return nil, nil, attempted
		}
		sc, _ := parseHelp(cmdOut)
		_, fl := parseHelp(flagOut)
		return sc, fl, attempted
	}

	out := runHelp(cmd, "--help")
	if out == "" {
		out = runHelp(cmd, "-h")
	}
	if out == "" {
		return nil, nil, attempted
	}
	sc, fl := parseHelp(out)
	return sc, fl, attempted
}

func runHelp(cmd string, args ...string) string {
	return runHelpTimeout(300*time.Millisecond, cmd, args...)
}

func runHelpTimeout(timeout time.Duration, cmd string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	command := exec.CommandContext(ctx, cmd, args...)
	command.Stdin = nil
	command.Env = append(os.Environ(), "PAGER=cat", "GIT_PAGER=cat", "LESS=FRX", "TERM=dumb")
	var buf bytes.Buffer
	command.Stdout = &buf
	command.Stderr = &buf
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return ""
		}
	}
	return buf.String()
}

func discoverHelpWithTimeout(cmd string, timeout time.Duration) ([]systemSubcommand, bool) {
	if timeout <= 0 {
		sc, _, _ := discoverHelp(cmd)
		return sc, true
	}
	ch := make(chan struct {
		cmds []systemSubcommand
		ok   bool
	}, 1)
	go func() {
		sc, _, _ := discoverHelp(cmd)
		ch <- struct {
			cmds []systemSubcommand
			ok   bool
		}{cmds: sc, ok: true}
	}()
	select {
	case res := <-ch:
		return res.cmds, res.ok
	case <-time.After(timeout):
		return nil, false
	}
}

func parseHelp(output string) ([]systemSubcommand, []string) {
	var subcommands []systemSubcommand
	cmdSeen := make(map[string]struct{})

	section := ""
	for _, line := range strings.Split(output, "\n") {
		trim := strings.TrimSpace(line)
		lower := strings.ToLower(trim)
		switch {
		case strings.HasSuffix(lower, "commands:") || strings.Contains(lower, "available commands") || strings.HasSuffix(lower, "subcommands:"):
			section = "commands"
			continue
		}
		if trim == line && strings.Contains(lower, "commands") {
			section = "commands"
			continue
		}

		if section == "commands" {
			name, desc := parseSubcommandLine(line)
			if name != "" {
				if _, ok := cmdSeen[name]; !ok {
					subcommands = append(subcommands, systemSubcommand{Name: name, Description: desc})
					cmdSeen[name] = struct{}{}
				}
			}
			continue
		}

	}

	return subcommands, nil
}

var subcommandLineRe = regexp.MustCompile(`^\s+([a-zA-Z0-9][a-zA-Z0-9_-]*)\s*(.*)$`)

func parseSubcommandLine(line string) (string, string) {
	m := subcommandLineRe.FindStringSubmatch(line)
	if len(m) == 0 {
		return "", ""
	}
	name := strings.TrimSpace(m[1])
	if name == "" || strings.HasPrefix(name, "-") {
		return "", ""
	}
	return name, strings.TrimSpace(m[2])
}

func allowlisted(name string) bool {
	switch name {
	case "git", "docker", "kubectl", "npm", "pnpm", "yarn", "go", "cargo", "pip", "pip3", "brew":
		return true
	default:
		return false
	}
}

func formatFlags(flags []string) string {
	if len(flags) == 0 {
		return ""
	}
	return "flags: " + strings.Join(flags, ", ")
}

func loadSystemCache() systemCache {
	data, err := os.ReadFile(systemCachePath())
	if err != nil {
		return systemCache{Commands: make(map[string]systemCacheEntry)}
	}
	var sc systemCache
	if err := json.Unmarshal(data, &sc); err != nil || sc.Commands == nil {
		return systemCache{Commands: make(map[string]systemCacheEntry)}
	}
	return sc
}

func writeSystemCache(sc systemCache) error {
	path := systemCachePath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.Marshal(sc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func systemCachePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".cache", "palet", "system.json")
	}
	return filepath.Join(os.TempDir(), "palet-system.json")
}

func fileHash(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmtSizeTime(info.Size(), info.ModTime().UnixNano())
}

func fmtSizeTime(size int64, nsec int64) string {
	return strconv.FormatInt(size, 10) + ":" + strconv.FormatInt(nsec, 10)
}

func hashPathEnv() string {
	pathEnv := os.Getenv("PATH")
	h := md5.New()
	_, _ = h.Write([]byte(pathEnv))
	statTimeout := pathDirTimeout()
	for _, dir := range filepath.SplitList(pathEnv) {
		if dir == "" {
			continue
		}
		if info, err := statWithTimeout(dir, statTimeout); err == nil {
			fmt.Fprintf(h, "|%s:%d", dir, info.ModTime().UnixNano())
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func statWithTimeout(path string, timeout time.Duration) (os.FileInfo, error) {
	if timeout <= 0 {
		return os.Stat(path)
	}

	ch := make(chan struct {
		info os.FileInfo
		err  error
	}, 1)

	go func() {
		info, err := os.Stat(path)
		ch <- struct {
			info os.FileInfo
			err  error
		}{info: info, err: err}
	}()

	select {
	case res := <-ch:
		return res.info, res.err
	case <-time.After(timeout):
		if strings.EqualFold(os.Getenv("PALET_SCAN_VERBOSE"), "1") ||
			strings.EqualFold(os.Getenv("PALET_SCAN_VERBOSE"), "true") {
			fmt.Fprintf(os.Stderr, "palet: skip PATH stat (timeout %s): %s\n", timeout, path)
		}
		return nil, fmt.Errorf("stat timeout")
	}
}

func pathDirTimeout() time.Duration {
	if v := os.Getenv("PALET_PATH_DIR_TIMEOUT_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return 200 * time.Millisecond
}

func readDirWithTimeout(dir string, timeout time.Duration) ([]os.DirEntry, error) {
	if timeout <= 0 {
		return os.ReadDir(dir)
	}

	ch := make(chan struct {
		entries []os.DirEntry
		err     error
	}, 1)

	go func() {
		entries, err := os.ReadDir(dir)
		ch <- struct {
			entries []os.DirEntry
			err     error
		}{entries: entries, err: err}
	}()

	select {
	case res := <-ch:
		return res.entries, res.err
	case <-time.After(timeout):
		if strings.EqualFold(os.Getenv("PALET_SCAN_VERBOSE"), "1") ||
			strings.EqualFold(os.Getenv("PALET_SCAN_VERBOSE"), "true") {
			fmt.Fprintf(os.Stderr, "palet: skip PATH dir (timeout %s): %s\n", timeout, dir)
		}
		return nil, fmt.Errorf("read timeout")
	}
}

func startBackgroundScan(cmds []pathCommand, cache systemCache, allowAll bool) {
	if len(cmds) == 0 {
		return
	}
	if strings.EqualFold(os.Getenv("PALET_SCAN_BACKGROUND"), "off") {
		return
	}
	copyCache := systemCache{
		PathHash:     cache.PathHash,
		PathCommands: cache.PathCommands,
		Commands:     make(map[string]systemCacheEntry, len(cache.Commands)),
	}
	for k, v := range cache.Commands {
		copyCache.Commands[k] = v
	}

	go func() {
		deadline := time.Now().Add(30 * time.Second)
		verbose := scanVerbose()
		if verbose {
			fmt.Fprintln(os.Stderr, "palet: background scan start")
		}

		var targets []pathCommand
		for _, cmd := range cmds {
			if !allowAll && !allowlisted(cmd.Name) {
				continue
			}
			hash := fileHash(cmd.Path)
			if cached, ok := copyCache.Commands[cmd.Path]; ok && cached.Hash == hash {
				continue
			}
			targets = append(targets, cmd)
		}
		if len(targets) > 0 {
			sendScanProgress(ScanProgress{Current: 0, Total: len(targets)})
		}

		current := 0
		for _, cmd := range targets {
			if time.Now().After(deadline) {
				if verbose {
					fmt.Fprintln(os.Stderr, "palet: background scan hit deadline")
				}
				break
			}
			hash := fileHash(cmd.Path)
			startCmd := time.Now()
			sc, ok := discoverHelpWithTimeout(cmd.Name, 400*time.Millisecond)
			if !ok {
				sc = nil
			}
			current++
			sendScanProgress(ScanProgress{Current: current, Total: len(targets), Name: cmd.Name})
			if verbose {
				fmt.Fprintf(os.Stderr, "palet: scan %s (%s)\n", cmd.Name, time.Since(startCmd))
			}
			copyCache.Commands[cmd.Path] = systemCacheEntry{
				Hash:        hash,
				Subcommands: sc,
				Flags:       nil,
			}
		}
		_ = writeSystemCache(copyCache)
		if len(targets) > 0 {
			sendScanProgress(ScanProgress{Current: current, Total: len(targets), Done: true})
		}
		if verbose {
			fmt.Fprintln(os.Stderr, "palet: background scan done")
		}
	}()
}

func scanVerbose() bool {
	return strings.EqualFold(os.Getenv("PALET_SCAN_VERBOSE"), "1") ||
		strings.EqualFold(os.Getenv("PALET_SCAN_VERBOSE"), "true")
}
