package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/decisionpolicy"
	"github.com/yusefmosiah/go-choir/internal/hostbuild"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/updater"
)

// The release witness (S6-0, docs/definitions/choir-lp-s6-commit-gate-2026-10-20.md,
// docs/problems/s6-activation-trusts-not-compares-2026-10-20.md):
//
//	GET /api/computers/<id>/self-development/operations/<op>/release-witness
//
// joins, for any operation in any state, the decision, the frozen bundle, the
// declared test result, the host build, the applied manifest, the heads, the
// checkpoint, the route and the effective release, and names every pairwise
// join as compared at the activation gate or merely recorded.
//
// It is a projection of the tape, the operation row and the updater store. It
// is never an authority: nothing it returns is read back as input, it has no
// store of its own, it keeps no process-local state, and it writes nothing.
// The release draft's applied-only refusal (api_release.go) is unchanged.
//
// `compared_at_gate` is never set by the code that evaluates a pair. It is
// read from selfdevGatePairs, a constant table kept next to the gate it
// describes, and TestReleaseWitnessGateTableMatchesTheGateCode fails when the
// gate source changes without the table.
const releaseWitnessSchema = "choir.selfdev.release_witness.v1"

// witnessMaxHostBuildDirs bounds the staged host-build directories one read
// opens, so the route cannot be made to read an unbounded incoming/ store.
const witnessMaxHostBuildDirs = 256

// Where a pair is compared, when it is.
const (
	witnessAtDecision  = "decision"
	witnessAtFreeze    = "freeze"
	witnessAtMaterial  = "materializer"
	witnessAtHostBuild = "host_build"
	witnessAtUpdater   = "updater"
	witnessAtReducer   = "reducer"
	witnessAtCheckpt   = "checkpoint_authority"
	witnessAtRoute     = "route_cas"
	witnessAtNowhere   = "none"
)

// witnessAnchor ties a table row to the gate source. A row that says a pair is
// compared names a needle that must appear in the file; a row that says it is
// only recorded names a needle that must stay absent (or under a count), so a
// change that adds the compare fails the table test until the row is updated.
// Files are relative to this package directory.
type witnessAnchor struct {
	File   string
	Needle string
	Min    int // the needle must appear at least this many times
	Max    int // and, when Max >= 0 and Min == 0, at most this many
}

type selfdevGatePair struct {
	Pair       string
	Left       string
	Right      string
	Compared   bool
	ComparedAt string
	GateRef    string
	Note       string
	Anchors    []witnessAnchor
}

// selfdevGatePairs is the single statement of which joins the activation gate
// compares. "Gate" is any code path that can refuse before the release is
// activated: decision admission, the materializer, the host-build
// verification, the reducer and the updater's apply. Freeze is upstream and is
// reported as ComparedAt == "freeze" with Compared == false.
var selfdevGatePairs = []selfdevGatePair{
	{
		Pair: "decision_operation", Left: "decision event", Right: "operation row",
		Compared: true, ComparedAt: witnessAtDecision,
		GateRef: "self_development_decision_binding.go:verifyFinalizedSelfDevelopmentDecision",
		Note:    "operation identity, bundle digest, verifier ref, authority, receipt and projection",
		Anchors: []witnessAnchor{{File: "self_development_decision_binding.go", Needle: "decision binding: operation identity mismatch", Min: 1}},
	},
	{
		Pair: "decision_expected_heads", Left: "decision request expected heads", Right: "live head at decision",
		Compared: true, ComparedAt: witnessAtDecision,
		GateRef: "api_self_development.go:decideSelfDevelopment (head binding) and the append CAS",
		Note:    "the head at decision is not stored; a decision on the tape implies the append CAS and the reducer matched it",
		Anchors: []witnessAnchor{{File: "api_self_development.go", Needle: "decision head binding changed", Min: 1}},
	},
	{
		Pair: "bundle_operation", Left: "frozen bundle file", Right: "operation row",
		Compared: true, ComparedAt: witnessAtMaterial,
		GateRef: "self_development_materializer.go:materializeSelfDevelopmentOperation (bundle digest, Validate, identity)",
		Anchors: []witnessAnchor{
			{File: "self_development_materializer.go", Needle: "DigestBytes(rawBundle) != operation.BundleDigest", Min: 1},
			{File: "self_development_materializer.go", Needle: "selfDevelopmentBundleMatchesOperation(bundle, operation)", Min: 1},
		},
	},
	{
		Pair: "test_receipts_exit", Left: "test receipts' exit status", Right: "bundle's cited test receipts",
		Compared: false, ComparedAt: witnessAtFreeze,
		GateRef: "tools_capsule.go:freezeReceiptOutcomesError (freeze only)",
		Note:    "compared once at Freeze; the decision, the materializer and the updater never open a test receipt",
		Anchors: []witnessAnchor{
			{File: "tools_capsule.go", Needle: "freezeReceiptOutcomesError(buildRecipeRef", Min: 1},
			{File: "self_development_decision_binding.go", Needle: "ExitCode", Max: 0},
			{File: "self_development_decision_binding.go", Needle: "JobReceiptOutcomeRefusal", Max: 0},
			{File: "self_development_materializer.go", Needle: "ExitCode", Max: 0},
			{File: "self_development_materializer.go", Needle: "JobReceiptOutcomeRefusal", Max: 0},
			{File: "self_development_host_build.go", Needle: "JobReceiptOutcomeRefusal", Max: 0},
			{File: "api_self_development.go", Needle: "JobReceiptOutcomeRefusal", Max: 0},
		},
	},
	{
		Pair: "bundle_source_patch", Left: "source.patch bytes", Right: "bundle source_patch_sha256",
		Compared: true, ComparedAt: witnessAtHostBuild,
		GateRef: "self_development_host_build.go:selfDevelopmentHostBuild (readBoundSourcePatch)",
		Note:    "only on the host-build path; a frontend-only candidate has no source patch",
		Anchors: []witnessAnchor{{File: "self_development_host_build.go", Needle: "readBoundSourcePatch(", Min: 1}},
	},
	{
		Pair: "bundle_host_build", Left: "host job result", Right: "operation, bundle digest, patch, base, base image",
		Compared: true, ComparedAt: witnessAtHostBuild,
		GateRef: "self_development_host_build.go:selfDevelopmentHostBuild (hostbuild.VerifyJob)",
		Anchors: []witnessAnchor{{File: "self_development_host_build.go", Needle: "hostbuild.VerifyJob(", Min: 2}},
	},
	{
		Pair: "builder_receipt_patch", Left: "builder-receipt.json content", Right: "approved patch and base",
		Compared: false, ComparedAt: witnessAtNowhere,
		GateRef: "none on the guest (the host compares the builder's patch to the job, hostbuild/service.go)",
		Note:    "the guest hashes the receipt file against the signed result and never parses it",
		Anchors: []witnessAnchor{
			{File: "self_development_host_build.go", Needle: "builder.ClosureResult", Max: 0},
			{File: "self_development_materializer.go", Needle: "builder.ClosureResult", Max: 0},
			{File: "../updater/closure.go", Needle: "builder-receipt", Max: 0},
		},
	},
	{
		Pair: "manifest_closure_commit", Left: "applied manifest closure, code commit, base", Right: "host job result",
		Compared: true, ComparedAt: witnessAtUpdater,
		GateRef: "updater/closure.go:materializeReleaseClosure and verifyEntrypointBuildCommit; updater/runtime.go health identity",
		Anchors: []witnessAnchor{
			{File: "../updater/closure.go", Needle: "sum != manifest.ClosureDigest", Min: 1},
			{File: "../updater/closure.go", Needle: "verifyEntrypointBuildCommit(entry, manifest.CodeCommit)", Min: 1},
		},
	},
	{
		Pair: "manifest_builder_receipt", Left: "manifest builder_receipt_digest", Right: "host job result",
		Compared: false, ComparedAt: witnessAtNowhere,
		GateRef: "none (a by-construction copy in layeredSelfDevelopmentManifest)",
		Note:    "only the builder-receipt.json file entry is hashed; the manifest field is carried, not checked",
		Anchors: []witnessAnchor{
			{File: "../updater/closure.go", Needle: "BuilderReceiptDigest", Max: 0},
			{File: "../updater/runtime.go", Needle: "BuilderReceiptDigest", Max: 0},
			{File: "../updater/updater.go", Needle: "BuilderReceiptDigest", Max: 2},
		},
	},
	{
		Pair: "manifest_decision", Left: "applied manifest accepted head and digest", Right: "decision event and operation release digest",
		Compared: true, ComparedAt: witnessAtUpdater,
		GateRef: "updater/updater.go:validateApplyRequest; updater.ReadPinnedManifest",
		Anchors: []witnessAnchor{{File: "../updater/updater.go", Needle: "request.Manifest.AcceptedEventHead != request.AcceptedEventHead", Min: 1}},
	},
	{
		Pair: "decision_pending_transition", Left: "materialization events' decision ref", Right: "head's pending transition",
		Compared: true, ComparedAt: witnessAtReducer,
		GateRef: "computerevent/reducer.go:Reduce (materialization_started, materialization_applied)",
		Note:    "the same lock also keeps desired and effective heads fixed between decision and apply",
		Anchors: []witnessAnchor{{File: "../computerevent/reducer.go", Needle: "materialization does not bind pending transition", Min: 1}},
	},
	{
		Pair: "frozen_surface_decision_heads", Left: "surface the candidate was frozen against", Right: "surface the decision expected",
		Compared: false, ComparedAt: witnessAtNowhere,
		GateRef: "none (the decision compares the request to the live head, never to the candidate's pin)",
		Note:    "the start pin is overwritten at decision, so the witness cannot derive this join afterwards",
		Anchors: []witnessAnchor{
			{File: "api_self_development.go", Needle: "operation.BaseHead", Max: 2},
			{File: "self_development_decision_binding.go", Needle: "operation.BaseHead", Max: 0},
			// 1 bundle-to-operation join plus 2 reads in verificationPayloadNamesOperation,
			// which walks from the rollback's head to its target operation; neither
			// compares the frozen surface with the decision.
			{File: "self_development_materializer.go", Needle: "operation.BaseHead", Max: 3},
		},
	},
	{
		Pair: "checkpoint_event", Left: "operation checkpoint ref", Right: "checkpoint_published event on the tape",
		Compared: true, ComparedAt: witnessAtCheckpt,
		GateRef: "self_development_materializer.go:recordMaterializationApplied (PublishCheckpoint joins the certificate fields)",
		Anchors: []witnessAnchor{{File: "self_development_materializer.go", Needle: "selfdevControl.PublishCheckpoint(", Min: 1}},
	},
	{
		Pair: "route_event", Left: "operation route certificate", Right: "route_projection_updated event on the tape",
		Compared: true, ComparedAt: witnessAtRoute,
		GateRef: "self_development_materializer.go:recordMaterializationApplied (route CAS ExpectedGeneration)",
		Anchors: []witnessAnchor{{File: "self_development_materializer.go", Needle: "ExpectedGeneration: expectedGeneration", Min: 1}},
	},
	{
		Pair: "serving_current", Left: "serving release digest", Right: "current release digest",
		Compared: true, ComparedAt: witnessAtUpdater,
		GateRef: "updater/runtime.go health identity probe (release digest, marker, schema, build commit)",
		Anchors: []witnessAnchor{{File: "../updater/runtime.go", Needle: "health.ReleaseDigest != releaseDigest", Min: 1}},
	},
	{
		Pair: "effective_release_operation", Left: "effective release", Right: "applied operation's release",
		Compared: true, ComparedAt: witnessAtUpdater,
		GateRef: "updater/runtime.go health identity probe, at apply",
		Note:    "compared once at apply; nothing compares it afterwards, so a later release or a restore makes this unknown, not bound",
		Anchors: []witnessAnchor{{File: "../updater/runtime.go", Needle: "health.ReleaseDigest != releaseDigest", Min: 1}},
	},
	{
		Pair: "divergence_derived", Left: "effective release", Right: "platform release",
		Compared: false, ComparedAt: witnessAtNowhere,
		GateRef: "none (vmctl DivergenceStatus is a manual flag; no divergent_release refusal exists)",
		Anchors: []witnessAnchor{
			{File: "platform_update.go", Needle: "divergent_release", Max: 0},
			{File: "self_development_materializer.go", Needle: "divergent_release", Max: 0},
		},
	},
}

// WitnessBinding is one pairwise join of the witness.
type WitnessBinding struct {
	Pair           string `json:"pair"`
	Left           string `json:"left"`
	Right          string `json:"right"`
	ComparedAtGate bool   `json:"compared_at_gate"`
	ComparedAt     string `json:"compared_at"`
	GateRef        string `json:"gate_ref"`
	// Bound: true when the two recorded sides agree now, false when they
	// disagree, null when the witness cannot derive the join (the reason says
	// why). It is the witness's observation; compared_at_gate says whether a
	// gate would have refused a mismatch.
	Bound  *bool  `json:"bound"`
	Reason string `json:"reason"`
	Note   string `json:"note,omitempty"`
}

type WitnessSummary struct {
	Pairs          int `json:"pairs"`
	ComparedAtGate int `json:"compared_at_gate"`
	RecordedOnly   int `json:"recorded_only"`
	Bound          int `json:"bound"`
	Unbound        int `json:"unbound"`
	Unknown        int `json:"unknown"`
	// RecordedOnlyUnbound counts joins no gate compares and that do not
	// currently agree: the number S6 exists to drive to zero.
	RecordedOnlyUnbound int `json:"recorded_only_unbound"`
}

type WitnessOperation struct {
	State           string   `json:"state"`
	TrajectoryID    string   `json:"trajectory_id"`
	CapsuleID       string   `json:"capsule_id,omitempty"`
	BaseHead        string   `json:"base_head"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
	TerminalError   string   `json:"terminal_error,omitempty"`
	LastIntentError string   `json:"last_intent_error,omitempty"`
	VerifierRefs    []string `json:"verifier_refs"`
}

type WitnessDecision struct {
	Present                    bool   `json:"present"`
	EventKind                  string `json:"event_kind,omitempty"`
	EventDigest                string `json:"event_digest,omitempty"`
	ReceiptID                  string `json:"receipt_id,omitempty"`
	AuthorityKind              string `json:"authority_kind,omitempty"`
	ModeReceipt                string `json:"mode_receipt,omitempty"`
	ExpectedDesiredEventHead   string `json:"expected_desired_event_head,omitempty"`
	ExpectedEffectiveEventHead string `json:"expected_effective_event_head,omitempty"`
	// The rest of the decision-time binding: with the expected heads and the
	// caller's own key, reason and bundle it is what a same-key replay of
	// the decision has to carry (the decision moved the current head).
	ExpectedPendingTransitionRef     string `json:"expected_pending_transition_ref,omitempty"`
	ExpectedDesiredStateCommitment   string `json:"expected_desired_state_commitment,omitempty"`
	ExpectedEffectiveStateCommitment string `json:"expected_effective_state_commitment,omitempty"`
	NextDesiredEventHead             string `json:"next_desired_event_head,omitempty"`
	NextEffectiveEventHead           string `json:"next_effective_event_head,omitempty"`
	// GateVerified is the result of the gate's own verifier
	// (verifyFinalizedSelfDevelopmentDecision) run on the stored decision.
	GateVerified *bool  `json:"gate_verified"`
	GateError    string `json:"gate_error,omitempty"`
	Error        string `json:"error,omitempty"`
}

type WitnessBundle struct {
	Retained          bool     `json:"retained"`
	Final             bool     `json:"final"`
	Digest            string   `json:"digest,omitempty"`
	Bound             *bool    `json:"bound"`
	Valid             *bool    `json:"valid"`
	ValidationError   string   `json:"validation_error,omitempty"`
	ContentDigest     string   `json:"content_digest,omitempty"`
	BaseEventHead     string   `json:"base_event_head,omitempty"`
	SourcePatchSHA256 string   `json:"source_patch_sha256,omitempty"`
	SourcePatchBase   string   `json:"source_patch_base_commit,omitempty"`
	TestReceiptRefs   []string `json:"test_receipt_refs"`
	RuntimeFileCount  int      `json:"runtime_file_count"`
	BinaryInputCount  int      `json:"binary_input_count"`
	Error             string   `json:"error,omitempty"`
}

type WitnessTestReceipt struct {
	Ref           string `json:"ref"`
	Resolved      bool   `json:"resolved"`
	ExitCode      *int   `json:"exit_code"`
	JobState      string `json:"job_state,omitempty"`
	CommandSHA256 string `json:"command_sha256,omitempty"`
	// OutcomeRefusal is capsule.JobReceiptOutcomeRefusal, the function Freeze
	// refuses with, run on the receipt as it reads now.
	OutcomeRefusal string `json:"outcome_refusal,omitempty"`
	Error          string `json:"error,omitempty"`
}

type WitnessTestResult struct {
	Receipts []WitnessTestReceipt `json:"receipts"`
	AllOK    *bool                `json:"all_ok"`
	Note     string               `json:"note"`
}

type WitnessHostBuild struct {
	Required                bool   `json:"required"`
	Found                   bool   `json:"found"`
	GateVerified            *bool  `json:"gate_verified"`
	JobID                   string `json:"job_id,omitempty"`
	State                   string `json:"state,omitempty"`
	OperationID             string `json:"operation_id,omitempty"`
	BundleDigest            string `json:"bundle_digest,omitempty"`
	SourcePatchSHA256       string `json:"source_patch_sha256,omitempty"`
	BaseCommit              string `json:"base_commit,omitempty"`
	BaseImageManifestDigest string `json:"base_image_manifest_digest,omitempty"`
	CodeCommit              string `json:"code_commit,omitempty"`
	ClosureDigest           string `json:"closure_digest,omitempty"`
	BuilderReceiptDigest    string `json:"builder_receipt_digest,omitempty"`
	// BuilderReceipt is what builder-receipt.json itself says, parsed by the
	// witness (no gate parses it).
	BuilderReceiptSourcePatchSHA256 string `json:"builder_receipt_source_patch_sha256,omitempty"`
	BuilderReceiptBaseCommit        string `json:"builder_receipt_base_commit,omitempty"`
	BuilderReceiptCodeCommit        string `json:"builder_receipt_code_commit,omitempty"`
	Candidates                      int    `json:"candidates"`
	Error                           string `json:"error,omitempty"`
}

type WitnessApplied struct {
	ReleaseDigest           string `json:"release_digest,omitempty"`
	ManifestPresent         bool   `json:"manifest_present"`
	Marker                  string `json:"marker,omitempty"`
	AcceptedEventHead       string `json:"accepted_event_head,omitempty"`
	Layered                 bool   `json:"layered"`
	CodeCommit              string `json:"code_commit,omitempty"`
	BaseCommit              string `json:"base_commit,omitempty"`
	ClosureDigest           string `json:"closure_digest,omitempty"`
	BuilderReceiptDigest    string `json:"builder_receipt_digest,omitempty"`
	BaseImageManifestDigest string `json:"base_image_manifest_digest,omitempty"`
	FileCount               int    `json:"file_count"`
	MaterializationReceipt  string `json:"materialization_receipt,omitempty"`
	AppliedEventPresent     bool   `json:"applied_event_present"`
	AppliedEventDecisionRef string `json:"applied_event_decision_ref,omitempty"`
	Error                   string `json:"error,omitempty"`
}

type WitnessHeads struct {
	OperationBaseHead      string              `json:"operation_base_head"`
	OperationDesiredHead   string              `json:"operation_desired_head"`
	OperationEffectiveHead string              `json:"operation_effective_head"`
	Live                   *EffectiveEventHead `json:"live,omitempty"`
	LivePendingTransition  string              `json:"live_pending_transition,omitempty"`
	Error                  string              `json:"error,omitempty"`
}

type WitnessCheckpoint struct {
	Ref              string `json:"ref,omitempty"`
	EventPresent     bool   `json:"event_present"`
	EventProposedRef string `json:"event_proposed_ref,omitempty"`
	EventDecisionRef string `json:"event_decision_ref,omitempty"`
}

type WitnessRoute struct {
	Certificate        string  `json:"certificate,omitempty"`
	Generation         *uint64 `json:"generation,omitempty"`
	Receipt            string  `json:"receipt,omitempty"`
	CodeRef            string  `json:"code_ref,omitempty"`
	ArtifactProgramRef string  `json:"artifact_program_ref,omitempty"`
	EventPresent       bool    `json:"event_present"`
	EventProposedRef   string  `json:"event_proposed_ref,omitempty"`
	EventDecisionRef   string  `json:"event_decision_ref,omitempty"`
}

// ReleaseWitness is the answer of the release-witness route.
type ReleaseWitness struct {
	Schema      string `json:"schema"`
	Authority   string `json:"authority"`
	ComputerID  string `json:"computer_id"`
	OperationID string `json:"operation_id"`
	ReadAt      string `json:"read_at"`
	// ReadConsistent is false when the operation row moved while the witness
	// read the other stores and the read could not be repeated to a stable
	// answer (a materialization in flight): the sections may describe two
	// moments.
	ReadConsistent   bool              `json:"read_consistent"`
	Operation        WitnessOperation  `json:"operation"`
	Decision         WitnessDecision   `json:"decision"`
	Bundle           WitnessBundle     `json:"bundle"`
	TestResult       WitnessTestResult `json:"test_result"`
	HostBuild        WitnessHostBuild  `json:"host_build"`
	Applied          WitnessApplied    `json:"applied"`
	Heads            WitnessHeads      `json:"heads"`
	Checkpoint       WitnessCheckpoint `json:"checkpoint"`
	Route            WitnessRoute      `json:"route"`
	EffectiveRelease *EffectiveRelease `json:"effective_release,omitempty"`
	Bindings         []WitnessBinding  `json:"bindings"`
	Summary          WitnessSummary    `json:"summary"`
}

// witnessBindings collects the observations. It can only attach an
// observation to a pair that exists in selfdevGatePairs; compared_at_gate
// comes from the table and is not settable here.
type witnessBindings struct {
	bound   map[string]*bool
	reasons map[string]string
}

func newWitnessBindings() *witnessBindings {
	return &witnessBindings{bound: map[string]*bool{}, reasons: map[string]string{}}
}

func witnessBoolPtr(v bool) *bool { return &v }

func (b *witnessBindings) observe(pair string, bound *bool, reason string) {
	found := false
	for _, row := range selfdevGatePairs {
		if row.Pair == pair {
			found = true
			break
		}
	}
	if !found {
		panic("release witness: observation for a pair that is not in selfdevGatePairs: " + pair)
	}
	b.bound[pair], b.reasons[pair] = bound, reason
}

func (b *witnessBindings) unknown(pair, reason string) { b.observe(pair, nil, reason) }

func (b *witnessBindings) list() ([]WitnessBinding, WitnessSummary) {
	out := make([]WitnessBinding, 0, len(selfdevGatePairs))
	var sum WitnessSummary
	for _, row := range selfdevGatePairs {
		bound, reason := b.bound[row.Pair], b.reasons[row.Pair]
		if _, seen := b.reasons[row.Pair]; !seen {
			reason = "not evaluated"
		}
		out = append(out, WitnessBinding{
			Pair: row.Pair, Left: row.Left, Right: row.Right, ComparedAtGate: row.Compared, ComparedAt: row.ComparedAt,
			GateRef: row.GateRef, Bound: bound, Reason: reason, Note: row.Note,
		})
		sum.Pairs++
		if row.Compared {
			sum.ComparedAtGate++
		} else {
			sum.RecordedOnly++
		}
		switch {
		case bound == nil:
			sum.Unknown++
		case *bound:
			sum.Bound++
		default:
			sum.Unbound++
			if !row.Compared {
				sum.RecordedOnlyUnbound++
			}
		}
	}
	return out, sum
}

// witnessOpenExecutionReceipt opens an execution receipt by reference. A test
// replaces it; production reads the capsule executor's receipt store.
var witnessOpenExecutionReceipt = func(rt *Runtime, ref string) (capsule.ExecutionReceipt, error) {
	if rt == nil || rt.capsuleExecutor == nil {
		return capsule.ExecutionReceipt{}, errWitnessNoExecutor
	}
	return rt.capsuleExecutor.OpenExecutionReceipt(ref)
}

var errWitnessNoExecutor = witnessError("no capsule executor on this runtime")

type witnessError string

func (e witnessError) Error() string { return string(e) }

// releaseWitness serves the route. Reads only.
func (h *APIHandler) releaseWitness(w http.ResponseWriter, r *http.Request, computerID, operationID string) {
	operation, err := h.rt.selfdevOperations.Get(r.Context(), computerID, operationID)
	if err != nil {
		writeAPIJSON(w, http.StatusNotFound, apiError{Error: "self-development operation not found"})
		return
	}
	witness := h.rt.buildReleaseWitness(r.Context(), operation)
	w.Header().Set("Cache-Control", "no-store")
	writeAPIJSON(w, http.StatusOK, witness)
}

// buildReleaseWitness reads the operation's joins, repeating the read when the
// operation row moved underneath it.
func (rt *Runtime) buildReleaseWitness(ctx context.Context, operation selfdev.Operation) ReleaseWitness {
	var witness ReleaseWitness
	consistent := false
	for attempt := 0; attempt < 3; attempt++ {
		witness = rt.readReleaseWitness(ctx, operation)
		after, err := rt.selfdevOperations.Get(ctx, operation.ComputerID, operation.OperationID)
		if err == nil && after.State == operation.State && after.UpdatedAt == operation.UpdatedAt {
			consistent = true
			break
		}
		if err != nil {
			break
		}
		operation = after
	}
	witness.ReadConsistent = consistent
	return witness
}

func (rt *Runtime) readReleaseWitness(ctx context.Context, operation selfdev.Operation) ReleaseWitness {
	bindings := newWitnessBindings()
	out := ReleaseWitness{
		Schema:      releaseWitnessSchema,
		Authority:   "projection: the tape, the operation row and the updater store are the authority; this answer is never read back as input",
		ComputerID:  operation.ComputerID,
		OperationID: operation.OperationID,
		ReadAt:      time.Now().UTC().Format(time.RFC3339Nano),
		Operation: WitnessOperation{
			State: operation.State, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID, BaseHead: operation.BaseHead,
			CreatedAt: operation.CreatedAt, UpdatedAt: operation.UpdatedAt, TerminalError: witnessTruncate(operation.TerminalError, 300),
			LastIntentError: witnessTruncate(operation.LastIntentError, 300), VerifierRefs: append([]string{}, operation.VerifierRefs...),
		},
	}

	consensus := rt.witnessDecision(ctx, operation, &out, bindings)
	bundle, bundleOK := rt.witnessBundle(operation, &out, bindings)
	rt.witnessTestResult(bundle, bundleOK, &out, bindings)
	rt.witnessHostBuild(operation, bundle, bundleOK, consensus, &out, bindings)
	rt.witnessApplied(ctx, operation, &out, bindings)
	rt.witnessHeads(ctx, operation, &out)
	rt.witnessCheckpointAndRoute(ctx, operation, &out, bindings)

	effective := rt.EffectiveRelease(ctx)
	out.EffectiveRelease = &effective
	rt.witnessEffective(ctx, operation, effective, &out, bindings)

	out.Bindings, out.Summary = bindings.list()
	return out
}

// witnessDecision reads the decision and runs the gate's own verifier on it.
// It reports whether a consensus decision accepted the operation (which makes
// the host build mandatory).
func (rt *Runtime) witnessDecision(ctx context.Context, operation selfdev.Operation, out *ReleaseWitness, b *witnessBindings) (consensus bool) {
	if rt.store == nil {
		out.Decision.Error = "event store unavailable"
		b.unknown("decision_operation", "event store unavailable")
		b.unknown("decision_expected_heads", "event store unavailable")
		return false
	}
	transition, found, err := rt.store.FinalizedDecisionForOperation(ctx, operation.ComputerID, operation.OperationID, operation.TrajectoryID, operation.CapsuleID)
	if err != nil {
		out.Decision.Error = "decision unreadable: " + witnessTruncate(err.Error(), 200)
		b.unknown("decision_operation", out.Decision.Error)
		b.unknown("decision_expected_heads", out.Decision.Error)
		return false
	}
	if !found {
		reason := "no finalized decision on the tape for this operation"
		b.unknown("decision_operation", reason)
		b.unknown("decision_expected_heads", reason)
		return false
	}
	event := transition.Request.Event
	out.Decision = WitnessDecision{
		Present: true, EventKind: string(event.EventKind), EventDigest: transition.Request.EventDigest, ReceiptID: transition.Receipt.ReceiptID,
		ExpectedDesiredEventHead: event.ExpectedDesiredEventHead, ExpectedEffectiveEventHead: event.ExpectedEffectiveEventHead,
		ExpectedPendingTransitionRef:     event.ExpectedPendingTransitionRef,
		ExpectedDesiredStateCommitment:   event.ExpectedDesiredStateCommitment,
		ExpectedEffectiveStateCommitment: event.ExpectedEffectiveStateCommitment,
		NextDesiredEventHead:             transition.Request.Next.DesiredEventHead, NextEffectiveEventHead: transition.Request.Next.EffectiveEventHead,
	}
	if _, isConsensus := decisionpolicy.ReceiptDigestFromAuthority(event.AuthorityRef); isConsensus {
		consensus = true
	}
	// The gate's verifier compares the operation's state-dependent head
	// projection against the heads right after the decision; once the
	// operation has materialized those legitimately differ. Run it as the
	// decision-time operation (state awaiting_approval) so it compares the
	// identity, authority, verifier and recorded decision fields, and compare
	// the desired head separately below.
	decisionTime := operation
	decisionTime.State = selfdev.StateAwaitingApproval
	verified, verifyErr := verifyFinalizedSelfDevelopmentDecision(decisionTime, transition)
	if verifyErr != nil {
		out.Decision.GateVerified, out.Decision.GateError = witnessBoolPtr(false), verifyErr.Error()
		b.observe("decision_operation", witnessBoolPtr(false), verifyErr.Error())
	} else {
		out.Decision.GateVerified = witnessBoolPtr(true)
		out.Decision.AuthorityKind, out.Decision.ModeReceipt = verified.AuthorityKind, verified.ModeReceiptDigest
		reason := "the gate's verifier accepts the stored decision for this operation"
		bound := true
		if operation.DesiredHead != "" && operation.DesiredHead != transition.Request.Next.DesiredEventHead && operation.State != selfdev.StateAwaitingApproval {
			bound, reason = false, "the operation's desired head is not the head the decision produced"
		}
		b.observe("decision_operation", witnessBoolPtr(bound), reason)
	}
	// Expected heads: the append CAS and the reducer refuse a decision whose
	// expected heads differ from the head, so a decision on the tape whose
	// recorded expectations are well formed was matched at append.
	wellFormed := computerevent.IsSHA256(event.ExpectedDesiredEventHead) && computerevent.IsSHA256(event.ExpectedEffectiveEventHead) &&
		computerevent.IsSHA256(event.ExpectedDesiredStateCommitment) && computerevent.IsSHA256(event.ExpectedEffectiveStateCommitment)
	if wellFormed {
		b.observe("decision_expected_heads", witnessBoolPtr(true), "the decision is on the tape, so the append matched its expected heads to the head then (the head at decision is not stored)")
	} else {
		b.observe("decision_expected_heads", witnessBoolPtr(false), "the decision event carries malformed expected heads")
	}
	return consensus
}

func (rt *Runtime) witnessBundle(operation selfdev.Operation, out *ReleaseWitness, b *witnessBindings) (transaction.CapsuleEffectBundle, bool) {
	out.Bundle.TestReceiptRefs = []string{}
	if strings.TrimSpace(rt.selfdevUpdaterRoot) == "" || !computerevent.IsSHA256(operation.BundleDigest) {
		out.Bundle.Error = "no frozen bundle digest on the operation yet, or no updater store"
		b.unknown("bundle_operation", out.Bundle.Error)
		return transaction.CapsuleEffectBundle{}, false
	}
	out.Bundle.Digest = operation.BundleDigest
	bundleRoot := filepath.Join(rt.selfdevUpdaterRoot, "incoming", operation.BundleDigest)
	bundle, _, err := readBoundSelfDevelopmentBundle(bundleRoot, operation)
	if err != nil {
		out.Bundle.Error = err.Error()
		if _, statErr := os.Lstat(bundleRoot); statErr != nil {
			b.unknown("bundle_operation", "the frozen bundle is not retained on this computer")
		} else {
			out.Bundle.Retained = true
			out.Bundle.Bound = witnessBoolPtr(false)
			b.observe("bundle_operation", witnessBoolPtr(false), err.Error())
		}
		return transaction.CapsuleEffectBundle{}, false
	}
	_, finalErr := os.Lstat(filepath.Join(bundleRoot, "bundle.json"))
	final := finalErr == nil
	out.Bundle.Retained, out.Bundle.Final, out.Bundle.Bound = true, final, witnessBoolPtr(true)
	out.Bundle.ContentDigest, out.Bundle.BaseEventHead = bundle.ContentDigest, bundle.BaseEventHead
	out.Bundle.SourcePatchSHA256, out.Bundle.SourcePatchBase = bundle.SourcePatchSHA256, bundle.SourcePatchBaseCommit
	out.Bundle.TestReceiptRefs = append([]string{}, bundle.TestReceipts...)
	out.Bundle.RuntimeFileCount = len(bundle.RuntimeFiles)
	out.Bundle.BinaryInputCount = len(selfDevelopmentBundleBinaryInputs(bundle))
	if validateErr := bundle.Validate(final); validateErr != nil {
		out.Bundle.Valid, out.Bundle.ValidationError = witnessBoolPtr(false), validateErr.Error()
		b.observe("bundle_operation", witnessBoolPtr(false), "the bundle file hashes to the operation's digest and names its identity, but the gate's validation refuses it: "+validateErr.Error())
	} else {
		out.Bundle.Valid = witnessBoolPtr(true)
		b.observe("bundle_operation", witnessBoolPtr(true), "the bundle file hashes to the operation's digest, names its identity and validates")
	}
	// bundle_source_patch: the gate reads the patch through readBoundSourcePatch.
	if bundle.SourcePatchSHA256 == "" {
		b.unknown("bundle_source_patch", "the bundle carries no source patch (a frontend-only candidate)")
	} else if _, status, patchErr := readBoundSourcePatch(filepath.Join(bundleRoot, "source.patch"), bundle.SourcePatchSHA256); patchErr != nil {
		if status == http.StatusGone {
			b.unknown("bundle_source_patch", patchErr.Error())
		} else {
			b.observe("bundle_source_patch", witnessBoolPtr(false), patchErr.Error())
		}
	} else {
		b.observe("bundle_source_patch", witnessBoolPtr(true), "source.patch hashes to the bundle's source_patch_sha256")
	}
	return bundle, true
}

// witnessTestResult opens each cited test receipt and applies the same
// outcome function Freeze refuses with.
func (rt *Runtime) witnessTestResult(bundle transaction.CapsuleEffectBundle, bundleOK bool, out *ReleaseWitness, b *witnessBindings) {
	out.TestResult = WitnessTestResult{
		Receipts: []WitnessTestReceipt{},
		Note:     "exit status is compared at Freeze only; a bundle that froze cites receipts that were successful then",
	}
	if !bundleOK {
		b.unknown("test_receipts_exit", "no bundle to read the cited test receipts from")
		return
	}
	allOK, anyUnresolved := true, false
	for _, ref := range bundle.TestReceipts {
		entry := WitnessTestReceipt{Ref: ref}
		receipt, err := witnessOpenExecutionReceipt(rt, ref)
		if err != nil {
			entry.Error = witnessTruncate(err.Error(), 200)
			anyUnresolved = true
		} else {
			code := receipt.ExitCode
			entry.Resolved, entry.ExitCode, entry.JobState = true, &code, receipt.JobState
			entry.CommandSHA256 = computerevent.DigestBytes([]byte(receipt.Command))
			entry.OutcomeRefusal = capsule.JobReceiptOutcomeRefusal(receipt, true)
			if entry.OutcomeRefusal != "" {
				allOK = false
			}
		}
		out.TestResult.Receipts = append(out.TestResult.Receipts, entry)
	}
	switch {
	case len(bundle.TestReceipts) == 0:
		b.unknown("test_receipts_exit", "the bundle cites no test receipt")
	case anyUnresolved && allOK:
		b.unknown("test_receipts_exit", "a cited test receipt cannot be opened now (the executor's receipt store is guest-local)")
	case !allOK:
		out.TestResult.AllOK = witnessBoolPtr(false)
		b.observe("test_receipts_exit", witnessBoolPtr(false), "a cited test receipt records a failure now; Freeze would have refused it, and no later gate compares it")
	default:
		out.TestResult.AllOK = witnessBoolPtr(true)
		b.observe("test_receipts_exit", witnessBoolPtr(true), "every cited test receipt records a success; this was compared at Freeze only")
	}
}

type witnessBuilderReceipt struct {
	SourcePatchSHA256     string `json:"source_patch_sha256"`
	SourcePatchBaseCommit string `json:"source_patch_base_commit"`
	CodeCommit            string `json:"code_commit"`
}

func (rt *Runtime) witnessHostBuild(operation selfdev.Operation, bundle transaction.CapsuleEffectBundle, bundleOK, consensus bool, out *ReleaseWitness, b *witnessBindings) {
	if !bundleOK {
		b.unknown("bundle_host_build", "no bundle")
		b.unknown("builder_receipt_patch", "no bundle")
		return
	}
	out.HostBuild.Required = consensus || len(selfDevelopmentBundleBinaryInputs(bundle)) > 0
	if !out.HostBuild.Required {
		reason := "no host build on this path: a frontend-only candidate applies as the capsule's files (a consensus decision would require one)"
		b.unknown("bundle_host_build", reason)
		b.unknown("builder_receipt_patch", reason)
		return
	}
	incoming := filepath.Join(rt.selfdevUpdaterRoot, "incoming")
	wantDir := ""
	var gateRequest *hostbuild.Request
	if request, err := rt.selfDevelopmentHostBuildRequest(operation, bundle); err == nil {
		if jobID, idErr := request.JobID(); idErr == nil {
			wantDir = "host-build-" + jobID
			gateRequest = &request
		}
	}
	entries, _ := os.ReadDir(incoming)
	type candidate struct {
		dir string
		job hostbuild.Job
	}
	var matches []candidate
	scanned := 0
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "host-build-") {
			continue
		}
		if scanned++; scanned > witnessMaxHostBuildDirs {
			break
		}
		raw, err := os.ReadFile(filepath.Join(incoming, entry.Name(), "host-build.json"))
		if err != nil {
			continue
		}
		var job hostbuild.Job
		if json.Unmarshal(raw, &job) != nil || job.OperationID != operation.OperationID || job.BundleDigest != operation.BundleDigest {
			continue
		}
		matches = append(matches, candidate{dir: entry.Name(), job: job})
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].dir < matches[j].dir })
	out.HostBuild.Candidates = len(matches)
	if len(matches) == 0 {
		reason := "a host build is required, and none is staged on this computer (not built yet, failed, or pruned)"
		b.unknown("bundle_host_build", reason)
		b.unknown("builder_receipt_patch", reason)
		return
	}
	chosen := matches[0]
	for _, m := range matches {
		if m.dir == wantDir {
			chosen = m
			break
		}
	}
	out.HostBuild.Found = true
	job := chosen.job
	out.HostBuild.JobID, out.HostBuild.State = job.JobID, job.State
	out.HostBuild.OperationID, out.HostBuild.BundleDigest = job.OperationID, job.BundleDigest
	out.HostBuild.SourcePatchSHA256, out.HostBuild.BaseCommit, out.HostBuild.BaseImageManifestDigest = job.SourcePatchSHA256, job.BaseCommit, job.BaseImageManifestDigest
	if job.Result != nil {
		out.HostBuild.CodeCommit, out.HostBuild.ClosureDigest, out.HostBuild.BuilderReceiptDigest = job.Result.CodeCommit, job.Result.ClosureDigest, job.Result.BuilderReceiptDigest
	}
	// bundle_host_build: the gate's own staged-build check.
	switch {
	case gateRequest == nil || rt.selfdevControl == nil:
		b.unknown("bundle_host_build", "the gate's request identity or platform key is unavailable here (the booted image manifest is unreadable or the control credential is absent)")
	case chosen.dir != wantDir:
		out.HostBuild.GateVerified = witnessBoolPtr(false)
		b.observe("bundle_host_build", witnessBoolPtr(false), "the staged host build is not the build of this request's job id (the base image may have moved)")
	default:
		if _, ok := readStagedHostBuild(filepath.Join(incoming, chosen.dir), *gateRequest, rt.selfdevControl.PublicKey()); ok {
			out.HostBuild.GateVerified = witnessBoolPtr(true)
			b.observe("bundle_host_build", witnessBoolPtr(true), "the gate's staged-build check (hostbuild.VerifyJob and the file hashes) accepts the staged build for this request")
		} else {
			out.HostBuild.GateVerified = witnessBoolPtr(false)
			b.observe("bundle_host_build", witnessBoolPtr(false), "the gate's staged-build check refuses the staged build for this request")
		}
	}
	// builder_receipt_patch: what builder-receipt.json itself says, which no
	// gate parses.
	raw, err := os.ReadFile(filepath.Join(incoming, chosen.dir, "builder-receipt.json"))
	if err != nil {
		b.unknown("builder_receipt_patch", "builder-receipt.json is not retained")
		return
	}
	var receipt witnessBuilderReceipt
	if json.Unmarshal(raw, &receipt) != nil {
		b.observe("builder_receipt_patch", witnessBoolPtr(false), "builder-receipt.json does not parse")
		return
	}
	out.HostBuild.BuilderReceiptSourcePatchSHA256, out.HostBuild.BuilderReceiptBaseCommit, out.HostBuild.BuilderReceiptCodeCommit = receipt.SourcePatchSHA256, receipt.SourcePatchBaseCommit, receipt.CodeCommit
	switch {
	case receipt.SourcePatchSHA256 != bundle.SourcePatchSHA256 || !strings.EqualFold(receipt.SourcePatchBaseCommit, bundle.SourcePatchBaseCommit):
		b.observe("builder_receipt_patch", witnessBoolPtr(false), "the builder receipt names a different patch or base than the approved bundle")
	case job.Result != nil && receipt.CodeCommit != job.Result.CodeCommit:
		b.observe("builder_receipt_patch", witnessBoolPtr(false), "the builder receipt's code commit differs from the host job result's")
	default:
		b.observe("builder_receipt_patch", witnessBoolPtr(true), "the builder receipt names the approved patch and base (read by the witness; no gate reads it)")
	}
}

func (rt *Runtime) witnessApplied(ctx context.Context, operation selfdev.Operation, out *ReleaseWitness, b *witnessBindings) {
	out.Applied.ReleaseDigest = operation.ReleaseDigest
	out.Applied.MaterializationReceipt = operation.MaterializationReceipt
	notApplied := "the operation has no applied release"
	if !computerevent.IsSHA256(operation.ReleaseDigest) || strings.TrimSpace(rt.selfdevUpdaterRoot) == "" {
		b.unknown("manifest_decision", notApplied)
		b.unknown("manifest_closure_commit", notApplied)
		b.unknown("manifest_builder_receipt", notApplied)
	} else if manifest, _, err := updater.ReadPinnedManifest(rt.selfdevUpdaterRoot, operation.ReleaseDigest); err != nil {
		out.Applied.Error = "the pinned manifest is unavailable: " + witnessTruncate(err.Error(), 160)
		b.unknown("manifest_decision", out.Applied.Error)
		b.unknown("manifest_closure_commit", out.Applied.Error)
		b.unknown("manifest_builder_receipt", out.Applied.Error)
	} else {
		out.Applied.ManifestPresent, out.Applied.Marker, out.Applied.AcceptedEventHead = true, manifest.Marker, manifest.AcceptedEventHead
		out.Applied.Layered = manifest.LayeringEntrypoint != "" && manifest.ClosureDigest != ""
		out.Applied.CodeCommit, out.Applied.BaseCommit, out.Applied.ClosureDigest = manifest.CodeCommit, manifest.BaseCommit, manifest.ClosureDigest
		out.Applied.BuilderReceiptDigest, out.Applied.BaseImageManifestDigest = manifest.BuilderReceiptDigest, manifest.BaseImageManifestDigest
		out.Applied.FileCount = len(manifest.Files)
		if operation.DecisionEvent == "" {
			b.unknown("manifest_decision", "the operation records no decision event")
		} else if manifest.AcceptedEventHead == operation.DecisionEvent {
			b.observe("manifest_decision", witnessBoolPtr(true), "the manifest's accepted head is the decision event, and its content digest is the operation's release digest")
		} else {
			b.observe("manifest_decision", witnessBoolPtr(false), "the manifest's accepted head is not the operation's decision event")
		}
		rt.witnessManifestVsHostBuild(manifest, out, b)
	}
	// The applied event on the tape.
	if rt.store != nil && operation.DecisionEvent != "" {
		event, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, "selfdev-"+string(computerevent.EventMaterializationApplied)+"-"+operation.DecisionEvent)
		if err == nil && found {
			out.Applied.AppliedEventPresent, out.Applied.AppliedEventDecisionRef = true, event.DecisionRef
		}
	}
	switch {
	case !out.Applied.AppliedEventPresent:
		b.unknown("decision_pending_transition", "no materialization_applied event for this decision on the tape (not applied, or a rollback operation)")
	case out.Applied.AppliedEventDecisionRef == operation.DecisionEvent:
		b.observe("decision_pending_transition", witnessBoolPtr(true), "the materialization_applied event binds the decision as the pending transition")
	default:
		b.observe("decision_pending_transition", witnessBoolPtr(false), "the materialization_applied event binds a different decision")
	}
	// frozen_surface_decision_heads is the recorded-only join the witness
	// cannot derive: the start pin is overwritten at decision.
	b.unknown("frozen_surface_decision_heads", "not derivable: the candidate's start pin is overwritten at decision, and no gate compares it")
}

func (rt *Runtime) witnessManifestVsHostBuild(manifest updater.ReleaseManifest, out *ReleaseWitness, b *witnessBindings) {
	if !out.HostBuild.Found || out.HostBuild.ClosureDigest == "" {
		reason := "no staged host build to compare the manifest with (plain release, or the build is not retained)"
		b.unknown("manifest_closure_commit", reason)
		b.unknown("manifest_builder_receipt", reason)
		return
	}
	if manifest.ClosureDigest == out.HostBuild.ClosureDigest && manifest.CodeCommit == out.HostBuild.CodeCommit &&
		manifest.BaseCommit == out.HostBuild.BaseCommit && manifest.BaseImageManifestDigest == out.HostBuild.BaseImageManifestDigest {
		b.observe("manifest_closure_commit", witnessBoolPtr(true), "the manifest's closure digest, code commit, base commit and base image are the host job result's")
	} else {
		b.observe("manifest_closure_commit", witnessBoolPtr(false), "the manifest differs from the host job result in closure, code commit, base commit or base image")
	}
	if manifest.BuilderReceiptDigest == out.HostBuild.BuilderReceiptDigest {
		b.observe("manifest_builder_receipt", witnessBoolPtr(true), "the manifest's builder receipt digest is the host job result's (a by-construction copy)")
	} else {
		b.observe("manifest_builder_receipt", witnessBoolPtr(false), "the manifest's builder receipt digest differs from the host job result's")
	}
}

func (rt *Runtime) witnessHeads(ctx context.Context, operation selfdev.Operation, out *ReleaseWitness) {
	out.Heads = WitnessHeads{OperationBaseHead: operation.BaseHead, OperationDesiredHead: operation.DesiredHead, OperationEffectiveHead: operation.EffectiveHead}
	if rt.store == nil {
		out.Heads.Error = "event store unavailable"
		return
	}
	head, err := rt.store.Head(ctx, operation.ComputerID)
	if err != nil || head == nil {
		out.Heads.Error = "live head unreadable"
		return
	}
	out.Heads.Live = &EffectiveEventHead{Sequence: head.Sequence, Canonical: head.CanonicalEventHead, Desired: head.DesiredEventHead, Effective: head.EffectiveEventHead}
	out.Heads.LivePendingTransition = head.PendingTransitionRef
}

func (rt *Runtime) witnessCheckpointAndRoute(ctx context.Context, operation selfdev.Operation, out *ReleaseWitness, b *witnessBindings) {
	out.Checkpoint.Ref = operation.CheckpointRef
	out.Route = WitnessRoute{
		Certificate: operation.RouteCertificate, Generation: operation.RouteGeneration, Receipt: operation.RouteReceipt,
		CodeRef: operation.CodeRef, ArtifactProgramRef: operation.ArtifactProgramRef,
	}
	if rt.store == nil || operation.DecisionEvent == "" {
		b.unknown("checkpoint_event", "no decision event, or no event store")
		b.unknown("route_event", "no decision event, or no event store")
		return
	}
	if event, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, "selfdev-checkpoint-published-"+operation.DecisionEvent); err == nil && found {
		out.Checkpoint.EventPresent, out.Checkpoint.EventProposedRef, out.Checkpoint.EventDecisionRef = true, event.ProposedEffectRef, event.DecisionRef
	}
	if event, found, err := rt.store.EventByIdempotency(ctx, operation.ComputerID, "selfdev-route-projection-updated-"+operation.DecisionEvent); err == nil && found {
		out.Route.EventPresent, out.Route.EventProposedRef, out.Route.EventDecisionRef = true, event.ProposedEffectRef, event.DecisionRef
	}
	switch {
	case operation.CheckpointRef == "" && !out.Checkpoint.EventPresent:
		b.unknown("checkpoint_event", "no checkpoint yet")
	case !out.Checkpoint.EventPresent || operation.CheckpointRef == "":
		b.observe("checkpoint_event", witnessBoolPtr(false), "the operation row and the tape disagree on whether a checkpoint was published")
	case "checkpoint:sha256:"+out.Checkpoint.EventProposedRef == operation.CheckpointRef && out.Checkpoint.EventDecisionRef == operation.DecisionEvent:
		b.observe("checkpoint_event", witnessBoolPtr(true), "the checkpoint_published event names the operation's checkpoint and decision")
	default:
		b.observe("checkpoint_event", witnessBoolPtr(false), "the checkpoint_published event does not name the operation's checkpoint and decision")
	}
	switch {
	case operation.RouteCertificate == "" && !out.Route.EventPresent:
		b.unknown("route_event", "no route projection yet")
	case !out.Route.EventPresent || operation.RouteCertificate == "":
		b.observe("route_event", witnessBoolPtr(false), "the operation row and the tape disagree on whether a route was projected")
	case out.Route.EventProposedRef == operation.RouteCertificate && out.Route.EventDecisionRef == operation.DecisionEvent:
		b.observe("route_event", witnessBoolPtr(true), "the route_projection_updated event names the operation's route certificate and decision")
	default:
		b.observe("route_event", witnessBoolPtr(false), "the route_projection_updated event does not name the operation's route certificate and decision")
	}
}

func (rt *Runtime) witnessEffective(ctx context.Context, operation selfdev.Operation, effective EffectiveRelease, out *ReleaseWitness, b *witnessBindings) {
	switch {
	case effective.ServingReleaseDigest == "" || effective.ReleaseDigest == "":
		b.unknown("serving_current", "the serving or current release digest is unavailable (a computer running its base release has no startup manifest)")
	case effective.ServingReleaseDigest == effective.ReleaseDigest:
		b.observe("serving_current", witnessBoolPtr(true), "the process serves the release `current` points at")
	default:
		b.observe("serving_current", witnessBoolPtr(false), "the process serves a different release than `current` points at (a swap awaiting restart, or a restore that restaged files only)")
	}
	switch {
	case operation.State != selfdev.StateApplied:
		b.unknown("effective_release_operation", "the operation is "+operation.State+", not applied")
	case effective.ReleaseDigest == "":
		b.unknown("effective_release_operation", "the effective release is unreadable here")
	case effective.ReleaseDigest == operation.ReleaseDigest:
		b.observe("effective_release_operation", witnessBoolPtr(true), "the running release is this operation's release")
	default:
		if other, ok := rt.appliedOperationForRelease(ctx, effective.ReleaseDigest); ok && other.OperationID != operation.OperationID {
			b.unknown("effective_release_operation", "superseded: operation "+other.OperationID+" is the effective release")
		} else {
			b.unknown("effective_release_operation", "the effective release is not this operation's and no applied operation owns it (a restore, a platform release, or the base release)")
		}
	}
}

func witnessTruncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
