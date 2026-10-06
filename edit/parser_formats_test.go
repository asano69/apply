package edit

import (
	"slices"
	"strings"
	"testing"
)

// Placeholders keep the test inputs free of real fence and marker lines.
var formatReplacer = strings.NewReplacer(
	"FENCE", "```",
	"<SEARCH>", "<<<<<<< SEARCH",
	"<DIV>", "=======",
	"<REPLACE>", ">>>>>>> REPLACE",
)

func TestParseEditBlocks_Formats(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []EditBlock
	}{
		{
			name:  "new file note (en)",
			input: "a.go (new file)\nFENCEgo\npackage a\nFENCE\n",
			want:  []EditBlock{{Path: "a.go", Original: "", Updated: "package a\n"}},
		},
		{
			name:  "new file note (ja, full-width)",
			input: "a.go（新規）\nFENCEgo\npackage a\nFENCE\n",
			want:  []EditBlock{{Path: "a.go", Original: "", Updated: "package a\n"}},
		},
		{
			name:  "new file note (ja, half-width)",
			input: "a.go(新規)\nFENCEgo\npackage a\nFENCE\n",
			want:  []EditBlock{{Path: "a.go", Original: "", Updated: "package a\n"}},
		},
		{
			name:  "new file with markdown decoration",
			input: "**`dir/a.go`** (new file):\nFENCEgo\npackage a\nFENCE\n",
			want:  []EditBlock{{Path: "dir/a.go", Original: "", Updated: "package a\n"}},
		},
		{
			name:  "code block without a path is ignored",
			input: "Example:\nFENCEgo\nfoo()\nFENCE\n",
			want:  nil,
		},
		{
			name:  "code block after a blank line is ignored",
			input: "a.go\n\nFENCEgo\nfoo()\nFENCE\n",
			want:  nil,
		},
		{
			name:  "path before fence, block inside",
			input: "a.go\nFENCEgo\n<SEARCH>\nold\n<DIV>\nnew\n<REPLACE>\nFENCE\n",
			want:  []EditBlock{{Path: "a.go", Original: "old\n", Updated: "new\n"}},
		},
		{
			name:  "path inside fence",
			input: "FENCEgo\na.go\n<SEARCH>\nold\n<DIV>\nnew\n<REPLACE>\nFENCE\n",
			want:  []EditBlock{{Path: "a.go", Original: "old\n", Updated: "new\n"}},
		},
		{
			name:  "no fence at all",
			input: "a.go\n<SEARCH>\nold\n<DIV>\nnew\n<REPLACE>\n",
			want:  []EditBlock{{Path: "a.go", Original: "old\n", Updated: "new\n"}},
		},
		{
			name:  "blank lines between path and SEARCH",
			input: "a.go\n\n\n<SEARCH>\nold\n<DIV>\nnew\n<REPLACE>\n",
			want:  []EditBlock{{Path: "a.go", Original: "old\n", Updated: "new\n"}},
		},
		{
			name:  "blank line between a prose line and SEARCH reuses the previous path",
			input: "a.go\n<SEARCH>\nold1\n<DIV>\nnew1\n<REPLACE>\nSome text\n\n<SEARCH>\nold2\n<DIV>\nnew2\n<REPLACE>\n",
			want: []EditBlock{
				{Path: "a.go", Original: "old1\n", Updated: "new1\n"},
				{Path: "a.go", Original: "old2\n", Updated: "new2\n"},
			},
		},
		{
			name: "several blocks under one path",
			input: "a.go\n" +
				"<SEARCH>\nold1\n<DIV>\nnew1\n<REPLACE>\n\n" +
				"<SEARCH>\nold2\n<DIV>\nnew2\n<REPLACE>\n",
			want: []EditBlock{
				{Path: "a.go", Original: "old1\n", Updated: "new1\n"},
				{Path: "a.go", Original: "old2\n", Updated: "new2\n"},
			},
		},
		{
			name: "new file followed by an edit of another file",
			input: "a.go (new file)\nFENCEgo\npackage a\nFENCE\n" +
				"b.go\n<SEARCH>\nold\n<DIV>\nnew\n<REPLACE>\n",
			want: []EditBlock{
				{Path: "a.go", Original: "", Updated: "package a\n"},
				{Path: "b.go", Original: "old\n", Updated: "new\n"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEditBlocks(formatReplacer.Replace(tt.input), DefaultFence)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(got.Edits, tt.want) {
				t.Errorf("edits = %+v, want %+v", got.Edits, tt.want)
			}
		})
	}
}

func TestParseEditBlocks_MissingPath(t *testing.T) {
	input := formatReplacer.Replace("<SEARCH>\nold\n<DIV>\nnew\n<REPLACE>\n")

	_, err := ParseEditBlocks(input, DefaultFence)
	if _, ok := err.(*ParseError); !ok {
		t.Fatalf("expected *ParseError, got %v", err)
	}
}
