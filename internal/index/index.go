package index

import (
	"github.com/Thitipong-PP/palet/internal/config"
)

// Entry is one searchable, flat command record.
type Entry struct {
	Group   config.Group
	Command config.Command
}

// Build flattens a slice of groups into a single Entry slice.
// Allocation is done once up front.
func Build(groups []config.Group) []Entry {
	total := 0
	for i := range groups {
		total += len(groups[i].Commands)
	}
	out := make([]Entry, 0, total)
	for _, g := range groups {
		for _, c := range g.Commands {
			out = append(out, Entry{Group: g, Command: c})
		}
	}
	return out
}

// AllTags returns unique, non-empty tags from both Command and Group.
func (e Entry) AllTags() []string {
	seen := make(map[string]struct{})
	var tags []string
	for _, t := range append(e.Command.Tags, e.Group.Tags...) {
		clean := t
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; !ok {
			seen[clean] = struct{}{}
			tags = append(tags, clean)
		}
	}
	return tags
}
