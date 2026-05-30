package search

import (
	"strings"

	"github.com/Thitipong-PP/palet/internal/index"
)

const (
	scoreExactPrefix  = 100
	scorePluginPrefix = 90
	scoreNameContains = 70
	scoreDescContains = 50
	scorePluginName   = 40
	scoreFuzzy        = 20
)

// Query returns entries matching query, ranked by relevance.
// An empty query returns all entries unchanged.
func Query(entries []index.Entry, query string) []index.Entry {
	if query == "" {
		return entries
	}
	q := normalize(query)

	type hit struct {
		entry index.Entry
		score int
	}

	hits := make([]hit, 0, len(entries))
	for _, e := range entries {
		if s := score(e, q); s > 0 {
			hits = append(hits, hit{e, s})
		}
	}

	// insertion sort — list is small (hundreds of commands at most)
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].score > hits[j-1].score; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}

	out := make([]index.Entry, len(hits))
	for i, h := range hits {
		out[i] = h.entry
	}
	return out
}

func score(e index.Entry, q string) int {
	name := normalize(e.Command.Name)
	desc := normalize(e.Command.Description)
	plugin := normalize(e.Plugin.Name)
	combined := plugin + " " + name
	tokens := strings.Fields(q)
	lastToken := ""
	if len(tokens) > 0 {
		lastToken = tokens[len(tokens)-1]
	}
	if q == plugin {
		if name == plugin {
			return scorePluginName
		}
		if strings.HasPrefix(name, "sub") {
			return scoreExactPrefix + 30
		}
		return scoreExactPrefix + 10
	}
	if len(q) <= 1 {
		if strings.HasPrefix(name, q) {
			return scoreExactPrefix
		}
		if strings.HasPrefix(combined, q) {
			return scorePluginPrefix
		}
		return 0
	}
	if len(tokens) > 1 {
		if allTokensIn(name, tokens) && wordStartsWith(name, lastToken) {
			return scoreExactPrefix + 20
		}
		if allTokensIn(name, tokens) {
			return scoreExactPrefix + 5
		}
		if allTokensIn(combined, tokens) && wordStartsWith(combined, lastToken) {
			return scoreNameContains + 15
		}
		if allTokensIn(combined, tokens) {
			return scoreNameContains + 10
		}
	}

	switch {
	case strings.HasPrefix(name, q):
		return scoreExactPrefix
	case strings.HasPrefix(combined, q):
		return scorePluginPrefix
	case strings.HasPrefix(plugin+name, q):
		return scorePluginPrefix
	case anyTokenPrefixWord(name, tokens):
		return scoreNameContains + 5
	case anyTokenPrefixWord(combined, tokens):
		return scoreNameContains
	case strings.Contains(name, q):
		return scoreNameContains - 5
	case strings.Contains(combined, q):
		return scoreNameContains - 10
	case strings.Contains(desc, q):
		return scoreDescContains
	case strings.Contains(plugin, q):
		return scorePluginName
	case len(q) >= 3 && (fuzzy(name, q) || fuzzy(combined, q)):
		return scoreFuzzy
	}
	return 0
}

// fuzzy returns true if every rune of pattern appears in s in order.
func fuzzy(s, pattern string) bool {
	pi := 0
	for _, ch := range s {
		if pi < len(pattern) && ch == rune(pattern[pi]) {
			pi++
		}
	}
	return pi == len(pattern)
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer("_", " ", "-", " ", "/", " ", ".", " ")
	return replacer.Replace(s)
}

func allTokensIn(s string, tokens []string) bool {
	for _, t := range tokens {
		if t == "" {
			continue
		}
		if !strings.Contains(s, t) {
			return false
		}
	}
	return true
}

func anyTokenPrefixWord(s string, tokens []string) bool {
	words := strings.Fields(s)
	for _, t := range tokens {
		if t == "" {
			continue
		}
		for _, w := range words {
			if strings.HasPrefix(w, t) {
				return true
			}
		}
	}
	return false
}

func wordStartsWith(s string, token string) bool {
	if token == "" {
		return false
	}
	for _, w := range strings.Fields(s) {
		if strings.HasPrefix(w, token) {
			return true
		}
	}
	return false
}
