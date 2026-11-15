package podcastworker

import (
	"context"
	"log"
	"sync"
	"time"
)

// The long-running mode (docs/design/podcast-clips-2026-11-15.md section 3.1):
// poll the platform for episodes that need a transcript, transcribe ONE at a
// time, post it, and sleep with a backoff when there is nothing to do.
//
// Restarting is safe by construction: the platform's episode list is the
// queue, and an episode leaves it only when its transcript is stored, so a
// kill mid-episode leaves the episode pending and the next start does it
// again. No claim is held server side (one worker per fleet tonight); a
// second worker would at worst repeat an episode, because the transcript put
// replaces. A failing episode is skipped for a growing time, in memory, so one
// bad enclosure cannot hold the queue.

// Backend is what the loop needs of the platform.
type Backend interface {
	PendingEpisodes(ctx context.Context, since time.Time, limit int) ([]PendingEpisode, error)
	PutTranscript(ctx context.Context, episodeID, engine string, r Result) error
}

// Engine transcribes one enclosure.
type Engine interface {
	Transcribe(ctx context.Context, enclosureURL string) (Result, error)
	Engine() string
}

// ServeConfig is the loop's tuning. Zero values take the defaults.
type ServeConfig struct {
	Since       time.Duration // how far back episodes are transcribed (default 72 h)
	ListLimit   int           // episodes asked for each poll (default 40)
	MinIdle     time.Duration // first idle sleep (default 30 s)
	MaxIdle     time.Duration // longest idle sleep (default 10 min)
	FailBackoff time.Duration // first skip of a failing episode (default 15 min)
	MaxFailSkip time.Duration // longest skip (default 6 h)
	Intake      func(ctx context.Context) error
	IntakeEvery time.Duration // how often Intake runs (default 30 min); 0 with Intake nil is never
	ReportEvery time.Duration // the volume line (default 1 h)
	Now         func() time.Time
	Sleep       func(ctx context.Context, d time.Duration) // a test seam
	Logf        func(format string, args ...any)
}

func (c *ServeConfig) defaults() {
	if c.Since == 0 {
		c.Since = 72 * time.Hour
	}
	if c.ListLimit == 0 {
		c.ListLimit = 40
	}
	if c.MinIdle == 0 {
		c.MinIdle = 30 * time.Second
	}
	if c.MaxIdle == 0 {
		c.MaxIdle = 10 * time.Minute
	}
	if c.FailBackoff == 0 {
		c.FailBackoff = 15 * time.Minute
	}
	if c.MaxFailSkip == 0 {
		c.MaxFailSkip = 6 * time.Hour
	}
	if c.Intake != nil && c.IntakeEvery == 0 {
		c.IntakeEvery = 30 * time.Minute
	}
	if c.ReportEvery == 0 {
		c.ReportEvery = time.Hour
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	if c.Sleep == nil {
		c.Sleep = sleepCtx
	}
	if c.Logf == nil {
		c.Logf = log.Printf
	}
}

func sleepCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

type failure struct {
	count int
	until time.Time
}

// Serve runs until ctx is cancelled. It returns ctx.Err().
func Serve(ctx context.Context, b Backend, e Engine, cfg ServeConfig) error {
	cfg.defaults()
	var (
		failed     = map[string]failure{}
		idle       = cfg.MinIdle
		lastIntake time.Time
		windowFrom = cfg.Now()
		stats      struct {
			stored, failed int
			audio          float64
			wall           time.Duration
		}
		mu sync.Mutex
	)
	report := func(force bool) {
		mu.Lock()
		defer mu.Unlock()
		now := cfg.Now()
		if !force && now.Sub(windowFrom) < cfg.ReportEvery {
			return
		}
		cfg.Logf("podcast-worker volume: %d transcripts stored, %d failed, %.0f s of audio, %.0f s of work, in the last %.0f minutes",
			stats.stored, stats.failed, stats.audio, stats.wall.Seconds(), now.Sub(windowFrom).Minutes())
		stats.stored, stats.failed, stats.audio, stats.wall = 0, 0, 0, 0
		windowFrom = now
	}
	for ctx.Err() == nil {
		report(false)
		if cfg.Intake != nil && cfg.Now().Sub(lastIntake) >= cfg.IntakeEvery {
			lastIntake = cfg.Now()
			if err := cfg.Intake(ctx); err != nil && ctx.Err() == nil {
				cfg.Logf("podcast-worker: intake failed: %v", err)
			}
		}
		pending, err := b.PendingEpisodes(ctx, cfg.Now().Add(-cfg.Since), cfg.ListLimit)
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			cfg.Logf("podcast-worker: listing failed: %v", err)
			cfg.Sleep(ctx, idle)
			idle = min(idle*2, cfg.MaxIdle)
			continue
		}
		var next *PendingEpisode
		for i := range pending {
			if f, bad := failed[pending[i].EpisodeID]; bad && cfg.Now().Before(f.until) {
				continue
			}
			next = &pending[i]
			break
		}
		if next == nil {
			cfg.Sleep(ctx, idle)
			idle = min(idle*2, cfg.MaxIdle)
			continue
		}
		idle = cfg.MinIdle
		started := cfg.Now()
		res, err := e.Transcribe(ctx, next.EnclosureURL)
		if err == nil {
			err = b.PutTranscript(ctx, next.EpisodeID, e.Engine(), res)
		}
		mu.Lock()
		stats.wall += cfg.Now().Sub(started)
		if err != nil {
			if ctx.Err() != nil {
				mu.Unlock()
				break // killed mid-episode: it stays pending
			}
			f := failed[next.EpisodeID]
			f.count++
			skip := cfg.FailBackoff << (f.count - 1)
			if skip > cfg.MaxFailSkip || skip <= 0 {
				skip = cfg.MaxFailSkip
			}
			f.until = cfg.Now().Add(skip)
			failed[next.EpisodeID] = f
			stats.failed++
			mu.Unlock()
			cfg.Logf("podcast-worker: episode %s (%s) failed (%d): %v; skipped for %s", next.EpisodeID, next.Show, f.count, err, skip)
			continue
		}
		delete(failed, next.EpisodeID)
		stats.stored++
		stats.audio += res.Seconds
		mu.Unlock()
		cfg.Logf("podcast-worker: episode %s (%s): %d segments, %.0f s of audio in %.0f s", next.EpisodeID, next.Show, len(res.Segments), res.Seconds, cfg.Now().Sub(started).Seconds())
	}
	report(true)
	return ctx.Err()
}
