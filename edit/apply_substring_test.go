package edit

import (
	"os"
	"path/filepath"
	"testing"
)

// applySubstring applies one edit to a.txt holding content and returns the
// resulting file content and the error.
func applySubstring(t *testing.T, content, original, updated string) (string, error) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "a.txt")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	edits := []EditBlock{{Path: "a.txt", Original: original, Updated: updated}}
	_, err := ApplyEdits(edits, root, ApplyOptions{})

	got, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	return string(got), err
}

// Regression test: a SEARCH text that starts in the middle of a line must be
// applied when it occurs exactly once in the file.
func TestApplyEdits_MidLineMatchReplacedWhenUnique(t *testing.T) {
	file := "The diff must contain a filename; `apply` takes no arguments:\nnext line\n"
	want := "The diff must contain a filename; `apply` takes no arguments other than `-h`:\nnext line\n"

	got, err := applySubstring(t, file,
		"`apply` takes no arguments:\n",
		"`apply` takes no arguments other than `-h`:\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestApplyEdits_MidLineMatchFailsWhenAmbiguous(t *testing.T) {
	file := "x foo y\nz foo w\n"

	got, err := applySubstring(t, file, "foo\n", "bar\n")
	if _, ok := err.(*ApplyError); !ok {
		t.Fatalf("expected *ApplyError, got %v", err)
	}
	if got != file {
		t.Errorf("file was modified: %q", got)
	}
}

func TestApplyEdits_MidLineMatchFailsWhenAbsent(t *testing.T) {
	file := "x foo y\n"

	got, err := applySubstring(t, file, "qux\n", "bar\n")
	if _, ok := err.(*ApplyError); !ok {
		t.Fatalf("expected *ApplyError, got %v", err)
	}
	if got != file {
		t.Errorf("file was modified: %q", got)
	}
}
