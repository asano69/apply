package edit

import "testing"

// Regression test: bodies always end with "\n", so splitting on "\n" yields
// a trailing empty element that must not hide the closing fence.
func TestStripQuotedWrapping(t *testing.T) {
	tests := []struct {
		name string
		text string
		path string
		want string
	}{
		{
			name: "filename and fence",
			text: "filename.ext\n```\nbody\nmore\n```\n",
			path: "dir/filename.ext",
			want: "body\nmore\n",
		},
		{
			name: "fence only",
			text: "```\nbody\n```\n",
			want: "body\n",
		},
		{
			name: "no wrapping",
			text: "body\nmore\n",
			want: "body\nmore\n",
		},
		{
			name: "lone fence line does not panic",
			text: "```\n",
			want: "```\n",
		},
		{
			name: "empty",
			text: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripQuotedWrapping(tt.text, tt.path, DefaultFence)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}
