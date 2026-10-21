package agentcore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Carried patch (failure 15, problem 1, option A,
// docs/problems/engineering-completion-path-cluster-2026-10-20.md). When an
// engineering round ends without a finished candidate and the work is retried
// on the same trajectory, the retry runs in a fresh capsule from the base
// tree. The failed round's source tree is already durable (the candidate its
// terminal Report minted); this file derives it into an inert, digest-bound
// patch, stages it content-addressed and read only, and tells the retry's
// engineer about it. No capsule survives its round and nothing is applied: the
// engineer chooses to apply the patch inside its own capsule, re-tests it
// there, and the normal Freeze, verify, approval and apply judge the result.
//
// The offer is decided once per assignment and remembered
// (<updater root>/carried/offers/), so a replayed spawn after a crash reads
// the same decision. Failing to carry never fails the retry.

const (
	carriedOffered = "offered"
	carriedRefused = "refused"

	carriedOfferSchema = "choir.carried_patch_offer.v1"
	carriedFactsSchema = "choir.carried_patch.v1"
)

var errCarriedPatchTampered = errors.New("staged carried patch no longer matches its digest")

// carriedCandidate is one prior round's changed source tree, with what the
// offer needs to know about the round that made it.
type carriedCandidate struct {
	CandidateID  string
	CandidateRef string
	AssignmentID string
	Attempt      uint64
	// BaseRef is the round's own pinned source (its binding's
	// SourceArtifactRef).
	BaseRef   string
	Kind      types.EngineeringAssignmentKind
	Result    types.EngineeringAssignmentResultKind
	CreatedAt time.Time
}

type carriedPatchHistory interface {
	CarriedCandidates(ctx context.Context, ownerID, computerID, trajectoryID string) ([]carriedCandidate, error)
}

type carriedPatchDeriver interface {
	DeriveCarriedPatch(ctx context.Context, baseRef, candidateRef string) (capsule.CarriedPatch, error)
	CheckCarriedPatchApplies(ctx context.Context, patch capsule.CarriedPatch, retryBaseRef string) error
}

type carriedPatchDeps struct {
	History carriedPatchHistory
	Deriver carriedPatchDeriver
	// Root is the updater root; staged patches live under <Root>/carried.
	Root string
}

type carriedPatchInput struct {
	OwnerID, ComputerID, TrajectoryID string
	AssignmentID                      string
	Attempt                           uint64
	// RetryBaseRef is the retry's own pinned source.
	RetryBaseRef string
}

// carriedPatchFacts is carried.json beside the staged patch: what binds the
// patch to where it came from.
type carriedPatchFacts struct {
	Schema             string   `json:"schema"`
	PatchSHA256        string   `json:"patch_sha256"`
	Size               int      `json:"size"`
	BaseRef            string   `json:"base_ref"`
	CandidateID        string   `json:"candidate_id"`
	CandidateRef       string   `json:"candidate_ref"`
	SourceAssignmentID string   `json:"source_assignment_id"`
	SourceAttempt      uint64   `json:"source_attempt"`
	SourceResult       string   `json:"source_result"`
	Files              []string `json:"files"`
	Omitted            []string `json:"omitted,omitempty"`
}

// carriedPatchOffer is the decision for one assignment. Status is "" when
// there is nothing to say.
type carriedPatchOffer struct {
	Schema       string `json:"schema"`
	AssignmentID string `json:"assignment_id"`
	Attempt      uint64 `json:"attempt"`
	Status       string `json:"status"`
	Reason       string `json:"reason,omitempty"`

	PatchSHA256        string   `json:"patch_sha256,omitempty"`
	Size               int      `json:"size,omitempty"`
	BaseRef            string   `json:"base_ref,omitempty"`
	RetryBaseRef       string   `json:"retry_base_ref,omitempty"`
	BaseMoved          bool     `json:"base_moved,omitempty"`
	CandidateID        string   `json:"candidate_id,omitempty"`
	SourceAssignmentID string   `json:"source_assignment_id,omitempty"`
	SourceAttempt      uint64   `json:"source_attempt,omitempty"`
	SourceResult       string   `json:"source_result,omitempty"`
	Files              []string `json:"files,omitempty"`
	Omitted            []string `json:"omitted,omitempty"`
	OlderCandidates    []string `json:"older_candidates,omitempty"`

	// Dir is the staged directory; derived from the root and the digest on
	// every read, never trusted from disk.
	Dir string `json:"-"`
}

// pickCarriedCandidate chooses the one round to carry: the newest changed
// implementation candidate that is not the retry's own. A finished
// (completed) newest round ends the chain and nothing is carried. The other
// candidates are returned for reference only.
func pickCarriedCandidate(entries []carriedCandidate, selfAssignmentID string) (*carriedCandidate, []string) {
	var eligible []carriedCandidate
	for _, entry := range entries {
		if entry.AssignmentID == selfAssignmentID || entry.Kind != types.EngineeringAssignmentImplementation || strings.TrimSpace(entry.CandidateRef) == "" {
			continue
		}
		eligible = append(eligible, entry)
	}
	if len(eligible) == 0 {
		return nil, nil
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if !eligible[i].CreatedAt.Equal(eligible[j].CreatedAt) {
			return eligible[i].CreatedAt.After(eligible[j].CreatedAt)
		}
		return eligible[i].CandidateID > eligible[j].CandidateID
	})
	if eligible[0].Result == types.EngineeringResultCompleted {
		return nil, nil
	}
	var older []string
	for _, entry := range eligible[1:] {
		older = append(older, entry.CandidateID)
	}
	return &eligible[0], older
}

func carriedOfferPath(root, assignmentID string, attempt uint64) string {
	return filepath.Join(root, "carried", "offers", url.PathEscape(assignmentID)+fmt.Sprintf("-%d.json", attempt))
}

// offerCarriedPatch decides, once, what the assignment's engineer is told
// about an earlier round's work, and stages the patch when it is offered. It
// never fails: a problem carrying the work is a refusal the desk reads, and
// the retry starts clean.
func offerCarriedPatch(ctx context.Context, deps carriedPatchDeps, in carriedPatchInput) carriedPatchOffer {
	if existing := readCarriedOffer(deps.Root, in.AssignmentID, in.Attempt); existing.Status != "" {
		return existing
	}
	if deps.History == nil || deps.Deriver == nil || strings.TrimSpace(deps.Root) == "" {
		return carriedPatchOffer{}
	}
	entries, err := deps.History.CarriedCandidates(ctx, in.OwnerID, in.ComputerID, in.TrajectoryID)
	if err != nil {
		log.Printf("agentcore: carried patch: read trajectory %s candidates: %v", in.TrajectoryID, err)
		return carriedPatchOffer{}
	}
	chosen, older := pickCarriedCandidate(entries, in.AssignmentID)
	if chosen == nil {
		return carriedPatchOffer{}
	}
	offer := carriedPatchOffer{
		Schema: carriedOfferSchema, AssignmentID: in.AssignmentID, Attempt: in.Attempt,
		BaseRef: chosen.BaseRef, RetryBaseRef: in.RetryBaseRef, BaseMoved: chosen.BaseRef != in.RetryBaseRef,
		CandidateID: chosen.CandidateID, SourceAssignmentID: chosen.AssignmentID, SourceAttempt: chosen.Attempt,
		SourceResult: string(chosen.Result), OlderCandidates: older,
	}
	refuse := func(persist bool, format string, args ...any) carriedPatchOffer {
		offer.Status, offer.Reason = carriedRefused, fmt.Sprintf(format, args...)
		if persist {
			writeCarriedOffer(deps.Root, offer)
		}
		return offer
	}
	patch, err := deps.Deriver.DeriveCarriedPatch(ctx, chosen.BaseRef, chosen.CandidateRef)
	switch {
	case errors.Is(err, capsule.ErrCarriedPatchEmpty):
		return carriedPatchOffer{}
	case errors.Is(err, capsule.ErrCarriedPatchTooLarge):
		return refuse(true, "the earlier round's patch is too large to carry (%v)", err)
	case errors.Is(err, capsule.ErrSubjectArtifactUnavailable):
		return refuse(true, "the earlier round's candidate artifact is unavailable (%v)", err)
	case errors.Is(err, capsule.ErrCarriedPatchOutsideSource):
		return refuse(true, "the earlier round's patch names a path outside the source tree (%v)", err)
	case err != nil:
		log.Printf("agentcore: carried patch: derive candidate %s: %v", chosen.CandidateID, err)
		return refuse(false, "the earlier round's patch could not be derived (%v)", err)
	}
	// The deriver is not trusted with the digest or the paths: both are
	// recomputed here before anything is staged.
	sum := sha256.Sum256(patch.Patch)
	if len(patch.Patch) == 0 || patch.SHA256 != hex.EncodeToString(sum[:]) {
		return refuse(true, "the earlier round's patch does not match its digest")
	}
	if len(patch.Patch) > capsule.CarriedPatchMaxBytes {
		return refuse(true, "the earlier round's patch is too large to carry (%d bytes, bound %d)", len(patch.Patch), capsule.CarriedPatchMaxBytes)
	}
	if _, err := capsule.CarriedPatchPaths(patch.Patch); err != nil {
		return refuse(true, "the earlier round's patch names a path outside the source tree (%v)", err)
	}
	if offer.BaseMoved {
		if err := deps.Deriver.CheckCarriedPatchApplies(ctx, patch, in.RetryBaseRef); err != nil {
			return refuse(true, "the earlier round's base (%s) differs from this assignment's base (%s) and its patch does not apply: %v",
				carriedBaseLabel(chosen.BaseRef), carriedBaseLabel(in.RetryBaseRef), err)
		}
	}
	facts := carriedPatchFacts{
		Schema: carriedFactsSchema, PatchSHA256: patch.SHA256, Size: len(patch.Patch), BaseRef: chosen.BaseRef,
		CandidateID: chosen.CandidateID, CandidateRef: chosen.CandidateRef,
		SourceAssignmentID: chosen.AssignmentID, SourceAttempt: chosen.Attempt, SourceResult: string(chosen.Result),
		Files: patch.Files, Omitted: patch.Omitted,
	}
	dir, err := stageCarriedPatch(deps.Root, patch.Patch, facts)
	if err != nil {
		log.Printf("agentcore: carried patch: stage %s: %v", patch.SHA256, err)
		return refuse(false, "the earlier round's patch could not be staged (%v)", err)
	}
	offer.Status, offer.PatchSHA256, offer.Size, offer.Files, offer.Omitted, offer.Dir =
		carriedOffered, patch.SHA256, len(patch.Patch), patch.Files, patch.Omitted, dir
	writeCarriedOffer(deps.Root, offer)
	return offer
}

// stageCarriedPatch writes source.patch and carried.json under
// <root>/carried/<sha256>/, read only. An existing staging is kept when it
// verifies and replaced when it does not.
func stageCarriedPatch(root string, patch []byte, facts carriedPatchFacts) (string, error) {
	parent := filepath.Join(root, "carried")
	target := filepath.Join(parent, facts.PatchSHA256)
	if verifyStagedCarriedPatch(target, facts.PatchSHA256) == nil {
		return target, nil
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	canonical, err := json.Marshal(facts)
	if err != nil {
		return "", err
	}
	tmp, err := os.MkdirTemp(parent, ".stage-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	if err := os.WriteFile(filepath.Join(tmp, "source.patch"), patch, 0o444); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, "carried.json"), canonical, 0o444); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return "", err
	}
	if _, err := os.Lstat(target); err == nil {
		if err := os.RemoveAll(target); err != nil {
			return "", err
		}
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", err
	}
	return target, nil
}

// verifyStagedCarriedPatch re-reads a staged patch: exactly the two files,
// the patch bytes hash to sha, they are within the bound and name only source
// paths, and carried.json is bound to the same digest.
func verifyStagedCarriedPatch(dir, sha string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 2 {
		return errCarriedPatchTampered
	}
	patch, err := os.ReadFile(filepath.Join(dir, "source.patch"))
	if err != nil || len(patch) == 0 || len(patch) > capsule.CarriedPatchMaxBytes {
		return errCarriedPatchTampered
	}
	sum := sha256.Sum256(patch)
	if hex.EncodeToString(sum[:]) != sha {
		return errCarriedPatchTampered
	}
	if _, err := capsule.CarriedPatchPaths(patch); err != nil {
		return fmt.Errorf("%w: %v", errCarriedPatchTampered, err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "carried.json"))
	if err != nil {
		return errCarriedPatchTampered
	}
	var facts carriedPatchFacts
	if json.Unmarshal(raw, &facts) != nil || facts.Schema != carriedFactsSchema || facts.PatchSHA256 != sha {
		return errCarriedPatchTampered
	}
	return nil
}

func writeCarriedOffer(root string, offer carriedPatchOffer) {
	path := carriedOfferPath(root, offer.AssignmentID, offer.Attempt)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Printf("agentcore: carried patch: record offer: %v", err)
		return
	}
	raw, err := json.Marshal(offer)
	if err != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".offer-")
	if err != nil {
		log.Printf("agentcore: carried patch: record offer: %v", err)
		return
	}
	_, writeErr := tmp.Write(raw)
	closeErr := tmp.Close()
	if writeErr != nil || closeErr != nil || os.Chmod(tmp.Name(), 0o444) != nil || os.Rename(tmp.Name(), path) != nil {
		_ = os.Remove(tmp.Name())
		log.Printf("agentcore: carried patch: record offer for %s failed", offer.AssignmentID)
	}
}

// readCarriedOffer returns the recorded decision for an assignment, with an
// offered patch re-verified: bytes that no longer match are a refusal, never a
// mount or a name.
func readCarriedOffer(root, assignmentID string, attempt uint64) carriedPatchOffer {
	if strings.TrimSpace(root) == "" || strings.TrimSpace(assignmentID) == "" {
		return carriedPatchOffer{}
	}
	raw, err := os.ReadFile(carriedOfferPath(root, assignmentID, attempt))
	if err != nil {
		return carriedPatchOffer{}
	}
	var offer carriedPatchOffer
	if json.Unmarshal(raw, &offer) != nil || offer.Schema != carriedOfferSchema || offer.AssignmentID != assignmentID || offer.Attempt != attempt {
		return carriedPatchOffer{}
	}
	if offer.Status != carriedOffered {
		offer.Dir = ""
		return offer
	}
	dir := filepath.Join(root, "carried", offer.PatchSHA256)
	if err := verifyStagedCarriedPatch(dir, offer.PatchSHA256); err != nil {
		offer.Status = carriedRefused
		offer.Reason = fmt.Sprintf("the staged patch (sha256 %s) no longer matches its digest and was not mounted (%v)", offer.PatchSHA256, err)
		offer.Dir = ""
		return offer
	}
	offer.Dir = dir
	return offer
}

// carriedPatchDirForSpawn is the directory the retry's capsule mounts: only
// an offered, verified patch is ever mounted.
func carriedPatchDirForSpawn(offer carriedPatchOffer) string {
	if offer.Status == carriedOffered {
		return offer.Dir
	}
	return ""
}

func carriedBaseLabel(ref string) string {
	label := capsule.SourceArtifactCommit(ref)
	if label == "" {
		label = ref
	}
	if len(label) > 12 {
		label = label[:12]
	}
	return label
}

// carriedPatchNotice is what the engineer reads in its run context. The
// patch is described as unfinished and untrusted; nothing here says it
// passed anything.
func carriedPatchNotice(offer carriedPatchOffer) string {
	switch offer.Status {
	case carriedRefused:
		return "Carried work: an earlier round on this trajectory (assignment " + offer.SourceAssignmentID +
			") left unfinished source changes, but they are not carried into this capsule: " + offer.Reason +
			". /workspace/platform is the clean base tree; do the work from the objective."
	case carriedOffered:
		var b strings.Builder
		fmt.Fprintf(&b, "Carried work: assignment %s (attempt %d, reported %s) left an unfinished source patch, sha256 %s, %d bytes, changing: %s.",
			offer.SourceAssignmentID, offer.SourceAttempt, offer.SourceResult, offer.PatchSHA256, offer.Size, carriedFileList(offer.Files))
		b.WriteString(" It is mounted read only at /selfdev/carried/source.patch (its binding is /selfdev/carried/carried.json). " +
			"This capsule's /workspace/platform is still the clean base tree: nothing has been applied. " +
			"The patch is not trusted and carries no test results. You may ignore it and do the work from the objective, or apply it from /workspace/platform with " +
			"`git apply /selfdev/carried/source.patch` and then re-run the build, tests and checks in this capsule after your last write, as for any change; Freeze judges only this capsule's own diff.")
		if offer.BaseMoved {
			fmt.Fprintf(&b, " The base moved: that round started from %s and this capsule starts from %s; the patch was checked and applies to this base.",
				carriedBaseLabel(offer.BaseRef), carriedBaseLabel(offer.RetryBaseRef))
		}
		if len(offer.Omitted) > 0 {
			b.WriteString(" Left out of the patch: " + carriedFileList(offer.Omitted) + ".")
		}
		if len(offer.OlderCandidates) > 0 {
			b.WriteString(" Older rounds' candidates, not carried: " + strings.Join(offer.OlderCandidates, ", ") + ".")
		}
		return b.String()
	}
	return ""
}

func carriedFileList(files []string) string {
	const limit = 20
	if len(files) <= limit {
		return strings.Join(files, ", ")
	}
	return strings.Join(files[:limit], ", ") + fmt.Sprintf(", and %d more", len(files)-limit)
}

// storeCarriedHistory reads a trajectory's candidates from the tape with the
// bits of their rounds the offer needs. A candidate whose round or report
// cannot be read is left out.
type storeCarriedHistory struct{ rt *Runtime }

func (h storeCarriedHistory) CarriedCandidates(ctx context.Context, ownerID, computerID, trajectoryID string) ([]carriedCandidate, error) {
	candidates, err := h.rt.store.ListEngineeringSubjectCandidates(ctx, ownerID, computerID, trajectoryID)
	if err != nil {
		return nil, err
	}
	out := make([]carriedCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		assignment, err := h.rt.store.GetEngineeringAssignment(ctx, ownerID, computerID, candidate.AssignmentID, candidate.Attempt)
		if err != nil {
			continue
		}
		report, err := h.rt.store.GetEngineeringAssignmentReport(ctx, ownerID, computerID, candidate.SourceReportID)
		if err != nil {
			continue
		}
		out = append(out, carriedCandidate{
			CandidateID: candidate.CandidateID, CandidateRef: candidate.ArtifactRef, AssignmentID: candidate.AssignmentID,
			Attempt: candidate.Attempt, BaseRef: assignment.Binding.SourceArtifactRef, Kind: assignment.Binding.Kind,
			Result: report.Result, CreatedAt: candidate.CreatedAt,
		})
	}
	return out, nil
}

// offerCarriedPatchForAssignment is the spawn saga's call: the offer for an
// implementation assignment, or nothing for any other kind or a runtime with
// no store, executor or updater root.
func (rt *Runtime) offerCarriedPatchForAssignment(ctx context.Context, assignment types.EngineeringAssignment, retryBaseRef string) carriedPatchOffer {
	binding := assignment.Binding
	if rt == nil || rt.store == nil || rt.capsuleExecutor == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" ||
		binding.Kind != types.EngineeringAssignmentImplementation {
		return carriedPatchOffer{}
	}
	return offerCarriedPatch(ctx, carriedPatchDeps{History: storeCarriedHistory{rt: rt}, Deriver: rt.capsuleExecutor, Root: rt.selfdevUpdaterRoot},
		carriedPatchInput{OwnerID: binding.OwnerID, ComputerID: binding.ComputerID, TrajectoryID: binding.TrajectoryID,
			AssignmentID: assignment.AssignmentID, Attempt: binding.Attempt, RetryBaseRef: retryBaseRef})
}

// carriedPatchNoticeForRun is the run-context text for an implementation
// assignment's engineer, from the recorded decision.
func (rt *Runtime) carriedPatchNoticeForRun(rec *types.RunRecord) string {
	if rt == nil || rec == nil || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" ||
		metadataStringValue(rec.Metadata, "assignment_kind") != string(types.EngineeringAssignmentImplementation) {
		return ""
	}
	assignmentID := metadataStringValue(rec.Metadata, "assignment_id")
	attempt := uint64(metadataIntValue(rec.Metadata, "assignment_attempt"))
	return carriedPatchNotice(readCarriedOffer(rt.selfdevUpdaterRoot, assignmentID, attempt))
}
