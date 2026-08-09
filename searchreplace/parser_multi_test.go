package searchreplace

import "testing"

// Regression test: multiple SEARCH/REPLACE blocks separated by arbitrary
// text must all be parsed, in order, regardless of what sits between them.
func TestParseEditBlocks_MultipleBlocksWithArbitraryTextBetween(t *testing.T) {
	content := `file1.txt
<<<<<<< SEARCH
old1
=======
new1
>>>>>>> REPLACE
some random commentary here
that spans multiple lines

file2.txt
<<<<<<< SEARCH
old2
=======
new2
>>>>>>> REPLACE
more random commentary

file3.txt
<<<<<<< SEARCH
old3
=======
new3
>>>>>>> REPLACE
trailing commentary`

	result, err := ParseEditBlocks(content, DefaultFence)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Edits) != 3 {
		t.Fatalf("expected 3 edits, got %d: %+v", len(result.Edits), result.Edits)
	}

	want := []EditBlock{
		{Path: "file1.txt", Original: "old1\n", Updated: "new1\n"},
		{Path: "file2.txt", Original: "old2\n", Updated: "new2\n"},
		{Path: "file3.txt", Original: "old3\n", Updated: "new3\n"},
	}
	for i, w := range want {
		got := result.Edits[i]
		if got != w {
			t.Errorf("edit %d = %+v, want %+v", i, got, w)
		}
	}
}
