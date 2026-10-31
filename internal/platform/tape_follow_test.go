package platform

// The tape follow route (tape_follow.go) and its follower (internal/
// tapefollow): failure modes.
//
//  F1  Pulling in pages is not the same as one export: N bounded bundles,
//      applied in order, do not hold the receipts, events, head row and
//      snapshot a single full export holds, or do not verify as one tape.
//  F2  A source that advanced after a sync is not followed, or an up-to-date
//      follower is sent a bundle (an export per poll).
//  F3  A crash mid-apply (after verify, after the rename, before the
//      commit) leaves a store that does not reopen to the previous verified
//      state, or that cannot resume to the head.
//  F4  A lost network (a cut stream, a stalled stream) leaves a partial
//      bundle applied, or the follower cannot resume.
//  F5  A tampered bundle (a flipped byte, a forged manifest head) is
//      applied, or a refused one changes the store.
//  F6  A follower whose head is not on the tape (a fork) is sent a delta
//      onto it instead of a refusal, or keeps retrying.
//  F7  The route serves without the internal credential and an owner, or
//      serves exports past its rate bounds.
//  F8  The local chain is not the host's import format: importing its
//      bundles in order into an empty platform store does not answer like
//      the source.
//  F9  A bundle signed by a key the follower does not trust (a forged
//      server, or a consistent bundle re-signed) is applied; or a bounded
//      bootstrap page stops before the snapshot_recorded event that names
//      its root, so the root cannot be authenticated.

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/tapefollow"
)

// followServer serves the follow route of c as the proxy would (the
// internal marker and the owner), with fault injection.
type followServer struct {
	*httptest.Server
	handler *Handler
	trust   *bundle.TrustSet
	mu      sync.Mutex
	// fault, when set, serves the request instead of the handler.
	fault    func(w http.ResponseWriter, r *http.Request, next http.Handler) bool
	requests atomic.Int64
}

func newFollowServer(t *testing.T, c *drillComputer) *followServer {
	t.Helper()
	// A clock that moves a minute per call keeps the bucket full.
	var tick atomic.Int64
	prev := tapeFollowLimits
	tapeFollowLimits = newTapeFollowLimiter()
	tapeFollowLimits.now = func() time.Time { return time.Unix(0, 0).Add(time.Duration(tick.Add(1)) * time.Minute) }
	t.Cleanup(func() { tapeFollowLimits = prev })
	fs := &followServer{handler: &Handler{service: c.service, eventCAS: c.cas}, trust: c.trust()}
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := strings.CutPrefix(r.URL.Path, "/api/computers/")
		id, ok2 := strings.CutSuffix(id, "/tape/bundle")
		if !ok || !ok2 {
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		q.Set("computer_id", id)
		r.URL.Path, r.URL.RawQuery = tapeFollowRoute, q.Encode()
		r.Header.Set("X-Internal-Caller", "true")
		r.Header.Set("X-Authenticated-User", "owner-follow")
		fs.handler.HandleTapeFollowBundle(w, r)
	})
	fs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.requests.Add(1)
		fs.mu.Lock()
		fault := fs.fault
		fs.mu.Unlock()
		if fault != nil && fault(w, r, inner) {
			return
		}
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(fs.Close)
	return fs
}

func (fs *followServer) setFault(f func(w http.ResponseWriter, r *http.Request, next http.Handler) bool) {
	fs.mu.Lock()
	fs.fault = f
	fs.mu.Unlock()
}

func newFollower(t *testing.T, fs *followServer, id, dir string, maxEvents int64) (*tapefollow.Store, *tapefollow.Syncer) {
	t.Helper()
	store, err := tapefollow.Open(dir, id, fs.trust)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store, &tapefollow.Syncer{
		Store:     store,
		Client:    &tapefollow.Client{Backend: fs.URL, HTTP: fs.Client(), StallTimeout: 300 * time.Millisecond},
		MaxEvents: maxEvents,
	}
}

// catchUp steps until the follower is at the source head.
func catchUp(t *testing.T, s *tapefollow.Syncer) int {
	t.Helper()
	steps := 0
	for {
		more, err := s.Step(context.Background())
		if err != nil {
			t.Fatalf("step %d: %v", steps, err)
		}
		steps++
		if !more {
			return steps
		}
		if steps > 1000 {
			t.Fatal("follower never caught up")
		}
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
}

func TestTapeFollowIncrementalEqualsFullExport(t *testing.T) {
	const id = "computer-follow-incremental"
	c := newDrillComputer(t, id)
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	c.appendMixed(4, "pre")
	snap := c.snapshot(3)
	c.appendSnapshotRecorded(snap.Root)
	c.appendMixed(10, "post")
	fs := newFollowServer(t, c)
	dir := filepath.Join(t.TempDir(), id)
	store, syncer := newFollower(t, fs, id, dir, 3)

	if steps := catchUp(t, syncer); steps != 4 { // 10 events in pages of 3
		t.Fatalf("caught up in %d pulls, want 4", steps)
	}
	followerMatchesExport(t, c, store)
	first := store.BundleDirs()[0]
	for _, chunk := range snap.Chunks {
		info, err := os.Stat(filepath.Join(first, bundle.ChunksDir, chunk))
		if err != nil {
			t.Fatalf("bootstrap bundle lacks chunk %s: %v", chunk, err)
		}
		if info.Mode().Perm()&0o222 != 0 {
			t.Fatalf("chunk %s is writable (%v): the local copy is read-only", chunk, info.Mode())
		}
	}

	// F2: an up-to-date poll is a head read, not a bundle.
	before := fs.requests.Load()
	if more, err := syncer.Step(context.Background()); err != nil || more {
		t.Fatalf("up-to-date step = %v, %v", more, err)
	}
	if n := len(store.BundleDirs()); n != 4 || fs.requests.Load() != before+1 {
		t.Fatalf("an up-to-date poll changed the store (%d bundles) or made %d requests", n, fs.requests.Load()-before)
	}
	// F2: the source advances; the follower follows.
	c.appendMixed(5, "later")
	catchUp(t, syncer)
	followerMatchesExport(t, c, store)
	if st, _ := store.State(); st.HeadSeq != int64(c.head().Sequence) {
		t.Fatalf("local head %d, source %d", st.HeadSeq, c.head().Sequence)
	}

	// F8: the chain imports, bundle by bundle, into an empty platform store
	// that then answers like the source.
	dst := newDrillComputer(t, id)
	for _, b := range store.BundleDirs() {
		if _, err := bundle.Import(context.Background(), b, dst.store.db, dst.root, c.trust()); err != nil {
			t.Fatalf("import %s: %v", filepath.Base(b), err)
		}
	}
	assertSameComputer(t, c, dst, snap.HeadSequence)
}

// followerMatchesExport is F1.
func followerMatchesExport(t *testing.T, c *drillComputer, store *tapefollow.Store) {
	t.Helper()
	fullDir := filepath.Join(t.TempDir(), "full")
	full := mustExport(t, c, fullDir)
	st, err := store.VerifyAll()
	if err != nil {
		t.Fatalf("verify the local copy: %v", err)
	}
	if st.StartSeq != full.Tape.StartSeq || st.StartHead != full.Tape.StartHead || st.HeadSeq != full.Tape.HeadSeq || st.HeadDigest != full.Tape.HeadDigest {
		t.Fatalf("local copy (%d %s, %d %s], full export (%d %s, %d %s]", st.StartSeq, st.StartHead, st.HeadSeq, st.HeadDigest, full.Tape.StartSeq, full.Tape.StartHead, full.Tape.HeadSeq, full.Tape.HeadDigest)
	}
	if full.SnapshotRoot == nil || st.SnapshotRoot != full.SnapshotRoot.Root {
		t.Fatalf("local snapshot root %q, full export %+v", st.SnapshotRoot, full.SnapshotRoot)
	}
	var got []string
	dirs := store.BundleDirs()
	for _, dir := range dirs {
		for _, line := range readLines(t, filepath.Join(dir, bundle.ReceiptsName)) {
			if line != "" {
				got = append(got, line)
			}
		}
	}
	want := readLines(t, filepath.Join(fullDir, bundle.ReceiptsName))
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("the chain's %d receipts differ from the full export's %d", len(got), len(want))
	}
	last, err := bundle.ReadManifest(dirs[len(dirs)-1])
	if err != nil {
		t.Fatal(err)
	}
	if last.Tape.Head != full.Tape.Head {
		t.Fatalf("last bundle head row %+v, source %+v", last.Tape.Head, full.Tape.Head)
	}
}

func TestTapeFollowCrashMidApplyResumes(t *testing.T) {
	const id = "computer-follow-crash"
	c := newDrillComputer(t, id)
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	snap := c.snapshot(2)
	c.appendSnapshotRecorded(snap.Root)
	c.appendMixed(9, "x")
	fs := newFollowServer(t, c)
	dir := filepath.Join(t.TempDir(), id)
	for _, point := range []string{"verified", "renamed"} {
		t.Run(point, func(t *testing.T) {
			store, syncer := newFollower(t, fs, id, dir, 3)
			if _, err := syncer.Step(context.Background()); err != nil {
				t.Fatal(err)
			}
			committed, _ := store.State()
			crash := errors.New("crash at " + point)
			restore := tapefollow.SetApplyHookForTesting(func(p string) error {
				if p == point {
					return crash
				}
				return nil
			})
			if _, err := syncer.Step(context.Background()); !errors.Is(err, crash) {
				t.Fatalf("step = %v, want the crash", err)
			}
			restore()
			_ = store.Close()

			reopened, err := tapefollow.Open(dir, id, fs.trust)
			if err != nil {
				t.Fatalf("reopen after the crash: %v", err)
			}
			st, _ := reopened.State()
			if st.HeadSeq != committed.HeadSeq || st.HeadDigest != committed.HeadDigest || len(st.Bundles) != len(committed.Bundles) {
				t.Fatalf("after a crash at %s the store is at %d with %d bundles, want %d with %d", point, st.HeadSeq, len(st.Bundles), committed.HeadSeq, len(committed.Bundles))
			}
			if _, err := reopened.VerifyAll(); err != nil {
				t.Fatalf("the reopened store does not verify: %v", err)
			}
			entries, _ := os.ReadDir(filepath.Join(dir, tapefollow.BundlesDir))
			if len(entries) != len(st.Bundles) {
				t.Fatalf("%d bundle directories, %d named: the crashed apply's was not cleared", len(entries), len(st.Bundles))
			}
			if staged, _ := os.ReadDir(filepath.Join(dir, tapefollow.StagingDir)); len(staged) != 0 {
				t.Fatalf("staging holds %d entries after reopen", len(staged))
			}
			_ = reopened.Close()
			// Each subtest starts the store over.
			if err := os.RemoveAll(dir); err != nil {
				_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
					if err == nil && info.IsDir() {
						_ = os.Chmod(p, 0o750)
					}
					return nil
				})
				_ = os.RemoveAll(dir)
			}
		})
	}
	store, syncer := newFollower(t, fs, id, dir, 3)
	catchUp(t, syncer)
	followerMatchesExport(t, c, store)
}

func TestTapeFollowNetworkLossResumes(t *testing.T) {
	const id = "computer-follow-network"
	c := newDrillComputer(t, id)
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	snap := c.snapshot(2)
	c.appendSnapshotRecorded(snap.Root)
	c.appendMixed(8, "n")
	fs := newFollowServer(t, c)
	store, syncer := newFollower(t, fs, id, filepath.Join(t.TempDir(), id), 4)
	catchUpErr := func() error { _, err := syncer.Step(context.Background()); return err }

	// A cut stream: the first bytes, then the connection drops.
	var cuts atomic.Int64
	fs.setFault(func(w http.ResponseWriter, r *http.Request, next http.Handler) bool {
		if cuts.Add(1) > 2 {
			return false
		}
		rec := httptest.NewRecorder()
		next.ServeHTTP(rec, r)
		for k, v := range rec.Header() {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		body := rec.Body.Bytes()
		_, _ = w.Write(body[:len(body)/2])
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	})
	for i := 0; i < 2; i++ {
		if err := catchUpErr(); err == nil {
			t.Fatalf("pull %d over a cut stream succeeded", i)
		}
		if _, ok := store.State(); ok {
			t.Fatal("a cut stream applied a bundle")
		}
	}
	// A stalled stream: headers and a little, then nothing.
	fs.setFault(func(w http.ResponseWriter, r *http.Request, next http.Handler) bool {
		w.Header().Set("Content-Type", "application/x-tar")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(make([]byte, 512))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
		return true
	})
	if err := catchUpErr(); err == nil || !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("pull over a stalled stream = %v, want a stall", err)
	}
	// The network comes back: the follower resumes from its committed head.
	fs.setFault(nil)
	catchUp(t, syncer)
	followerMatchesExport(t, c, store)
}

// retar rewrites a bundle tar stream through edit (name, bytes) -> bytes.
func retar(t *testing.T, raw []byte, edit func(name string, data []byte) []byte) []byte {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(raw))
	var out bytes.Buffer
	tw := tar.NewWriter(&out)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(tr)
		data = edit(hdr.Name, data)
		hdr.Size = int64(len(data))
		_ = tw.WriteHeader(hdr)
		_, _ = tw.Write(data)
	}
	_ = tw.Close()
	return out.Bytes()
}

func TestTapeFollowTamperedBundleRefused(t *testing.T) {
	const id = "computer-follow-tamper"
	c := newDrillComputer(t, id)
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	snap := c.snapshot(2)
	c.appendSnapshotRecorded(snap.Root)
	c.appendMixed(6, "t")
	fs := newFollowServer(t, c)
	store, syncer := newFollower(t, fs, id, filepath.Join(t.TempDir(), id), 3)
	if _, err := syncer.Step(context.Background()); err != nil {
		t.Fatal(err)
	}
	committed, _ := store.State()
	other := computerevent.DigestBytes([]byte("not the head"))
	for name, edit := range map[string]func(string, []byte) []byte{
		"flipped pack byte": func(n string, d []byte) []byte {
			if n == bundle.PackName && len(d) > 10 {
				d[len(d)/2] ^= 0x01
			}
			return d
		},
		"forged manifest head": func(n string, d []byte) []byte {
			if n == bundle.ManifestName {
				m := bytes.ReplaceAll(d, []byte(`"head_digest": "`), []byte(`"head_digest": "`+other+`","was": "`))
				return m
			}
			return d
		},
		"dropped receipt line": func(n string, d []byte) []byte {
			if n == bundle.ReceiptsName {
				lines := bytes.SplitAfter(d, []byte("\n"))
				return bytes.Join(lines[1:], nil)
			}
			return d
		},
		"a smuggled file": nil,
	} {
		t.Run(name, func(t *testing.T) {
			fs.setFault(func(w http.ResponseWriter, r *http.Request, next http.Handler) bool {
				rec := httptest.NewRecorder()
				next.ServeHTTP(rec, r)
				for k, v := range rec.Header() {
					w.Header()[k] = v
				}
				body := rec.Body.Bytes()
				if rec.Code == http.StatusOK {
					if edit != nil {
						body = retar(t, body, edit)
					} else {
						var buf bytes.Buffer
						tw := tar.NewWriter(&buf)
						_ = tw.WriteHeader(&tar.Header{Name: "../escape", Size: 1, Mode: 0o644, Typeflag: tar.TypeReg})
						_, _ = tw.Write([]byte("x"))
						_ = tw.Close()
						body = buf.Bytes()
					}
				}
				w.WriteHeader(rec.Code)
				_, _ = w.Write(body)
				return true
			})
			if _, err := syncer.Step(context.Background()); err == nil {
				t.Fatal("a tampered bundle was applied")
			}
			st, _ := store.State()
			if st.HeadSeq != committed.HeadSeq || len(st.Bundles) != len(committed.Bundles) {
				t.Fatalf("a refused bundle changed the store: %d bundles at %d", len(st.Bundles), st.HeadSeq)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(store.Dir()), "escape")); err == nil {
				t.Fatal("a tar entry escaped the staging directory")
			}
		})
	}
	fs.setFault(nil)
	catchUp(t, syncer)
	followerMatchesExport(t, c, store)
}

func TestTapeFollowForkAndAuthority(t *testing.T) {
	const id = "computer-follow-fork"
	c := newDrillComputer(t, id)
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	c.appendMixed(5, "f")
	fs := newFollowServer(t, c)
	store, syncer := newFollower(t, fs, id, filepath.Join(t.TempDir(), id), 2)
	// No snapshot root: the bootstrap is the tape from genesis.
	catchUp(t, syncer)
	st, _ := store.State()
	if st.StartSeq != 0 || st.StartHead != computerevent.ZeroHead || st.HeadSeq != 6 {
		t.Fatalf("genesis follower state %+v", st)
	}
	// F6: a head that is not on the tape is refused 409, as a fork.
	forked := st
	forked.HeadSeq, forked.HeadDigest = 3, computerevent.DigestBytes([]byte("fork"))
	_, err := syncer.Client.Pull(context.Background(), id, &forked, 2, 1<<20, t.TempDir())
	if !tapefollow.IsForked(err) {
		t.Fatalf("pull from a forked head = %v, want 409", err)
	}
	ahead := st
	ahead.HeadSeq = 99
	if _, err := syncer.Client.Pull(context.Background(), id, &ahead, 2, 1<<20, t.TempDir()); !tapefollow.IsForked(err) {
		t.Fatalf("pull from past the head = %v, want 409", err)
	}

	// F7: without the internal marker and an owner the route refuses.
	h := fs.handler
	for name, set := range map[string]func(*http.Request){
		"no marker": func(r *http.Request) { r.Header.Set("X-Authenticated-User", "o") },
		"no owner":  func(r *http.Request) { r.Header.Set("X-Internal-Caller", "true") },
	} {
		req := httptest.NewRequest(http.MethodGet, tapeFollowRoute+"?computer_id="+url.QueryEscape(id), nil)
		set(req)
		rec := httptest.NewRecorder()
		h.HandleTapeFollowBundle(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s: status %d", name, rec.Code)
		}
	}
	for _, q := range []string{"&branch=b7", "&have_seq=3", "&have_head=" + st.HeadDigest, "&max_events=0", "&max_events=20001"} {
		req := httptest.NewRequest(http.MethodGet, tapeFollowRoute+"?computer_id="+id+q, nil)
		req.Header.Set("X-Internal-Caller", "true")
		req.Header.Set("X-Authenticated-User", "o")
		rec := httptest.NewRecorder()
		h.HandleTapeFollowBundle(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("query %s: status %d", q, rec.Code)
		}
	}
}

func TestTapeFollowRateLimit(t *testing.T) {
	l := newTapeFollowLimiter()
	now := time.Unix(1000, 0)
	l.now = func() time.Time { return now }
	release, ok, _ := l.admit("a", true)
	if !ok {
		t.Fatal("first export refused")
	}
	if _, ok, _ := l.admit("a", true); ok {
		t.Fatal("a second concurrent export of one computer admitted")
	}
	r2, ok, _ := l.admit("b", true)
	if !ok {
		t.Fatal("another computer's export refused")
	}
	if _, ok, _ := l.admit("c", true); ok {
		t.Fatalf("a third concurrent export admitted (global bound %d)", tapeFollowGlobal)
	}
	release()
	r2()
	n := 0
	for {
		rel, ok, wait := l.admit("d", false)
		if !ok {
			if wait <= 0 {
				t.Fatal("a refusal without a wait")
			}
			break
		}
		rel()
		n++
	}
	if n < tapeFollowBurst-1 || n > tapeFollowBurst {
		t.Fatalf("admitted %d requests in one instant, want the burst %d", n, tapeFollowBurst)
	}
	now = now.Add(3 * tapeFollowRefill)
	if _, ok, _ := l.admit("d", false); !ok {
		t.Fatal("the bucket did not refill")
	}
}

// TestTapeFollowCost measures the host's cost of one streamed export per MB
// (corpusd's CPU and allocation; PostgreSQL's own CPU is not counted). Run
// with CHOIR_TAPE_FOLLOW_COST=1.
func TestTapeFollowCost(t *testing.T) {
	if os.Getenv("CHOIR_TAPE_FOLLOW_COST") == "" {
		t.Skip("set CHOIR_TAPE_FOLLOW_COST=1 to measure")
	}
	const id = "computer-follow-cost"
	c := newDrillComputer(t, id)
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	snap := c.snapshot(16) // ~64 KiB of chunks
	c.appendSnapshotRecorded(snap.Root)
	payload := make([]byte, 32<<10)
	for i := 0; i < 400; i++ {
		payload[0], payload[1] = byte(i), byte(i>>8)
		c.appendEvent(computerevent.EventMessageRecorded, "text/plain", payload)
	}
	fs := newFollowServer(t, c)
	cpu := func() time.Duration {
		var ru syscall.Rusage
		_ = syscall.Getrusage(syscall.RUSAGE_SELF, &ru)
		return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
	}
	for _, maxEvents := range []int{2000, 100} {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		c0, t0 := cpu(), time.Now()
		var total int64
		var have string
		for {
			u := fs.URL + "/api/computers/" + id + "/tape/bundle?max_events=" + strconv.Itoa(maxEvents) + have
			resp, err := fs.Client().Get(u)
			if err != nil {
				t.Fatal(err)
			}
			if resp.StatusCode == http.StatusNoContent {
				resp.Body.Close()
				break
			}
			n, _ := io.Copy(io.Discard, bufio.NewReader(resp.Body))
			resp.Body.Close()
			total += n
			have = "&have_seq=" + resp.Header.Get("X-Choir-Bundle-Head-Seq") + "&have_head=" + resp.Header.Get("X-Choir-Bundle-Head-Digest")
		}
		elapsed, used := time.Since(t0), cpu()-c0
		runtime.ReadMemStats(&after)
		mb := float64(total) / (1 << 20)
		t.Logf("max_events=%d: %.2f MB streamed in %s; corpusd-side CPU %.1f ms/MB, allocated %.2f MB/MB, heap in use %.1f MB",
			maxEvents, mb, elapsed.Round(time.Millisecond), float64(used.Milliseconds())/mb, float64(after.TotalAlloc-before.TotalAlloc)/(1<<20)/mb, float64(after.HeapInuse)/(1<<20))
	}
}

func TestTapeFollowAuthenticity(t *testing.T) {
	const id = "computer-follow-authentic"
	c := newDrillComputer(t, id)
	c.appendEvent(computerevent.EventGenesisImported, "", nil)
	snap := c.snapshot(2)
	c.appendMixed(5, "between")
	c.appendSnapshotRecorded(snap.Root) // named 6 events after its head
	c.appendMixed(3, "after")
	fs := newFollowServer(t, c)

	// F9: a follower trusting another platform key refuses every bundle.
	stranger := newDrillComputer(t, "computer-follow-stranger")
	untrusting, err := tapefollow.Open(filepath.Join(t.TempDir(), id), id, stranger.trust())
	if err != nil {
		t.Fatal(err)
	}
	defer untrusting.Close()
	s := &tapefollow.Syncer{Store: untrusting, Client: &tapefollow.Client{Backend: fs.URL, HTTP: fs.Client()}, MaxEvents: 2}
	if _, err := s.Step(context.Background()); err == nil || !strings.Contains(err.Error(), "bundle refused") {
		t.Fatalf("a bundle signed by an untrusted key: step = %v, want a refusal", err)
	}
	if _, ok := untrusting.State(); ok {
		t.Fatal("an unauthenticated bundle was applied")
	}

	// F9: the bootstrap page reaches the naming event although it is past
	// max_events, and then pages resume at max_events.
	store, syncer := newFollower(t, fs, id, filepath.Join(t.TempDir(), id+"-trusting"), 2)
	if _, err := syncer.Step(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	st, _ := store.State()
	if st.SnapshotRoot != snap.Root || st.HeadSeq != snap.HeadSequence+6 {
		t.Fatalf("bootstrap ends at %d (snapshot %q), want the naming event at %d", st.HeadSeq, st.SnapshotRoot, snap.HeadSequence+6)
	}
	catchUp(t, syncer)
	followerMatchesExport(t, c, store)
}
