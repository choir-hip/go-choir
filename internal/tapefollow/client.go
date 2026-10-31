package tapefollow

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/bundle"
)

// Client pulls bundles of the owner's computer from a Choir server:
//
//	GET <backend>/api/computers/<id>/tape/bundle?branch=main[&have_seq=&have_head=][&max_events=]
//
// It is the follower's only network call, and it is a GET: the follower
// never sends anything that could change the computer.
type Client struct {
	// Backend is the server's base URL (https://choir.news).
	Backend string
	// HTTP carries the credential: a cookie jar for the app's session, or
	// Authorize for an API key. It should have no whole-request timeout
	// (a bootstrap streams for minutes); StallTimeout bounds a dead stream.
	HTTP *http.Client
	// Authorize, when set, adds the credential to each request.
	Authorize func(*http.Request)
	// StallTimeout cancels a pull that receives nothing for this long
	// (default one minute): a lost network ends the pull, which the syncer
	// retries from its committed head.
	StallTimeout time.Duration
}

// HTTPError is a refused pull.
type HTTPError struct {
	Status     int
	Message    string
	RetryAfter time.Duration
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("tapefollow: server answered %d: %s", e.Status, e.Message)
}

// IsForked reports a 409: the head the follower holds is not on the
// computer's tape (a fork, or a store of another history). Retrying cannot
// fix it; the owner decides.
func IsForked(err error) bool {
	var h *HTTPError
	return errors.As(err, &h) && h.Status == http.StatusConflict
}

// IsUnauthorized reports a missing, expired or insufficient credential.
func IsUnauthorized(err error) bool {
	var h *HTTPError
	return errors.As(err, &h) && (h.Status == http.StatusUnauthorized || h.Status == http.StatusForbidden)
}

// Pulled says what one pull fetched.
type Pulled struct {
	// UpToDate: the server's head is the one the follower holds (204).
	UpToDate      bool
	SourceHeadSeq int64
	BundleHeadSeq int64
}

// Pull fetches the next bundle into the empty directory dest: a bootstrap
// when have is nil, a delta after have's head otherwise. At most maxBytes are
// accepted. It does not verify or apply (Store.Apply does).
func (c *Client) Pull(ctx context.Context, computerID string, have *State, maxEvents, maxBytes int64, dest string) (Pulled, error) {
	base, err := url.Parse(strings.TrimRight(c.Backend, "/"))
	if err != nil || !base.IsAbs() {
		return Pulled{}, fmt.Errorf("tapefollow: invalid backend %q", c.Backend)
	}
	u := base.JoinPath("api", "computers", computerID, "tape", "bundle")
	q := url.Values{"branch": {Branch}}
	if have != nil {
		q.Set("have_seq", strconv.FormatInt(have.HeadSeq, 10))
		q.Set("have_head", have.HeadDigest)
	}
	if maxEvents > 0 {
		q.Set("max_events", strconv.FormatInt(maxEvents, 10))
	}
	u.RawQuery = q.Encode()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return Pulled{}, err
	}
	req.Header.Set("Accept", "application/x-tar")
	if c.Authorize != nil {
		c.Authorize(req)
	}
	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	stall := c.StallTimeout
	if stall <= 0 {
		stall = time.Minute
	}
	// The server exports the page before its first byte: wait longer for
	// the headers than between body reads.
	watch := newStallWatch(5*stall, cancel)
	defer watch.stop()
	resp, err := client.Do(req)
	if err != nil {
		return Pulled{}, fmt.Errorf("tapefollow: pull: %w", err)
	}
	defer resp.Body.Close()
	watch.window(stall)
	out := Pulled{}
	out.SourceHeadSeq, _ = strconv.ParseInt(resp.Header.Get("X-Choir-Source-Head-Seq"), 10, 64)
	switch resp.StatusCode {
	case http.StatusNoContent:
		out.UpToDate = true
		return out, nil
	case http.StatusOK:
	default:
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		e := &HTTPError{Status: resp.StatusCode, Message: strings.TrimSpace(string(msg))}
		if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
			e.RetryAfter = time.Duration(s) * time.Second
		}
		return out, e
	}
	out.BundleHeadSeq, _ = strconv.ParseInt(resp.Header.Get("X-Choir-Bundle-Head-Seq"), 10, 64)
	if err := bundle.ReadTar(watch.reader(resp.Body), dest, maxBytes); err != nil {
		if watch.fired() {
			return out, fmt.Errorf("tapefollow: pull stalled for %s: %w", stall, err)
		}
		return out, fmt.Errorf("tapefollow: pull: %w", err)
	}
	return out, nil
}

// stallWatch cancels the pull when no byte arrives for d.
type stallWatch struct {
	mu     sync.Mutex
	timer  *time.Timer
	d      time.Duration
	isDone bool
	hit    bool
}

func newStallWatch(d time.Duration, cancel func()) *stallWatch {
	w := &stallWatch{d: d}
	w.timer = time.AfterFunc(d, func() {
		w.mu.Lock()
		w.hit = true
		w.mu.Unlock()
		cancel()
	})
	return w
}

func (w *stallWatch) reader(r io.Reader) io.Reader { return stallReader{r: r, w: w} }

func (w *stallWatch) window(d time.Duration) {
	w.mu.Lock()
	w.d = d
	w.mu.Unlock()
	w.touch()
}

func (w *stallWatch) touch() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.isDone {
		w.timer.Reset(w.d)
	}
}

func (w *stallWatch) fired() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.hit
}

func (w *stallWatch) stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.isDone = true
	w.timer.Stop()
}

type stallReader struct {
	r io.Reader
	w *stallWatch
}

func (s stallReader) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if n > 0 {
		s.w.touch()
	}
	return n, err
}
