package search

import (
	"testing"

	"github.com/Thitipong-PP/palet/internal/config"
	"github.com/Thitipong-PP/palet/internal/index"
)

func sampleEntries() []index.Entry {
	return []index.Entry{
		{
			Group: config.Group{
				Name:        "apt",
				Description: "Package manager",
				Tags:        []string{"debian", "ubuntu", "sysadmin"},
			},
			Command: config.Command{
				Name:        "install",
				Description: "Install package",
				Template:    "apt install {{.pkg}}",
				Tags:        []string{"package", "install"},
			},
		},
		{
			Group: config.Group{
				Name:        "systemctl",
				Description: "System service manager",
				Tags:        []string{"system", "service"},
			},
			Command: config.Command{
				Name:        "restart",
				Description: "Restart service",
				Template:    "systemctl restart {{.svc}}",
				Tags:        []string{"restart", "daemon"},
			},
		},
		{
			Group: config.Group{
				Name:        "tar",
				Description: "Archive tool",
				Tags:        []string{"archive", "compress"},
			},
			Command: config.Command{
				Name:        "extract",
				Description: "Extract tar.gz archive",
				Template:    "tar -xzf {{.file}}",
				Tags:        []string{"extract", "untar"},
			},
		},
	}
}

func TestExplicitTagSearch(t *testing.T) {
	entries := sampleEntries()

	// 1. Search with '#' prefix
	results := Query(entries, "#debian")
	if len(results) != 1 || results[0].Group.Name != "apt" {
		t.Fatalf("expected apt for #debian, got %v", results)
	}

	// 2. Search with 'tag:' prefix
	results = Query(entries, "tag:service")
	if len(results) != 1 || results[0].Group.Name != "systemctl" {
		t.Fatalf("expected systemctl for tag:service, got %v", results)
	}

	// 3. Search with '--tag ' prefix
	results = Query(entries, "--tag archive")
	if len(results) != 1 || results[0].Group.Name != "tar" {
		t.Fatalf("expected tar for --tag archive, got %v", results)
	}

	// 4. Standalone '#' matches all entries that have tags
	results = Query(entries, "#")
	if len(results) != 3 {
		t.Fatalf("expected all 3 tagged entries for '#', got %d", len(results))
	}

	// 5. Non-matching tag query
	results = Query(entries, "#nonexistent")
	if len(results) != 0 {
		t.Fatalf("expected 0 results for #nonexistent, got %d", len(results))
	}
}

func TestNaturalTagSearch(t *testing.T) {
	entries := sampleEntries()

	// Natural search should match by tag even without '#' prefix
	results := Query(entries, "ubuntu")
	if len(results) == 0 || results[0].Group.Name != "apt" {
		t.Fatalf("expected apt to rank top for query 'ubuntu', got %v", results)
	}

	results = Query(entries, "untar")
	if len(results) == 0 || results[0].Group.Name != "tar" {
		t.Fatalf("expected tar to rank top for query 'untar', got %v", results)
	}
}

func TestEmptyQuery(t *testing.T) {
	entries := sampleEntries()
	results := Query(entries, "")
	if len(results) != len(entries) {
		t.Fatalf("expected %d entries for empty query, got %d", len(entries), len(results))
	}
}

func BenchmarkQuery(b *testing.B) {
	// Generate 2000 simulated entries
	entries := make([]index.Entry, 2000)
	for i := 0; i < 2000; i++ {
		entries[i] = index.Entry{
			Group: config.Group{Name: "group"},
			Command: config.Command{
				Name:        "command",
				Description: "some description",
			},
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Query(entries, "com")
	}
}

