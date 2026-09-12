package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFilesPutSendsRawBytes(t *testing.T) {
	var gotMethod, gotPath, gotCT, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		gotCT = r.Header.Get("Content-Type")
		raw, _ := io.ReadAll(r.Body)
		gotBody = string(raw)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"operation":"updated"}`))
	}))
	defer srv.Close()
	local := filepath.Join(t.TempDir(), "overlay.toml")
	os.WriteFile(local, []byte("[roles.engineering]\n"), 0o644)
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	var stdout bytes.Buffer
	code := runFiles([]string{"put", "--local", local, "System/model-policy-overlays/x.toml"}, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("files put failed: %d", code)
	}
	if gotMethod != http.MethodPut {
		t.Fatalf("method = %s", gotMethod)
	}
	if gotPath != "/api/files/System/model-policy-overlays/x.toml" {
		t.Fatalf("path = %s", gotPath)
	}
	if gotCT != "application/octet-stream" {
		t.Fatalf("content-type = %s", gotCT)
	}
	if gotBody != "[roles.engineering]\n" {
		t.Fatalf("body = %q", gotBody)
	}
}

func TestFilesGetWritesRawStdout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/files/a/b.txt" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte("bytes"))
	}))
	defer srv.Close()
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	var stdout bytes.Buffer
	if code := runFiles([]string{"get", "a/b.txt"}, &stdout, io.Discard); code != 0 {
		t.Fatalf("files get failed: %d", code)
	}
	if stdout.String() != "bytes" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestFilesMkdirPosts(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	t.Setenv(hostEnvVar, srv.URL)
	t.Setenv(apiKeyEnvVar, "choir_sk_test")
	if code := runFiles([]string{"mkdir", "System/model-policy-overlays"}, io.Discard, io.Discard); code != 0 {
		t.Fatalf("files mkdir failed: %d", code)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/files/System/model-policy-overlays" {
		t.Fatalf("got %s %s", gotMethod, gotPath)
	}
}

func TestFilesRequiresSubcommand(t *testing.T) {
	if code := runFiles(nil, io.Discard, io.Discard); code != 2 {
		t.Fatalf("code = %d", code)
	}
	if code := runFiles([]string{"put"}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("code = %d", code)
	}
	if code := runFiles([]string{"bogus"}, io.Discard, io.Discard); code != 2 {
		t.Fatalf("code = %d", code)
	}
}
