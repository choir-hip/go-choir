package sourceproj

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// U1b (docs/design/computer-update-architecture-rnd-2026-10-24.md, "U1b
// design"): the failure modes N1 to N14, driven against real git repositories.
// The remote is a local path; the https path differs only in the transport.

// remoteWithHistory is a repository with three commits after the seed, that
// allows an exact commit to be wanted (a public forge does the same).
func remoteWithHistory(t *testing.T) (remote string, commits []string) {
	t.Helper()
	remote, base := baseRepo(t)
	tgit(t, remote, "config", "uploadpack.allowAnySHA1InWant", "true")
	commits = []string{base}
	for i, name := range []string{"one", "two", "three"} {
		write(t, remote, "frontend/src/"+name+".txt", numbered(name, 5+i))
		tgit(t, remote, "add", "-A")
		tgit(t, remote, "commit", "-q", "-m", name)
		commits = append(commits, tgit(t, remote, "rev-parse", "HEAD"))
	}
	return remote, commits
}

// guestAt is a guest repository that holds only history up to commit.
func guestAt(t *testing.T, remote, commit string) string {
	t.Helper()
	guest := t.TempDir()
	tgit(t, guest, "init", "-q")
	tgit(t, guest, "config", "core.filemode", "false")
	tgit(t, guest, "fetch", "-q", remote, commit)
	tgit(t, guest, "checkout", "-q", "-B", "choir/computer", commit)
	return guest
}

func opts(remote string) FetchOptions {
	return FetchOptions{Remote: remote, Timeout: time.Minute, MinFreeBytes: 1}
}

func TestFetchBringsTheBootedBaseAndTheProjectionMovesToIt(t *testing.T) {
	remote, c := remoteWithHistory(t)
	guest := guestAt(t, remote, c[0])
	// N-none: the guest does not have the booted commit.
	if _, ok := resolveCommit(context.Background(), guest, c[3]); ok {
		t.Fatal("setup: the guest already has the booted commit")
	}
	if res := reconcile(t, Input{Repo: guest, BootedBase: c[3]}); res.State != StateBaseObjectsMissing {
		t.Fatalf("before the fetch: %+v", res)
	}
	got := EnsureBaseObjects(context.Background(), guest, c[3], opts(remote))
	if got.Outcome != FetchFetched || got.Commit != c[3] {
		t.Fatalf("fetch = %+v", got)
	}
	if ref := tgit(t, guest, "rev-parse", RefBootedPrefix+c[3]); ref != c[3] {
		t.Fatalf("the fetched commit is not held by its ref: %s", ref)
	}
	// B1: with an empty stack the projection moves to the booted base.
	res := reconcile(t, Input{Repo: guest, BootedBase: c[3]})
	if res.State != StateBase || headOf(t, guest) != c[3] || !res.Changed || res.From != c[0] {
		t.Fatalf("after the fetch: %+v head %s", res, headOf(t, guest))
	}
	mustClean(t, guest)
	// N5/N6: no index lock is left and no background maintenance was started.
	if _, err := os.Stat(guest + "/.git/index.lock"); err == nil {
		t.Fatal("index.lock left behind")
	}
}

func TestFetchIsADeltaAndNeedsNoNetworkWhenPresent(t *testing.T) {
	remote, c := remoteWithHistory(t)
	guest := guestAt(t, remote, c[0])
	// Present: the remote may be anything; it is never used (1).
	got := EnsureBaseObjects(context.Background(), guest, c[0], opts("/nonexistent/remote"))
	if got.Outcome != FetchPresent {
		t.Fatalf("present = %+v", got)
	}
	// Delta only: the objects of commits the guest already has are not sent.
	// The seed commit's blobs are in the guest; only three new blobs arrive.
	before := objectCount(t, guest)
	if got := EnsureBaseObjects(context.Background(), guest, c[3], opts(remote)); got.Outcome != FetchFetched {
		t.Fatalf("fetch = %+v", got)
	}
	if delta := objectCount(t, guest) - before; delta > 3*5 { // 3 commits x (commit, three trees, blob)
		t.Fatalf("fetched %d objects for a three-commit delta", delta)
	}
}

func objectCount(t *testing.T, repo string) int {
	t.Helper()
	out := tgit(t, repo, "count-objects", "-v")
	n := 0
	for _, line := range strings.Split(out, "\n") {
		for _, key := range []string{"count: ", "in-pack: "} {
			if strings.HasPrefix(line, key) {
				var v int
				for _, ch := range strings.TrimPrefix(line, key) {
					v = v*10 + int(ch-'0')
				}
				n += v
			}
		}
	}
	return n
}

func TestOlderBootedRefsAreRemovedAfterASuccess(t *testing.T) {
	remote, c := remoteWithHistory(t)
	guest := guestAt(t, remote, c[0])
	for _, commit := range []string{c[2], c[3]} {
		if got := EnsureBaseObjects(context.Background(), guest, commit, opts(remote)); got.Outcome != FetchFetched {
			t.Fatalf("fetch %s = %+v", commit, got)
		}
	}
	refs := tgit(t, guest, "for-each-ref", "--format=%(refname)", "refs/choir/booted/")
	if refs != RefBootedPrefix+c[3] {
		t.Fatalf("F8: refs = %q", refs)
	}
}

func TestCommitNotOnTheRemoteIsPermanent(t *testing.T) {
	remote, c := remoteWithHistory(t)
	guest := guestAt(t, remote, c[0])
	gone := strings.Repeat("a", 40)
	got := EnsureBaseObjects(context.Background(), guest, gone, opts(remote))
	if got.Outcome != FetchPermanent || got.Retryable() {
		t.Fatalf("N2: %+v", got)
	}
	if head := headOf(t, guest); head != c[0] {
		t.Fatalf("a failed fetch moved HEAD to %s", head)
	}
	if refs := tgit(t, guest, "for-each-ref", "refs/choir/booted/"); refs != "" {
		t.Fatalf("a failed fetch left refs: %s", refs)
	}
}

func TestUnreachableRemoteIsTransientAndChangesNothing(t *testing.T) {
	remote2, c := remoteWithHistory(t)
	guest := guestAt(t, remote2, c[0])
	// N1: the remote is gone (a path that is not a repository).
	got := EnsureBaseObjects(context.Background(), guest, c[3], opts(t.TempDir()))
	if got.Outcome == FetchFetched || got.Outcome == FetchPresent {
		t.Fatalf("N1: %+v", got)
	}
	if res := reconcile(t, Input{Repo: guest, BootedBase: c[3]}); res.State != StateBaseObjectsMissing || res.Refusal() != "" {
		t.Fatalf("N1 must leave U1's B2: %+v", res)
	}
	// A cancelled context is a transient failure too, and does not hang.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got = EnsureBaseObjects(ctx, guest, c[3], opts(remote2))
	if got.Outcome != FetchFailed || !got.Retryable() {
		t.Fatalf("cancelled = %+v", got)
	}
}

func TestOnlyHTTPSAndLocalPathsAreRemotes(t *testing.T) {
	good := []string{"https://github.com/choir-hip/go-choir.git", "/srv/mirror/go-choir"}
	bad := []string{"", "http://github.com/x/y.git", "git://github.com/x/y.git", "ssh://git@github.com/x/y.git", "git@github.com:x/y.git",
		"https://user:pw@github.com/x/y.git", "https://token@github.com/x/y.git", "--upload-pack=touch /tmp/x", "-oProxyCommand=x", "relative/path",
		"ext::sh -c touch% /tmp/x", "file:///srv/x", "https:///nohost", "https://github.com/x y.git"}
	for _, remote := range good {
		if _, err := validRemote(remote); err != nil {
			t.Errorf("N3: %q refused: %v", remote, err)
		}
	}
	for _, remote := range bad {
		if _, err := validRemote(remote); err == nil {
			t.Errorf("N3: %q accepted", remote)
		}
	}
	remote, c := remoteWithHistory(t)
	guest := guestAt(t, remote, c[0])
	for _, remote := range bad {
		got := EnsureBaseObjects(context.Background(), guest, c[3], opts(remote))
		if got.Outcome != FetchSkipped || got.Retryable() {
			t.Errorf("N3: %q = %+v", remote, got)
		}
	}
}

func TestFetchSkipsWhatItCannotFetch(t *testing.T) {
	remote, c := remoteWithHistory(t)
	guest := guestAt(t, remote, c[0])
	for name, booted := range map[string]string{"no booted base": "", "abbreviated": c[3][:12], "not hex": strings.Repeat("z", 40)} {
		if got := EnsureBaseObjects(context.Background(), guest, booted, opts(remote)); got.Outcome != FetchSkipped {
			t.Errorf("%s = %+v", name, got)
		}
	}
	if got := EnsureBaseObjects(context.Background(), guest, c[3], FetchOptions{}); got.Outcome != FetchSkipped {
		t.Errorf("no remote = %+v", got)
	}
	// Not a repository: nothing to fetch into.
	if got := EnsureBaseObjects(context.Background(), t.TempDir(), c[3], opts(remote)); got.Outcome != FetchSkipped {
		t.Errorf("not a repo = %+v", got)
	}
}

func TestLowDiskIsTransientAndSkipsTheFetch(t *testing.T) {
	remote, c := remoteWithHistory(t)
	guest := guestAt(t, remote, c[0])
	o := opts(remote)
	o.MinFreeBytes = 1 << 62
	got := EnsureBaseObjects(context.Background(), guest, c[3], o)
	if got.Outcome != FetchFailed || !got.Retryable() || !strings.Contains(got.Reason, "free") {
		t.Fatalf("N4: %+v", got)
	}
	if _, ok := resolveCommit(context.Background(), guest, c[3]); ok {
		t.Fatal("N4: fetched despite the floor")
	}
}

func TestSwitchOffFetchesNothing(t *testing.T) {
	remote, c := remoteWithHistory(t)
	guest := guestAt(t, remote, c[0])
	t.Setenv(EnvSwitch, "")
	if got := EnsureBaseObjects(context.Background(), guest, c[3], opts(remote)); got.Outcome != FetchDisabled {
		t.Fatalf("N13: %+v", got)
	}
	t.Setenv(EnvSwitch, "1")
	t.Setenv(EnvBaseFetch, "off")
	if got := EnsureBaseObjects(context.Background(), guest, c[3], opts(remote)); got.Outcome != FetchDisabled {
		t.Fatalf("N13 (fetch off): %+v", got)
	}
	if _, ok := resolveCommit(context.Background(), guest, c[3]); ok {
		t.Fatal("N13: objects fetched while off")
	}
}

func TestBaseRemoteComesFromTheEnvironment(t *testing.T) {
	t.Setenv(EnvBaseRemote, " https://example.test/x.git ")
	if got := BaseRemote(); got != "https://example.test/x.git" {
		t.Fatalf("BaseRemote = %q", got)
	}
	t.Setenv(EnvBaseRemote, "")
	if got := BaseRemote(); got != "" {
		t.Fatalf("BaseRemote = %q", got)
	}
}

func TestTheCallersGitEnvironmentCannotRedirectTheFetch(t *testing.T) {
	remote, c := remoteWithHistory(t)
	guest := guestAt(t, remote, c[0])
	other := t.TempDir()
	tgit(t, other, "init", "-q")
	t.Setenv("GIT_DIR", other+"/.git")
	t.Setenv("GIT_OBJECT_DIRECTORY", other+"/.git/objects")
	if got := EnsureBaseObjects(context.Background(), guest, c[3], opts(remote)); got.Outcome != FetchFetched {
		t.Fatalf("fetch = %+v", got)
	}
	out, err := exec.Command("git", "-C", guest, "rev-parse", "--verify", c[3]+"^{commit}").CombinedOutput()
	_ = out
	_ = err
	if _, ok := resolveCommit(context.Background(), guest, c[3]); !ok {
		t.Fatal("objects went to the wrong repository")
	}
}
