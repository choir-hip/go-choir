package yaegikernel

import (
	"testing"
)

func TestCleanGoSource(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "raw Go without fences",
			input:    "package main\n\nfunc main() {}\n",
			expected: "package main\n\nfunc main() {}",
		},
		{
			name:     "fenced with ```go",
			input:    "```go\npackage main\n\nfunc main() {}\n```",
			expected: "package main\n\nfunc main() {}",
		},
		{
			name:     "fenced with ``` (no language tag)",
			input:    "```\npackage main\n\nfunc main() {}\n```",
			expected: "package main\n\nfunc main() {}",
		},
		{
			name:     "fenced with ~~~go",
			input:    "~~~go\npackage main\n\nfunc main() {}\n~~~",
			expected: "package main\n\nfunc main() {}",
		},
		{
			name:     "leading whitespace before fence",
			input:    "   \n```go\npackage main\n\nfunc main() {}\n```   \n",
			expected: "package main\n\nfunc main() {}",
		},
		{
			name:     "Go source containing raw string literal with backticks inside body",
			input:    "```go\npackage main\n\nfunc main() {\n\tmsg := `raw string with backticks`\n\t_ = msg\n}\n```",
			expected: "package main\n\nfunc main() {\n\tmsg := `raw string with backticks`\n\t_ = msg\n}",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   \n\t  ",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CleanGoSource(tc.input)
			if got != tc.expected {
				t.Fatalf("CleanGoSource(%q):\ngot:  %q\nwant: %q", tc.input, got, tc.expected)
			}
		})
	}
}
