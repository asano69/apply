package searchreplace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// pyWhitespace mirrors the character set Python's str.strip()/lstrip() trim by default.
const pyWhitespace = " \t\n\r\f\v"

// prep ensures content ends with a newline and splits it into lines (keeping line endings).
func prep(content string) (string, []string) {
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return content, splitLinesKeepEnds(content)
}

func linesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func perfectReplace(wholeLines, partLines, replaceLines []string) (string, bool) {
	partLen := len(partLines)
	for i := 0; i+partLen <= len(wholeLines); i++ {
		if linesEqual(wholeLines[i:i+partLen], partLines) {
			result := append([]string{}, wholeLines[:i]...)
			result = append(result, replaceLines...)
			result = append(result, wholeLines[i+partLen:]...)
			return strings.Join(result, ""), true
		}
	}
	return "", false
}

func matchButForLeadingWhitespace(wholeLines, partLines []string) (string, bool) {
	num := len(wholeLines)

	for i := 0; i < num; i++ {
		if strings.TrimLeft(wholeLines[i], pyWhitespace) != strings.TrimLeft(partLines[i], pyWhitespace) {
			return "", false
		}
	}

	adds := map[string]bool{}
	for i := 0; i < num; i++ {
		if strings.TrimSpace(wholeLines[i]) == "" {
			continue
		}
		prefixLen := len(wholeLines[i]) - len(partLines[i])
		if prefixLen < 0 || prefixLen > len(wholeLines[i]) {
			return "", false
		}
		adds[wholeLines[i][:prefixLen]] = true
	}

	if len(adds) != 1 {
		return "", false
	}
	for k := range adds {
		return k, true
	}
	return "", false
}

func leadingWhitespaceLen(s string) int {
	trimmed := strings.TrimLeft(s, pyWhitespace)
	return len(s) - len(trimmed)
}

func outdent(lines []string, n int) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		if strings.TrimSpace(l) == "" || len(l) < n {
			out[i] = l
			continue
		}
		out[i] = l[n:]
	}
	return out
}

// replacePartWithMissingLeadingWhitespace handles the common case where an
// LLM uniformly drops (or partially drops) leading indentation in both the
// SEARCH and REPLACE sections.
func replacePartWithMissingLeadingWhitespace(wholeLines, partLines, replaceLines []string) (string, bool) {
	minLeading := -1
	for _, l := range partLines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if n := leadingWhitespaceLen(l); minLeading == -1 || n < minLeading {
			minLeading = n
		}
	}
	for _, l := range replaceLines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if n := leadingWhitespaceLen(l); minLeading == -1 || n < minLeading {
			minLeading = n
		}
	}

	if minLeading > 0 {
		partLines = outdent(partLines, minLeading)
		replaceLines = outdent(replaceLines, minLeading)
	}

	numPartLines := len(partLines)
	for i := 0; i+numPartLines <= len(wholeLines); i++ {
		addLeading, ok := matchButForLeadingWhitespace(wholeLines[i:i+numPartLines], partLines)
		if !ok {
			continue
		}

		adjusted := make([]string, len(replaceLines))
		for j, l := range replaceLines {
			if strings.TrimSpace(l) != "" {
				adjusted[j] = addLeading + l
			} else {
				adjusted[j] = l
			}
		}

		result := append([]string{}, wholeLines[:i]...)
		result = append(result, adjusted...)
		result = append(result, wholeLines[i+numPartLines:]...)
		return strings.Join(result, ""), true
	}

	return "", false
}

func perfectOrWhitespace(wholeLines, partLines, replaceLines []string) (string, bool) {
	if result, ok := perfectReplace(wholeLines, partLines, replaceLines); ok {
		return result, true
	}
	return replacePartWithMissingLeadingWhitespace(wholeLines, partLines, replaceLines)
}

var dotsRe = regexp.MustCompile(`(?m)(^\s*\.\.\.\n)`)

// splitKeepDelim mirrors Python's re.split() with a single capturing group:
// the delimiter matches are kept in the result, alternating with the text
// in between them.
func splitKeepDelim(re *regexp.Regexp, s string) []string {
	matches := re.FindAllStringIndex(s, -1)
	if matches == nil {
		return []string{s}
	}
	var result []string
	last := 0
	for _, m := range matches {
		result = append(result, s[last:m[0]])
		result = append(result, s[m[0]:m[1]])
		last = m[1]
	}
	result = append(result, s[last:])
	return result
}

// tryDotDotDots handles SEARCH/REPLACE blocks that elide unchanged code with
// "..." lines. Returns ok=false (no error) if the block has no "..." lines at
// all, so the caller can fall through to other strategies.
func tryDotDotDots(whole, part, replace string) (string, bool, error) {
	partPieces := splitKeepDelim(dotsRe, part)
	replacePieces := splitKeepDelim(dotsRe, replace)

	if len(partPieces) != len(replacePieces) {
		return "", false, fmt.Errorf("unpaired ... in SEARCH/REPLACE block")
	}

	if len(partPieces) == 1 {
		return "", false, nil
	}

	for i := 1; i < len(partPieces); i += 2 {
		if partPieces[i] != replacePieces[i] {
			return "", false, fmt.Errorf("unmatched ... in SEARCH/REPLACE block")
		}
	}

	var partChunks, replaceChunks []string
	for i := 0; i < len(partPieces); i += 2 {
		partChunks = append(partChunks, partPieces[i])
	}
	for i := 0; i < len(replacePieces); i += 2 {
		replaceChunks = append(replaceChunks, replacePieces[i])
	}

	result := whole
	for idx := range partChunks {
		partChunk := partChunks[idx]
		replaceChunk := replaceChunks[idx]

		if partChunk == "" && replaceChunk == "" {
			continue
		}

		if partChunk == "" && replaceChunk != "" {
			if !strings.HasSuffix(result, "\n") {
				result += "\n"
			}
			result += replaceChunk
			continue
		}

		count := strings.Count(result, partChunk)
		if count != 1 {
			return "", false, fmt.Errorf("SEARCH chunk is not uniquely present in the file")
		}

		result = strings.Replace(result, partChunk, replaceChunk, 1)
	}

	return result, true, nil
}

// replaceMostSimilarChunk finds `part` inside `whole` (via exact,
// whitespace-tolerant, "..." elided, or fuzzy matching, in that order) and
// replaces it with `replace`. Returns ok=false if nothing matched well enough.
func replaceMostSimilarChunk(whole, part, replace string) (string, bool) {
	whole, wholeLines := prep(whole)
	part, partLines := prep(part)
	_, replaceLines := prep(replace)

	if result, ok := perfectOrWhitespace(wholeLines, partLines, replaceLines); ok {
		return result, true
	}

	// Drop leading empty line: LLMs sometimes add one spuriously.
	if len(partLines) > 2 && strings.TrimSpace(partLines[0]) == "" {
		if result, ok := perfectOrWhitespace(wholeLines, partLines[1:], replaceLines); ok {
			return result, true
		}
	}

	// Try to handle blocks that elide unchanged code with "...".
	if result, ok, err := tryDotDotDots(whole, part, replace); err == nil && ok {
		return result, true
	}

	// Last resort: fuzzy matching.
	if result := replaceClosestEditDistance(wholeLines, part, partLines, replaceLines); result != "" {
		return result, true
	}

	return "", false
}

// stripQuotedWrapping removes an LLM's extraneous filename line and/or
// fenced-code wrapping around the actual SEARCH/REPLACE text.
func stripQuotedWrapping(text, path string, fence Fence) string {
	if text == "" {
		return text
	}

	lines := strings.Split(text, "\n")

	if path != "" && len(lines) > 0 && strings.HasSuffix(strings.TrimSpace(lines[0]), filepath.Base(path)) {
		lines = lines[1:]
	}

	if len(lines) > 0 &&
		strings.HasPrefix(lines[0], fence.Open) &&
		strings.HasPrefix(lines[len(lines)-1], fence.Close) {
		lines = lines[1 : len(lines)-1]
	}

	result := strings.Join(lines, "\n")
	if result != "" && !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result
}

// doReplace applies one edit against a file's content. hasContent
// distinguishes "file doesn't exist and we're not creating it" (ok=false,
// no newContent) from every other outcome.
func doReplace(path string, content *string, beforeText, afterText string, fence Fence) (newContent string, ok bool) {
	beforeText = stripQuotedWrapping(beforeText, path, fence)
	afterText = stripQuotedWrapping(afterText, path, fence)

	if content == nil {
		if _, err := os.Stat(path); os.IsNotExist(err) && strings.TrimSpace(beforeText) == "" {
			// Create a new, empty file; content becomes "".
			f, ferr := os.Create(path)
			if ferr == nil {
				f.Close()
			}
			empty := ""
			content = &empty
		} else {
			return "", false
		}
	}

	if strings.TrimSpace(beforeText) == "" {
		// Append to an existing file, or populate a newly created one.
		return *content + afterText, true
	}

	return replaceMostSimilarChunk(*content, beforeText, afterText)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func resolvePath(root, path string) (string, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}

	var candidate string
	if filepath.IsAbs(path) {
		candidate = filepath.Clean(path)
	} else {
		candidate = filepath.Join(rootAbs, path)
	}

	rel, err := filepath.Rel(rootAbs, candidate)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", &PathEscapeError{Msg: fmt.Sprintf(
			"Refusing to edit path '%s' because it resolves outside root '%s'.", path, rootAbs)}
	}

	return candidate, nil
}

func relativeTo(path, root string) string {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(rootAbs, path)
	if err != nil {
		return path
	}
	return rel
}

// ApplyOptions configures ApplyEdits / ApplyDiff.
type ApplyOptions struct {
	// ChatFiles are extra files that failed edits may be retried against,
	// e.g. the single file scripts/apply was invoked with.
	ChatFiles []string
	Fence     Fence
	// DryRun validates that edits would apply without writing anything to disk.
	DryRun bool
}

// ApplyEdits applies a set of already-parsed edits to files under root.
func ApplyEdits(edits []EditBlock, root string, opts ApplyOptions) (ApplyResult, error) {
	fence := opts.Fence
	if fence.Open == "" {
		fence = DefaultFence
	}

	var fallbackFiles []string
	for _, cf := range opts.ChatFiles {
		resolved, err := resolvePath(root, cf)
		if err != nil {
			return ApplyResult{}, err
		}
		fallbackFiles = append(fallbackFiles, resolved)
	}

	var failed, passed, updatedEdits []EditBlock

	for _, edit := range edits {
		path := edit.Path
		original := edit.Original
		updated := edit.Updated

		fullPath, err := resolvePath(root, path)
		if err != nil {
			return ApplyResult{}, err
		}

		var newContent string
		var applied bool

		if fileExists(fullPath) {
			data, rerr := os.ReadFile(fullPath)
			if rerr != nil {
				return ApplyResult{}, rerr
			}
			s := string(data)
			newContent, applied = doReplace(fullPath, &s, original, updated, fence)
		} else if strings.TrimSpace(original) == "" {
			newContent, applied = doReplace(fullPath, nil, original, updated, fence)
		}

		// If that failed and this isn't a "create new file" edit, try any
		// other files that were added to the chat.
		// https://github.com/Aider-AI/aider/issues/2258
		if !applied && strings.TrimSpace(original) != "" {
			for _, candidate := range fallbackFiles {
				data, rerr := os.ReadFile(candidate)
				if rerr != nil {
					continue
				}
				s := string(data)
				content, ok := doReplace(candidate, &s, original, updated, fence)
				if ok {
					newContent = content
					applied = true
					path = relativeTo(candidate, root)
					fullPath = candidate
					break
				}
			}
		}

		updatedEdits = append(updatedEdits, EditBlock{Path: path, Original: original, Updated: updated})

		if applied {
			if !opts.DryRun {
				if err := os.WriteFile(fullPath, []byte(newContent), 0o644); err != nil {
					return ApplyResult{}, err
				}
			}
			passed = append(passed, edit)
		} else {
			failed = append(failed, edit)
		}
	}

	if len(failed) == 0 {
		return ApplyResult{UpdatedEdits: updatedEdits}, nil
	}

	return ApplyResult{}, buildApplyError(root, failed, passed, updatedEdits, fence, opts.DryRun)
}

func buildApplyError(root string, failed, passed, updatedEdits []EditBlock, fence Fence, dryRun bool) error {
	var b strings.Builder

	fmt.Fprintf(&b, "# %d SEARCH/REPLACE %s failed to match!\n", len(failed), wordForm(len(failed)))

	for _, edit := range failed {
		var content string
		if fullPath, err := resolvePath(root, edit.Path); err == nil {
			if data, rerr := os.ReadFile(fullPath); rerr == nil {
				content = string(data)
			}
		}

		fmt.Fprintf(&b, "\n## SearchReplaceNoExactMatch: This SEARCH block failed to exactly match lines in %s\n", edit.Path)
		fmt.Fprintf(&b, "<<<<<<< SEARCH\n%s=======\n%s>>>>>>> REPLACE\n\n", edit.Original, edit.Updated)

		if didYouMean := findSimilarLines(edit.Original, content); didYouMean != "" {
			fmt.Fprintf(&b, "Did you mean to match some of these actual lines from %s?\n\n%s\n%s\n%s\n\n",
				edit.Path, fence.Open, didYouMean, fence.Close)
		}

		if edit.Updated != "" && strings.Contains(content, edit.Updated) {
			fmt.Fprintf(&b, "Are you sure you need this SEARCH/REPLACE block?\nThe REPLACE lines are already in %s!\n\n", edit.Path)
		}
	}

	b.WriteString("The SEARCH section must exactly match an existing block of lines including all white space, comments, indentation, docstrings, etc\n")

	if len(passed) > 0 {
		label := wordForm(len(passed))
		if dryRun {
			fmt.Fprintf(&b, "\n# The other %d SEARCH/REPLACE %s would apply successfully.\n", len(passed), label)
		} else {
			fmt.Fprintf(&b, "\n# The other %d SEARCH/REPLACE %s were applied successfully.\nDon't re-send them.\nJust reply with fixed versions of the %s above that failed to match.\n",
				len(passed), label, wordForm(len(failed)))
		}
	}

	return &ApplyError{
		Message:      b.String(),
		Failed:       failed,
		Passed:       passed,
		UpdatedEdits: updatedEdits,
	}
}

// ApplyDiff parses SEARCH/REPLACE blocks out of llmResponse and applies them
// to files under root. It is the Go equivalent of search_replace.apply_diff.
func ApplyDiff(llmResponse, root string, opts ApplyOptions) (ApplyResult, error) {
	fence := opts.Fence
	if fence.Open == "" {
		fence = DefaultFence
		opts.Fence = fence
	}

	parsed, err := ParseEditBlocks(llmResponse, fence)
	if err != nil {
		return ApplyResult{}, err
	}
	if len(parsed.Edits) == 0 {
		return ApplyResult{}, &ParseError{Msg: "No SEARCH/REPLACE blocks found in the LLM response."}
	}

	return ApplyEdits(parsed.Edits, root, opts)
}
