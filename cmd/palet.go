package main

import (
	"fmt"
	"os"

	"github.com/Thitipong-PP/palet/internal/index"
	"github.com/Thitipong-PP/palet/internal/tui"
)

func main() {
	entries := index.LoadCached()
	if err := tui.Start(entries); err != nil {
		fmt.Fprintln(os.Stderr, "palet:", err)
		os.Exit(1)
	}
}
