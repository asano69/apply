package edit

import (
	"os"
	"path/filepath"
	"testing"
)

// Regression test: a new-file block must not append to an existing file.
func TestApplyEdits_NewFileOverExistingFileFails(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "a.go")
	if err := os.WriteFile(path, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	edits := []EditBlock{{Path: "a.go", Original: "", Updated: "package a\n"}}
	_, err := ApplyEdits(edits, root, ApplyOptions{})
	if _, ok := err.(*FileExistsError); !ok {
		t.Fatalf("expected *FileExistsError, got %v", err)
	}

	got, rerr := os.ReadFile(path)
	if rerr != nil {
		t.Fatal(rerr)
	}
	if string(got) != "package a\n" {
		t.Errorf("file was modified: %q", got)
	}
}

func TestApplyEdits_NewFileCreatesFile(t *testing.T) {
	root := t.TempDir()

	edits := []EditBlock{{Path: "a.go", Original: "", Updated: "package a\n"}}
	if _, err := ApplyEdits(edits, root, ApplyOptions{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(root, "a.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "package a\n" {
		t.Errorf("content = %q", got)
	}
}
