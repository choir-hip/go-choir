package platform

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/bundle"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// The tape follow route serves a follower (the owner's Mac) the next bundle
// of a computer's tape (internal/bundle follow.go):
//
//	GET /internal/computers/tape/bundle?computer_id=<id>&branch=main
//	    [&have_seq=<n>&have_head=<sha256>] [&max_events=<n>]
//
// Without have_seq it is a bootstrap: the newest snapshot root, its chunks
// and the tape from its head (the tape from genesis when there is no root),
// at most max_events events. With have_seq/have_head (the head the follower
// holds) it is a delta of at most max_events events after it. 204 says the
// follower is at the head; 409 says have_head is not on this tape (a fork or
// another computer); 410 a tombstoned tape. A 200 body is the bundle as a tar
// (bundle.WriteTar), manifest last; the X-Choir-* headers name the range and
// the source head, so the follower knows whether to pull again at once.
//
// Only the proxy reaches it (the host-internal credential), after joining the
// caller to the computer's owner in vmctl (proxy tape_follow.go). It reads
// only PostgreSQL rows and sealed host files, never the guest, so a poll does
// not wake a hibernated computer. Private payloads are pinned as encrypted
// envelopes, so the follower receives ciphertext; their keys are not served
// here (later: the computer key in the Mac's Keychain).
const tapeFollowRoute = "/internal/computers/tape/bundle"

const (
	tapeFollowDefaultEvents = 2000
	tapeFollowMaxEvents     = 20000
	// tapeFollowGlobal bounds concurrent exports on the host: each holds a
	// temp directory and a database connection while it streams.
	tapeFollowGlobal = 2
	// A computer gets one export at a time and a bucket of tapeFollowBurst
	// requests refilled at one per tapeFollowRefill: a bootstrap pulls a
	// page a second, a steady follower polls every half minute.
	tapeFollowBurst  = 10
	tapeFollowRefill = time.Second
	// tapeFollowStaleTemp is when an abandoned temp export (a crash
	// mid-stream) is removed by the next request.
	tapeFollowStaleTemp = time.Hour
	// tapeFollowWriteSpan is the whole request's write bound (export and
	// stream), the public edge's own bound (Caddy's 15 minutes).
	tapeFollowWriteSpan = 15 * time.Minute
)

type tapeFollowLimiter struct {
	mu       sync.Mutex
	global   int
	inFlight map[string]bool
	tokens   map[string]float64
	last     map[string]time.Time
	now      func() time.Time
}

func newTapeFollowLimiter() *tapeFollowLimiter {
	return &tapeFollowLimiter{inFlight: map[string]bool{}, tokens: map[string]float64{}, last: map[string]time.Time{}, now: time.Now}
}

// admit takes a token for computerID and, when export is set, its export
// slot. It returns the release func, or false and the wait to advertise.
func (l *tapeFollowLimiter) admit(computerID string, export bool) (func(), bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	tokens, seen := l.tokens[computerID]
	if !seen {
		tokens = tapeFollowBurst
	} else {
		tokens += float64(now.Sub(l.last[computerID])) / float64(tapeFollowRefill)
		if tokens > tapeFollowBurst {
			tokens = tapeFollowBurst
		}
	}
	l.last[computerID] = now
	if tokens < 1 {
		l.tokens[computerID] = tokens
		return nil, false, time.Duration((1 - tokens) * float64(tapeFollowRefill))
	}
	if export && (l.inFlight[computerID] || l.global >= tapeFollowGlobal) {
		l.tokens[computerID] = tokens
		return nil, false, 2 * time.Second
	}
	l.tokens[computerID] = tokens - 1
	if !export {
		return func() {}, true, 0
	}
	l.inFlight[computerID] = true
	l.global++
	return func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		delete(l.inFlight, computerID)
		l.global--
	}, true, 0
}

var tapeFollowLimits = newTapeFollowLimiter()

func (h *Handler) HandleTapeFollowBundle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if !trustedInternalCaller(r) || strings.TrimSpace(r.Header.Get("X-Authenticated-User")) == "" {
		writeJSON(w, http.StatusForbidden, apiError{Error: "internal caller with an authenticated owner required"})
		return
	}
	if h == nil || h.service == nil || h.service.store == nil || h.eventCAS == nil || h.service.artifactsRoot == "" {
		writeJSON(w, http.StatusServiceUnavailable, apiError{Error: "tape follow unavailable"})
		return
	}
	q := r.URL.Query()
	computerID := strings.TrimSpace(q.Get("computer_id"))
	if computerID == "" || computerID != q.Get("computer_id") || strings.ContainsAny(computerID, `/\`) || computerID == "." || computerID == ".." {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "computer_id is required"})
		return
	}
	if branch := q.Get("branch"); branch != "" && branch != "main" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "only the main branch is served"})
		return
	}
	maxEvents := int64(tapeFollowDefaultEvents)
	if raw := q.Get("max_events"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n <= 0 || n > tapeFollowMaxEvents {
			writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("max_events must be within 1..%d", tapeFollowMaxEvents)})
			return
		}
		maxEvents = n
	}
	haveSeq, haveHead, bootstrap := int64(-1), q.Get("have_head"), q.Get("have_seq") == ""
	if !bootstrap {
		n, err := strconv.ParseInt(q.Get("have_seq"), 10, 64)
		if err != nil || n < 0 || !computerevent.IsSHA256(haveHead) {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "have_seq and have_head (sha256) name the head the follower holds"})
			return
		}
		haveSeq = n
	} else if haveHead != "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "have_head needs have_seq"})
		return
	}

	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(tapeFollowWriteSpan))
	ctx := r.Context()
	head, err := h.eventCAS.Head(ctx, computerID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "event head unavailable"})
		return
	}
	if head == nil {
		writeJSON(w, http.StatusNotFound, apiError{Error: "computer event head not initialized"})
		return
	}
	w.Header().Set("X-Choir-Source-Head-Seq", strconv.FormatUint(head.Sequence, 10))
	// The up-to-date answer is a head read, not an export.
	if !bootstrap && uint64(haveSeq) >= head.Sequence {
		if uint64(haveSeq) == head.Sequence && haveHead == head.CanonicalEventHead {
			release, ok, wait := tapeFollowLimits.admit(computerID, false)
			if !ok {
				w.Header().Set("Retry-After", strconv.Itoa(int(wait/time.Second)+1))
				writeJSON(w, http.StatusTooManyRequests, apiError{Error: "tape follow is rate limited; retry later"})
				return
			}
			release()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusConflict, apiError{Error: "the head the follower holds is not on this tape"})
		return
	}
	release, ok, wait := tapeFollowLimits.admit(computerID, true)
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(wait/time.Second)+1))
		writeJSON(w, http.StatusTooManyRequests, apiError{Error: "tape follow is rate limited; retry later"})
		return
	}
	defer release()

	tmpRoot := filepath.Join(h.service.artifactsRoot, "bundle-follow-tmp")
	sweepTapeFollowTemp(tmpRoot)
	if err := os.MkdirAll(tmpRoot, 0o750); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "tape follow temp unavailable"})
		return
	}
	out, err := os.MkdirTemp(tmpRoot, "export-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "tape follow temp unavailable"})
		return
	}
	defer os.RemoveAll(out)

	started := time.Now()
	opts := bundle.ExportOptions{
		ComputerID: computerID, Out: out, FromSeq: -1, MaxEvents: maxEvents, ChunksByReference: true,
		OwnerID:  strings.TrimSpace(r.Header.Get("X-Authenticated-User")),
		Producer: bundle.Producer{Host: hostnameOr("corpusd"), Tool: "corpusd tape follow"},
	}
	var m bundle.Manifest
	switch {
	case bootstrap:
		m, err = bundle.Export(ctx, h.service.store.db, h.service.artifactsRoot, opts)
		if err != nil && strings.Contains(err.Error(), "has no snapshot root") {
			opts.FromSeq = 0
			m, err = bundle.Export(ctx, h.service.store.db, h.service.artifactsRoot, opts)
		}
	default:
		m, err = bundle.ExportSince(ctx, h.service.store.db, h.service.artifactsRoot, haveSeq, haveHead, opts)
	}
	if err != nil {
		switch {
		case errors.Is(err, bundle.ErrHistoryMismatch):
			writeJSON(w, http.StatusConflict, apiError{Error: "the head the follower holds is not on this tape"})
		case strings.Contains(err.Error(), "is tombstoned"):
			writeJSON(w, http.StatusGone, apiError{Error: "the computer's tape is tombstoned"})
		case errors.Is(err, context.Canceled):
		default:
			log.Printf("tape follow: export %s: %v", computerID, err)
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "bundle export failed"})
		}
		return
	}
	exported := time.Since(started)
	hdr := w.Header()
	hdr.Set("Content-Type", "application/x-tar")
	hdr.Set("Cache-Control", "no-store")
	hdr.Set("X-Choir-Bundle-Start-Seq", strconv.FormatInt(m.Tape.StartSeq, 10))
	hdr.Set("X-Choir-Bundle-Head-Seq", strconv.FormatInt(m.Tape.HeadSeq, 10))
	hdr.Set("X-Choir-Bundle-Head-Digest", m.Tape.HeadDigest)
	w.WriteHeader(http.StatusOK)
	if err := bundle.WriteTar(w, out, h.service.artifactsRoot, m); err != nil {
		// The status is sent; cutting the stream is the signal (a tar
		// without its manifest never verifies).
		log.Printf("tape follow: stream %s: %v", computerID, err)
		panic(http.ErrAbortHandler)
	}
	log.Printf("tape follow: %s (%d,%d] events=%d payloads=%d chunks=%d export=%s total=%s",
		computerID, m.Tape.StartSeq, m.Tape.HeadSeq, m.Counts.Events, m.Counts.Payloads, m.Counts.Chunks, exported.Round(time.Millisecond), time.Since(started).Round(time.Millisecond))
}

func sweepTapeFollowTemp(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err == nil && strings.HasPrefix(entry.Name(), "export-") && time.Since(info.ModTime()) > tapeFollowStaleTemp {
			_ = os.RemoveAll(filepath.Join(root, entry.Name()))
		}
	}
}

func hostnameOr(fallback string) string {
	if h, err := os.Hostname(); err == nil && h != "" {
		return h
	}
	return fallback
}
