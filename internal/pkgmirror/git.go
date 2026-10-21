package pkgmirror

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// git: read-only mirrors of public repositories, served over git's dumb
// HTTP protocol so that nothing but GET and HEAD crosses the capsule's
// egress route (a smart clone POSTs to git-upload-pack). The mirror keeps
// one bare copy per repository, fetched from a fixed forge with hooks off,
// redirects off, https only and every object fsck-checked, and refreshes it
// when a client asks for info/refs, at most once per refresh interval. The
// dumb protocol has no shallow or partial clone; that is a named residual.
//
//	/git/<forge>/<owner>/<repo>[.git]/info/refs[?service=git-upload-pack]
//	/git/<forge>/<owner>/<repo>[.git]/HEAD
//	/git/<forge>/<owner>/<repo>[.git]/objects/info/packs
//	/git/<forge>/<owner>/<repo>[.git]/objects/<2 hex>/<38 hex>
//	/git/<forge>/<owner>/<repo>[.git]/objects/pack/pack-<40 hex>.{pack,idx}
//
// A capsule's git reaches it through url.<base>/git/github.com/.insteadOf.

// EcosystemGit names the git mirror.
const EcosystemGit = "git"

// DefaultGitForges are the forges the git mirror clones from, over https.
var DefaultGitForges = []string{"github.com", "gitlab.com", "codeberg.org"}

// Git defaults.
const (
	DefaultGitRepoCap  = 2 << 30
	DefaultGitMaxBytes = 16 << 30
	DefaultGitRefresh  = time.Minute

	gitCloneTimeout = 15 * time.Minute
	gitFetchTimeout = 5 * time.Minute
	gitWatchEvery   = 500 * time.Millisecond
)

// GitConfig configures the git mirror.
type GitConfig struct {
	// Binary is the git executable; "git" on PATH when empty.
	Binary string
	// Upstreams overrides a forge's base URL (tests); file:// is admitted
	// only for an overridden base.
	Upstreams map[string]string
	// RepoCap bounds one repository on disk, MaxBytes all of them; least
	// recently used repositories are removed past MaxBytes.
	RepoCap  int64
	MaxBytes int64
	// Refresh is the least time between upstream fetches of one repository;
	// negative refreshes on every info/refs.
	Refresh time.Duration
}

var (
	gitName       = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,99}$`)
	gitLooseDir   = regexp.MustCompile(`^[0-9a-f]{2}$`)
	gitLooseFile  = regexp.MustCompile(`^[0-9a-f]{38}$`)
	gitPackFile   = regexp.MustCompile(`^pack-[0-9a-f]{40}\.(pack|idx)$`)
	errGitCapHit  = errors.New("repository exceeds the mirror's size cap")
	errGitMissing = errors.New("repository is not mirrored")
)

// gitMirror holds the bare copies under root/repos.
type gitMirror struct {
	binary    string
	forges    map[string]string // forge host -> base URL
	allowFile bool
	repoCap   int64
	maxBytes  int64
	refresh   time.Duration
	root      string

	mu       sync.Mutex
	flights  map[string]*flight
	fetched  map[string]time.Time
	lastUsed map[string]time.Time
	sizes    map[string]int64
}

func newGitMirror(root string, cfg GitConfig) (*gitMirror, error) {
	g := &gitMirror{
		binary: cfg.Binary, forges: map[string]string{}, repoCap: cfg.RepoCap, maxBytes: cfg.MaxBytes,
		refresh: cfg.Refresh, root: root,
		flights: map[string]*flight{}, fetched: map[string]time.Time{}, lastUsed: map[string]time.Time{}, sizes: map[string]int64{},
	}
	if g.binary == "" {
		g.binary = "git"
	}
	if g.repoCap <= 0 {
		g.repoCap = DefaultGitRepoCap
	}
	if g.maxBytes <= 0 {
		g.maxBytes = DefaultGitMaxBytes
	}
	if g.refresh == 0 {
		g.refresh = DefaultGitRefresh
	}
	for _, f := range DefaultGitForges {
		g.forges[f] = "https://" + f
	}
	for host, base := range cfg.Upstreams {
		g.forges[host] = strings.TrimRight(base, "/")
		if strings.HasPrefix(base, "file://") {
			g.allowFile = true
		}
	}
	for _, d := range []string{"repos", "tmp", "home"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return nil, err
		}
	}
	// A clone that never finished.
	if entries, err := os.ReadDir(filepath.Join(root, "tmp")); err == nil {
		for _, e := range entries {
			_ = os.RemoveAll(filepath.Join(root, "tmp", e.Name()))
		}
	}
	// Measure the copies already here, so the total cap holds across restarts.
	forges, _ := os.ReadDir(filepath.Join(root, "repos"))
	for _, f := range forges {
		owners, _ := os.ReadDir(filepath.Join(root, "repos", f.Name()))
		for _, o := range owners {
			repos, _ := os.ReadDir(filepath.Join(root, "repos", f.Name(), o.Name()))
			for _, r := range repos {
				key := f.Name() + "/" + o.Name() + "/" + r.Name()
				g.sizes[key] = dirBytes(filepath.Join(root, "repos", filepath.FromSlash(key)))
				if info, err := r.Info(); err == nil {
					g.lastUsed[key] = info.ModTime()
				}
			}
		}
	}
	return g, nil
}

// gitRequest is a checked git path.
type gitRequest struct {
	key      string // forge/owner/repo.git, the copy's path under repos
	upstream string // the forge URL of the repository
	file     string // slash path inside the bare repository
	refs     bool   // info/refs: refresh first
}

func (g *gitMirror) parse(rest, query string) (gitRequest, error) {
	if strings.Contains(rest, "%") {
		return gitRequest{}, errRefused
	}
	segs := strings.SplitN(rest, "/", 4)
	if len(segs) != 4 {
		return gitRequest{}, errRefused
	}
	forge, owner, repo, file := segs[0], segs[1], strings.TrimSuffix(segs[2], ".git"), segs[3]
	base, ok := g.forges[forge]
	if !ok || !gitName.MatchString(owner) || !gitName.MatchString(repo) || strings.HasPrefix(repo, ".") {
		return gitRequest{}, errRefused
	}
	req := gitRequest{key: forge + "/" + owner + "/" + repo + ".git", upstream: base + "/" + owner + "/" + repo + ".git", file: file}
	parts := strings.Split(file, "/")
	switch {
	case file == "info/refs":
		req.refs = true
		if query != "" && query != "service=git-upload-pack" {
			return gitRequest{}, errRefused
		}
		return req, nil
	case query != "":
		return gitRequest{}, errRefused
	case file == "HEAD", file == "objects/info/packs":
		return req, nil
	case len(parts) == 3 && parts[0] == "objects" && gitLooseDir.MatchString(parts[1]) && gitLooseFile.MatchString(parts[2]):
		return req, nil
	case len(parts) == 3 && parts[0] == "objects" && parts[1] == "pack" && gitPackFile.MatchString(parts[2]):
		return req, nil
	}
	return gitRequest{}, errRefused
}

func (g *gitMirror) repoDir(key string) string {
	return filepath.Join(g.root, "repos", filepath.FromSlash(key))
}

func (s *Server) serveGit(w http.ResponseWriter, r *http.Request, rest string, rec *Record, finish finishFunc) {
	req, err := s.git.parse(rest, r.URL.RawQuery)
	if err != nil {
		http.Error(w, "not found: not a git mirror path", http.StatusNotFound)
		finish(OutcomeRefused, http.StatusNotFound, 0, err.Error())
		return
	}
	outcome := OutcomeHit
	if req.refs {
		release, err := s.acquire(r.Context(), rec.Source)
		if err != nil {
			http.Error(w, "request cancelled", http.StatusServiceUnavailable)
			finish(OutcomeUpstreamError, http.StatusServiceUnavailable, 0, err.Error())
			return
		}
		fetched, err := s.git.ensure(req)
		release()
		if err != nil {
			status := http.StatusBadGateway
			if errors.Is(err, errGitCapHit) {
				status = http.StatusInsufficientStorage
			}
			http.Error(w, "git mirror: "+err.Error(), status)
			finish(OutcomeUpstreamError, status, 0, err.Error())
			return
		}
		if fetched {
			outcome = OutcomeFetched
		}
	}
	path := filepath.Join(s.git.repoDir(req.key), filepath.FromSlash(req.file))
	if info, err := os.Lstat(path); err != nil || !info.Mode().IsRegular() {
		http.Error(w, "not found", http.StatusNotFound)
		finish(OutcomeNotFound, http.StatusNotFound, 0, "")
		return
	}
	s.git.touch(req.key)
	ctype := "application/octet-stream"
	if req.refs || req.file == "HEAD" || req.file == "objects/info/packs" {
		ctype = "text/plain; charset=utf-8"
	}
	n, err := serveFile(w, r, path, ctype)
	if err != nil {
		finish(OutcomeUpstreamError, http.StatusInternalServerError, n, err.Error())
		return
	}
	finish(outcome, http.StatusOK, n, "")
}

func (g *gitMirror) touch(key string) {
	g.mu.Lock()
	g.lastUsed[key] = time.Now()
	g.mu.Unlock()
}

// ensure clones or refreshes the copy once however many requests wait. It
// reports whether it went upstream. A refresh that fails leaves the last
// good copy in service.
func (g *gitMirror) ensure(req gitRequest) (bool, error) {
	g.mu.Lock()
	_, have := g.sizes[req.key]
	if have && g.refresh >= 0 && time.Since(g.fetched[req.key]) < g.refresh {
		g.mu.Unlock()
		return false, nil
	}
	f, ok := g.flights[req.key]
	if !ok {
		f = &flight{done: make(chan struct{})}
		g.flights[req.key] = f
		go func() {
			f.err = g.update(req, have)
			g.mu.Lock()
			delete(g.flights, req.key)
			if f.err == nil {
				g.fetched[req.key] = time.Now()
			}
			g.mu.Unlock()
			close(f.done)
		}()
	}
	f.waiters++
	g.mu.Unlock()
	<-f.done
	if f.err != nil && have && !errors.Is(f.err, errGitCapHit) {
		if _, err := os.Stat(g.repoDir(req.key)); err == nil {
			return false, nil
		}
	}
	return true, f.err
}

// update clones a missing copy into tmp and moves it into place, or fetches
// into the existing copy; either way it then rewrites the dumb protocol's
// info files and enforces the caps.
func (g *gitMirror) update(req gitRequest, have bool) error {
	dir := g.repoDir(req.key)
	if !have {
		tmp, err := os.MkdirTemp(filepath.Join(g.root, "tmp"), "clone-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		work := filepath.Join(tmp, "repo.git")
		if err := g.run(gitCloneTimeout, work, "", "clone", "--bare", "--quiet", "--template=", "--", req.upstream, work); err != nil {
			return err
		}
		if err := g.run(gitFetchTimeout, "", work, "update-server-info"); err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		if err := os.Rename(work, dir); err != nil {
			return err
		}
	} else {
		if err := g.run(gitFetchTimeout, dir, dir, "fetch", "--prune", "--quiet", "origin", "+refs/heads/*:refs/heads/*", "+refs/tags/*:refs/tags/*"); err != nil {
			if errors.Is(err, errGitCapHit) {
				g.remove(req.key)
			}
			return err
		}
		if err := g.run(gitFetchTimeout, "", dir, "update-server-info"); err != nil {
			return err
		}
	}
	size := dirBytes(dir)
	g.mu.Lock()
	g.sizes[req.key] = size
	g.lastUsed[req.key] = time.Now()
	g.mu.Unlock()
	g.evict(req.key)
	return nil
}

// run runs one git command with a fixed, minimal environment and fixed
// safety settings. watch, when set, is a directory whose size is held under
// the repository cap while the command runs; past it the command's process
// group is killed.
func (g *gitMirror) run(timeout time.Duration, watch, dir string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	protocols := []string{"-c", "protocol.allow=never", "-c", "protocol.https.allow=always"}
	if g.allowFile {
		protocols = append(protocols, "-c", "protocol.file.allow=always")
	}
	full := append(protocols,
		"-c", "http.followRedirects=false",
		"-c", "core.hooksPath=/dev/null",
		"-c", "transfer.fsckObjects=true",
		// Never repack or delete a pack: a dumb-protocol reader may be
		// part way through the pack list. git's fetch otherwise starts a
		// detached maintenance that repacks (gc.auto alone does not stop it).
		"-c", "gc.auto=0",
		"-c", "maintenance.auto=false",
		"-c", "fetch.writeCommitGraph=false",
		"-c", "credential.helper=",
		"-c", "core.askPass=",
	)
	if dir != "" {
		full = append([]string{"-C", dir}, full...)
	}
	full = append(full, args...)
	cmd := exec.CommandContext(ctx, g.binary, full...)
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + filepath.Join(g.root, "home"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_ASKPASS=/bin/false",
		"LANG=C",
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	var overCap bool
	var watchDone, stop chan struct{}
	if watch != "" {
		watchDone, stop = make(chan struct{}), make(chan struct{})
		go func() {
			defer close(watchDone)
			t := time.NewTicker(gitWatchEvery)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					if dirBytes(watch) > g.repoCap {
						overCap = true
						cancel()
						return
					}
				}
			}
		}()
	}
	err := cmd.Wait()
	if watchDone != nil {
		close(stop)
		<-watchDone
	}
	if overCap || (watch != "" && err == nil && dirBytes(watch) > g.repoCap) {
		return errGitCapHit
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 512 {
			msg = msg[:512]
		}
		return fmt.Errorf("git %s: %v: %s", args[0], err, msg)
	}
	return nil
}

// remove takes a copy out of service: renamed into tmp first, so a reader
// never sees half a repository, then deleted.
func (g *gitMirror) remove(key string) {
	dir := g.repoDir(key)
	if tmp, err := os.MkdirTemp(filepath.Join(g.root, "tmp"), "evict-"); err == nil {
		if os.Rename(dir, filepath.Join(tmp, "repo.git")) == nil {
			dir = tmp
		} else {
			os.Remove(tmp)
		}
	}
	_ = os.RemoveAll(dir)
	g.mu.Lock()
	delete(g.sizes, key)
	delete(g.fetched, key)
	delete(g.lastUsed, key)
	g.mu.Unlock()
}

// evict removes least recently used copies, never keep or one being
// fetched, until the total is under MaxBytes.
func (g *gitMirror) evict(keep string) {
	g.mu.Lock()
	var total int64
	keys := []string{}
	for k, n := range g.sizes {
		total += n
		if _, busy := g.flights[k]; k != keep && !busy {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return g.lastUsed[keys[i]].Before(g.lastUsed[keys[j]]) })
	sizes := g.sizes
	g.mu.Unlock()
	for _, k := range keys {
		if total <= g.maxBytes {
			return
		}
		g.mu.Lock()
		n := sizes[k]
		g.mu.Unlock()
		g.remove(k)
		total -= n
	}
}

// dirBytes is the size of the regular files under dir.
func dirBytes(dir string) int64 {
	var n int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}
