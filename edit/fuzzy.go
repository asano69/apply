package edit

import (
	"math"
	"runtime"
	"strings"
	"sync"
)

// ratio approximates Python's difflib.SequenceMatcher(None, a, b).ratio()
// using a longest-common-subsequence based similarity: 2*LCS / (len(a)+len(b)).
func ratio(a, b string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	ra, rb := []rune(a), []rune(b)
	lcs := lcsLen(ra, rb)
	return 2.0 * float64(lcs) / float64(len(ra)+len(rb))
}

// ratioLines is the line-sequence equivalent of ratio, comparing whole lines
// as atomic elements (mirroring SequenceMatcher over a list of lines).
func ratioLines(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	lcs := lcsLenStrings(a, b)
	return 2.0 * float64(lcs) / float64(len(a)+len(b))
}

func lcsLen(a, b []rune) int {
	n, m := len(a), len(b)
	prev := make([]int, m+1)
	curr := make([]int, m+1)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] >= curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
	}
	return prev[m]
}

func lcsLenStrings(a, b []string) int {
	n, m := len(a), len(b)
	prev := make([]int, m+1)
	curr := make([]int, m+1)
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if a[i-1] == b[j-1] {
				curr[j] = prev[j-1] + 1
			} else if prev[j] >= curr[j-1] {
				curr[j] = prev[j]
			} else {
				curr[j] = curr[j-1]
			}
		}
		prev, curr = curr, prev
	}
	return prev[m]
}

// replaceClosestEditDistance is the last-resort fuzzy replacement strategy:
// it searches whole_lines for the chunk most similar to part (within +/-10%
// of part's line count) and swaps it for replace_lines if similar enough.
func replaceClosestEditDistance(wholeLines []string, part string, partLines []string, replaceLines []string) string {
	const similarityThresh = 0.8

	maxSimilarity := 0.0
	chunkStart, chunkEnd := -1, -1

	const scale = 0.1
	minLen := int(math.Floor(float64(len(partLines)) * (1 - scale)))
	maxLen := int(math.Ceil(float64(len(partLines)) * (1 + scale)))

	// Enumerate candidates in the original order (length, then start) so the
	// tie-break below stays deterministic.
	var windows [][2]int
	for length := minLen; length < maxLen; length++ {
		if length <= 0 {
			continue
		}
		for i := 0; i+length <= len(wholeLines); i++ {
			windows = append(windows, [2]int{i, i + length})
		}
	}

	// Score every window concurrently; each goroutine writes only its own slot.
	scores := make([]float64, len(windows))
	sem := make(chan struct{}, runtime.GOMAXPROCS(0))
	var wg sync.WaitGroup
	for idx, w := range windows {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			scores[idx] = ratio(strings.Join(wholeLines[w[0]:w[1]], ""), part)
		}()
	}
	wg.Wait()

	// Reduce sequentially: the first window with the highest score wins.
	for idx, sim := range scores {
		if sim > maxSimilarity && sim > 0 {
			maxSimilarity = sim
			chunkStart = windows[idx][0]
			chunkEnd = windows[idx][1]
		}
	}

	if maxSimilarity < similarityThresh {
		return ""
	}

	result := append([]string{}, wholeLines[:chunkStart]...)
	result = append(result, replaceLines...)
	result = append(result, wholeLines[chunkEnd:]...)
	return strings.Join(result, "")
}

// findSimilarLines looks for the region of contentText most similar to
// searchText, for use in "did you mean...?" error suggestions.
func findSimilarLines(searchText, contentText string) string {
	const threshold = 0.6

	searchLines := strings.Split(searchText, "\n")
	contentLines := strings.Split(contentText, "\n")

	bestRatio := 0.0
	var bestMatch []string
	bestMatchI := -1

	for i := 0; i+len(searchLines) <= len(contentLines); i++ {
		chunk := contentLines[i : i+len(searchLines)]
		r := ratioLines(searchLines, chunk)
		if r > bestRatio {
			bestRatio = r
			bestMatch = chunk
			bestMatchI = i
		}
	}

	if bestRatio < threshold || bestMatch == nil {
		return ""
	}

	if len(bestMatch) > 0 && len(searchLines) > 0 &&
		bestMatch[0] == searchLines[0] && bestMatch[len(bestMatch)-1] == searchLines[len(searchLines)-1] {
		return strings.Join(bestMatch, "\n")
	}

	const contextLines = 5
	end := bestMatchI + len(searchLines) + contextLines
	if end > len(contentLines) {
		end = len(contentLines)
	}
	start := bestMatchI - contextLines
	if start < 0 {
		start = 0
	}

	return strings.Join(contentLines[start:end], "\n")
}
