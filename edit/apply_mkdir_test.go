package edit

import (
	"os"
	"path/filepath"
	"testing"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestApplyDiff_NewFileInMissingDirCreatesDirAfterConfirm(t *testing.T) {
	root := t.TempDir()
	diff := readFixture(t, "new_file_in_new_dir.txt")

	opts := ApplyOptions{ConfirmMkdir: func(string) bool { return true }}
	if _, err := ApplyDiff(diff, root, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "internal", "parser", "links_test.go")); err != nil {
		t.Errorf("file was not created: %v", err)
	}
}

func TestApplyDiff_NewFileInMissingDirDeclined(t *testing.T) {
	tests := []struct {
		name    string
		confirm func(string) bool
	}{
		{"user says no", func(string) bool { return false }},
		{"no confirm callback", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			diff := readFixture(t, "new_file_in_new_dir.txt")

			_, err := ApplyDiff(diff, root, ApplyOptions{ConfirmMkdir: tt.confirm})
			if _, ok := err.(*DirMissingError); !ok {
				t.Fatalf("expected *DirMissingError, got %v", err)
			}

			if _, serr := os.Stat(filepath.Join(root, "internal")); !os.IsNotExist(serr) {
				t.Errorf("directory must not be created, stat err = %v", serr)
			}
		})
	}
}
