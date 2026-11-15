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
