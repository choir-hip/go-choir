package pkgmirror

import (
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Failure modes pinned here (the station file's list):
//   F10 a repository name runs a git option, reaches another host, a local
//       path or another protocol; a clone grows past its cap;
//   F11 the dumb protocol paths serve a file outside the repository's
//       objects, refs and info;
//   and the real git client cannot clone through the mirror.

type gitHarness struct {
	*harness
	git      string
	upstream string // directory standing in for https://github.com
}

func newGitHarness(t *testing.T, mutate func(*Config)) *gitHarness {
	t.Helper()
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	up := t.TempDir()
	h := newHarness(t, func(c *Config) {
		c.Ecosystems = append(c.Ecosystems, EcosystemGit)
		c.Git.Upstreams = map[string]string{"github.com": "file://" + up}
		c.Git.Refresh = -1 // every info/refs refreshes
		if mutate != nil {
			mutate(c)
		}
	})
	return &gitHarness{harness: h, git: git, upstream: up}
}

func (g *gitHarness) run(dir string, args ...string) string {
	g.t.Helper()
	cmd := exec.Command(g.git, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		g.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// publish makes owner/repo.git upstream with one commit of file=body.
func (g *gitHarness) publish(owner, repo, file, body string) string {
	g.t.Helper()
	work := g.t.TempDir()
	g.run(work, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(work, file), []byte(body), 0o644); err != nil {
		g.t.Fatal(err)
	}
	g.run(work, "add", file)
	g.run(work, "commit", "-q", "-m", "c")
	bare := filepath.Join(g.upstream, owner, repo+".git")
	if err := os.MkdirAll(filepath.Dir(bare), 0o755); err != nil {
		g.t.Fatal(err)
	}
	g.run(work, "clone", "-q", "--bare", work, bare)
	return work
}

func (g *gitHarness) clone(url string, extra ...string) (string, error) {
	dst := filepath.Join(g.t.TempDir(), "c")
	cmd := exec.Command(g.git, append(append([]string{"clone", "-q"}, extra...), url, dst)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return dst, nil
}

func TestGitCloneAndRefreshThroughTheMirror(t *testing.T) {
	g := newGitHarness(t, nil)
	work := g.publish("acme", "tool", "README", "one\n")
	url := g.ts.URL + "/git/github.com/acme/tool"

	dst, err := g.clone(url)
	if err != nil {
		t.Fatalf("clone through the mirror: %v\n%s", err, dst)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "README")); string(b) != "one\n" {
		t.Fatalf("cloned README = %q", b)
	}
	// The same repository by its .git name is the same mirror copy.
	if out, err := g.clone(url + ".git"); err != nil {
		t.Fatalf("clone by .git name: %v\n%s", err, out)
	}

	packDir := filepath.Join(g.dir, "git", "repos", "github.com", "acme", "tool.git", "objects", "pack")
	firstPacks, _ := filepath.Glob(filepath.Join(packDir, "pack-*.pack"))
	if len(firstPacks) == 0 {
		t.Fatal("the mirror copy has no pack")
	}

	// A new upstream commit reaches the next clone (refresh on info/refs).
	if err := os.WriteFile(filepath.Join(work, "README"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	g.run(work, "commit", "-q", "-am", "c2")
	g.run(work, "push", "-q", filepath.Join(g.upstream, "acme", "tool.git"), "main")
	dst, err = g.clone(url)
	if err != nil {
		t.Fatalf("clone after the upstream moved: %v\n%s", err, dst)
	}
	if b, _ := os.ReadFile(filepath.Join(dst, "README")); string(b) != "two\n" {
		t.Fatalf("README after refresh = %q, want two", b)
	}
	// A refresh never repacks: a dumb-protocol reader may be part way
	// through the pack list it read before (git's detached maintenance).
	time.Sleep(500 * time.Millisecond)
	for _, p := range firstPacks {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("a refresh removed pack %s: %v", filepath.Base(p), err)
		}
	}
}

func TestGitServesStaleWithinTheRefreshInterval(t *testing.T) {
	g := newGitHarness(t, func(c *Config) { c.Git.Refresh = time.Hour })
	g.publish("acme", "tool", "README", "one\n")
	url := g.ts.URL + "/git/github.com/acme/tool"
	if out, err := g.clone(url); err != nil {
		t.Fatalf("first clone: %v\n%s", err, out)
	}
	// Upstream gone: the mirror answers from its copy without refetching.
	if err := os.RemoveAll(filepath.Join(g.upstream, "acme")); err != nil {
		t.Fatal(err)
	}
	if out, err := g.clone(url); err != nil {
		t.Fatalf("second clone within the interval: %v\n%s", err, out)
	}
}

func TestGitPathsOutsideTheGrammarAreRefused(t *testing.T) {
	g := newGitHarness(t, nil)
	g.publish("acme", "tool", "README", "one\n")
	for _, path := range []string{
		"/git/evil.example/acme/tool/info/refs",
		"/git/127.0.0.1/acme/tool/info/refs",
		"/git/github.com/-uacme/tool/info/refs",
		"/git/github.com/acme/-tool/info/refs",
		"/git/github.com/acme/.git/info/refs",
		"/git/github.com/../tool/info/refs",
		"/git/github.com/acme/../tool/info/refs",
		"/git/github.com/acme/tool/x/info/refs",
		"/git/github.com/acme/tool/config",
		"/git/github.com/acme/tool/description",
		"/git/github.com/acme/tool/hooks/pre-receive",
		"/git/github.com/acme/tool/objects/../config",
		"/git/github.com/acme/tool/objects/pack/pack-x.pack",
		"/git/github.com/acme/tool/objects/zz/" + strings.Repeat("a", 38),
		"/git/github.com/acme/tool/info/refs?service=git-receive-pack",
		"/git/github.com/acme/tool/info/refs?service=git-upload-pack&x=1",
		"/git/github.com/acme/tool/git-upload-pack",
		"/git/github.com/acme%2ftool/info/refs",
	} {
		if status, _ := g.get(path); status == http.StatusOK {
			t.Errorf("F11: GET %s = 200", path)
		}
	}
	if entries, _ := os.ReadDir(filepath.Join(g.dir, "git", "repos")); len(entries) != 0 {
		t.Fatalf("F10: refused paths created mirror copies: %v", entries)
	}
	// A git POST never reaches the mirror's repositories.
	if status, _, _ := g.do(http.MethodPost, "/git/github.com/acme/tool/git-upload-pack"); status != http.StatusMethodNotAllowed {
		t.Fatalf("POST git-upload-pack = %d, want 405", status)
	}
}

func TestGitSmartProbeGetsTheDumbAnswer(t *testing.T) {
	g := newGitHarness(t, nil)
	g.publish("acme", "tool", "README", "one\n")
	status, hdr, body := g.do(http.MethodGet, "/git/github.com/acme/tool/info/refs?service=git-upload-pack")
	if status != http.StatusOK || strings.Contains(hdr.Get("Content-Type"), "git-upload-pack") || !strings.Contains(body, "\trefs/heads/main\n") {
		t.Fatalf("info/refs = %d %q %q, want the dumb ref list", status, hdr.Get("Content-Type"), body)
	}
}

func TestGitShallowCloneIsNotSupported(t *testing.T) {
	// A named residual (station file): the dumb protocol has no shallow or
	// partial clone. This pins it so the day it changes is visible.
	g := newGitHarness(t, nil)
	g.publish("acme", "tool", "README", "one\n")
	if _, err := g.clone(g.ts.URL+"/git/github.com/acme/tool", "--depth", "1"); err == nil {
		t.Fatal("a shallow clone succeeded through the dumb protocol; update the station's residual")
	}
}

func TestGitCloneOverTheCapIsRefusedAndRemoved(t *testing.T) {
	g := newGitHarness(t, func(c *Config) { c.Git.RepoCap = 16 << 10 })
	// 256 KiB that does not compress.
	noise := make([]byte, 256<<10)
	rand.New(rand.NewSource(1)).Read(noise)
	g.publish("acme", "big", "blob", string(noise))
	if out, err := g.clone(g.ts.URL + "/git/github.com/acme/big"); err == nil {
		t.Fatalf("F10: a repository past the cap was cloned: %s", out)
	}
	if _, err := os.Stat(filepath.Join(g.dir, "git", "repos", "github.com", "acme", "big.git")); !os.IsNotExist(err) {
		t.Fatalf("F10: the over-cap copy stayed in the cache: %v", err)
	}
	if entries, _ := os.ReadDir(filepath.Join(g.dir, "git", "tmp")); len(entries) != 0 {
		t.Fatalf("F10: the over-cap clone left temp files: %v", entries)
	}
}

func TestGitDisabledServesNothing(t *testing.T) {
	g := newGitHarness(t, func(c *Config) { c.Ecosystems = []string{EcosystemNPM} })
	g.publish("acme", "tool", "README", "one\n")
	if status, body := g.get("/git/github.com/acme/tool/info/refs"); status != http.StatusNotFound || !strings.Contains(body, "not enabled") {
		t.Fatalf("F7: git off = %d %q", status, body)
	}
}
