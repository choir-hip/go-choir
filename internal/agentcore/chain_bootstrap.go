package agentcore

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/buildinfo"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// ErrChainBootstrapUnavailable means the runtime cannot establish a canonical
// chain head through the product path. It is a capability/identity failure,
// not a clean already-bootstrapped result.
var ErrChainBootstrapUnavailable = errors.New("chain bootstrap unavailable")

// ChainBootstrapReport is the product receipt for a plain-computer canonical
// chain bootstrap. It establishes a canonical event head but publishes no
// checkpoint, writes no self-development operation or baseline, and performs
// no effect. It is a precondition for replay eligibility, not restore
// authority.
type ChainBootstrapReport struct {
	ComputerID            string              `json:"computer_id"`
	AlreadyBootstrapped   bool                `json:"already_bootstrapped"`
	Head                  *computerevent.Head `json:"head"`
	TargetStateCommitment string              `json:"target_state_commitment"`
	CodeRef               string              `json:"code_ref"`
	ArtifactProgramRef    string              `json:"artifact_program_ref"`
	AppendedEvent         bool                `json:"appended_event"`
	PublishedCheckpoint   bool                `json:"published_checkpoint"`
	WroteSelfDevOperation bool                `json:"wrote_selfdev_operation"`
}

// BootstrapChain appends exactly one EventGenesisImported for a pre-genesis
// computer, binding the deployed release identity actually running in the
// guest. The state commitment derives from the guest's build commit and guest
// image manifest digest only; live Dolt contents are never folded into it.
// Idempotent: a computer that already has a head returns that head without a
// write.
func (rt *Runtime) BootstrapChain(ctx context.Context, ownerID, computerID string) (ChainBootstrapReport, error) {
	var report ChainBootstrapReport
	if rt == nil || rt.store == nil || rt.eventAppender == nil {
		return report, fmt.Errorf("%w: event projection authority is not configured", ErrChainBootstrapUnavailable)
	}
	computerID = strings.TrimSpace(computerID)
	if computerID == "" || computerID != strings.TrimSpace(rt.cfg.ComputerID) {
		return report, fmt.Errorf("%w: computer binding does not match runtime", ErrChainBootstrapUnavailable)
	}
	if err := ctx.Err(); err != nil {
		return report, err
	}

	head, err := rt.store.Head(ctx, computerID)
	if err != nil {
		return report, fmt.Errorf("chain bootstrap: read event head: %w", err)
	}
	if head != nil {
		report.ComputerID = computerID
		report.AlreadyBootstrapped = true
		report.Head = head
		return report, nil
	}

	// Release identity is derived inside the guest, never accepted from the
	// request. The build commit is the running binary's immutable identity and
	// must agree with the separately observed deploy marker, matching the
	// execution-identity endpoint's fail-closed rule.
	build := buildinfo.Snapshot("autoputer")
	if build.Commit == "" || build.Commit == "local" || build.DeployedCommit == "" || build.DeployedCommit != build.Commit {
		return report, fmt.Errorf("%w: incomplete or conflicting build/deploy identity", ErrChainBootstrapUnavailable)
	}
	manifest, err := digestIdentityArtifact("guest-image-manifest", os.Getenv("CHOIR_GUEST_IMAGE_MANIFEST"))
	if err != nil {
		return report, fmt.Errorf("%w: guest image manifest identity: %v", ErrChainBootstrapUnavailable, err)
	}
	codeRef := "git:" + build.Commit
	artifactRef := "guest-image:" + manifest.SHA256
	commitment, err := computerevent.StateCommitment(computerevent.EffectiveStateRefs{
		ReducerVersion:     computerevent.ReducerVersionV1,
		CodeRef:            codeRef,
		ArtifactProgramRef: artifactRef,
		EmbeddedDoltRefs:   nil,
	})
	if err != nil {
		return report, fmt.Errorf("chain bootstrap: compute state commitment: %w", err)
	}

	eventID, err := computerevent.NewEventID()
	if err != nil {
		return report, fmt.Errorf("chain bootstrap: event id: %w", err)
	}
	event := computerevent.Event{
		SchemaVersion:                computerevent.SchemaVersionV1,
		EventID:                      eventID,
		ComputerID:                   computerID,
		EventKind:                    computerevent.EventGenesisImported,
		OccurredAt:                   time.Now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey:               "lifecycle-bootstrap-chain:" + computerID,
		ActorProfile:                 agentprofile.Super,
		AuthorityRef:                 "external-owner-genesis:" + ownerID,
		PrivacyClass:                 "owner",
		PayloadCommitment:            commitment,
		ReducerVersion:               computerevent.ReducerVersionV1,
		ResultingEffectiveCommitment: commitment,
		RequestCommitment:            computerevent.ZeroHead,
	}
	if _, err := rt.eventAppender.AppendNew(ctx, event, computerevent.TransitionInput{TargetStateCommitment: commitment}, nil); err != nil {
		// A concurrent owner request may have won the CAS. Re-read and return
		// the converged head instead of surfacing a duplicate-genesis conflict.
		if head, headErr := rt.store.Head(ctx, computerID); headErr == nil && head != nil {
			report.ComputerID = computerID
			report.AlreadyBootstrapped = true
			report.Head = head
			return report, nil
		}
		return report, fmt.Errorf("chain bootstrap: append genesis: %w", err)
	}
	head, err = rt.store.Head(ctx, computerID)
	if err != nil || head == nil {
		return report, fmt.Errorf("chain bootstrap: read head after append: %v", err)
	}
	report.ComputerID = computerID
	report.Head = head
	report.TargetStateCommitment = commitment
	report.CodeRef = codeRef
	report.ArtifactProgramRef = artifactRef
	report.AppendedEvent = true
	report.PublishedCheckpoint = false
	report.WroteSelfDevOperation = false
	return report, nil
}

func (h *APIHandler) bootstrapComputerChain(w http.ResponseWriter, r *http.Request, ownerID, computerID string) {
	if r.Method != http.MethodPost {
		writeAPIJSON(w, http.StatusMethodNotAllowed, apiError{Error: "method not allowed"})
		return
	}
	if h == nil || h.rt == nil {
		writeAPIJSON(w, http.StatusServiceUnavailable, apiError{Error: "chain bootstrap authority unavailable"})
		return
	}
	report, err := h.rt.BootstrapChain(r.Context(), ownerID, computerID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrChainBootstrapUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeAPIJSON(w, status, apiError{Error: err.Error()})
		return
	}
	status := http.StatusOK
	if report.AppendedEvent {
		status = http.StatusCreated
	}
	writeAPIJSON(w, status, report)
}
