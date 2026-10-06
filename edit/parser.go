package edit

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var (
	headPattern     = regexp.MustCompile(`^<{5,9} SEARCH>?\s*$`)
	dividerPattern  = regexp.MustCompile(`^={5,9}\s*$`)
	updatedPattern  = regexp.MustCompile(`^>{5,9} REPLACE\s*$`)
	tripleBackticks = "```"

	// pathNotes are trailing annotations that LLMs append to a path line,
	// e.g. "(new file)". Add a pattern here to support a new notation.
	pathNotes = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\s*[(（](new file|new|新規ファイル|新規)[)）]\s*$`),
	}
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

// lineKind is the role a single line plays in an LLM response.
type lineKind int

const (
	kindText    lineKind = iota // prose, a path line, or code content
	kindFence                   // opening or closing code fence
	kindSearch                  // "<<<<<<< SEARCH"
	kindDivider                 // "======="
	kindReplace                 // ">>>>>>> REPLACE"
)

// classify decides what kind of line this is.
func classify(line string, fence Fence) lineKind {
	t := strings.TrimSpace(line)
	switch {
	case headPattern.MatchString(t):
		return kindSearch
	case dividerPattern.MatchString(t):
		return kindDivider
	case updatedPattern.MatchString(t):
		return kindReplace
	case strings.HasPrefix(t, fence.Open) || strings.HasPrefix(t, tripleBackticks):
		return kindFence
	}
	return kindText
}

// parsePath returns the file path on a line after removing notes such as
// "(new file)", markdown decoration and a trailing ":". It returns "" if the
// line does not look like a bare path (no spaces, contains "." or "/").
func parsePath(line string) string {
	// Strip the trailing ":" first: pathNotes are anchored to the end of the line.
	s := strings.TrimSuffix(strings.TrimSpace(line), ":")
	for _, re := range pathNotes {
		s = re.ReplaceAllString(s, "")
	}
	s = strings.TrimSpace(strings.TrimLeft(s, "#"))
	s = strings.Trim(s, "`*")

	if s == "" || strings.ContainsAny(s, " \t") || !strings.ContainsAny(s, "./") {
		return ""
	}
	return s
}

// blockParser walks the lines of a response and collects edit blocks.
type blockParser struct {
	lines []string
	pos   int
	fence Fence
	edits []EditBlock

	// candidate is the path on the line just before the current position.
	candidate string
	// current is the path of the last edit; blocks without their own path reuse it.
	current string
}

// findOriginalUpdateBlocks scans content for SEARCH/REPLACE blocks and
// new-file code blocks.
func findOriginalUpdateBlocks(content string, fence Fence) ([]EditBlock, error) {
	p := &blockParser{lines: splitLinesKeepEnds(content), fence: fence}

	for p.pos < len(p.lines) {
		line := p.lines[p.pos]
		p.pos++

		switch classify(line, fence) {
		case kindSearch:
			if err := p.readBlock(); err != nil {
				return nil, err
			}
		case kindFence:
			p.readNewFile()
		case kindText:
			// Blank and prose lines reset the candidate, so a path only counts
			// when it sits directly before a block or a fence.
			p.candidate = parsePath(line)
		}
	}

	return p.edits, nil
}

// readBlock reads a SEARCH/REPLACE block; the SEARCH line is already consumed.
func (p *blockParser) readBlock() error {
	path := p.candidate
	if path == "" {
		path = p.current
	}
	if path == "" {
		return &ParseError{Msg: fmt.Sprintf(missingFile, p.fence.Open)}
	}

	original, ok := p.readUntil(kindDivider)
	if !ok {
		return p.unexpectedEnd(fmt.Sprintf("`%s`", dividerErr))
	}

	updated, ok := p.readUntil(kindReplace, kindDivider)
	if !ok {
		return p.unexpectedEnd(fmt.Sprintf("`%s` or `%s`", updatedErr, dividerErr))
	}

	p.addEdit(path, original, updated)
	return nil
}

// readNewFile handles an opening fence directly after a path line. If the
// fence holds a SEARCH block it is left to the main loop; otherwise the whole
// fenced body is the content of a new file. The fence line is already consumed.
func (p *blockParser) readNewFile() {
	if p.candidate == "" || p.nextKind() == kindSearch {
		return
	}

	body, _ := p.readUntil(kindFence)
	p.addEdit(p.candidate, "", body)
}

// nextKind returns the kind of the next unread line.
func (p *blockParser) nextKind() lineKind {
	if p.pos >= len(p.lines) {
		return kindText
	}
	return classify(p.lines[p.pos], p.fence)
}

// readUntil collects lines up to the first line whose kind is in stop and
// consumes that line. It reports false if the input ends first.
func (p *blockParser) readUntil(stop ...lineKind) (string, bool) {
	var buf []string
	for p.pos < len(p.lines) {
		line := p.lines[p.pos]
		p.pos++
		if slices.Contains(stop, classify(line, p.fence)) {
			return strings.Join(buf, ""), true
		}
		buf = append(buf, line)
	}
	return strings.Join(buf, ""), false
}

func (p *blockParser) addEdit(path, original, updated string) {
	p.edits = append(p.edits, EditBlock{Path: path, Original: original, Updated: updated})
	p.current = path
	p.candidate = ""
}

func (p *blockParser) unexpectedEnd(expected string) error {
	return &ParseError{Msg: fmt.Sprintf("%s\n^^^ Expected %s", strings.Join(p.lines, ""), expected)}
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
