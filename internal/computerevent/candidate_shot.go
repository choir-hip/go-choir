package computerevent

import (
	"fmt"
	"strings"
)

// The candidate_shot event (docs/design/source-only-freeze-and-candidate-shots-2026-11-06.md
// 3.3). A platform operation whose bundle carries a ShotPlan is shot by the
// platform after the host build and before the swap: the host-built
// candidate is booted beside the running build and the plan's views are
// shot against both. This event is the durable result, and the materializer
// refuses to swap without a passing one bound to the operation and to the
// host build job the swap would install.
//
// Failure modes decided first:
//
//   - An event for another operation or another host build must never
//     unlock a swap (FM11, FM5): the idempotency key names both, the payload
//     repeats both, and the reader checks all three.
//   - An event written after the decision is no longer pending (a stale
//     worker finishing late) must not be accepted: the reducer binds
//     DecisionRef to the live pending transition.
//   - A verdict that is neither pass nor fail, or a pass with no shot,
//     is refused at validation (FM8: never a pass by default).
//   - The event moves no head: it is a causal fact, like a verification.

// CandidateShotMediaType is the payload's media type.
const CandidateShotMediaType = "application/vnd.choir.candidate-shot+json"

// CandidateShot verdicts.
const (
	CandidateShotPass = "pass"
	CandidateShotFail = "fail"
)

// CandidateShotKey is the event's idempotency key: one event per operation
// and host build job.
func CandidateShotKey(operationID, hostBuildJobID string) string {
	return "selfdev-" + string(EventCandidateShot) + "-" + operationID + "-" + hostBuildJobID
}

// CandidateShotFile is one shot the worker took, bound by digest.
type CandidateShotFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// CandidateShotCopyReceipt measures the state copy the shots ran over.
type CandidateShotCopyReceipt struct {
	CopyMs int64 `json:"copy_ms"`
	ScanMs int64 `json:"scan_ms"`
	Bytes  int64 `json:"bytes"`
}

// CandidateShotPayload is the candidate_shot event's payload.
type CandidateShotPayload struct {
	OperationID    string `json:"operation_id"`
	BundleDigest   string `json:"bundle_digest"`
	HostBuildJobID string `json:"host_build_job_id"`
	// PlanSHA256 is the frozen plan's digest; one plan drove both runs.
	PlanSHA256 string `json:"plan_sha256"`
	Verdict    string `json:"verdict"`
	// Cause names why a fail failed ("shot worker died", a step, a review
	// finding). Required for a fail.
	Cause string `json:"cause,omitempty"`
	// Shots are the before and after PNGs (and the video, by name) the
	// worker took. Required for a pass.
	Shots []CandidateShotFile `json:"shots,omitempty"`
	// Review is the per-shot review's note, recorded verbatim.
	Review string                    `json:"review,omitempty"`
	Copy   *CandidateShotCopyReceipt `json:"copy,omitempty"`
}

// Validate checks the payload's own shape. The binding to an operation and
// a host build is checked by the reader against the operation it serves.
func (p CandidateShotPayload) Validate() error {
	if strings.TrimSpace(p.OperationID) == "" || strings.TrimSpace(p.HostBuildJobID) == "" ||
		!IsSHA256(p.BundleDigest) || !IsSHA256(p.PlanSHA256) {
		return fmt.Errorf("candidate shot: operation, bundle digest, host build job and plan digest are required")
	}
	switch p.Verdict {
	case CandidateShotPass:
		if len(p.Shots) == 0 {
			return fmt.Errorf("candidate shot: a pass records the shots it took")
		}
	case CandidateShotFail:
		if strings.TrimSpace(p.Cause) == "" {
			return fmt.Errorf("candidate shot: a fail names its cause")
		}
	default:
		return fmt.Errorf("candidate shot: verdict %q is neither pass nor fail", p.Verdict)
	}
	for _, shot := range p.Shots {
		if strings.TrimSpace(shot.Name) == "" || !IsSHA256(shot.SHA256) || shot.Bytes < 0 {
			return fmt.Errorf("candidate shot: shot %q is not content-addressed", shot.Name)
		}
	}
	return nil
}
