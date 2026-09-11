package agentcore

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

const rlmReplayFixtureVersion = 1

const (
	rlmReplayCapsuleExec       = "capsule_exec"
	rlmReplayCapsuleReadFile   = "capsule_read_file"
	rlmReplayCapsuleWriteFile  = "capsule_write_file"
	rlmReplayCapsuleListDir    = "capsule_list_dir"
	rlmReplayCommitTransaction = "commit_transaction"
	rlmReplayInspectBundle     = "inspect_self_development_bundle"
	rlmReplayRecordVerification = "record_self_development_verification"
	rlmReplayAssignmentResult  = "record_assignment_result"
	rlmReplayUpdateCoagent     = "update_coagent"
)

var rlmReplayOperations = map[string]struct{}{
	rlmReplayCapsuleExec: {}, rlmReplayCapsuleReadFile: {}, rlmReplayCapsuleWriteFile: {},
	rlmReplayCapsuleListDir: {}, rlmReplayCommitTransaction: {}, rlmReplayInspectBundle: {},
	rlmReplayRecordVerification: {}, rlmReplayAssignmentResult: {}, rlmReplayUpdateCoagent: {},
}

// rlmReplayFixture is the versioned capture envelope. ExpectedReceipt contains
// only the per-operation canonical receipt projection, never a raw tool result.
type rlmReplayFixture struct {
	FixtureVersion    int             `json:"fixture_version"`
	Operation         string          `json:"operation"`
	SemanticIdentity  string          `json:"semantic_identity"`
	Request           json.RawMessage `json:"request"`
	Environment       json.RawMessage `json:"environment"`
	ExpectedReceipt   json.RawMessage `json:"expected_receipt"`
	ExclusionsApplied []string        `json:"exclusions_applied"`
	CapturedAt        time.Time       `json:"captured_at"`
	CaptureBuildSHA   string          `json:"capture_build_sha"`
}

// rlmReplayCanonicalReceipt is the byte- and digest-stable comparison value.
type rlmReplayCanonicalReceipt struct {
	JSON   []byte
	SHA256 string
}

// rlmReplayProofMode records the P4 claim boundary. Rows 1-4 are R8
// conformance rows and therefore live-only; rows 5-7 remain recorded-fixture
// proofs until effects-off golden capture is decided; rows 8-9 require live
// durable-store resolution as their deletion proof.
type rlmReplayProofMode string

const (
	rlmReplayLiveCapture      rlmReplayProofMode = "live_capture"
	rlmReplayRecordedFixture  rlmReplayProofMode = "recorded_fixture"
)

var rlmReplayProofPlan = map[string]rlmReplayProofMode{
	rlmReplayCapsuleExec:       rlmReplayLiveCapture,
	rlmReplayCapsuleReadFile:   rlmReplayLiveCapture,
	rlmReplayCapsuleWriteFile:  rlmReplayLiveCapture,
	rlmReplayCapsuleListDir:    rlmReplayLiveCapture,
	rlmReplayCommitTransaction: rlmReplayRecordedFixture,
	rlmReplayInspectBundle:     rlmReplayRecordedFixture,
	rlmReplayRecordVerification: rlmReplayRecordedFixture,
	rlmReplayAssignmentResult:  rlmReplayLiveCapture,
	rlmReplayUpdateCoagent:     rlmReplayLiveCapture,
}

func canonicalRLMReplayReceipt(operation string, receipt any, exclusions []string) (rlmReplayCanonicalReceipt, error) {
	projected, err := projectRLMReplayReceipt(operation, receipt, exclusions)
	if err != nil {
		return rlmReplayCanonicalReceipt{}, err
	}
	canonical, err := computerevent.CanonicalJSON(projected)
	if err != nil {
		return rlmReplayCanonicalReceipt{}, fmt.Errorf("canonical replay receipt: %w", err)
	}
	return rlmReplayCanonicalReceipt{JSON: canonical, SHA256: computerevent.DigestBytes(canonical)}, nil
}

func projectRLMReplayReceipt(operation string, receipt any, exclusions []string) (map[string]any, error) {
	if _, ok := rlmReplayOperations[operation]; !ok {
		return nil, fmt.Errorf("replay: unknown operation %q", operation)
	}
	value, err := replayObject(receipt)
	if err != nil {
		return nil, err
	}
	for _, exclusion := range exclusions {
		delete(value, exclusion)
	}
	var fields []string
	switch operation {
	case rlmReplayCapsuleExec:
		fields = []string{"command", "cwd", "exit_code", "stdout_sha256", "stderr_sha256"}
	case rlmReplayCapsuleReadFile:
		fields = []string{"path", "content_sha256"}
	case rlmReplayCapsuleWriteFile:
		fields = []string{"path", "content_sha256", "bytes_written"}
	case rlmReplayCapsuleListDir:
		fields = []string{"path", "entries"}
	case rlmReplayCommitTransaction:
		fields = []string{"operation_id", "trajectory_id", "base_event_head", "content_digest", "change_count", "classifier_version", "classifier_digest", "groups", "state"}
	case rlmReplayInspectBundle:
		fields = []string{"operation_id", "content_digest", "source_tree_ref", "runtime_artifact_ref", "base_event_head", "runtime_files", "build_recipe_ref", "test_receipts", "dependency_toolchain_refs", "resource_receipts", "execution_receipts", "classifier_version", "classifier_digest", "groups"}
	case rlmReplayRecordVerification:
		fields = []string{"operation_id", "decision", "state", "verifier_ref"}
	case rlmReplayAssignmentResult:
		fields = []string{"assignment_id", "attempt", "proposition_digest", "disposition", "result", "verdict", "summary", "evidence_refs", "command_ids", "output_digests", "candidate", "replay", "report_id"}
	case rlmReplayUpdateCoagent:
		fields = []string{"update_id", "caller_agent_id", "target_agent_id", "channel_id", "trajectory_id", "packet_kind", "packet_digest", "durable_cursor"}
	}
	out := make(map[string]any, len(fields))
	for _, field := range fields {
		if raw, ok := value[field]; ok {
			out[field] = normalizeRLMReplayValue(field, raw)
		}
	}
	return out, nil
}

func replayObject(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("replay: marshal receipt: %w", err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("replay: receipt is not an object: %w", err)
	}
	return out, nil
}

func normalizeRLMReplayValue(field string, value any) any {
	switch v := value.(type) {
	case string:
		if isRLMReplayDigestField(field) && isLowerableHexDigest(v) {
			return strings.ToLower(v)
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = normalizeRLMReplayValue("", v[i])
		}
		if rlmReplayOrderIsNotSemantic(field) {
			sort.Slice(out, func(i, j int) bool {
			left, _ := computerevent.CanonicalJSON(out[i])
			right, _ := computerevent.CanonicalJSON(out[j])
			return string(left) < string(right)
		})
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = normalizeRLMReplayValue(key, item)
		}
		return out
	default:
		return value
	}
}

func isRLMReplayDigestField(field string) bool {
	return strings.Contains(field, "digest") || strings.Contains(field, "sha256") || field == "base_event_head"
}

func isLowerableHexDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func rlmReplayOrderIsNotSemantic(field string) bool {
	switch field {
	case "entries", "runtime_files", "test_receipts", "dependency_toolchain_refs", "resource_receipts", "execution_receipts", "evidence_refs", "command_ids", "output_digests":
		return true
	default:
		return false
	}
}

func canonicalRLMReplayInput(operation string, request any) (rlmReplayCanonicalReceipt, error) {
	if _, ok := rlmReplayOperations[operation]; !ok {
		return rlmReplayCanonicalReceipt{}, fmt.Errorf("replay: unknown operation %q", operation)
	}
	input, err := replayObject(request)
	if err != nil {
		return rlmReplayCanonicalReceipt{}, err
	}
	var fields []string
	switch operation {
	case rlmReplayCapsuleExec:
		fields = []string{"command", "args", "cwd"}
	case rlmReplayCapsuleReadFile, rlmReplayCapsuleListDir:
		fields = []string{"path"}
	case rlmReplayCapsuleWriteFile:
		fields = []string{"path", "content_sha256"}
		if content, ok := input["content"].(string); ok {
			input["content_sha256"] = computerevent.DigestBytes([]byte(content))
		}
	case rlmReplayCommitTransaction:
		fields = []string{"build_recipe_ref", "test_receipts", "dependency_toolchain_refs"}
	case rlmReplayInspectBundle:
		fields = []string{"operation_id", "bundle_digest"}
	case rlmReplayRecordVerification:
		fields = []string{"operation_id", "bundle_digest", "decision", "verifier_refs"}
	case rlmReplayAssignmentResult:
		// The frozen row-8 proposition deliberately excludes summary-only
		// changes, which replay rather than conflict.
		fields = []string{"assignment_id", "attempt", "result", "verdict", "evidence_refs", "execution_refs", "candidate", "disposition"}
	case rlmReplayUpdateCoagent:
		fields = []string{"target_agent_id", "to_desk", "packet", "msg_kind", "body"}
	}
	projected := make(map[string]any, len(fields))
	for _, field := range fields {
		if value, ok := input[field]; ok {
			projected[field] = normalizeRLMReplayValue(field, value)
		}
	}
	canonical, err := computerevent.CanonicalJSON(projected)
	if err != nil {
		return rlmReplayCanonicalReceipt{}, fmt.Errorf("canonical replay input: %w", err)
	}
	return rlmReplayCanonicalReceipt{JSON: canonical, SHA256: computerevent.DigestBytes(canonical)}, nil
}

var errRLMReplayIdentityConflict = errors.New("replay semantic identity conflicts with canonical input")

type rlmReplayIdentityBinding struct {
	operation string
	inputHash string
}

// rlmReplayIdentityJournal is deliberately caller-owned: it is the harness
// replacement for today's broker-minted rcpt_* IDs. It guards before dispatch.
type rlmReplayIdentityJournal struct {
	mu       sync.Mutex
	bindings map[string]rlmReplayIdentityBinding
}

func (j *rlmReplayIdentityJournal) claim(operation, semanticIdentity string, request any) (replay bool, err error) {
	if strings.TrimSpace(semanticIdentity) == "" {
		return false, fmt.Errorf("replay semantic identity is required")
	}
	canonical, err := canonicalRLMReplayInput(operation, request)
	if err != nil {
		return false, err
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.bindings == nil {
		j.bindings = make(map[string]rlmReplayIdentityBinding)
	}
	binding, found := j.bindings[semanticIdentity]
	if !found {
		j.bindings[semanticIdentity] = rlmReplayIdentityBinding{operation: operation, inputHash: canonical.SHA256}
		return false, nil
	}
	if binding.operation != operation || binding.inputHash != canonical.SHA256 {
		return false, errRLMReplayIdentityConflict
	}
	return true, nil
}

// rlmReplayDispatchJournal supplies the mandatory transient effect census
// witness. Record is called immediately before dispatch, never after it.
type rlmReplayDispatchJournal struct {
	mu       sync.Mutex
	attempts map[string]int
}

func (j *rlmReplayDispatchJournal) record(semanticIdentity string) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.attempts == nil {
		j.attempts = make(map[string]int)
	}
	j.attempts[semanticIdentity]++
	return j.attempts[semanticIdentity]
}

func (j *rlmReplayDispatchJournal) attemptsFor(semanticIdentity string) int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.attempts[semanticIdentity]
}

type rlmReplayWitness struct {
	Name             string `json:"name"`
	SemanticIdentity string `json:"semantic_identity"`
	Value            string `json:"value"`
}

type rlmReplayEffectCensus struct {
	ReceiptClass     string             `json:"receipt_class"`
	SemanticIdentity string             `json:"semantic_identity"`
	Witnesses        []rlmReplayWitness `json:"witnesses"`
}

// rlmReplayWitnessNames is the durable-store census contract. Each class has
// at least one independently resolved durable witness; transient effects also
// retain the pre-dispatch journal required to detect a re-execution.
var rlmReplayWitnessNames = map[string][]string{
	"durable_execution":      {"receipt_artifact", "worktree_digest"},
	"transient_observation":  {"dispatch_admission_attempt"},
	"transient_mutation":     {"filesystem_write_invocation", "dispatch_admission_attempt"},
	"selfdev_freeze":         {"event_range", "operation_transition", "frozen_bundle_file", "worktree_digest"},
	"read_only_inspection":   {"frozen_bundle_file", "receipt_artifact"},
	"selfdev_verify":         {"event_range", "operation_transition", "receipt_artifact"},
	"lifecycle_fate":         {"lifecycle_command_row", "update_row", "mailbox_row", "receipt_artifact"},
	"lifecycle_update":       {"update_row", "mailbox_row", "durable_cursor"},
}

func rlmReplayReadCensus(receiptClass, identity string, observed map[string]string) (rlmReplayEffectCensus, error) {
	names, ok := rlmReplayWitnessNames[receiptClass]
	if !ok {
		return rlmReplayEffectCensus{}, fmt.Errorf("replay effect census unknown receipt class %q", receiptClass)
	}
	census := rlmReplayEffectCensus{ReceiptClass: receiptClass, SemanticIdentity: identity, Witnesses: make([]rlmReplayWitness, 0, len(names))}
	for _, name := range names {
		value := observed[name]
		if strings.TrimSpace(value) == "" {
			return rlmReplayEffectCensus{}, fmt.Errorf("replay effect census missing %s witness", name)
		}
		census.Witnesses = append(census.Witnesses, rlmReplayWitness{Name: name, SemanticIdentity: identity, Value: value})
	}
	return census, census.validate()
}

func (c rlmReplayEffectCensus) validate() error {
	if c.ReceiptClass == "" || c.SemanticIdentity == "" || len(c.Witnesses) == 0 {
		return fmt.Errorf("replay effect census requires receipt class, identity, and witnesses")
	}
	for _, witness := range c.Witnesses {
		if witness.Name == "" || witness.Value == "" || witness.SemanticIdentity != c.SemanticIdentity {
			return fmt.Errorf("replay effect census witness is not bound to its semantic identity")
		}
	}
	return nil
}

func rlmReplayTransientWriteCensus(identity, sentinel string, dispatches *rlmReplayDispatchJournal) (rlmReplayEffectCensus, error) {
	return rlmReplayReadCensus("transient_mutation", identity, map[string]string{
		"filesystem_write_invocation": sentinel,
		"dispatch_admission_attempt":  fmt.Sprintf("%d", dispatches.attemptsFor(identity)),
	})
}

func rlmReplayUpdateCensus(identity string, update types.CoagentSourcePacket) (rlmReplayEffectCensus, error) {
	return rlmReplayReadCensus("lifecycle_update", identity, map[string]string{
		"update_row":     update.UpdateID,
		"mailbox_row":    fmt.Sprintf("%d", update.MessageSeq),
		"durable_cursor": fmt.Sprintf("%d", update.MessageSeq),
	})
}

// rlmReplayLegacyAdapter invokes the legacy JSON Tool.Func directly with the
// bound capsule context rather than recreating tool behavior in the harness.
type rlmReplayLegacyAdapter struct {
	Tool    toolregistry.Tool
	ToolCtx *CapsuleToolCtx
	ExecCtx toolregistry.ExecutionContext
}

func (a rlmReplayLegacyAdapter) invoke(ctx context.Context, request json.RawMessage) (map[string]any, error) {
	ctx = WithCapsuleCtx(ctx, a.ToolCtx)
	ctx = toolregistry.WithExecutionContext(ctx, a.ExecCtx)
	raw, err := a.Tool.Func(ctx, request)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("decode legacy tool result: %w", err)
	}
	return out, nil
}

// rlmReplaySuccessorAdapter binds a caller-selected identity before staging.
// Mutation paths stage through Tray and then the actual rlmCallReduction
// commit path; a replay therefore never reaches stage or commit again.
type rlmReplaySuccessorAdapter struct {
	Identities *rlmReplayIdentityJournal
	Dispatches *rlmReplayDispatchJournal
	Reduction  *rlmCallReduction
}

func (a rlmReplaySuccessorAdapter) stageAndCommit(ctx context.Context, operation, identity string, request any, tray *yaegikernel.Tray, stage func(*yaegikernel.Tray) error) (bool, error) {
	replay, err := a.Identities.claim(operation, identity, request)
	if err != nil || replay {
		return replay, err
	}
	if a.Dispatches != nil {
		a.Dispatches.record(identity)
	}
	if err := stage(tray); err != nil {
		return false, err
	}
	if a.Reduction == nil {
		return false, fmt.Errorf("replay successor reduction is required")
	}
	if err := a.Reduction.commit(ctx, tray.Drain()); err != nil {
		return false, err
	}
	return false, nil
}

// rlmReplayInspectAdapter accepts the in-cell mount inspector as a dependency.
// inspectMountedBundleAt is intentionally package-private in yaegikernel, so
// this package cannot call it against a fixture root without widening that
// production API. The caller supplies that exact function when the package
// exposes a test-safe mount-root entry point.
type rlmReplayInspectAdapter struct {
	InspectMountedBundleAt func(root string) (map[string]any, error)
}

func (a rlmReplayInspectAdapter) inspect(root string) (map[string]any, error) {
	if a.InspectMountedBundleAt == nil {
		return nil, fmt.Errorf("replay inspect successor is unavailable: inspectMountedBundleAt is not exported")
	}
	return a.InspectMountedBundleAt(root)
}

// rlmReplayStateMutatingRead proves transient observations are memoized under
// one semantic identity while a new identity sees the changed backing state.
type rlmReplayStateMutatingRead struct {
	identities *rlmReplayIdentityJournal
	values     map[string]string
}

func (r *rlmReplayStateMutatingRead) observe(operation, identity string, request any, read func() (string, error)) (string, bool, error) {
	replay, err := r.identities.claim(operation, identity, request)
	if err != nil {
		return "", false, err
	}
	if replay {
		return r.values[identity], true, nil
	}
	value, err := read()
	if err != nil {
		return "", false, err
	}
	if r.values == nil {
		r.values = make(map[string]string)
	}
	r.values[identity] = value
	return value, false, nil
}

var errRLMReplayBreakGlassRewarm = errors.New("replay rewarm requires owner break-glass: restart the host runtime through the owner deployment control")

// rlmReplayForceRewarm uses the local runtime's existing restart recovery path
// when available. No non-SSH public control exists in this package, so a nil
// runtime returns the named owner-executed break-glass sentinel.
func rlmReplayForceRewarm(ctx context.Context, rt *Runtime) error {
	if rt == nil {
		return errRLMReplayBreakGlassRewarm
	}
	rt.rewarmInterruptedLifecycleActivations(ctx)
	rt.rewarmInterruptedPersistentSuperActors(ctx)
	return nil
}

func rlmReplayGoldenPath(operation string) (string, error) {
	if _, ok := rlmReplayOperations[operation]; !ok {
		return "", fmt.Errorf("replay golden unknown operation %q", operation)
	}
	return filepath.Join("..", "..", "docs", "evidence", "rlm-replay", operation+"-golden-v1.json"), nil
}

func writeRLMReplayGolden(fixture rlmReplayFixture) error {
	if fixture.FixtureVersion != rlmReplayFixtureVersion || strings.TrimSpace(fixture.CaptureBuildSHA) == "" {
		return fmt.Errorf("replay golden requires fixture version %d and capture build SHA", rlmReplayFixtureVersion)
	}
	path, err := rlmReplayGoldenPath(fixture.Operation)
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(body, '\n'), 0o644)
}

func readRLMReplayGolden(operation, captureBuildSHA string) (rlmReplayFixture, error) {
	path, err := rlmReplayGoldenPath(operation)
	if err != nil {
		return rlmReplayFixture{}, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return rlmReplayFixture{}, err
	}
	var fixture rlmReplayFixture
	if err := json.Unmarshal(body, &fixture); err != nil {
		return rlmReplayFixture{}, err
	}
	if fixture.FixtureVersion != rlmReplayFixtureVersion || fixture.Operation != operation || fixture.CaptureBuildSHA != captureBuildSHA {
		return rlmReplayFixture{}, fmt.Errorf("replay golden binding mismatch")
	}
	return fixture, nil
}

func TestRLMReplayFixtureRoundTrip(t *testing.T) {
	fixture := rlmReplayFixture{
		FixtureVersion: rlmReplayFixtureVersion, Operation: rlmReplayCapsuleReadFile, SemanticIdentity: "read:one",
		Request: json.RawMessage(`{"path":"/workspace/a"}`), Environment: json.RawMessage(`{"computer_id":"test"}`),
		ExpectedReceipt: json.RawMessage(`{"content_sha256":"abc","path":"/workspace/a"}`), ExclusionsApplied: []string{"broker_receipt_id"},
		CapturedAt: time.Date(2026, 9, 11, 0, 0, 0, 0, time.UTC), CaptureBuildSHA: "73815790",
	}
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var got rlmReplayFixture
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.FixtureVersion != fixture.FixtureVersion || got.Operation != fixture.Operation || got.SemanticIdentity != fixture.SemanticIdentity || string(got.ExpectedReceipt) != string(fixture.ExpectedReceipt) || got.CaptureBuildSHA != fixture.CaptureBuildSHA {
		t.Fatalf("fixture round trip = %+v, want %+v", got, fixture)
	}
}

func TestRLMReplayCanonicalizerStableDigest(t *testing.T) {
	digest := strings.ToUpper(strings.Repeat("ab", 32))
	first, err := canonicalRLMReplayReceipt(rlmReplayInspectBundle, map[string]any{
		"operation_id": "operation-1", "content_digest": digest, "runtime_files": []any{
			map[string]any{"path": "z.go", "sha256": digest}, map[string]any{"path": "a.go", "sha256": digest},
		}, "execution_receipts": []any{"rcpt-z", "rcpt-a"}, "broker_receipt_id": "entropy-a",
	}, []string{"broker_receipt_id"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := canonicalRLMReplayReceipt(rlmReplayInspectBundle, map[string]any{
		"execution_receipts": []any{"rcpt-a", "rcpt-z"}, "runtime_files": []any{
			map[string]any{"sha256": strings.ToLower(digest), "path": "a.go"}, map[string]any{"sha256": strings.ToLower(digest), "path": "z.go"},
		}, "content_digest": strings.ToLower(digest), "operation_id": "operation-1", "broker_receipt_id": "entropy-b",
	}, []string{"broker_receipt_id"})
	if err != nil {
		t.Fatal(err)
	}
	if first.SHA256 != second.SHA256 || string(first.JSON) != string(second.JSON) {
		t.Fatalf("canonical receipts differ:\n%s\n%s", first.JSON, second.JSON)
	}
}

func TestRLMReplaySemanticIdentityConflictPrecedesDispatch(t *testing.T) {
	identities := &rlmReplayIdentityJournal{}
	dispatches := &rlmReplayDispatchJournal{}
	if replay, err := identities.claim(rlmReplayCapsuleWriteFile, "write:one", map[string]any{"path": "a", "content": "one"}); err != nil || replay {
		t.Fatalf("first identity claim replay=%t err=%v", replay, err)
	}
	dispatches.record("write:one")
	if _, err := identities.claim(rlmReplayCapsuleWriteFile, "write:one", map[string]any{"path": "a", "content": "two"}); !errors.Is(err, errRLMReplayIdentityConflict) {
		t.Fatalf("changed request error = %v, want conflict", err)
	}
	if got := dispatches.attemptsFor("write:one"); got != 1 {
		t.Fatalf("conflict dispatched %d times, want 1", got)
	}
}

func TestRLMReplayStateMutatingReadFixture(t *testing.T) {
	state := "before"
	fixture := &rlmReplayStateMutatingRead{identities: &rlmReplayIdentityJournal{}}
	first, replay, err := fixture.observe(rlmReplayCapsuleReadFile, "read:one", map[string]any{"path": "a"}, func() (string, error) { return state, nil })
	if err != nil || replay || first != "before" {
		t.Fatalf("first read = %q replay=%t err=%v", first, replay, err)
	}
	state = "after"
	same, replay, err := fixture.observe(rlmReplayCapsuleReadFile, "read:one", map[string]any{"path": "a"}, func() (string, error) { return state, nil })
	if err != nil || !replay || same != "before" {
		t.Fatalf("same identity read = %q replay=%t err=%v", same, replay, err)
	}
	fresh, replay, err := fixture.observe(rlmReplayCapsuleReadFile, "read:two", map[string]any{"path": "a"}, func() (string, error) { return state, nil })
	if err != nil || replay || fresh != "after" {
		t.Fatalf("new identity read = %q replay=%t err=%v", fresh, replay, err)
	}
}

func TestRLMReplayEffectCensusDurableUpdate(t *testing.T) {
	_, store := testRuntime(t)
	ctx := context.Background()
	update := types.CoagentSourcePacket{
		UpdateID: "replay-census-update", OwnerID: "owner", AgentID: "engineering:writer", TargetAgentID: "supervisor", ChannelID: "channel", TrajectoryID: "trajectory", Role: "engineering",
		Packet: types.CoagentSourcePacketPayload{SchemaVersion: types.CoagentSourcePacketSchemaV1, Kind: "evidence_update", Summary: "replay census"}, Content: "replay census", CreatedAt: time.Now().UTC(),
	}
	message := &types.ChannelMessage{ChannelID: update.ChannelID, From: "run", FromAgentID: update.AgentID, ToAgentID: update.TargetAgentID, TrajectoryID: update.TrajectoryID, Role: update.Role, Content: update.Content, Timestamp: update.CreatedAt}
	stored, created, err := store.DispatchWorkerUpdate(ctx, update, message)
	if err != nil || !created {
		t.Fatalf("dispatch durable update created=%t err=%v", created, err)
	}
	resolved, err := store.GetWorkerUpdate(ctx, update.OwnerID, update.UpdateID)
	if err != nil || resolved.MessageSeq != stored.MessageSeq {
		t.Fatalf("resolve durable update = %+v err=%v", resolved, err)
	}
	census, err := rlmReplayUpdateCensus("update:one", resolved)
	if err != nil || census.validate() != nil || len(census.Witnesses) < 3 {
		t.Fatalf("durable census = %+v err=%v", census, err)
	}
}

func TestRLMReplayEffectCensusTransientWrite(t *testing.T) {
	dispatches := &rlmReplayDispatchJournal{}
	dispatches.record("write:one")
	path := filepath.Join(t.TempDir(), "sentinel")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	census, err := rlmReplayTransientWriteCensus("write:one", fmt.Sprintf("%d:%d", info.ModTime().UnixNano(), info.Size()), dispatches)
	if err != nil {
		t.Fatal(err)
	}
	if census.Witnesses[0].Name != "filesystem_write_invocation" || census.Witnesses[1].Value != "1" {
		t.Fatalf("transient census = %+v", census)
	}
}
