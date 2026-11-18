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
// run (both, once), candidates (rank the paper's stories against the stored
// transcripts into a file for a person or an agent to judge; no model call),
// put-clips (send the candidates judged support > 0 through the operator
// route), match (rank, apply the rules of internal/podcastmatch and write the
// clips that clear them, no model call; --dry-run prints instead; serve runs
// it too), match-eval (precision and recall against judged candidates files,
// no key needed) and serve (intake and transcribe, for good: it polls for episodes that need
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
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/podcastmatch"
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
		fmt.Fprintln(os.Stderr, "usage: podcast-worker {intake|transcribe|run|serve|candidates|put-clips|match|match-eval|repair} [flags]")
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
	minFreeGB := fs.Float64("min-free-gb", 15, "refuse to start a download when the work directory's filesystem has less than this many GiB free")
	workDir := fs.String("work-dir", "", "scratch directory for downloads (default: the system temp)")
	stories := fs.Int("stories", 60, "candidates: how many of the front's newest stories")
	perStory := fs.Int("per-story", 4, "candidates: passages offered per story, at most one per episode")
	outFile := fs.String("out", "", "candidates: the file to write")
	inFile := fs.String("in", "", "put-clips: the judged candidates file")
	dryRun := fs.Bool("dry-run", false, "match: print the clips it would write and write nothing")
	matchState := fs.String("match-state", "", "match, serve: the file that remembers what was matched and written (default: beside the work directory)")
	judged := fs.String("judged", "", "match-eval: comma separated judged candidates files")
	minScore := fs.Float64("min-score", podcastmatch.DefaultParams.MinScore, "match: the clip's BM25 as a share of a perfect passage's")
	minMargin := fs.Float64("min-margin", podcastmatch.DefaultParams.MinMargin, "match: how far the story's score must exceed the next story's, as a fraction")
	minShare := fs.Float64("min-entity-share", podcastmatch.DefaultParams.MinEntityShare, "match: the share of the story's distinctive terms the clip must carry")
	trackDir := fs.String("track-dir", "", "transcribe, match, serve: where each episode's fingerprint track is kept (default: beside the match state)")
	intakeEvery := fs.Duration("intake-every", 30*time.Minute, "serve: how often the feeds are polled")
	if err := fs.Parse(os.Args[2:]); err != nil {
		os.Exit(2)
	}
	if cmd != "intake" && cmd != "transcribe" && cmd != "run" && cmd != "serve" && cmd != "candidates" && cmd != "put-clips" && cmd != "match" && cmd != "match-eval" && cmd != "repair" {
		log.Fatalf("unknown command %q", cmd)
	}
	if *matchState == "" {
		*matchState = podcastworker.DefaultMatchStatePath(*workDir)
	}
	if *trackDir == "" {
		*trackDir = podcastworker.DefaultTrackDir(*workDir)
	}
	tracks := &podcastworker.TrackStore{Dir: *trackDir}
	params := podcastmatch.DefaultParams
	params.MinScore, params.MinMargin, params.MinEntityShare = *minScore, *minMargin, *minShare
	if cmd == "match-eval" {
		if *judged == "" {
			log.Fatal("match-eval needs --judged")
		}
		rows, err := podcastworker.EvalJudged(context.Background(), strings.Split(*judged, ","),
			map[string]podcastmatch.Params{"strict": podcastmatch.StrictParams, "default": podcastmatch.DefaultParams, "loose": podcastmatch.LooseParams},
			[]string{"strict", "default", "loose"})
		if err != nil {
			log.Fatalf("match-eval: %v", err)
		}
		fmt.Print(podcastworker.FormatEval(rows))
		return
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
		t := &podcastworker.Transcriber{FFmpeg: *ffmpeg, Parakeet: *parakeet, Model: *model, VAD: *vad, Threads: *threads, WorkDir: *workDir, MaxMinutes: *maxMinutes, MinFreeBytes: int64(*minFreeGB * (1 << 30))}
		matcher := &podcastworker.Matcher{B: client, Params: params, StatePath: *matchState, Logf: log.Printf, Tracks: tracks}
		repairer := &podcastworker.Repairer{B: client, Eng: t, Logf: log.Printf}
		cfg := podcastworker.ServeConfig{
			Match:  func(ctx context.Context) error { _, err := matcher.Run(ctx); return err },
			Repair: func(ctx context.Context) error { _, err := repairer.Run(ctx); return err },
			Stored: func(ep podcastworker.PendingEpisode, res podcastworker.Result) { keepTrack(tracks, ep.EpisodeID, res) },
			Since:  time.Duration(*sinceHours) * time.Hour, ListLimit: *maxEpisodes, IntakeEvery: *intakeEvery,
			Intake: func(ctx context.Context) error {
				// Free space is logged at each intake, so a filling disk is seen.
				log.Print(podcastworker.DescribeFreeSpace(*workDir))
				return intake(ctx, client, *feeds, *perFeed)
			},
		}
		if n, err := t.SweepWorkDir(); err != nil || n > 0 {
			log.Printf("work directory swept: %d leftover episode directories removed (%v)", n, err)
		}
		log.Printf("podcast-worker serving: engine %q, %d threads, one episode at a time", t.Engine(), *threads)
		if err := podcastworker.Serve(ctx, client, t, cfg); err != nil && ctx.Err() == nil {
			log.Fatalf("serve: %v", err)
		}
		log.Printf("podcast-worker stopped")
		return
	}
	if cmd == "match" {
		ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		m := &podcastworker.Matcher{B: client, Params: params, DryRun: *dryRun, StatePath: *matchState, Out: os.Stdout, Logf: log.Printf, Tracks: tracks}
		if _, err := m.Run(ctx); err != nil {
			log.Fatalf("match: %v", err)
		}
		return
	}
	if cmd == "repair" {
		if *model == "" || *vad == "" {
			log.Fatal("repair needs --model and --vad-model")
		}
		ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer stop()
		t := &podcastworker.Transcriber{FFmpeg: *ffmpeg, Parakeet: *parakeet, Model: *model, VAD: *vad, Threads: *threads, WorkDir: *workDir, MinFreeBytes: int64(*minFreeGB * (1 << 30))}
		if _, err := (&podcastworker.Repairer{B: client, Eng: t, DryRun: *dryRun, Logf: log.Printf}).Run(ctx); err != nil {
			log.Fatalf("repair: %v", err)
		}
		return
	}
	if cmd == "candidates" {
		if *outFile == "" {
			log.Fatal("candidates needs --out")
		}
		f, err := podcastworker.BuildCandidates(ctx, client, *stories, *perStory, log.Printf)
		if err != nil {
			log.Fatalf("candidates: %v", err)
		}
		if err := podcastworker.WriteCandidates(*outFile, f); err != nil {
			log.Fatalf("candidates: %v", err)
		}
		log.Printf("candidates: %d passages for the front's newest %d stories written to %s; set \"support\" (0 to 1) on the ones that report the same event, then run put-clips", len(f.Candidates), *stories, *outFile)
		return
	}
	if cmd == "put-clips" {
		if *inFile == "" {
			log.Fatal("put-clips needs --in")
		}
		clips, err := podcastworker.JudgedClips(*inFile)
		if err != nil {
			log.Fatalf("put-clips: %v", err)
		}
		if len(clips) == 0 {
			log.Fatalf("put-clips: no candidate in %s has a support above 0", *inFile)
		}
		stored, refused := client.PutClips(ctx, clips)
		for _, r := range refused {
			log.Printf("refused: %q", r)
		}
		log.Printf("put-clips: %d of %d clips stored, %d refused", stored, len(clips), len(refused))
		if len(refused) > 0 {
			os.Exit(1)
		}
		return
	}
	if cmd == "intake" || cmd == "run" {
		if err := intake(ctx, client, *feeds, *perFeed); err != nil {
			log.Fatalf("intake: %v", err)
		}
		log.Print(podcastworker.DescribeFreeSpace(*workDir))
	}
	if cmd == "transcribe" || cmd == "run" {
		if *model == "" || *vad == "" {
			log.Fatal("transcribe needs --model and --vad-model")
		}
		t := &podcastworker.Transcriber{FFmpeg: *ffmpeg, Parakeet: *parakeet, Model: *model, VAD: *vad, Threads: *threads, WorkDir: *workDir, MaxMinutes: *maxMinutes, MinFreeBytes: int64(*minFreeGB * (1 << 30))}
		if err := transcribe(ctx, client, t, tracks, time.Duration(*sinceHours)*time.Hour, *maxEpisodes); err != nil {
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
	// The operator's list is the public list: say so first, so its clips may be shown.
	if err := c.ListFeeds(ctx, seed); err != nil {
		log.Printf("the operator's feed list could not be published (it is still polled): %v", err)
	}
	// The feeds polled are the operator's list, and only it.
	httpc := podcastworker.SafeClient(60 * time.Second)
	var all []podcastworker.Episode
	dropped := 0
	for _, f := range seed {
		eps, err := podcastworker.FetchFeed(ctx, httpc, f.URL, f.Show)
		if err != nil {
			log.Printf("feed %q skipped: %v", f.URL, err)
			continue
		}
		if len(eps) > perFeed {
			eps = eps[:perFeed]
		}
		// Items the server would refuse are not posted, so they cannot cost the
		// others their request.
		good, bad := podcastworker.ValidEpisodes(eps, time.Now())
		dropped += bad
		all = append(all, good...)
	}
	if len(all) == 0 {
		return fmt.Errorf("no feed gave a usable item (%d dropped)", dropped)
	}
	recorded, fresh, skipped, err := c.PostEpisodes(ctx, all)
	log.Printf("intake: %d feeds, episodes recorded: %d (new %d), %d dropped here, %d skipped by the server", len(seed), recorded, fresh, dropped, skipped)
	return err
}

// keepTrack stores an episode's fingerprint track for the matcher, and drops
// tracks older than the clips' story window by a margin.
func keepTrack(tracks *podcastworker.TrackStore, episodeID string, res podcastworker.Result) {
	if err := tracks.Save(episodeID, res); err != nil {
		log.Printf("episode %s: fingerprint track not kept (its clips will be written without a fingerprint): %v", episodeID, err)
	}
	tracks.Prune(10 * 24 * time.Hour)
}

func transcribe(ctx context.Context, c *podcastworker.Client, t *podcastworker.Transcriber, tracks *podcastworker.TrackStore, since time.Duration, max int) error {
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
		res, rep, err := podcastworker.PrepareResult(res)
		if rep.Any() {
			log.Printf("episode %s (%s): segments repaired before storing: %s", ep.EpisodeID, ep.Show, rep)
		}
		if err == nil {
			err = c.PutTranscript(ctx, ep.EpisodeID, t.Engine(), res)
		}
		if err != nil {
			log.Printf("episode %s (%s): not stored: %v", ep.EpisodeID, ep.Show, err)
			continue
		}
		keepTrack(tracks, ep.EpisodeID, res)
		done++
		log.Printf("episode %s (%s): %d segments, %.0f s of audio in %.0f s", ep.EpisodeID, ep.Show, len(res.Segments), res.Seconds, time.Since(started).Seconds())
	}
	log.Printf("transcripts stored: %d of %d", done, len(pending))
	return nil
}
