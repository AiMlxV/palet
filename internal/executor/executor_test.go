package executor

import (
	"testing"
)

func TestBuildCommand(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vals     map[string]string
		want     string
		wantErr  bool
	}{
		{
			name:     "simple substitution",
			template: "apt install {{.pkg}}",
			vals:     map[string]string{"pkg": "curl"},
			want:     "apt install curl",
		},
		{
			name:     "boolean true conditional",
			template: "git checkout {{if .new}}-b {{end}}{{.branch}}",
			vals:     map[string]string{"new": "true", "branch": "feature"},
			want:     "git checkout -b feature",
		},
		{
			name:     "boolean false conditional",
			template: "git checkout {{if .new}}-b {{end}}{{.branch}}",
			vals:     map[string]string{"new": "false", "branch": "main"},
			want:     "git checkout main",
		},
		{
			name:     "empty fields collapsing",
			template: "docker run   {{.image}}   {{.cmd}}",
			vals:     map[string]string{"image": "alpine", "cmd": "sh"},
			want:     "docker run alpine sh",
		},
		{
			name:     "missing key zeroed",
			template: "curl {{.url}} {{.flags}}",
			vals:     map[string]string{"url": "https://example.com"},
			want:     "curl https://example.com <no value>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := BuildCommand(tt.template, tt.vals)
			if (err != nil) != tt.wantErr {
				t.Fatalf("BuildCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("BuildCommand() got = %q, want %q", got, tt.want)
			}
		})
	}
}
