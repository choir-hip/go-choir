package store

import "context"

// ReadConsistency says whether an event read must include telemetry that is
// still in the sealer's open window (design section 4.4).
type ReadConsistency int

const (
	// ReadConsistent is the default: a ListEvents* call first seals and waits
	// for the pending telemetry window, so the read sees every event this
	// process has accepted. Anything that decides from events, or cites an
	// event by id, must read this way (run acceptance, the Texture workflow
	// verifier, run-memory compaction, WebSocket catch-up).
	ReadConsistent ReadConsistency = iota
	// ReadRelaxed lets a polled UI endpoint read the tape as it is, lagging by
	// at most the window. It is explicit and rare. In this implementation a
	// Relaxed read still seals first (phase 0 and 1 of the design ship
	// everything Consistent); the marker classifies the call site so phase 3
	// can relax it if the counters show reads costing batch size.
	ReadRelaxed
)

type readConsistencyKey struct{}

// WithReadConsistency marks ctx with the read class of the ListEvents* calls
// made under it. Absent, the class is ReadConsistent.
func WithReadConsistency(ctx context.Context, c ReadConsistency) context.Context {
	return context.WithValue(ctx, readConsistencyKey{}, c)
}

func readConsistencyOf(ctx context.Context) ReadConsistency {
	if c, ok := ctx.Value(readConsistencyKey{}).(ReadConsistency); ok {
		return c
	}
	return ReadConsistent
}

// beforeEventRead is called first by every ListEvents* method. It counts the
// read by class and, when batching is on, seals the pending window.
func (s *Store) beforeEventRead(ctx context.Context) {
	if readConsistencyOf(ctx) == ReadRelaxed {
		s.telemetryStats.readsRelaxed.Add(1)
	} else {
		s.telemetryStats.readsConsistent.Add(1)
	}
	if sl := s.telemetry.Load(); sl != nil {
		sl.barrier(sealRead)
	}
}
