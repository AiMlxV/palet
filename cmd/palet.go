package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Thitipong-PP/palet/internal/index"
	"github.com/Thitipong-PP/palet/internal/tui"
)

func main() {
	tagFlag := flag.String("tag", "", "Filter commands by tag (e.g. --tag docker)")
	tFlag := flag.String("t", "", "Filter commands by tag (shorthand)")
	queryFlag := flag.String("query", "", "Initial search query")
	qFlag := flag.String("q", "", "Initial search query (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "PALET - Terminal command palette\n\nUsage:\n  palet [options] [query]\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	tag := *tagFlag
	if tag == "" {
		tag = *tFlag
	}

	query := *queryFlag
	if query == "" {
		query = *qFlag
	}
	if query == "" && flag.NArg() > 0 {
		query = flag.Arg(0)
	}

	entries := index.LoadCached()
	opts := tui.Options{
		InitialTag:   tag,
		InitialQuery: query,
	}

	if err := tui.StartWithOptions(entries, opts); err != nil {
		fmt.Fprintln(os.Stderr, "palet:", err)
		os.Exit(1)
	}
}
