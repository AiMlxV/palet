package search

import (
	"sort"
	"strings"

	"github.com/Thitipong-PP/palet/internal/index"
)

const (
	scoreExactTag     = 120
	scorePrefixTag    = 100
	scoreExactPrefix  = 100
	scorePluginPrefix = 90
	scoreTagContains  = 75
	scoreNameContains = 70
	scoreDescContains = 50
	scorePluginName   = 40
	scoreTagFuzzy     = 25
	scoreFuzzy        = 20
)

// Query returns entries matching query, ranked by relevance.
// An empty query returns all entries unchanged.
func Query(entries []index.Entry, query string) []index.Entry {
	if query == "" {
		return entries
	}

	raw := strings.TrimSpace(query)
	var isExplicitTag bool
	var tagQuery string

	switch {
	case strings.HasPrefix(raw, "#"):
		isExplicitTag = true
		tagQuery = normalize(strings.TrimPrefix(raw, "#"))
	case strings.HasPrefix(strings.ToLower(raw), "tag:"):
		isExplicitTag = true
		tagQuery = normalize(strings.TrimPrefix(raw, "tag:"))
	case strings.HasPrefix(strings.ToLower(raw), "--tag "):
		isExplicitTag = true
		tagQuery = normalize(strings.TrimPrefix(raw, "--tag "))
	case strings.HasPrefix(strings.ToLower(raw), "-t "):
		isExplicitTag = true
		tagQuery = normalize(strings.TrimPrefix(raw, "-t "))
	}

	type hit struct {
		entry index.Entry
		score int
	}

	hits := make([]hit, 0, len(entries))

	if isExplicitTag {
		for _, e := range entries {
			s := scoreExplicitTag(e, tagQuery)
			if s > 0 {
				hits = append(hits, hit{e, s})
			}
		}
	} else {
		q := normalize(query)
		for _, e := range entries {
			if s := score(e, q); s > 0 {
				hits = append(hits, hit{e, s})
			}
		}
	}

	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].score > hits[j].score
	})

	out := make([]index.Entry, len(hits))
	for i, h := range hits {
		out[i] = h.entry
	}
	return out
}

func scoreExplicitTag(e index.Entry, tagQuery string) int {
	tags := e.AllTags()
	if len(tags) == 0 {
		return 0
	}
	if tagQuery == "" {
		// Just typed '#' or 'tag:' -> match all entries with tags
		return scoreTagContains
	}
	best := 0
	for _, t := range tags {
		norm := normalize(t)
		switch {
		case norm == tagQuery:
			return scoreExactTag
		case strings.HasPrefix(norm, tagQuery):
			if scorePrefixTag > best {
				best = scorePrefixTag
			}
		case strings.Contains(norm, tagQuery):
			if scoreTagContains > best {
				best = scoreTagContains
			}
		case len(tagQuery) >= 2 && fuzzy(norm, tagQuery):
			if scoreTagFuzzy > best {
				best = scoreTagFuzzy
			}
		}
	}
	return best
}

func score(e index.Entry, q string) int {
	name := normalize(e.Command.Name)
	desc := normalize(e.Command.Description)
	group := normalize(e.Group.Name)
	tags := e.AllTags()
	tagStr := ""
	if len(tags) > 0 {
		normTags := make([]string, len(tags))
		for i, t := range tags {
			normTags[i] = normalize(t)
		}
		tagStr = strings.Join(normTags, " ")
	}

	combined := group + " " + name
	combinedWithTags := combined
	if tagStr != "" {
		combinedWithTags += " " + tagStr
	}

	tokens := strings.Fields(q)
	lastToken := ""
	if len(tokens) > 0 {
		lastToken = tokens[len(tokens)-1]
	}
	if q == group {
		if name == group {
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
		if tagStr != "" && strings.HasPrefix(tagStr, q) {
			return scoreTagContains
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
		if allTokensIn(combinedWithTags, tokens) {
			return scoreNameContains + 5
		}
	}

	// Check direct tag match in natural search
	tagScore := 0
	for _, t := range tags {
		norm := normalize(t)
		if norm == q {
			tagScore = scoreExactPrefix - 5
			break
		} else if strings.HasPrefix(norm, q) {
			if scoreTagContains+10 > tagScore {
				tagScore = scoreTagContains + 10
			}
		} else if strings.Contains(norm, q) {
			if scoreTagContains > tagScore {
				tagScore = scoreTagContains
			}
		}
	}

	switch {
	case strings.HasPrefix(name, q):
		return scoreExactPrefix
	case strings.HasPrefix(combined, q):
		return scorePluginPrefix
	case strings.HasPrefix(group+name, q):
		return scorePluginPrefix
	case tagScore > 0:
		return tagScore
	case anyTokenPrefixWord(name, tokens):
		return scoreNameContains + 5
	case anyTokenPrefixWord(combined, tokens):
		return scoreNameContains
	case anyTokenPrefixWord(combinedWithTags, tokens):
		return scoreNameContains - 2
	case strings.Contains(name, q):
		return scoreNameContains - 5
	case strings.Contains(combined, q):
		return scoreNameContains - 10
	case strings.Contains(desc, q):
		return scoreDescContains
	case strings.Contains(group, q):
		return scorePluginName
	case len(q) >= 3 && (fuzzy(name, q) || fuzzy(combined, q) || (tagStr != "" && fuzzy(tagStr, q))):
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
