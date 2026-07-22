package searchreplace

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	headPattern     = regexp.MustCompile(`^<{5,9} SEARCH>?\s*$`)
	dividerPattern  = regexp.MustCompile(`^={5,9}\s*$`)
	updatedPattern  = regexp.MustCompile(`^>{5,9} REPLACE\s*$`)
	tripleBackticks = "```"
)

const (
	headErr     = "<<<<<<< SEARCH"
	dividerErr  = "======="
	updatedErr  = ">>>>>>> REPLACE"
	missingFile = "Bad/missing filename. The filename must be alone on the line before the opening fence %s"
)

// splitLinesKeepEnds splits s into lines, keeping the trailing "\n" on each
// line (equivalent to Python's str.splitlines(keepends=True)).
func splitLinesKeepEnds(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// stripFilename cleans up a candidate filename line, stripping fences,
// leading "#", trailing ":", and surrounding backticks/asterisks.
func stripFilename(filename string, fence Fence) string {
	filename = strings.TrimSpace(filename)
	if filename == "..." {
		return ""
	}

	if fence.Open != "" && strings.HasPrefix(filename, fence.Open) {
		candidate := filename[len(fence.Open):]
		if candidate != "" && (strings.Contains(candidate, ".") || strings.Contains(candidate, "/")) {
			return candidate
		}
		return ""
	}

	if strings.HasPrefix(filename, tripleBackticks) {
		candidate := filename[len(tripleBackticks):]
		if candidate != "" && (strings.Contains(candidate, ".") || strings.Contains(candidate, "/")) {
			return candidate
		}
		return ""
	}

	filename = strings.TrimSuffix(filename, ":")
	filename = strings.TrimPrefix(filename, "#")
	filename = strings.TrimSpace(filename)
	filename = strings.Trim(filename, "`")
	filename = strings.Trim(filename, "*")
	return filename
}

// findFilename inspects up to the 3 lines preceding a SEARCH block to
// discover the file path the block applies to.
//
// Note: unlike search_replace-py's find_original_update_blocks, this never
// receives a list of "valid" chat filenames to fuzzy-match against, because
// apply_diff (the only entry point scripts/apply uses) never supplies one.
// That fuzzy-matching branch is therefore omitted here for simplicity.
func findFilename(precedingLines []string, fence Fence) string {
	start := 0
	if len(precedingLines) > 3 {
		start = len(precedingLines) - 3
	}

	var filenames []string
	for i := len(precedingLines) - 1; i >= start; i-- {
		line := precedingLines[i]
		if fname := stripFilename(line, fence); fname != "" {
			filenames = append(filenames, fname)
		}
		// Only keep looking further back as long as we keep seeing fences.
		if !strings.HasPrefix(line, fence.Open) && !strings.HasPrefix(line, tripleBackticks) {
			break
		}
	}

	if len(filenames) == 0 {
		return ""
	}

	// Prefer a candidate that looks like it has a file extension.
	for _, f := range filenames {
		if strings.Contains(f, ".") {
			return f
		}
	}

	return filenames[0]
}

// findOriginalUpdateBlocks scans content for SEARCH/REPLACE blocks.
func findOriginalUpdateBlocks(content string, fence Fence) ([]EditBlock, error) {
	lines := splitLinesKeepEnds(content)

	var edits []EditBlock
	var currentFilename string

	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])

		if headPattern.MatchString(trimmed) {
			windowStart := i - 3
			if windowStart < 0 {
				windowStart = 0
			}
			filename := findFilename(lines[windowStart:i], fence)

			if filename == "" {
				if currentFilename != "" {
					filename = currentFilename
				} else {
					return nil, &ParseError{Msg: fmt.Sprintf(missingFile, fence.Open)}
				}
			}
			currentFilename = filename

			var original, updated []string

			i++
			for i < len(lines) && !dividerPattern.MatchString(strings.TrimSpace(lines[i])) {
				original = append(original, lines[i])
				i++
			}
			if i >= len(lines) {
				return nil, &ParseError{Msg: fmt.Sprintf("%s\n^^^ Expected `%s`", strings.Join(lines[:i], ""), dividerErr)}
			}

			i++
			for i < len(lines) &&
				!updatedPattern.MatchString(strings.TrimSpace(lines[i])) &&
				!dividerPattern.MatchString(strings.TrimSpace(lines[i])) {
				updated = append(updated, lines[i])
				i++
			}
			if i >= len(lines) {
				return nil, &ParseError{Msg: fmt.Sprintf("%s\n^^^ Expected `%s` or `%s`", strings.Join(lines[:i], ""), updatedErr, dividerErr)}
			}

			edits = append(edits, EditBlock{
				Path:     filename,
				Original: strings.Join(original, ""),
				Updated:  strings.Join(updated, ""),
			})
		}

		i++
	}

	return edits, nil
}

// ParseEditBlocks parses all SEARCH/REPLACE blocks out of content.
func ParseEditBlocks(content string, fence Fence) (ParseResult, error) {
	if fence.Open == "" {
		fence = DefaultFence
	}
	edits, err := findOriginalUpdateBlocks(content, fence)
	if err != nil {
		return ParseResult{}, err
	}
	return ParseResult{Edits: edits}, nil
}
