package podcastworker

// The long-running loop (docs/design/podcast-clips-2026-11-15.md 3.1). Ways
// it can fail, written first:
//   S1. two episodes are transcribed at once, or the order is not newest first;
//   S2. a kill mid-episode stores a half transcript or marks the episode
//       done, so it is never redone;
//   S3. one failing episode holds the queue, or is retried at once and
//       forever;
//   S4. an idle worker polls at a fixed fast rate, or its sleep never
//       shrinks back after work arrives;
//   S5. the worker logs a line per poll, so the log volume follows the idle
//       rate (it logs a volume line each hour and a line per episode only).

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeBackend struct {
	mu      sync.Mutex
	pending []PendingEpisode
	put     []string
	polls   int
}

func (b *fakeBackend) PendingEpisodes(_ context.Context, _ time.Time, _ int) ([]PendingEpisode, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.polls++
	return append([]PendingEpisode(nil), b.pending...), nil
}

func (b *fakeBackend) PutTranscript(_ context.Context, id, _ string, _ Result) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.put = append(b.put, id)
	rest := b.pending[:0]
	for _, p := range b.pending {
		if p.EpisodeID != id {
			rest = append(rest, p)
		}
	}
	b.pending = rest
	return nil
}

type fakeEngine struct {
	mu      sync.Mutex
	running int
	maxRun  int
	order   []string
	fail    map[string]error
	onRun   func(url string)
}

func (e *fakeEngine) Engine() string { return "fake" }
func (e *fakeEngine) Transcribe(ctx context.Context, url string) (Result, error) {
	e.mu.Lock()
	e.running++
	e.maxRun = max(e.maxRun, e.running)
	e.order = append(e.order, url)
	fail := e.fail[url]
	e.mu.Unlock()
	if e.onRun != nil {
		e.onRun(url)
	}
	e.mu.Lock()
	e.running--
	e.mu.Unlock()
	if fail != nil {
		return Result{}, fail
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	return Result{Segments: []Segment{{Start: 0, End: 5, Text: "x"}}, Seconds: 60, FileSeconds: 61}, nil
}

func ep(id string) PendingEpisode {
	return PendingEpisode{EpisodeID: id, EnclosureURL: "u-" + id, Show: "s"}
}

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

func runServe(t *testing.T, b *fakeBackend, e *fakeEngine, stopAfterSleeps int, cfg ServeConfig) (sleeps []time.Duration, logs []string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	clk := &clock{t: time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)}
	cfg.Now = clk.now
	cfg.Logf = func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) }
	cfg.Sleep = func(_ context.Context, d time.Duration) {
		sleeps = append(sleeps, d)
		clk.advance(d)
		if len(sleeps) >= stopAfterSleeps {
			cancel()
		}
	}
	if err := Serve(ctx, b, e, cfg); !errors.Is(err, context.Canceled) {
		t.Fatalf("Serve returned %v", err)
	}
	return sleeps, logs
}

func TestS1_OneEpisodeAtATimeNewestFirst(t *testing.T) {
	b := &fakeBackend{pending: []PendingEpisode{ep("a"), ep("b"), ep("c")}}
	e := &fakeEngine{}
	runServe(t, b, e, 1, ServeConfig{})
	if e.maxRun != 1 {
		t.Fatalf("S1: %d transcriptions at once", e.maxRun)
	}
	if strings.Join(e.order, ",") != "u-a,u-b,u-c" || strings.Join(b.put, ",") != "a,b,c" {
		t.Fatalf("S1: order %v stored %v", e.order, b.put)
	}
}

func TestS2_AKillMidEpisodeStoresNothingAndLeavesItPending(t *testing.T) {
	b := &fakeBackend{pending: []PendingEpisode{ep("a"), ep("b")}}
	ctx, cancel := context.WithCancel(context.Background())
	e := &fakeEngine{onRun: func(string) { cancel() }} // the kill arrives during the first episode
	err := Serve(ctx, b, e, ServeConfig{Sleep: func(context.Context, time.Duration) {}, Logf: func(string, ...any) {}})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("S2: %v", err)
	}
	if len(b.put) != 0 || len(b.pending) != 2 {
		t.Fatalf("S2: stored %v, pending %v", b.put, b.pending)
	}
	// A new start finds both and does them.
	e2 := &fakeEngine{}
	runServe(t, b, e2, 1, ServeConfig{})
	if strings.Join(b.put, ",") != "a,b" {
		t.Fatalf("S2: after the restart stored %v", b.put)
	}
}

func TestS3_AFailingEpisodeIsSkippedForAGrowingTimeAndTheRestGoOn(t *testing.T) {
	b := &fakeBackend{pending: []PendingEpisode{ep("bad"), ep("ok")}}
	e := &fakeEngine{fail: map[string]error{"u-bad": errors.New("403")}}
	sleeps, _ := runServe(t, b, e, 3, ServeConfig{FailBackoff: time.Hour, MinIdle: time.Minute, MaxIdle: 4 * time.Minute})
	if strings.Join(b.put, ",") != "ok" {
		t.Fatalf("S3: stored %v", b.put)
	}
	tries := 0
	for _, u := range e.order {
		if u == "u-bad" {
			tries++
		}
	}
	if tries != 1 {
		t.Fatalf("S3: the failing episode was tried %d times inside its skip, want 1 (sleeps %v)", tries, sleeps)
	}
}

func TestS4_IdleSleepGrowsToACapAndResetsOnWork(t *testing.T) {
	b := &fakeBackend{}
	e := &fakeEngine{}
	sleeps, logs := runServe(t, b, e, 6, ServeConfig{MinIdle: 30 * time.Second, MaxIdle: 5 * time.Minute})
	want := []time.Duration{30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute}
	if fmt.Sprint(sleeps) != fmt.Sprint(want) {
		t.Fatalf("S4: sleeps %v, want %v", sleeps, want)
	}
	// S5: polls do not log; the hour's volume line does, once the hour is over.
	for _, l := range logs {
		if !strings.Contains(l, "volume") {
			t.Fatalf("S5: an idle poll logged %q", l)
		}
	}
	// Work arriving resets the sleep.
	b2 := &fakeBackend{}
	e2 := &fakeEngine{}
	n := 0
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got []time.Duration
	cfg := ServeConfig{MinIdle: time.Second, MaxIdle: time.Minute, Logf: func(string, ...any) {}, Sleep: func(_ context.Context, d time.Duration) {
		got = append(got, d)
		n++
		if n == 3 {
			b2.mu.Lock()
			b2.pending = []PendingEpisode{ep("late")}
			b2.mu.Unlock()
		}
		if n == 5 {
			cancel()
		}
	}}
	_ = Serve(ctx, b2, e2, cfg)
	if fmt.Sprint(got) != fmt.Sprint([]time.Duration{time.Second, 2 * time.Second, 4 * time.Second, time.Second, 2 * time.Second}) {
		t.Fatalf("S4: sleeps %v: after work the sleep must start over", got)
	}
}

func TestS5_VolumeLineOncePerHour(t *testing.T) {
	b := &fakeBackend{pending: []PendingEpisode{ep("a")}}
	e := &fakeEngine{}
	_, logs := runServe(t, b, e, 130, ServeConfig{MinIdle: time.Minute, MaxIdle: time.Minute})
	volume := 0
	for _, l := range logs {
		if strings.Contains(l, "volume") {
			volume++
		}
	}
	// 130 sleeps of a minute is two hours and ten minutes: two hourly lines and the last.
	if volume != 3 {
		t.Fatalf("S5: %d volume lines in %d log lines, want 3: %v", volume, len(logs), logs)
	}
}

func TestPoliteNeighbourSetsPriority(t *testing.T) {
	problems := BeAPoliteNeighbour()
	for _, p := range problems {
		t.Logf("polite: %v", p)
	}
	if MaxThreads != 4 {
		t.Fatal("the thread cap is four")
	}
}

// The disk guard (owner and lead, 2026-11-16: the service must never fill the
// shared disk). Ways it can fail, written first:
//
//	D1. a download starts with less free space than the floor;
//	D2. a refusal for space counts as the episode's failure and puts it on
//	    the skip list for hours, so it is never retried when space returns;
//	D3. the work directory keeps an episode's audio after its transcription
//	    ends, whatever the outcome.
func TestD1_TheDownloadIsRefusedUnderTheFloor(t *testing.T) {
	var asked string
	tr := &Transcriber{WorkDir: t.TempDir(), MinFreeBytes: 15 << 30, FFmpeg: "false", Parakeet: "false",
		FreeBytes: func(dir string) (uint64, error) { asked = dir; return 14 << 30, nil }}
	_, err := tr.Transcribe(context.Background(), "http://127.0.0.1:1/never")
	if !errors.Is(err, ErrLowDisk) || !strings.Contains(err.Error(), "14.0 GiB free") || asked != tr.WorkDir {
		t.Fatalf("D1: err = %v, asked %q", err, asked)
	}
	// At the floor the check passes (the download is then tried and fails on the dead address).
	tr.FreeBytes = func(string) (uint64, error) { return 15 << 30, nil }
	if _, err := tr.Transcribe(context.Background(), "http://127.0.0.1:1/never"); errors.Is(err, ErrLowDisk) || err == nil {
		t.Fatalf("D1: at the floor: %v", err)
	}
	// A filesystem that cannot be read is refused too, never assumed empty of care.
	tr.FreeBytes = func(string) (uint64, error) { return 0, errors.New("statfs failed") }
	if _, err := tr.Transcribe(context.Background(), "http://127.0.0.1:1/never"); err == nil || !strings.Contains(err.Error(), "statfs failed") {
		t.Fatalf("D1: unreadable: %v", err)
	}
	// The real reader answers something sensible for the temp dir.
	if free, err := freeBytes(t.TempDir()); err != nil || free == 0 {
		t.Fatalf("D1: freeBytes = %d %v", free, err)
	}
}

func TestD2_ALowDiskIsNotTheEpisodesFailure(t *testing.T) {
	b := &fakeBackend{pending: []PendingEpisode{ep("a")}}
	low := true
	e := &fakeEngine{fail: map[string]error{}}
	eng := engineFunc{engine: e, before: func(url string) error {
		if low {
			return fmt.Errorf("%w: 3.0 GiB free", ErrLowDisk)
		}
		return nil
	}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var logs []string
	var sleeps []time.Duration
	cfg := ServeConfig{MinIdle: time.Second, MaxIdle: time.Minute, FailBackoff: 24 * time.Hour,
		Logf: func(f string, a ...any) { logs = append(logs, fmt.Sprintf(f, a...)) },
		Sleep: func(_ context.Context, d time.Duration) {
			sleeps = append(sleeps, d)
			if len(sleeps) == 2 {
				low = false // space is back
			}
			if len(sleeps) == 4 {
				cancel()
			}
		}}
	_ = Serve(ctx, b, eng, cfg)
	if len(b.put) != 1 || b.put[0] != "a" {
		t.Fatalf("D2: stored %v, sleeps %v, logs %v", b.put, sleeps, logs)
	}
	refusals := 0
	for _, l := range logs {
		if strings.Contains(l, "download refused") {
			refusals++
		}
	}
	if refusals != 2 {
		t.Fatalf("D2: %d refusals logged: %v", refusals, logs)
	}
}

// engineFunc wraps the fake engine with a check that runs first.
type engineFunc struct {
	engine *fakeEngine
	before func(url string) error
}

func (e engineFunc) Engine() string { return "fake" }
func (e engineFunc) Transcribe(ctx context.Context, url string) (Result, error) {
	if err := e.before(url); err != nil {
		return Result{}, err
	}
	return e.engine.Transcribe(ctx, url)
}

func TestD3_NoAudioSurvivesATranscription(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stubs")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Range", "bytes 0-0/4096")
			w.WriteHeader(http.StatusPartialContent)
			_, _ = w.Write([]byte{0})
			return
		}
		_, _ = w.Write(make([]byte, 4096))
	}))
	defer srv.Close()
	bin := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(bin, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	// A stub ffmpeg: with -i alone it prints a duration; with an output it
	// writes a 10 second WAV-sized file to the last argument.
	ffmpeg := write("ffmpeg", `for last; do :; done
case "$last" in *.wav) head -c 320044 /dev/zero > "$last" ;; *) echo "  Duration: 00:01:05.50, start: 0.0" >&2; exit 1 ;; esac`)
	parakeet := write("parakeet", `echo '{"text":"x","words":[{"w":"Hello","start":0.1,"end":0.4},{"w":"world.","start":0.5,"end":0.9}]}'`)
	work := t.TempDir()
	tr := &Transcriber{FFmpeg: ffmpeg, Parakeet: parakeet, Model: "m/tdt_ctc-110m-q8_0.gguf", VAD: "v", WorkDir: work, MinFreeBytes: -1, HTTP: srv.Client()}
	res, err := tr.Transcribe(context.Background(), srv.URL+"/a.mp3")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Segments) != 1 || res.Segments[0].Text != "Hello world." || res.Seconds < 9.9 || res.Seconds > 10.1 || res.FileSeconds != 65.5 {
		t.Fatalf("D3: result = %+v", res)
	}
	if res.StableFile == nil || !*res.StableFile {
		t.Fatalf("D3: the file was served unchanged (4096 bytes twice) but StableFile = %v", res.StableFile)
	}
	if left, _ := os.ReadDir(work); len(left) != 0 {
		t.Fatalf("D3: %d entries left in the work directory after a success", len(left))
	}
	// A failure leaves nothing either.
	tr.FFmpeg = write("badffmpeg", `exit 1`)
	if _, err := tr.Transcribe(context.Background(), srv.URL+"/a.mp3"); err == nil {
		t.Fatal("D3: a failing ffmpeg must fail the episode")
	}
	if left, _ := os.ReadDir(work); len(left) != 0 {
		t.Fatalf("D3: %d entries left in the work directory after a failure", len(left))
	}
}

// The work directory is the worker's own and bounded: a crash cannot leave it
// growing, and only its own episode directories are ever touched.
func TestD4_TheWorkDirIsBoundedAndSweptAndOnlyTheWorkersOwnIsTouched(t *testing.T) {
	work := t.TempDir()
	leftover := filepath.Join(work, "episode-123")
	if err := os.MkdirAll(leftover, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(leftover, "in.audio"), make([]byte, 3000), 0o644); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(work, "someone-elses-file")
	if err := os.WriteFile(foreign, make([]byte, 5000), 0o644); err != nil {
		t.Fatal(err)
	}
	foreignDir := filepath.Join(work, "other-dir")
	if err := os.MkdirAll(foreignDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tr := &Transcriber{WorkDir: work, MinFreeBytes: -1, WorkDirMaxBytes: 2000, FFmpeg: "false", Parakeet: "false"}
	// The leftover (3000 bytes) is over the 2000-byte bound: refused as a full disk.
	if _, err := tr.Transcribe(context.Background(), "http://127.0.0.1:1/x"); !errors.Is(err, ErrLowDisk) || !strings.Contains(err.Error(), "work directory") {
		t.Fatalf("D4: err = %v", err)
	}
	// Foreign files do not count toward it.
	tr.WorkDirMaxBytes = 4000
	if _, err := tr.Transcribe(context.Background(), "http://127.0.0.1:1/x"); errors.Is(err, ErrLowDisk) {
		t.Fatalf("D4: foreign bytes were counted: %v", err)
	}
	n, err := tr.SweepWorkDir()
	if err != nil || n != 1 {
		t.Fatalf("D4: swept %d (%v), want 1", n, err)
	}
	for _, keep := range []string{foreign, foreignDir} {
		if _, err := os.Stat(keep); err != nil {
			t.Fatalf("D4: the sweep touched %s: %v", keep, err)
		}
	}
	if _, err := os.Stat(leftover); err == nil {
		t.Fatal("D4: the leftover episode directory survived the sweep")
	}
	if line := DescribeFreeSpace(work); !strings.Contains(line, "GiB") {
		t.Fatalf("D4: free space line = %q", line)
	}
}
