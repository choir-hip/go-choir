package computerevent

import (
	"fmt"
	"strings"
)

// The shot_verdict event (docs/design/desk-shots-and-review-2026-11-07.md
// 3.2, 3.3). A platform operation whose bundle carries a ShotPlan is shot by
// management's verbs (choir.ShotStart, choir.ShotWait) after the host build
// and before the swap, and management records its judgment of the pair with
// choir.ShotVerdict. This event is that judgment, written only by the
// ShotVerdict reduction; the materializer refuses to swap without a passing
// one bound to the operation, the host build job, the shot job it judged, the
// bundle and the plan.
//
// It replaces the candidate_shot event, which the platform's own shot worker
// signed. EventCandidateShot stays a valid kind so the tape that holds those
// events still reduces; nothing writes it any more.
//
// Failure modes decided first:
//
//   - An event for another operation, host build or shot job must never
//     unlock a swap (FM2): the idempotency key names operation and host
//     build, the payload repeats both and the shot job, and the reader
//     checks all of them.
//   - An event written after the decision is no longer pending must not be
//     accepted: the reducer binds DecisionRef to the live pending transition.
//   - A verdict that is neither pass nor fail, a pass with no findings or
//     with a finding that is not a full set of answers, and a fail with no
//     reason are refused at validation (never a pass by default).
//   - The event moves no head: it is a causal fact, like a verification.

// ShotVerdictMediaType is the payload's media type.
const ShotVerdictMediaType = "application/vnd.choir.shot-verdict+json"

// Shot verdicts.
const (
	ShotVerdictPass = "pass"
	ShotVerdictFail = "fail"
)

// ShotVerdictKey is the event's idempotency key: one verdict per operation
// and host build job.
func ShotVerdictKey(operationID, hostBuildJobID string) string {
	return "shot-verdict:" + operationID + ":" + hostBuildJobID
}

// ShotFile is one shot the job took, bound by digest.
type ShotFile struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}

// ShotFindingRecord is management's finding on one screen pair: the digests of
// the before and after shots it looked at and its answers to the overlay's six
// questions (keys q1..q6; yes, no or n/a).
type ShotFindingRecord struct {
	Pair         string            `json:"pair"`
	BeforeSHA256 string            `json:"before_sha256"`
	AfterSHA256  string            `json:"after_sha256"`
	Answers      map[string]string `json:"answers"`
	Note         string            `json:"note,omitempty"`
}

// ShotJudge names who judged the pair, so release evidence says so.
type ShotJudge struct {
	AgentID string `json:"agent_id"`
	RunID   string `json:"run_id"`
}

// ShotVerdictPayload is the shot_verdict event's payload.
type ShotVerdictPayload struct {
	OperationID    string `json:"operation_id"`
	BundleDigest   string `json:"bundle_digest"`
	HostBuildJobID string `json:"host_build_job_id"`
	ShotJobID      string `json:"shot_job_id"`
	// PlanSHA256 is the frozen plan's digest; one plan drove both runs.
	PlanSHA256 string `json:"plan_sha256"`
	Verdict    string `json:"verdict"`
	// Reason names why a fail failed. Required for a fail.
	Reason   string              `json:"reason,omitempty"`
	Findings []ShotFindingRecord `json:"findings,omitempty"`
	// Shots are the before and after PNGs (and the video, by name) the job
	// took.
	Shots []ShotFile `json:"shots,omitempty"`
	Judge ShotJudge  `json:"judge"`
}

// Validate checks the payload's own shape. The binding to an operation, a
// host build and a shot job is checked by the reader against the operation it
// serves.
func (p ShotVerdictPayload) Validate() error {
	if strings.TrimSpace(p.OperationID) == "" || strings.TrimSpace(p.HostBuildJobID) == "" || strings.TrimSpace(p.ShotJobID) == "" ||
		!IsSHA256(p.BundleDigest) || !IsSHA256(p.PlanSHA256) {
		return fmt.Errorf("shot verdict: operation, bundle digest, host build job, shot job and plan digest are required")
	}
	switch p.Verdict {
	case ShotVerdictPass:
		if len(p.Findings) == 0 {
			return fmt.Errorf("shot verdict: a pass records its findings")
		}
	case ShotVerdictFail:
		if strings.TrimSpace(p.Reason) == "" {
			return fmt.Errorf("shot verdict: a fail names its reason")
		}
	default:
		return fmt.Errorf("shot verdict: verdict %q is neither pass nor fail", p.Verdict)
	}
	for _, shot := range p.Shots {
		if strings.TrimSpace(shot.Name) == "" || !IsSHA256(shot.SHA256) || shot.Bytes < 0 {
			return fmt.Errorf("shot verdict: shot %q is not content-addressed", shot.Name)
		}
	}
	for _, f := range p.Findings {
		if !IsSHA256(f.BeforeSHA256) || !IsSHA256(f.AfterSHA256) {
			return fmt.Errorf("shot verdict: finding %q does not name its shots by digest", f.Pair)
		}
	}
	return nil
}
