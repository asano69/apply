package searchreplace

// ParseError indicates the LLM response could not be parsed into edit blocks.
type ParseError struct {
	Msg string
}

func (e *ParseError) Error() string { return e.Msg }

// PathEscapeError indicates an edit tried to touch a path outside the root directory.
type PathEscapeError struct {
	Msg string
}

func (e *PathEscapeError) Error() string { return e.Msg }

// ApplyError indicates one or more SEARCH/REPLACE blocks failed to match.
type ApplyError struct {
	Message      string
	Failed       []EditBlock
	Passed       []EditBlock
	UpdatedEdits []EditBlock
}

func (e *ApplyError) Error() string { return e.Message }

// wordForm returns "block" or "blocks" depending on count, matching aider's messages.
func wordForm(n int) string {
	if n == 1 {
		return "block"
	}
	return "blocks"
}
