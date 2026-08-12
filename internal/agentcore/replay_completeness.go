package agentcore

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
	choirstore "github.com/yusefmosiah/go-choir/internal/store"
)

// ErrReplayCompletenessUnavailable means the runtime cannot expose the
// owner-authorized replay probe on this process. It is a capability/configuration
// failure, not a clean replay result.
var ErrReplayCompletenessUnavailable = errors.New("replay completeness probe unavailable")

const replayCompletenessSchemaVersion = 2

// ReplayCompletenessReport is the read-only evidence returned by the product
// replay probe. Replay state is built in a disposable Dolt workspace; the live
// runtime store is only observed.
type ReplayCompletenessReport struct {
	SchemaVersion int                               `json:"schema_version"`
	ComputerID    string                            `json:"computer_id"`
	CapturedAt    string                            `json:"captured_at"`
	LiveHead      *computerevent.Head               `json:"live_head"`
	ReplayHead    *computerevent.Head               `json:"replay_head"`
	Live          computerversion.ObservationSet    `json:"live"`
	Replay        computerversion.ObservationSet    `json:"replay"`
	Result        computerversion.EquivalenceResult `json:"result"`
	Eligibility   ReplayEligibility                 `json:"eligibility"`
	ProbeDigest   string                            `json:"probe_digest"`
}

// ReplayCompleteness performs the effects Definition's pre-drop measurement.
// It reconstructs the canonical event chain into a fresh workspace, hashes
// that projection and the live workspace through DoltStateExtractor, and
// returns the exact deterministic diff. No event is appended and the live
// store is never used as the replay projection.
func (rt *Runtime) ReplayCompleteness(ctx context.Context, computerID string) (ReplayCompletenessReport, error) {
	if rt == nil || rt.store == nil || rt.eventAppender == nil {
		return ReplayCompletenessReport{}, fmt.Errorf("%w: event projection authority is not configured", ErrReplayCompletenessUnavailable)
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return ReplayCompletenessReport{}, fmt.Errorf("%w: computer binding does not match runtime", ErrReplayCompletenessUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return ReplayCompletenessReport{}, err
	}

	liveHead, err := rt.store.Head(ctx, computerID)
	if err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: read live event head: %w", err)
	}
	version := replayCompletenessVersion(rt, liveHead)
	extractor := replayDoltStateExtractor(rt.store.TexturePath())
	live, err := extractor.Extract(ctx, computerversion.ExtractRequest{Name: "live-dolt", Version: version})
	if err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: extract live Dolt state: %w", err)
	}

	tempRoot, err := os.MkdirTemp("", "choir-replay-completeness-")
	if err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: create disposable workspace: %w", err)
	}
	defer os.RemoveAll(tempRoot)

	replayStore, err := choirstore.OpenFresh(filepath.Join(tempRoot, "runtime.db"))
	if err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: open disposable workspace: %w", err)
	}
	defer replayStore.Close()
	if err := rt.eventAppender.ReconstructInto(ctx, replayStore); err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: reconstruct event chain: %w", err)
	}
	replayHead, err := replayStore.Head(ctx, computerID)
	if err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: read replay event head: %w", err)
	}
	replayExtractor := replayDoltStateExtractor(replayStore.TexturePath())
	replay, err := replayExtractor.Extract(ctx, computerversion.ExtractRequest{Name: "event-replay-projection", Version: version})
	if err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: extract replay Dolt state: %w", err)
	}

	liveAfter, err := extractor.Extract(ctx, computerversion.ExtractRequest{Name: "live-dolt-after", Version: version})
	if err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: re-extract live Dolt state: %w", err)
	}
	liveHeadAfter, err := rt.store.Head(ctx, computerID)
	if err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: re-read live event head: %w", err)
	}
	if !sameReplayHead(liveHead, liveHeadAfter) {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: live event head changed during probe")
	}
	stable := (computerversion.EquivalenceChecker{}).CheckObservationSets(
		filterReplayHeadObservation(live), filterReplayHeadObservation(liveAfter),
	)
	if !stable.Equivalent() {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: live Dolt state changed during probe: %v", stable.Differences)
	}

	result := (computerversion.EquivalenceChecker{}).CheckObservationSets(
		filterReplayHeadObservation(live), filterReplayHeadObservation(replay),
	)
	report := ReplayCompletenessReport{
		SchemaVersion: replayCompletenessSchemaVersion,
		ComputerID:    computerID,
		CapturedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		LiveHead:      liveHead,
		ReplayHead:    replayHead,
		Live:          live,
		Replay:        replay,
		Result:        result,
		Eligibility:   replayEligibility(liveHead, replayHead, live, replay, result),
	}
	raw, err := computerevent.CanonicalJSON(report)
	if err != nil {
		return ReplayCompletenessReport{}, fmt.Errorf("replay completeness: encode report: %w", err)
	}
	report.ProbeDigest = computerevent.DigestBytes(raw)
	return report, nil
}

func replayCompletenessVersion(rt *Runtime, head *computerevent.Head) computerversion.ComputerVersion {
	codeRef := strings.TrimSpace(rt.selfdevStartupReleaseDigest)
	if codeRef == "" {
		codeRef = "runtime:" + strings.TrimSpace(rt.cfg.ComputerID)
	}
	artifactRef := "event-chain:none"
	if head != nil && strings.TrimSpace(head.CanonicalEventHead) != "" {
		artifactRef = "event-chain:" + head.CanonicalEventHead
	}
	return computerversion.ComputerVersion{
		CodeRef:            computerversion.CodeRef(codeRef),
		ArtifactProgramRef: computerversion.ArtifactProgramRef(artifactRef),
	}
}

func replayDoltStateExtractor(workspacePath string) computerversion.DoltStateExtractor {
	return computerversion.DoltStateExtractor{
		WorkspacePath: workspacePath,
		Database:      "texture",
		IgnoredContentColumns: map[string]map[string]struct{}{
			"computer_event_index": {
				"prepared_at":  {},
				"finalized_at": {},
			},
			"computer_event_projection_heads": {
				"updated_at": {},
			},
		},
	}
}

func filterReplayHeadObservation(obs computerversion.ObservationSet) computerversion.ObservationSet {
	filtered := make([]computerversion.Observation, 0, len(obs.Observations))
	for _, observation := range obs.Observations {
		if strings.HasPrefix(observation.Key, "dolt:") && strings.HasSuffix(observation.Key, ":head") {
			continue
		}
		filtered = append(filtered, observation)
	}
	obs.Observations = filtered
	return obs
}

func sameReplayHead(left, right *computerevent.Head) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
