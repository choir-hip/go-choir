package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/tapefollow"
)

// runFollow is the follower (internal/tapefollow) as a command: the same
// syncer the Mac app runs, with an API key instead of the app's session.
//
//	choir-bundle follow --computer <id> --header-file <file> [--backend https://choir.news]
//	    [--store <dir>] [--once] [--max-bundles N] [--max-events N] [--interval 30s]
//	choir-bundle follow --computer <id> --verify [--store <dir>]
//
// Every pulled bundle is authenticated against the trust set (the pinned
// platform key unless --no-pinned-trust, plus each --trust file).
// --header-file holds request headers, one "Name: value" per line (pass the
// key without printing it: --header-file <(printf 'Authorization: Bearer %s\n' "$KEY")).
// --once pulls until the local copy is at the server's head, then exits;
// --max-bundles stops after that many applied bundles. --verify re-verifies
// the local copy as one tape and exits. Each applied bundle prints one JSON
// line, and the command ends with the status line.
func runFollow(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir-bundle follow", flag.ContinueOnError)
	fs.SetOutput(stderr)
	backend := fs.String("backend", "https://choir.news", "Choir server")
	computer := fs.String("computer", "", "computer id")
	storeDir := fs.String("store", "", "local store directory (default: the app's, under Application Support/Choir/<computer-id>)")
	headerFile := fs.String("header-file", "", "file of request headers (Name: value per line)")
	once := fs.Bool("once", false, "pull until at the server's head, then exit")
	maxBundles := fs.Int("max-bundles", 0, "stop after this many applied bundles (0: no bound)")
	maxEvents := fs.Int64("max-events", tapefollow.DefaultMaxEvents, "events per pulled bundle")
	interval := fs.Duration("interval", tapefollow.DefaultInterval, "poll interval once caught up")
	verify := fs.Bool("verify", false, "verify the local copy and exit")
	var tf trustFlags
	tf.register(fs)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *computer == "" {
		fmt.Fprintln(stderr, "choir-bundle follow: --computer is required")
		return 2
	}
	dir := *storeDir
	if dir == "" {
		var err error
		if dir, err = tapefollow.DefaultStoreDir(*computer); err != nil {
			return fail(stderr, err)
		}
	}
	trust, err := tf.load()
	if err != nil {
		return fail(stderr, err)
	}
	store, err := tapefollow.Open(dir, *computer, trust)
	if err != nil {
		return fail(stderr, err)
	}
	defer store.Close()
	if *verify {
		st, err := store.VerifyAll()
		if err != nil {
			return fail(stderr, err)
		}
		emit(stdout, map[string]any{"verified": true, "store": dir, "computer_id": st.ComputerID, "start_seq": st.StartSeq,
			"head_seq": st.HeadSeq, "head_digest": st.HeadDigest, "bundles": len(st.Bundles), "bytes": st.Bytes, "snapshot_root": st.SnapshotRoot})
		return 0
	}
	headers, err := readHeaderFile(*headerFile)
	if err != nil {
		return fail(stderr, err)
	}
	client := &tapefollow.Client{
		Backend: *backend,
		HTTP:    &http.Client{},
		Authorize: func(r *http.Request) {
			for k, v := range headers {
				r.Header.Set(k, v)
			}
		},
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	applied := 0
	syncer := &tapefollow.Syncer{Store: store, Client: client, MaxEvents: *maxEvents, Interval: *interval}
	last := int64(-1)
	if st, ok := store.State(); ok {
		last = st.HeadSeq
	}
	syncer.OnStatus = func(st tapefollow.Status) {
		if st.HeadSeq != last {
			last = st.HeadSeq
			applied++
			emit(stdout, map[string]any{"applied": true, "head_seq": st.HeadSeq, "head_digest": st.HeadDigest, "source_head_seq": st.SourceHeadSeq, "bytes": st.Bytes})
		}
	}
	if *once || *maxBundles > 0 {
		failures := 0
		for {
			more, err := syncer.Step(ctx)
			if err != nil {
				// A rate limit or a lost network is retried from the
				// committed head; a fork, a refused bundle or a refused
				// credential is not.
				var h *tapefollow.HTTPError
				transient := ctx.Err() == nil && !tapefollow.IsForked(err) && !tapefollow.IsUnauthorized(err) &&
					!errors.Is(err, tapefollow.ErrNotChained) && !errors.Is(err, tapefollow.ErrStoreFull) &&
					!strings.Contains(err.Error(), "bundle refused") && (!errors.As(err, &h) || h.Status == http.StatusTooManyRequests || h.Status >= 500)
				if failures++; !transient || failures > 8 {
					emit(stdout, map[string]any{"status": syncer.Status().Line(), "error": err.Error()})
					return 1
				}
				wait := time.Duration(failures) * 2 * time.Second
				if h != nil && h.RetryAfter > wait {
					wait = h.RetryAfter
				}
				fmt.Fprintf(stderr, "choir-bundle follow: retrying in %s: %v\n", wait, err)
				select {
				case <-ctx.Done():
					return 1
				case <-time.After(wait):
				}
				continue
			}
			failures = 0
			if (!more && *once) || (*maxBundles > 0 && applied >= *maxBundles) || (!more && *maxBundles > 0) {
				break
			}
		}
		emit(stdout, map[string]any{"status": syncer.Status().Line(), "store": dir, "head_seq": syncer.Status().HeadSeq, "source_head_seq": syncer.Status().SourceHeadSeq})
		return 0
	}
	err = syncer.Run(ctx)
	emit(stdout, map[string]any{"status": syncer.Status().Line(), "store": dir})
	if err != nil && !errors.Is(err, context.Canceled) {
		return fail(stderr, err)
	}
	return 0
}

// readHeaderFile reads "Name: value" lines. The values are credentials and
// are never printed.
func readHeaderFile(path string) (map[string]string, error) {
	out := map[string]string{}
	if path == "" {
		return out, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("header file: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(io.LimitReader(f, 64<<10))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("header file: a line is not Name: value")
		}
		out[http.CanonicalHeaderKey(strings.TrimSpace(name))] = strings.TrimSpace(value)
	}
	return out, scanner.Err()
}
