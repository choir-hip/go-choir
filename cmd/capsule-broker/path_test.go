//go:build linux

package main

import (
	"path/filepath"
	"testing"
)

func TestResolveWithinSafe(t *testing.T) {
	base := "/mnt/merged"

	cases := []struct {
		rel  string
		want string
		ok   bool
	}{
		{"file.txt", "/mnt/merged/file.txt", true},
		{"subdir/file.txt", "/mnt/merged/subdir/file.txt", true},
		{"/", "/mnt/merged", true},
		{".", "/mnt/merged", true},
		{"subdir/../file.txt", "/mnt/merged/file.txt", true},
		// Traversal attempts — must fail.
		{"../../etc/passwd", "", false},
		{"../etc/shadow", "", false},
		{"subdir/../../etc/passwd", "", false},
		{"..", "", false},
	}

	for _, tc := range cases {
		got, err := resolveWithin(base, tc.rel)
		if tc.ok {
			if err != nil {
				t.Errorf("resolveWithin(%q, %q): unexpected error: %v", base, tc.rel, err)
				continue
			}
			want := filepath.Clean(tc.want)
			if got != want {
				t.Errorf("resolveWithin(%q, %q) = %q, want %q", base, tc.rel, got, want)
			}
		} else {
			if err == nil {
				t.Errorf("resolveWithin(%q, %q): expected error, got %q", base, tc.rel, got)
			}
		}
	}
}
