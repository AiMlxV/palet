package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Files returns all custom config files to load in priority order:
//  1. ./palet.yaml / ./palet.yml / ./custom.yaml (project-local custom config)
//  2. ./plugins/*.yaml (project-local plugins, optional)
//  3. ~/.config/palet/config.yaml / ~/.config/palet/commands.yaml (user custom config)
//  4. ~/.config/palet/custom/*.yaml
//  5. ~/.config/palet/plugins/*.yaml (user plugins, optional)
func Files() []string {
	var files []string

	if wd, err := os.Getwd(); err == nil {
		for _, name := range []string{"palet.yaml", "palet.yml", "custom.yaml", "custom.yml"} {
			p := filepath.Join(wd, name)
			if fileExists(p) {
				files = append(files, p)
			}
		}
		files = append(files, scanYamlDir(filepath.Join(wd, "plugins"))...)
	}

	if home, err := os.UserHomeDir(); err == nil {
		cfgDir := filepath.Join(home, ".config", "palet")
		for _, name := range []string{"config.yaml", "config.yml", "commands.yaml", "commands.yml"} {
			p := filepath.Join(cfgDir, name)
			if fileExists(p) {
				files = append(files, p)
			}
		}

		files = append(files, scanYamlDir(filepath.Join(cfgDir, "custom"))...)
		files = append(files, scanYamlDir(filepath.Join(cfgDir, "plugins"))...)
	}

	return files
}

func scanYamlDir(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext == ".yaml" || ext == ".yml" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// LoadAll loads every custom configuration found.
func LoadAll() []Group {
	var out []Group
	for _, f := range Files() {
		g, err := ParseFile(f)
		if err != nil {
			continue
		}
		out = append(out, g)
	}
	return out
}

// ParseFile parses a single YAML custom config file.
func ParseFile(path string) (Group, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Group{}, err
	}
	var g Group
	if err := yaml.Unmarshal(data, &g); err != nil {
		return Group{}, err
	}
	if g.Name == "" {
		base := filepath.Base(path)
		ext := filepath.Ext(base)
		g.Name = stringsTrimExt(base, ext)
	}
	return g, nil
}

func stringsTrimExt(name, ext string) string {
	if len(ext) > 0 && len(name) > len(ext) {
		return name[:len(name)-len(ext)]
	}
	return name
}
