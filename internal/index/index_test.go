package index

import (
	"reflect"
	"testing"

	"github.com/Thitipong-PP/palet/internal/config"
)

func TestEntryAllTags(t *testing.T) {
	e := Entry{
		Group: config.Group{
			Name: "apt",
			Tags: []string{"debian", "ubuntu", "package"},
		},
		Command: config.Command{
			Name: "install",
			Tags: []string{"package", "install", "debian"},
		},
	}

	tags := e.AllTags()
	expected := []string{"package", "install", "debian", "ubuntu"}
	if !reflect.DeepEqual(tags, expected) {
		t.Fatalf("expected tags %v, got %v", expected, tags)
	}
}
