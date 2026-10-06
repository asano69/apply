package edit

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	fuzzyFile   = "func hello() {\n\tprintln(\"hello world\")\n\treturn\n}\n"
	fuzzySearch = "func hello() {\n\tprintln(\"hello, world\")\n\treturn\n}\n" // differs by a comma
	fuzzyNew    = "func hello() {\n\treturn\n}\n"
)

// applyFuzzy applies one approximate edit to a.go and returns the resulting
// file content, the error, and the text that was offered for confirmation.
func applyFuzzy(t *testing.T, confirm func(path, searched, found string) bool) (content string, err error, offered string) {
	t.Helper()
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	if werr := os.WriteFile(path, []byte(fuzzyFile), 0o644); werr != nil {
		t.Fatal(werr)
	}

	var wrapped func(path, searched, found string) bool
	if confirm != nil {
		wrapped = func(p, s, f string) bool {
			offered = f
			return confirm(p, s, f)
		}
	}

	edits := []EditBlock{{Path: "a.go", Original: fuzzySearch, Updated: fuzzyNew}}
	_, err = ApplyEdits(edits, root, ApplyOptions{ConfirmFuzzy: wrapped})

	got, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	return string(got), err, offered
}

func TestApplyEdits_FuzzyAppliedWhenConfirmed(t *testing.T) {
	got, err, offered := applyFuzzy(t, func(_, _, _ string) bool { return true })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if offered != fuzzyFile {
		t.Errorf("offered = %q, want %q", offered, fuzzyFile)
	}
	if got != fuzzyNew {
		t.Errorf("content = %q, want %q", got, fuzzyNew)
	}
}

func TestApplyEdits_FuzzyDeclinedLeavesFileUntouched(t *testing.T) {
	got, err, _ := applyFuzzy(t, func(_, _, _ string) bool { return false })
	if _, ok := err.(*ApplyError); !ok {
		t.Fatalf("expected *ApplyError, got %v", err)
	}
	if got != fuzzyFile {
		t.Errorf("file was modified: %q", got)
	}
}

func TestApplyEdits_FuzzyNeverAppliedWithoutConfirm(t *testing.T) {
	got, err, _ := applyFuzzy(t, nil)
	if _, ok := err.(*ApplyError); !ok {
		t.Fatalf("expected *ApplyError, got %v", err)
	}
	if got != fuzzyFile {
		t.Errorf("file was modified: %q", got)
	}
}
