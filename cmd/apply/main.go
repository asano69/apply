// Command apply reads an LLM's SEARCH/REPLACE response from stdin and
// applies it to the files on disk. It is the Go port of scripts/apply.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	sr "search-replace-go/searchreplace"
)

const helpText = `apply - apply SEARCH/REPLACE blocks from stdin to files under the current directory.

Usage:
  wl-paste | apply
  apply -h | --help

Each SEARCH block must be preceded by the filename it applies to,
alone on its own line. apply takes no other arguments.
`

// stdinIsTerminal reports whether stdin is an interactive terminal,
// i.e. nothing has been piped into apply.
func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func main() {
	// The diff is read from stdin only. Each SEARCH block must be preceded
	// by the filename it applies to.
	if len(os.Args) > 1 {
		if arg := os.Args[1]; len(os.Args) == 2 && (arg == "-h" || arg == "--help") {
			fmt.Print(helpText)
			return
		}
		fmt.Fprintf(os.Stderr, "Unexpected arguments: %s\n\n%s", strings.Join(os.Args[1:], " "), helpText)
		os.Exit(2)
	}

	if stdinIsTerminal() {
		fmt.Print(helpText)
		return
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read stdin:", err)
		os.Exit(1)
	}

	result, err := sr.ApplyDiff(strings.TrimSpace(string(data)), ".", sr.ApplyOptions{})
	if err != nil {
		handleError(err)
		os.Exit(1)
	}

	for _, edit := range result.UpdatedEdits {
		printEditSummary(edit)
	}
}

// countLines returns the number of lines in s, treating a trailing newline
// as ending the last line rather than starting an extra empty one.
func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}

// printEditSummary logs what changed for a single applied edit: the file
// path plus how many lines were removed and added.
func printEditSummary(edit sr.EditBlock) {
	added := countLines(edit.Updated)
	if strings.TrimSpace(edit.Original) == "" {
		fmt.Printf("Created %s (+%d lines)\n", edit.Path, added)
		return
	}
	removed := countLines(edit.Original)
	fmt.Printf("Applied edit to %s (-%d/+%d lines)\n", edit.Path, removed, added)
}

func handleError(err error) {
	if parseErr, ok := err.(*sr.ParseError); ok {
		fmt.Fprintln(os.Stderr, "\nFailed to parse SEARCH/REPLACE block.")
		fmt.Fprintln(os.Stderr, "\nExpected input:")
		fmt.Fprintln(os.Stderr, "<<<<<<< SEARCH")
		fmt.Fprintln(os.Stderr, "old text")
		fmt.Fprintln(os.Stderr, "=======")
		fmt.Fprintln(os.Stderr, "new text")
		fmt.Fprintln(os.Stderr, ">>>>>>> REPLACE")
		fmt.Fprintln(os.Stderr, "\nThe filename must be alone on the line before each SEARCH block.")

		fmt.Fprintf(os.Stderr, "\nOriginal error: %s\n", parseErr.Error())
		return
	}

	fmt.Fprintf(os.Stderr, "Failed to apply diff: %s\n", err.Error())
}
