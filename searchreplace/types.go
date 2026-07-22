package searchreplace

// Fence is the pair of markers used to wrap a fenced code block, e.g. ``` / ```.
type Fence struct {
	Open  string
	Close string
}

// DefaultFence is the standard triple-backtick fence.
var DefaultFence = Fence{Open: "```", Close: "```"}

// EditBlock is a single SEARCH/REPLACE edit parsed from an LLM response.
type EditBlock struct {
	Path     string
	Original string
	Updated  string
}

// ParseResult holds all edit blocks found in a response.
type ParseResult struct {
	Edits []EditBlock
}

// ApplyResult holds the edits that were successfully applied to disk.
type ApplyResult struct {
	UpdatedEdits []EditBlock
}
