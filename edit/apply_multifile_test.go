package edit

import (
	"os"
	"path/filepath"
	"testing"
)

const (
	multiFileBefore = "\tabcdefghijk\n    0123456789\n"
	multiFileAfter  = "\tabcdefghijX\n    0123456789X\n"
)

// Regression test: consecutive blocks for one file, repeated for several
// files, must each be applied to the file named before them.
func TestApplyDiff_ConsecutiveBlocksForMultipleFiles(t *testing.T) {
	root := t.TempDir()
	files := []string{"internal/test/a.go", "internal/test/b.go"}

	for _, name := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(multiFileBefore), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	diff := formatReplacer.Replace(readFixture(t, "multi_file_consecutive_blocks.txt"))
	result, err := ApplyDiff(diff, root, ApplyOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.UpdatedEdits) != 4 {
		t.Errorf("expected 4 applied edits, got %d", len(result.UpdatedEdits))
	}

	for _, name := range files {
		got, rerr := os.ReadFile(filepath.Join(root, name))
		if rerr != nil {
			t.Fatal(rerr)
		}
		if string(got) != multiFileAfter {
			t.Errorf("%s = %q, want %q", name, got, multiFileAfter)
		}
	}
}
