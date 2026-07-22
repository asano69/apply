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

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Failed to read stdin:", err)
		os.Exit(1)
	}
	diffText := strings.TrimSpace(string(data))

	chatFiles := os.Args[1:]

	var llmResponse string
	if len(chatFiles) > 0 {
		if len(chatFiles) != 1 {
			fmt.Fprintln(os.Stderr, "Usage: apply <file>")
			fmt.Fprintln(os.Stderr, "Example:")
			fmt.Fprintln(os.Stderr, "wl-paste | scripts-go/apply mathweb/flask/app.py")
			os.Exit(2)
		}
		path := chatFiles[0]
		llmResponse = path + "\n```\n" + diffText + "\n```\n"
	} else {
		llmResponse = diffText
	}

	result, err := sr.ApplyDiff(llmResponse, ".", sr.ApplyOptions{ChatFiles: chatFiles})
	if err != nil {
		handleError(err, chatFiles)
		os.Exit(1)
	}

	for _, edit := range result.UpdatedEdits {
		fmt.Printf("Applied edit to %s\n", edit.Path)
	}
}

func handleError(err error, chatFiles []string) {
	if parseErr, ok := err.(*sr.ParseError); ok {
		fmt.Fprintln(os.Stderr, "\nFailed to parse SEARCH/REPLACE block.")
		fmt.Fprintln(os.Stderr, "\nExpected input:")
		fmt.Fprintln(os.Stderr, "<<<<<<< SEARCH")
		fmt.Fprintln(os.Stderr, "old text")
		fmt.Fprintln(os.Stderr, "=======")
		fmt.Fprintln(os.Stderr, "new text")
		fmt.Fprintln(os.Stderr, ">>>>>>> REPLACE")

		if len(chatFiles) == 0 {
			fmt.Fprintln(os.Stderr, "When no filename argument is supplied, the input must also contain a filename.")
		}

		fmt.Fprintf(os.Stderr, "\nOriginal error: %s\n", parseErr.Error())
		return
	}

	fmt.Fprintf(os.Stderr, "Failed to apply diff: %s\n", err.Error())
}
