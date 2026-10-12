package completion

import (
	"testing"
)

func TestDetectCommandArg(t *testing.T) {
	tests := []struct {
		cmd          string
		wantArgName  string
		wantNil      bool
	}{
		{"git status", "", true},
		{"git log", "", true},
		{"git branch", "branch", false},
		{"git checkout", "branch", false},
		{"git add", "path", false},
		{"docker compose up", "service", false},
		{"docker compose ps", "", true},
		{"docker ps", "", true},
		{"npm run", "script", false},
		{"cat", "path", false},
		{"clear", "", true},
		{"uptime", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			arg := DetectCommandArg(tt.cmd)
			if tt.wantNil {
				if arg != nil {
					t.Errorf("DetectCommandArg(%q) expected nil, got %v", tt.cmd, arg)
				}
			} else {
				if arg == nil {
					t.Fatalf("DetectCommandArg(%q) expected arg %q, got nil", tt.cmd, tt.wantArgName)
				}
				if arg.Name != tt.wantArgName {
					t.Errorf("DetectCommandArg(%q) name = %q, want %q", tt.cmd, arg.Name, tt.wantArgName)
				}
			}
		})
	}
}

func TestFilterCandidates(t *testing.T) {
	candidates := []string{"main", "master", "feature/auth", "feature/ui"}
	res := filterCandidates("feat", candidates)
	if len(res.Candidates) != 2 {
		t.Fatalf("expected 2 candidates for 'feat', got %v", res.Candidates)
	}
	if res.CommonPrefix != "feature/" {
		t.Errorf("expected common prefix 'feature/', got %q", res.CommonPrefix)
	}
}
