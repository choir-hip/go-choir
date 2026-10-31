package tapefollow

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Defaults bound one pull and the whole copy.
const (
	DefaultMaxEvents      = 2000
	DefaultMaxBundleBytes = 2 << 30
	DefaultMaxStoreBytes  = 64 << 30
	DefaultInterval       = 30 * time.Second
	DefaultMaxBackoff     = 5 * time.Minute
)

// ErrStoreFull stops pulling when the local copy reaches MaxStoreBytes.
var ErrStoreFull = errors.New("tapefollow: the local copy reached its size bound")

// Phase is where the syncer is.
type Phase string

const (
	PhaseStarting Phase = "starting"
	PhaseSyncing  Phase = "syncing"
	PhaseSynced   Phase = "synced"
	PhaseError    Phase = "error"
	// PhaseStopped: an error retrying cannot fix (a fork, a full store).
	PhaseStopped Phase = "stopped"
)

// Status is what the app's menu shows.
type Status struct {
	Phase         Phase     `json:"phase"`
	ComputerID    string    `json:"computer_id"`
	HeadSeq       int64     `json:"head_seq"`
	HeadDigest    string    `json:"head_digest,omitempty"`
	SyncedAt      time.Time `json:"synced_at,omitempty"`
	SourceHeadSeq int64     `json:"source_head_seq,omitempty"`
	Bytes         int64     `json:"bytes"`
	Err           string    `json:"error,omitempty"`
	ErrAt         time.Time `json:"error_at,omitempty"`
}

// Line is the one-line summary: "Synced to event N, 15:04".
func (s Status) Line() string {
	var base string
	switch {
	case s.SyncedAt.IsZero():
		base = "Not synced yet"
	case s.SourceHeadSeq > s.HeadSeq:
		base = fmt.Sprintf("Syncing: event %d of %d", s.HeadSeq, s.SourceHeadSeq)
	default:
		base = fmt.Sprintf("Synced to event %d, %s", s.HeadSeq, s.SyncedAt.Local().Format("Jan 2 15:04"))
	}
	if s.Err != "" {
		return base + " (error: " + s.Err + ")"
	}
	return base
}

// Syncer keeps Store following the computer through Client.
type Syncer struct {
	Store          *Store
	Client         *Client
	MaxEvents      int64
	MaxBundleBytes int64
	MaxStoreBytes  int64
	Interval       time.Duration
	MaxBackoff     time.Duration
	// OnStatus is called after every change of status (from the syncer's
	// goroutine).
	OnStatus func(Status)
	Now      func() time.Time

	mu     sync.Mutex
	status Status
	wake   chan struct{}
}

func (s *Syncer) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Status returns the current status.
func (s *Syncer) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.status
	if st.ComputerID == "" && s.Store != nil {
		st = s.fromStore(st)
	}
	return st
}

func (s *Syncer) fromStore(st Status) Status {
	st.ComputerID = s.Store.ComputerID()
	if local, ok := s.Store.State(); ok {
		st.HeadSeq, st.HeadDigest, st.SyncedAt, st.Bytes = local.HeadSeq, local.HeadDigest, local.SyncedAt, local.Bytes
	}
	return st
}

func (s *Syncer) update(f func(*Status)) {
	s.mu.Lock()
	st := s.fromStore(s.status)
	f(&st)
	s.status = st
	cb := s.OnStatus
	s.mu.Unlock()
	if cb != nil {
		cb(st)
	}
}

// SyncNow wakes a waiting Run.
func (s *Syncer) SyncNow() {
	s.mu.Lock()
	if s.wake == nil {
		s.wake = make(chan struct{}, 1)
	}
	ch := s.wake
	s.mu.Unlock()
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (s *Syncer) wakeCh() chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.wake == nil {
		s.wake = make(chan struct{}, 1)
	}
	return s.wake
}

// Step pulls and applies one bundle. more says the server's head is past the
// new local head, so the caller should step again at once.
func (s *Syncer) Step(ctx context.Context) (more bool, err error) {
	maxEvents, maxBytes, maxStore := s.MaxEvents, s.MaxBundleBytes, s.MaxStoreBytes
	if maxEvents <= 0 {
		maxEvents = DefaultMaxEvents
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBundleBytes
	}
	if maxStore <= 0 {
		maxStore = DefaultMaxStoreBytes
	}
	local, synced := s.Store.State()
	if synced && local.Bytes >= maxStore {
		return false, fmt.Errorf("%w (%d bytes)", ErrStoreFull, local.Bytes)
	}
	if remaining := maxStore - local.Bytes; remaining < maxBytes {
		maxBytes = remaining
	}
	stage, err := s.Store.NewStaging()
	if err != nil {
		return false, err
	}
	defer s.Store.DiscardStaging(stage) // gone after a successful Apply's rename
	var have *State
	if synced {
		have = &local
	}
	pulled, err := s.Client.Pull(ctx, s.Store.ComputerID(), have, maxEvents, maxBytes, stage)
	if err != nil {
		return false, err
	}
	if pulled.UpToDate {
		s.update(func(st *Status) { st.SourceHeadSeq = pulled.SourceHeadSeq; st.Phase = PhaseSynced; st.Err = "" })
		return false, nil
	}
	next, err := s.Store.Apply(stage, s.now())
	if err != nil {
		return false, err
	}
	more = pulled.SourceHeadSeq > next.HeadSeq
	s.update(func(st *Status) {
		st.SourceHeadSeq = pulled.SourceHeadSeq
		st.Err = ""
		st.Phase = PhaseSynced
		if more {
			st.Phase = PhaseSyncing
		}
	})
	return more, nil
}

// Run follows until ctx ends or an error retrying cannot fix. After an error
// it backs off (doubling to MaxBackoff, or the server's Retry-After); each
// pull restarts from the committed local head, so a lost network or a crash
// costs at most the page in flight.
func (s *Syncer) Run(ctx context.Context) error {
	interval, maxBackoff := s.Interval, s.MaxBackoff
	if interval <= 0 {
		interval = DefaultInterval
	}
	if maxBackoff <= 0 {
		maxBackoff = DefaultMaxBackoff
	}
	s.update(func(st *Status) { st.Phase = PhaseStarting })
	backoff := time.Duration(0)
	wake := s.wakeCh()
	for {
		more, err := s.Step(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		wait := interval
		switch {
		case err == nil:
			backoff = 0
			if more {
				continue
			}
		case IsForked(err) || errors.Is(err, ErrNotChained) || errors.Is(err, ErrStoreFull):
			s.update(func(st *Status) { st.Phase = PhaseStopped; st.Err = err.Error(); st.ErrAt = s.now() })
			return err
		default:
			if backoff == 0 {
				backoff = 2 * time.Second
			} else if backoff *= 2; backoff > maxBackoff {
				backoff = maxBackoff
			}
			wait = backoff
			var h *HTTPError
			if errors.As(err, &h) && h.RetryAfter > wait {
				wait = h.RetryAfter
			}
			if IsUnauthorized(err) && wait < interval {
				wait = interval
			}
			msg := err.Error()
			s.update(func(st *Status) { st.Phase = PhaseError; st.Err = msg; st.ErrAt = s.now() })
		}
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-wake:
			t.Stop()
		case <-t.C:
		}
	}
}
