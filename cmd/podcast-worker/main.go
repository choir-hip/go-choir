// Command podcast-worker is the operator's podcast job: it polls the feed
// list, posts the episodes to the platform, transcribes the ones that need a
// transcript with parakeet.cpp and posts the timed transcripts back
// (docs/design/podcast-clips-2026-11-15.md section 3).
//
//	podcast-worker run --server https://choir.news --api-key-file KEYFILE \
//	    --ffmpeg /path/ffmpeg --parakeet /path/parakeet-cli \
//	    --model /path/tdt_ctc-110m-q8_0.gguf --vad-model /path/silero-vad-f16.gguf
//
// Commands: intake (feeds to episodes), transcribe (episodes to transcripts),
// run (both, once) and serve (both, for good: it polls for episodes that need
// a transcript, transcribes one at a time at the lowest CPU and I/O priority
// with at most four threads, and sleeps with a backoff when idle; SIGTERM
// stops it and an unfinished episode stays pending). The API key is read from
// the file and never printed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastworker"
)

type feedList struct {
	Feeds []struct {
		Show string `json:"show"`
		URL  string `json:"url"`
	} `json:"feeds"`
}

func main() {
	if len(os.Args) < 2 || strings.HasPrefix(os.Args[1], "-") {
		fmt.Fprintln(os.Stderr, "usage: podcast-worker {intake|transcribe|run|serve} [flags]")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	server := fs.String("server", "https://choir.news", "the platform's site")
	keyFile := fs.String("api-key-file", "", "file holding an operator API key (never printed)")
	feeds := fs.String("feeds", "config/podcast-feeds.json", "the feed list")
	perFeed := fs.Int("per-feed", 3, "newest items of each feed to record")
	sinceHours := fs.Int("since-hours", 72, "transcribe episodes published within this many hours")
	maxEpisodes := fs.Int("max-episodes", 40, "most episodes to transcribe in one run")
	maxMinutes := fs.Int("max-minutes", 90, "transcribe at most this many minutes from the start of an episode")
	ffmpeg := fs.String("ffmpeg", "ffmpeg", "ffmpeg binary")
	parakeet := fs.String("parakeet", "parakeet-cli", "parakeet.cpp CLI binary")
	model := fs.String("model", "", "parakeet GGUF model (the 110M q8_0)")
	vad := fs.String("vad-model", "", "silero VAD GGUF")
	threads := fs.Int("threads", 4, "model threads")
	workDir := fs.String("work-dir", "", "scratch directory for downloads (default: the system temp)")
	intakeEvery := fs.Duration("intake-every", 30*time.Minute, "serve: how often the feeds are polled")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if cmd != "intake" && cmd != "transcribe" && cmd != "run" && cmd != "serve" {
		log.Fatalf("unknown command %q", cmd)
	}
	raw, err := os.ReadFile(*keyFile)
	if err != nil {
		log.Fatalf("read the API key file: %v", err)
	}
	client, err := podcastworker.NewClient(*server, string(raw))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	if *threads > podcastworker.MaxThreads {
		log.Printf("threads lowered from %d to %d: more gives nothing on this model", *threads, podcastworker.MaxThreads)
		*threads = podcastworker.MaxThreads
	}
	if cmd == "serve" {
		for _, problem := range podcastworker.BeAPoliteNeighbour() {
			log.Printf("politeness: %v", problem)
		}
		if *model == "" || *vad == "" {
			log.Fatal("serve needs --model and --vad-model")
		}
		var stop context.CancelFunc
		ctx, stop = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		t := &podcastworker.Transcriber{FFmpeg: *ffmpeg, Parakeet: *parakeet, Model: *model, VAD: *vad, Threads: *threads, WorkDir: *workDir, MaxMinutes: *maxMinutes}
		cfg := podcastworker.ServeConfig{
			Since: time.Duration(*sinceHours) * time.Hour, ListLimit: *maxEpisodes, IntakeEvery: *intakeEvery,
			Intake: func(ctx context.Context) error { return intake(ctx, client, *feeds, *perFeed) },
		}
		handled := map[string]bool{}
		cfg.Priority = func(ctx context.Context) error { return priority(ctx, client, handled) }
		log.Printf("podcast-worker serving: engine %q, %d threads, one episode at a time", t.Engine(), *threads)
		if err := podcastworker.Serve(ctx, client, t, cfg); err != nil && ctx.Err() == nil {
			log.Fatalf("serve: %v", err)
		}
		log.Printf("podcast-worker stopped")
		return
	}
	if cmd == "intake" || cmd == "run" {
		if err := intake(ctx, client, *feeds, *perFeed); err != nil {
			log.Fatalf("intake: %v", err)
		}
	}
	if cmd == "transcribe" || cmd == "run" {
		if *model == "" || *vad == "" {
			log.Fatal("transcribe needs --model and --vad-model")
		}
		t := &podcastworker.Transcriber{FFmpeg: *ffmpeg, Parakeet: *parakeet, Model: *model, VAD: *vad, Threads: *threads, WorkDir: *workDir, MaxMinutes: *maxMinutes}
		if err := transcribe(ctx, client, t, time.Duration(*sinceHours)*time.Hour, *maxEpisodes); err != nil {
			log.Fatalf("transcribe: %v", err)
		}
	}
}

func intake(ctx context.Context, c *podcastworker.Client, path string, perFeed int) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var list feedList
	if err := json.Unmarshal(raw, &list); err != nil {
		return fmt.Errorf("feed list %s: %w", path, err)
	}
	var seed []podcastworker.FeedSpec
	for _, f := range list.Feeds {
		seed = append(seed, podcastworker.FeedSpec{Show: f.Show, URL: f.URL})
	}
	// The feeds to poll are the operator's list and the feeds readers report.
	reported, err := c.WorkerFeeds(ctx)
	if err != nil {
		log.Printf("reported feeds unavailable (the operator's list alone is polled): %v", err)
	}
	feeds := podcastworker.MergeFeeds(seed, reported)
	httpc := &http.Client{Timeout: 60 * time.Second}
	var all []podcastworker.Episode
	for _, f := range feeds {
		eps, err := podcastworker.FetchFeed(ctx, httpc, f.URL, f.Show)
		if err != nil {
			log.Printf("feed %q skipped: %v", f.URL, err)
			continue
		}
		if len(eps) > perFeed {
			eps = eps[:perFeed]
		}
		all = append(all, eps...)
	}
	if len(all) == 0 {
		return fmt.Errorf("no feed gave an item")
	}
	recorded, fresh, err := c.PostEpisodes(ctx, all)
	if err != nil {
		return err
	}
	log.Printf("intake: %d feeds, episodes recorded: %d (new %d)", len(feeds), recorded, fresh)
	return nil
}

// priority polls the feeds of episodes a phone is playing, once for each, so
// they are in the queue at the next listing.
func priority(ctx context.Context, c *podcastworker.Client, handled map[string]bool) error {
	feeds, err := c.WorkerFeeds(ctx)
	if err != nil {
		return err
	}
	httpc := &http.Client{Timeout: 60 * time.Second}
	for _, f := range feeds {
		for _, w := range f.Wanted {
			key := f.FeedURL + "\x00" + w.GUID
			if handled[key] {
				continue
			}
			handled[key] = true
			eps, err := podcastworker.FetchFeed(ctx, httpc, f.FeedURL, f.Title)
			if err != nil {
				return err
			}
			if len(eps) > 5 {
				// Keep the newest few and the one wanted.
				keep := eps[:5]
				for _, e := range eps[5:] {
					if e.GUID == w.GUID {
						keep = append(keep, e)
					}
				}
				eps = keep
			}
			if _, _, err := c.PostEpisodes(ctx, eps); err != nil {
				return err
			}
			log.Printf("priority: polled %s for a playing episode", f.FeedURL)
		}
	}
	return nil
}

func transcribe(ctx context.Context, c *podcastworker.Client, t *podcastworker.Transcriber, since time.Duration, max int) error {
	pending, err := c.PendingEpisodes(ctx, time.Now().Add(-since), max)
	if err != nil {
		return err
	}
	log.Printf("episodes needing a transcript: %d", len(pending))
	done := 0
	for _, ep := range pending {
		started := time.Now()
		res, err := t.Transcribe(ctx, ep.EnclosureURL)
		if err != nil {
			log.Printf("episode %s (%s): skipped: %v", ep.EpisodeID, ep.Show, err)
			continue
		}
		if err := c.PutTranscript(ctx, ep.EpisodeID, t.Engine(), res); err != nil {
			log.Printf("episode %s (%s): not stored: %v", ep.EpisodeID, ep.Show, err)
			continue
		}
		done++
		log.Printf("episode %s (%s): %d segments, %.0f s of audio in %.0f s", ep.EpisodeID, ep.Show, len(res.Segments), res.Seconds, time.Since(started).Seconds())
	}
	log.Printf("transcripts stored: %d of %d", done, len(pending))
	return nil
}
