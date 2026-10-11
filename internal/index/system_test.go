package index

import (
	"testing"
)

func TestParseHelpFlags(t *testing.T) {
	helpOutput := `
Usage:  docker compose up [OPTIONS] [SERVICE...]

Create and start containers

Options:
      --build                        Build images before starting containers
  -d, --detach                       Detached mode: Run containers in the background
      --dry-run                      Execute command in dry run mode
`
	_, flags := parseHelp(helpOutput)
	if len(flags) < 3 {
		t.Fatalf("expected at least 3 flags, got %d: %v", len(flags), flags)
	}

	name, desc := parseFlagLine(flags[0])
	if name != "--build" {
		t.Errorf("expected '--build', got %q", name)
	}
	if desc != "Build images before starting containers" {
		t.Errorf("unexpected desc: %q", desc)
	}

	name2, desc2 := parseFlagLine(flags[1])
	if name2 != "-d, --detach" {
		t.Errorf("expected '-d, --detach', got %q", name2)
	}
	if desc2 != "Detached mode: Run containers in the background" {
		t.Errorf("unexpected desc: %q", desc2)
	}
}
