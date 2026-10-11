package config

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestParseCustomConfig(t *testing.T) {
	yamlData := `
name: custom-scripts
commands:
  - name: deploy
    template: "./deploy.sh"
    flags:
      - name: "--force"
        description: "Force deploy"
`
	var g Group
	if err := yaml.Unmarshal([]byte(yamlData), &g); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(g.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(g.Commands))
	}

	cmd := g.Commands[0]
	if len(cmd.Flags) != 1 {
		t.Fatalf("expected 1 flag, got %d", len(cmd.Flags))
	}

	if cmd.Flags[0].Value() != "--force" {
		t.Fatalf("expected flag '--force', got %q", cmd.Flags[0].Value())
	}
}
