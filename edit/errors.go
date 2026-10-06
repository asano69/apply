package edit

import "fmt"

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

// FileExistsError indicates a new-file block targets a file that already exists.
type FileExistsError struct {
	Path string
}

// Error returns a bilingual message, English first and then Japanese,
// because the CLI user may read either.
func (e *FileExistsError) Error() string {
	return fmt.Sprintf(
		"Refusing to create '%s' because it already exists. "+
			"A new-file block would append to it, so no files were changed. "+
			"Use a SEARCH/REPLACE block to edit the file instead.",
		e.Path)
}

// DirMissingError indicates a new-file block targets a directory that does
// not exist and the user did not agree to create it.
type DirMissingError struct {
	Dir string
}

func (e *DirMissingError) Error() string {
	return fmt.Sprintf(
		"Directory '%s' does not exist and was not created, so no files were changed. "+
			"Create the directory first, or agree to create it when asked.",
		e.Dir)
}

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
