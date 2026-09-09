package agentcore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/projectionbase"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// resolveRestoreBaseSource returns the verified-base source for recovery. An
// injected fake wins for tests; otherwise the platform source is built from
// CorpusdURL plus guest credentials per call. No source means no recovery:
// callers refuse, never replay genesis.
func (rt *Runtime) resolveRestoreBaseSource() (projectionbase.BaseSource, error) {
	if rt == nil || rt.eventAppender == nil {
		return nil, fmt.Errorf("%w: event projection authority is not configured", ErrRematerializeUnavailable)
	}
	if rt.restoreBaseSource != nil {
		return rt.restoreBaseSource, nil
	}
	baseURL := strings.TrimSpace(os.Getenv("CHOIR_PLATFORM_URL"))
	if baseURL == "" && strings.TrimSpace(rt.cfg.CorpusdURL) != "" {
		baseURL = strings.TrimSpace(rt.cfg.CorpusdURL)
	}
	creds := rt.selfdevControl
	if strings.TrimSpace(baseURL) == "" || creds == nil {
		return nil, fmt.Errorf("%w: restore base authority is not configured", ErrRematerializeUnavailable)
	}
	return projectionbase.NewHTTPSource(baseURL, projectionbase.CapabilityFunc(creds.Capability)), nil
}

// resolveRecoveryTarget proves the recovery target is the advertised
// watermark or descends from it using immutable tape, and returns its
// sequence. The enumeration is tail-bounded: it starts after W and stops at H.
func resolveRecoveryTarget(ctx context.Context, src projectionbase.BaseSource, computerID, targetHead string) (uint64, error) {
	sequence, baseRef, err := src.Watermark(ctx, computerID)
	if err != nil {
		return 0, err
	}
	if sequence == 0 || strings.TrimSpace(baseRef) == "" {
		return 0, fmt.Errorf("%w: no advertised base for %s", projectionbase.ErrBaseRefused, computerID)
	}
	descriptor, err := src.Descriptor(ctx, computerID, strings.TrimSpace(baseRef))
	if err != nil {
		return 0, err
	}
	return projectionbase.ResolveTargetSequence(ctx, src, computerID, targetHead, sequence, descriptor.CanonicalHead)
}

// installStagedBase installs the verified base for targetHead into stagingRoot
// and opens it. The returned store head is the watermark W; replay resumes
// after W through the existing reconstruct loop. Any failure refuses before
// the caller mutates the original realization.
func installStagedBase(ctx context.Context, src projectionbase.BaseSource, stagingRoot, markerName, computerID, targetHead string, targetSequence uint64) (*choirstore.Store, projectionbase.Descriptor, error) {
	descriptor, err := projectionbase.InstallVerifiedBase(ctx, src, stagingRoot, markerName, computerID, targetHead, targetSequence)
	if err != nil {
		return nil, projectionbase.Descriptor{}, err
	}
	staged, err := choirstore.Open(filepath.Join(stagingRoot, markerName))
	if err != nil {
		return nil, projectionbase.Descriptor{}, fmt.Errorf("rematerialize: open staged base: %w", err)
	}
	return staged, descriptor, nil
}

// restoreReplayObserver counts recovery-critical replay work for tail-bounded
// cost evidence: every applied sequence must fall in (W,H] exactly once, and
// no page may enumerate from before W. The first tail page legitimately
// starts its cursor at W.
type restoreReplayObserver struct {
	mu              sync.Mutex
	applied         int
	appliedMin      uint64
	appliedMax      uint64
	pages           int
	pageAfterMin    uint64
	pageAfterMinSet bool
	commits         int
}

func (o *restoreReplayObserver) PageFetched(afterSequence uint64, count int) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pages++
	if !o.pageAfterMinSet || afterSequence < o.pageAfterMin {
		o.pageAfterMin = afterSequence
		o.pageAfterMinSet = true
	}
}

func (o *restoreReplayObserver) RecordApplied(sequence uint64) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.applied++
	if o.applied == 1 || sequence < o.appliedMin {
		o.appliedMin = sequence
	}
	if sequence > o.appliedMax {
		o.appliedMax = sequence
	}
}

func (o *restoreReplayObserver) CheckpointCommitted(sequence uint64) {
	if o == nil {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.commits++
}

// tailReceipt verifies the observed replay against the asserted tail bounds.
// It proves zero prefix work and exactly-once contiguous tail application.
func (o *restoreReplayObserver) tailReceipt(watermarkSequence uint64, targetSequence uint64) (applied int, err error) {
	if o == nil {
		return 0, fmt.Errorf("rematerialize: replay observer is required")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.pageAfterMinSet && o.pageAfterMin < watermarkSequence {
		return 0, fmt.Errorf("%w: replay enumerated before watermark %d", projectionbase.ErrBaseRefused, watermarkSequence)
	}
	if o.applied > 0 {
		if o.appliedMin <= watermarkSequence {
			return 0, fmt.Errorf("%w: replay applied prefix sequence %d", projectionbase.ErrBaseRefused, o.appliedMin)
		}
		if o.appliedMax != targetSequence || o.applied != int(targetSequence-watermarkSequence) {
			return 0, fmt.Errorf("%w: tail application is not exactly-once (%d events, [%d,%d], want (%d,%d])", projectionbase.ErrBaseRefused, o.applied, o.appliedMin, o.appliedMax, watermarkSequence, targetSequence)
		}
	} else if watermarkSequence != targetSequence {
		return 0, fmt.Errorf("%w: non-empty tail after watermark %d applied nothing", projectionbase.ErrBaseRefused, watermarkSequence)
	}
	return o.applied, nil
}

var _ computerevent.ReplayObserver = (*restoreReplayObserver)(nil)
