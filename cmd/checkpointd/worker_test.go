package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/projectionbase"
)

// docs/problems/projection-job-blocked-by-a-cancel-2026-10-26.md: a cancelled
// or timed-out run is transient (`failed`, retried after backoff) even when it
// is wrapped in the deterministic-refusal sentinel; a real refusal stays
// `blocked`.
func TestProjectionRunStatusCancelIsFailedRefusalIsBlocked(t *testing.T) {
	refused := fmt.Errorf("%w: read-back witness mismatch: content root differs", projectionbase.ErrBaseRefused)
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"plain error", errors.New("disk i/o"), "failed"},
		{"deterministic refusal", refused, "blocked"},
		{"bare cancel", context.Canceled, "failed"},
		{"bare deadline", context.DeadlineExceeded, "failed"},
		{"cancel inside refusal", fmt.Errorf("%w: read-back witness: %w", projectionbase.ErrBaseRefused, context.Canceled), "failed"},
		{"deadline inside refusal", fmt.Errorf("%w: seed witness: %w", projectionbase.ErrBaseRefused, context.DeadlineExceeded), "failed"},
		{"cancel inside joined refusal", errors.Join(projectionbase.ErrBaseRefused, fmt.Errorf("extract: %w", context.Canceled)), "failed"},
	}
	for _, c := range cases {
		if got := projectionRunStatus(c.err); got != c.want {
			t.Errorf("%s: status = %q, want %q", c.name, got, c.want)
		}
	}
}

// A deterministic `blocked` site that is handed a transient cause is demoted.
func TestProjectionFailStatusDemotesTransientCauseOfBlockedSite(t *testing.T) {
	if got := projectionFailStatus("blocked", fmt.Errorf("checkpoint escrow unavailable: %w", context.Canceled)); got != "failed" {
		t.Fatalf("blocked site with cancel cause = %q, want failed", got)
	}
	if got := projectionFailStatus("blocked", errors.New("checkpoint escrow changed since job admission")); got != "blocked" {
		t.Fatalf("deterministic blocked site = %q, want blocked", got)
	}
	if got := projectionFailStatus("failed", errors.New("x")); got != "failed" {
		t.Fatalf("failed site = %q", got)
	}
}
