package provider

import (
	"os"
	"path/filepath"
	"testing"
)

// ChatGPT OAuth refresh tokens rotate: two refreshers on one auth file can
// each spend the same refresh token, and the second fails. Every user of an
// auth path in the process (one inference provider per model, the search
// adapter) shares one ChatGPTAuth, so its mutex covers them all. Review of
// PR #52, 2026-10-11.
func TestChatGPTAuthIsSharedPerPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	if err := os.WriteFile(path, []byte(`{"auth_mode":"chatgpt","tokens":{"access_token":"a","refresh_token":"r"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if SharedChatGPTAuth(path) != SharedChatGPTAuth(filepath.Join(dir, ".", "auth.json")) {
		t.Fatalf("one auth path must have one ChatGPTAuth")
	}
	if SharedChatGPTAuth(path) == SharedChatGPTAuth(filepath.Join(dir, "other.json")) {
		t.Fatalf("different auth paths must not share")
	}
	a, err := NewChatGPTProvider(ChatGPTConfig{ModelID: "gpt-a", AuthPath: path})
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewChatGPTProvider(ChatGPTConfig{ModelID: "gpt-b", AuthPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if a.auth != b.auth || a.auth != SharedChatGPTAuth(path) {
		t.Fatalf("inference providers on one auth path must share its refresher")
	}
}

// A refresh rewrites the auth file by rename, so a concurrent reader never
// parses a half-written file, and it leaves no temporary file behind.
func TestChatGPTAuthWriteReplacesTheFileWhole(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	auth := NewChatGPTAuth(ChatGPTAuthOptions{Path: path})
	if err := auth.write(&codexAuthFile{AuthMode: "chatgpt", Tokens: codexAuthTokens{AccessToken: "new"}}); err != nil {
		t.Fatal(err)
	}
	got, err := auth.Read()
	if err != nil || got.Tokens.AccessToken != "new" {
		t.Fatalf("read back: %+v %v", got, err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Fatalf("write must leave only the auth file, found %d entries", len(entries))
	}
	if info, _ := os.Stat(path); info.Mode().Perm() != 0o600 {
		t.Fatalf("auth file mode = %v, want 0600", info.Mode().Perm())
	}
}
