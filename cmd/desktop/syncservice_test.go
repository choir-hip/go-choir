package main

import (
	"path/filepath"
	"testing"
)

func TestFileProviderSocketPathPrefersExplicitOverride(t *testing.T) {
	got := fileProviderSocketPathFor("darwin", "/Users/alice", func(key string) string {
		if key == fileProviderSocketPathEnv {
			return " /tmp/choir-fp.sock "
		}
		if key == fileProviderAppGroupIDEnv {
			return "group.other"
		}
		return ""
	})

	want := "/tmp/choir-fp.sock"
	if got != want {
		t.Fatalf("socket path: got %q want %q", got, want)
	}
}

func TestFileProviderSocketPathUsesDefaultDarwinAppGroupContainer(t *testing.T) {
	got := fileProviderSocketPathFor("darwin", "/Users/alice", func(string) string { return "" })

	want := filepath.Join("/Users/alice", "Library", "Group Containers", defaultFileProviderAppGroup, "Choir", "fileprovider.sock")
	if got != want {
		t.Fatalf("socket path: got %q want %q", got, want)
	}
}

func TestFileProviderSocketPathUsesConfiguredDarwinAppGroupContainer(t *testing.T) {
	got := fileProviderSocketPathFor("darwin", "/Users/alice", func(key string) string {
		if key == fileProviderAppGroupIDEnv {
			return " TEAMID.group.news.choir "
		}
		return ""
	})

	want := filepath.Join("/Users/alice", "Library", "Group Containers", "TEAMID.group.news.choir", "Choir", "fileprovider.sock")
	if got != want {
		t.Fatalf("socket path: got %q want %q", got, want)
	}
}

func TestFileProviderSocketPathUsesDotChoirOutsideDarwin(t *testing.T) {
	got := fileProviderSocketPathFor("linux", "/home/alice", func(string) string { return "" })

	want := filepath.Join("/home/alice", ".choir", "fileprovider.sock")
	if got != want {
		t.Fatalf("socket path: got %q want %q", got, want)
	}
}
