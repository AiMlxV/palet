package config

import (
	"strings"
)

// Group represents a collection of custom commands (from custom config).
type Group struct {
	Name        string    `yaml:"name"        json:"name"`
	Description string    `yaml:"description" json:"description"`
	Tags        []string  `yaml:"tags"        json:"tags"`
	Commands    []Command `yaml:"commands"    json:"commands"`
}

type Command struct {
	Name        string   `yaml:"name"        json:"name"`
	Description string   `yaml:"description" json:"description"`
	Template    string   `yaml:"template"    json:"template"`
	Tags        []string `yaml:"tags"        json:"tags"`
	Args        []Arg    `yaml:"args"        json:"args"`
	Flags       []Flag   `yaml:"flags"       json:"flags"`
}

type Flag struct {
	Name        string `yaml:"name"        json:"name"`
	Flag        string `yaml:"flag"        json:"flag"`
	Description string `yaml:"description" json:"description"`
}

func (f Flag) Value() string {
	if f.Flag != "" {
		return f.Flag
	}
	for _, part := range strings.Split(f.Name, ",") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "-") {
			fields := strings.Fields(part)
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}
	return f.Name
}

type Arg struct {
	Name        string   `yaml:"name"        json:"name"`
	Description string   `yaml:"description" json:"description"`
	Type        string   `yaml:"type"        json:"type"`
	Required    bool     `yaml:"required"    json:"required"`
	Default     string   `yaml:"default"     json:"default"`
	Choices     []string `yaml:"choices"     json:"choices"`
	Flag        string   `yaml:"flag"        json:"flag"`
}

type ArgType string

const (
	ArgTypeString ArgType = "string"
	ArgTypeBool   ArgType = "bool"
	ArgTypeEnum   ArgType = "enum"
	ArgTypeFile   ArgType = "file"
	ArgTypeDir    ArgType = "dir"
)
