// RLM roster driver (P5-roster orchestration).
//
// `choir roster` opens frozen desk-task assignments through the existing
// owner inputs and collects per-run evidence. It creates no assignment
// authority: the persistent Super's `assign_co_super` tool remains the sole
// opener. Subcommands:
//
//	roster preflight  resolve overlay, pin task bytes, refuse live runs
//	roster start      submit the frozen instruction via texture tell
//	roster collect    poll reads and write the roster receipt
//
// No provider credentials touch this client; spend is bounded by the
// server-side subscription caps (auto-reload OFF).

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/modelpolicy"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

// rosterTaskRef names the frozen desk-task artifact every roster run serves.
const rosterTaskRef = "docs/evidence/choir-rlm-engineering-carrier-p0-freeze-2026-09-11.md#7"

// rosterTellVersion pins the wrapper instruction bytes around the frozen task.
// v2 replaced the `model_policy_overlay_id=<id>` prose literal (which the
// assignment opener refuses) with an argument-name instruction; v3 added the
// superseded-arm disposition after v2 arms were refused as duplicates; v4
// states the assign_co_super call and its overlay argument explicitly, because
// a v3 arm opened with the structured field empty (run metadata null) and the
// desk then silently served the base policy's unfunded deepseek model. v1/v2/v3
// receipts name their own wrapper.
const rosterTellVersion = "roster-v4"

// rosterEngineeringRole is the overlay role key the assignment path resolves
// (agentprofile.CoSuper is the token "engineering").
const rosterEngineeringRole = "engineering"

// rosterResolveResponse mirrors the model-policy resolve payload subset the
// preflight gate enforces.
type rosterResolveResponse struct {
	Role            string `json:"role"`
	OverlayID       string `json:"overlay_id"`
	Provider        string `json:"provider"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort"`
	Source          string `json:"source"`
	PolicyError     string `json:"policy_error"`
}

func runRoster(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "choir roster: subcommand required (preflight|start|collect)")
		return 2
	}
	sub := args[0]
	switch sub {
	case "preflight":
		return runRosterPreflight(args[1:], stdout, stderr)
	case "start":
		return runRosterStart(args[1:], stdout, stderr)
	case "collect":
		return runRosterCollect(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "choir roster: unknown subcommand %q\n", sub)
		return 2
	}
}

// rosterTaskBytes reads the frozen task file and enforces the pinned digest.
func rosterTaskBytes(taskFile, expectedSHA string) ([]byte, string, error) {
	raw, err := os.ReadFile(taskFile)
	if err != nil {
		return nil, "", fmt.Errorf("read task file: %w", err)
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	if strings.TrimSpace(expectedSHA) == "" {
		return nil, "", fmt.Errorf("--expected-task-sha256 is required")
	}
	if digest != strings.TrimSpace(expectedSHA) {
		return nil, "", fmt.Errorf("task digest %s does not match pinned %s", digest, strings.TrimSpace(expectedSHA))
	}
	return raw, digest, nil
}

func runRosterPreflight(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir roster preflight", flag.ContinueOnError)
	fs.SetOutput(stderr)
	overlayID := fs.String("overlay-id", "", "Owner-visible model-policy overlay id")
	expectProvider := fs.String("expect-provider", "", "Expected resolved provider id")
	expectModel := fs.String("expect-model", "", "Expected resolved model id")
	taskFile := fs.String("task-file", "", "File holding the exact frozen task bytes")
	expectedTaskSHA := fs.String("expected-task-sha256", "", "Pinned SHA-256 of the frozen task bytes")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir roster preflight: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*overlayID) == "" || strings.TrimSpace(*expectProvider) == "" || strings.TrimSpace(*expectModel) == "" {
		fmt.Fprintln(stderr, "choir roster preflight: --overlay-id, --expect-provider, and --expect-model are required")
		return 2
	}
	if _, taskDigest, err := rosterTaskBytes(strings.TrimSpace(*taskFile), *expectedTaskSHA); err != nil {
		fmt.Fprintf(stderr, "choir roster preflight: %v\n", err)
		return 1
	} else {
		_ = taskDigest
	}
	query := url.Values{}
	query.Set("role", rosterEngineeringRole)
	query.Set("overlay_id", strings.TrimSpace(*overlayID))
	var resolved rosterResolveResponse
	if err := c.do(http.MethodGet, "/api/model-policy/resolve?"+query.Encode(), nil, &resolved); err != nil {
		fmt.Fprintf(stderr, "choir roster preflight: resolve overlay: %v\n", err)
		return 1
	}
	if strings.TrimSpace(resolved.PolicyError) != "" {
		fmt.Fprintf(stderr, "choir roster preflight: overlay refused: %s\n", resolved.PolicyError)
		return 1
	}
	if resolved.Provider != strings.TrimSpace(*expectProvider) || resolved.Model != strings.TrimSpace(*expectModel) {
		fmt.Fprintf(stderr, "choir roster preflight: overlay resolves to %s/%s, want %s/%s (source %s)\n",
			resolved.Provider, resolved.Model, *expectProvider, *expectModel, resolved.Source)
		return 1
	}
	if live, describe, err := rosterLiveEngineeringRun(c); err != nil {
		fmt.Fprintf(stderr, "choir roster preflight: %v\n", err)
		return 1
	} else if live {
		fmt.Fprintf(stderr, "choir roster preflight: live engineering run present, refusing: %s\n", describe)
		return 1
	}
	return writeJSON(stdout, map[string]any{
		"schema":           "choir.roster_preflight.v1",
		"overlay_id":       strings.TrimSpace(*overlayID),
		"provider":         resolved.Provider,
		"model":            resolved.Model,
		"task_sha256":      strings.TrimSpace(*expectedTaskSHA),
		"task_ref":         rosterTaskRef,
		"live_run_refused": false,
	})
}

// rosterLiveEngineeringRun reports whether any non-terminal engineering
// (CoSuper) run is live. Unknown run shapes fail closed: refusing a roster
// open is cheap, colliding with a live assignment is not.
func rosterLiveEngineeringRun(c *client) (bool, string, error) {
	var resp json.RawMessage
	if err := c.do(http.MethodGet, "/api/runs?limit=50", nil, &resp); err != nil {
		return false, "", fmt.Errorf("list runs: %w", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(resp, &envelope); err != nil {
		return false, "", fmt.Errorf("parse run list: %w", err)
	}
	var entries []any
	for _, key := range []string{"runs", "items", "data"} {
		if list, ok := envelope[key].([]any); ok {
			entries = list
			break
		}
	}
	if entries == nil {
		return false, "", fmt.Errorf("run list has no runs/items/data array")
	}
	terminal := map[string]bool{"completed": true, "failed": true, "cancelled": true, "canceled": true, "terminal": true}
	for _, entry := range entries {
		row, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		// Match through the sanctioned vocabulary boundary: run-list profiles
		// may carry legacy V1 spellings, and V1 desk token literals are
		// forbidden in production sources (writer-purity gate), so never
		// match them directly. NormalizeRole covers live names;
		// ForwardV1ToV2 covers frozen V1 spellings.
		profile := modelpolicy.NormalizeRole(rosterFirstString(row, "agent_profile", "agentProfile", "profile", "role"))
		if profile == "" {
			if v2, ok := vocabmigrate.ForwardV1ToV2(rosterFirstString(row, "agent_profile", "agentProfile", "profile", "role")); ok {
				profile = v2
			}
		}
		if profile != agentprofile.CoSuper {
			continue
		}
		state := rosterFirstString(row, "state", "status", "disposition", "lifecycle_state")
		if state == "" {
			return true, fmt.Sprintf("run %s has unreadable state (fail closed)", rosterFirstString(row, "run_id", "id", "runId")), nil
		}
		if !terminal[strings.ToLower(state)] {
			return true, fmt.Sprintf("run %s state %s", rosterFirstString(row, "run_id", "id", "runId"), state), nil
		}
	}
	return false, "", nil
}

func rosterFirstString(row map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := row[k].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// rosterTellText builds the frozen wrapper instruction: overlay binding plus
// the frozen task bytes verbatim. The wrapper is fixed by rosterTellVersion;
// any byte change alters the tell and is visible in the receipt.
//
// The overlay id is named as a quoted value beside its argument name, never as
// the literal `model_policy_overlay_id=<id>`: the assignment opener fails
// closed when an objective carries that literal while the structured
// ModelPolicyOverlayID field is empty (cosuper_assignment_runtime.go), and
// management copies wrapper prose into objectives.
//
// v3 adds the superseded-arm disposition. v2 arms were refused with "preserve
// the single-active-assignment invariant. Do not open a duplicate": the
// harness mints one execution work item per tell and never dispositioned the
// old ones, so opening a new arm read as a duplicate. No owner-facing work-item
// disposition route exists, so the desk must clear them through its own
// reporting path, and the arm must be named as explicitly authorized.
func rosterTellText(overlayID string, task []byte) string {
	var b strings.Builder
	b.WriteString("ROSTER-V1 arm execution. Two required acts, in order. ")
	b.WriteString("(1) Disposition every earlier open ROSTER-V1 work item on this trajectory as cancelled with the reason \"superseded by a newer roster arm\", so no superseded arm stays active and none blocks this one. ")
	b.WriteString("(2) Call assign_co_super exactly once with kind=\"implementation\" and with its model_policy_overlay_id argument set to the value \"")
	b.WriteString(strings.TrimSpace(overlayID))
	b.WriteString("\" — a real JSON argument value, never text inside the objective. ")
	b.WriteString("The objective argument must be exactly the task below, byte-for-byte, with no preface, no paraphrase, and no extra steps. Exactly one call, never two:\n")
	b.Write(task)
	return b.String()
}

func runRosterStart(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir roster start", flag.ContinueOnError)
	fs.SetOutput(stderr)
	overlayID := fs.String("overlay-id", "", "Owner-visible model-policy overlay id")
	requestID := fs.String("request-id", "", "Stable client occurrence id (deterministic per campaign/arm); retries reuse it")
	docID := fs.String("doc", "", "Texture lifecycle document id receiving the owner instruction")
	taskFile := fs.String("task-file", "", "File holding the exact frozen task bytes")
	expectedTaskSHA := fs.String("expected-task-sha256", "", "Pinned SHA-256 of the frozen task bytes")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir roster start: %v\n", err)
		return 2
	}
	if strings.TrimSpace(*overlayID) == "" || strings.TrimSpace(*requestID) == "" || strings.TrimSpace(*docID) == "" {
		fmt.Fprintln(stderr, "choir roster start: --overlay-id, --request-id, and --doc are required")
		return 2
	}
	task, taskDigest, err := rosterTaskBytes(strings.TrimSpace(*taskFile), *expectedTaskSHA)
	if err != nil {
		fmt.Fprintf(stderr, "choir roster start: %v\n", err)
		return 1
	}
	var document struct {
		CurrentRevisionID string `json:"current_revision_id"`
	}
	if err := c.do(http.MethodGet, "/api/texture/documents/"+url.PathEscape(strings.TrimSpace(*docID)), nil, &document); err != nil {
		fmt.Fprintf(stderr, "choir roster start: read doc head: %v\n", err)
		return 1
	}
	if strings.TrimSpace(document.CurrentRevisionID) == "" {
		fmt.Fprintln(stderr, "choir roster start: doc has no current revision")
		return 1
	}
	body := map[string]string{
		"client_request_id":         strings.TrimSpace(*requestID),
		"content":                   rosterTellText(strings.TrimSpace(*overlayID), task),
		"expected_head_revision_id": strings.TrimSpace(document.CurrentRevisionID),
	}
	var response json.RawMessage
	if err := c.do(http.MethodPost, "/api/texture/documents/"+url.PathEscape(strings.TrimSpace(*docID))+"/tell", body, &response); err != nil {
		fmt.Fprintf(stderr, "choir roster start: tell: %v\n", err)
		return 1
	}
	var envelope struct {
		Schema string `json:"schema"`
	}
	if json.Unmarshal(response, &envelope) != nil || envelope.Schema != "choir.texture_owner_instruction.v1" {
		fmt.Fprintf(stderr, "choir roster start: unsupported owner-instruction schema %q\n", envelope.Schema)
		return 1
	}
	var out map[string]any
	if err := json.Unmarshal(response, &out); err != nil {
		fmt.Fprintf(stderr, "choir roster start: parse tell receipt: %v\n", err)
		return 1
	}
	out["roster_tell_version"] = rosterTellVersion
	out["overlay_id"] = strings.TrimSpace(*overlayID)
	out["task_sha256"] = taskDigest
	out["task_ref"] = rosterTaskRef
	return writeJSON(stdout, out)
}

// rosterReceipt is the per-run evidence record written by collect.
type rosterReceipt struct {
	Schema             string         `json:"schema"`
	RequestID          string         `json:"request_id"`
	OverlayID          string         `json:"overlay_id"`
	AssignmentID       string         `json:"assignment_id"`
	Attempt            uint64         `json:"attempt"`
	BoundRunID         string         `json:"bound_run_id"`
	TrajectoryID       string         `json:"trajectory_id"`
	TaskSHA256         string         `json:"task_sha256"`
	TaskRef            string         `json:"task_ref"`
	Pass               *bool          `json:"pass"`
	FailureMode        string         `json:"failure_mode,omitempty"`
	ServedProvider     string         `json:"served_provider,omitempty"`
	ServedModel        string         `json:"served_model,omitempty"`
	ServingID          string         `json:"serving_id,omitempty"`
	TokensIn           int64          `json:"tokens_in,omitempty"`
	TokensOut          int64          `json:"tokens_out,omitempty"`
	LatencyMS          int64          `json:"latency_ms,omitempty"`
	RunIDs             map[string]any `json:"run_ids,omitempty"`
	HealthIdentity     map[string]any `json:"health_identity,omitempty"`
	DeploySHA          string         `json:"deploy_sha,omitempty"`
	PolicySource       string         `json:"policy_source,omitempty"`
	CollectedAt        string         `json:"collected_at"`
	Notes              string         `json:"notes,omitempty"`
	NeedsHumanClassify bool           `json:"needs_human_classify"`
}

// rosterFailureBasePolicy names the arm-invalidating failure: the assignment
// resolved its provider/model from the computer's base policy instead of the
// arm's overlay, because the structured model_policy_overlay_id never reached
// the assignment path. Two live arms did exactly that and silently spent on an
// unfunded provider, so the receipt must fail loudly rather than record a pass.
const rosterFailureBasePolicy = "policy_source_not_arm_overlay"

// applyRosterPolicySource records the provider and model the bound run actually
// resolved and, when the arm's overlay did not serve it, marks the receipt
// failed with the observed source. The assignment records llm_policy_source at
// open time, so a base-policy arm is detectable before any provider call.
func applyRosterPolicySource(receipt *rosterReceipt, runDoc map[string]any, overlayID string) {
	metadata, _ := runDoc["metadata"].(map[string]any)
	if metadata == nil {
		return
	}
	receipt.ServedProvider = rosterFirstString(metadata, "llm_provider", "provider")
	receipt.ServedModel = rosterFirstString(metadata, "llm_model", "model")
	source := rosterFirstString(metadata, "llm_policy_source")
	if source == "" {
		return
	}
	receipt.PolicySource = source
	want := "/model-policy-overlays/" + strings.TrimSpace(overlayID) + ".toml"
	if strings.HasSuffix(source, want) {
		return
	}
	failed := false
	receipt.Pass = &failed
	receipt.FailureMode = rosterFailureBasePolicy
	receipt.NeedsHumanClassify = true
	receipt.Notes = fmt.Sprintf("assignment resolved %s/%s from %s, not the arm overlay %s: the structured model_policy_overlay_id did not reach the assignment path",
		receipt.ServedProvider, receipt.ServedModel, source, want)
}

func runRosterCollect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("choir roster collect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	requestID := fs.String("request-id", "", "Stable client occurrence id from roster start")
	overlayID := fs.String("overlay-id", "", "Owner-visible model-policy overlay id from roster start")
	assignmentID := fs.String("assignment", "", "Assignment id to collect (from the management trace)")
	attempt := fs.Uint64("attempt", 1, "Assignment attempt to collect")
	runID := fs.String("run", "", "Bound CoSuper run id (from the management trace)")
	trajectoryID := fs.String("trajectory", "", "Assignment trajectory id (from the management trace)")
	taskSHA := fs.String("task-sha256", "", "Pinned SHA-256 of the frozen task bytes")
	artifact := fs.String("artifact", "", "Output path for the roster receipt JSON (atomic write)")
	collectTimeout := fs.Duration("collect-timeout", 30*time.Minute, "Maximum collect wait for terminal disposition")
	pollInterval := fs.Duration("poll-interval", 15*time.Second, "Delay between collect polls")
	c, err := newClient(fs, args, stdout, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "choir roster collect: %v\n", err)
		return 2
	}
	for _, req := range []struct{ name, value string }{
		{"request-id", *requestID}, {"overlay-id", *overlayID}, {"assignment", *assignmentID},
		{"run", *runID}, {"trajectory", *trajectoryID}, {"artifact", *artifact},
	} {
		if strings.TrimSpace(req.value) == "" {
			fmt.Fprintf(stderr, "choir roster collect: --%s is required\n", req.name)
			return 2
		}
	}
	deadline := time.Now().Add(*collectTimeout)
	if *pollInterval <= 0 {
		*pollInterval = 15 * time.Second
	}
	for {
		receipt, done, err := rosterCollectOnce(c, strings.TrimSpace(*requestID), strings.TrimSpace(*overlayID),
			strings.TrimSpace(*assignmentID), *attempt, strings.TrimSpace(*runID), strings.TrimSpace(*trajectoryID),
			strings.TrimSpace(*taskSHA))
		if err != nil {
			fmt.Fprintf(stderr, "choir roster collect: %v\n", err)
			return 1
		}
		if done {
			if err := rosterWriteArtifact(strings.TrimSpace(*artifact), receipt); err != nil {
				fmt.Fprintf(stderr, "choir roster collect: %v\n", err)
				return 1
			}
			return writeJSON(stdout, receipt)
		}
		if time.Now().After(deadline) {
			receipt.NeedsHumanClassify = true
			receipt.Notes = "collect timeout before terminal disposition; receipt records partial evidence only"
			if err := rosterWriteArtifact(strings.TrimSpace(*artifact), receipt); err != nil {
				fmt.Fprintf(stderr, "choir roster collect: %v\n", err)
				return 1
			}
			fmt.Fprintf(stderr, "choir roster collect: timeout; partial receipt written\n")
			return 1
		}
		time.Sleep(*pollInterval)
	}
}

// rosterCollectOnce gathers one evidence snapshot. done reports terminal
// disposition observed; the receipt always carries what is known so far.
func rosterCollectOnce(c *client, requestID, overlayID, assignmentID string, attempt uint64, runID, trajectoryID, taskSHA string) (*rosterReceipt, bool, error) {
	receipt := &rosterReceipt{
		Schema:       "choir.roster_receipt.v1",
		RequestID:    requestID,
		OverlayID:    overlayID,
		AssignmentID: assignmentID,
		Attempt:      attempt,
		BoundRunID:   runID,
		TrajectoryID: trajectoryID,
		TaskSHA256:   taskSHA,
		TaskRef:      rosterTaskRef,
		CollectedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		RunIDs:       map[string]any{},
	}
	var health map[string]any
	if err := c.do(http.MethodGet, "/health/ready", nil, &health); err == nil {
		receipt.HealthIdentity = health
	}
	var runDoc map[string]any
	if err := c.do(http.MethodGet, "/api/runs/"+url.PathEscape(runID), nil, &runDoc); err != nil {
		receipt.Notes = fmt.Sprintf("run read: %v", err)
		return receipt, false, nil
	}
	receipt.RunIDs["bound_run"] = runDoc
	applyRosterPolicySource(receipt, runDoc, overlayID)
	if receipt.FailureMode == rosterFailureBasePolicy {
		return receipt, true, nil
	}
	disposition := rosterFirstString(runDoc, "disposition", "state", "status")
	switch strings.ToLower(disposition) {
	case "completed", "failed", "cancelled", "canceled", "terminal":
		return rosterCollectTerminal(c, receipt, runID, trajectoryID, assignmentID, attempt)
	default:
		receipt.Notes = fmt.Sprintf("run disposition %q not terminal; polling", disposition)
		return receipt, false, nil
	}
}

// rosterCollectTerminal reads the terminal evidence bundle. Pass/fail
// classification of model behavior stays human-adjudicated at this stage:
// the CLI records exact observations and marks needs_human_classify.
func rosterCollectTerminal(c *client, receipt *rosterReceipt, runID, trajectoryID, assignmentID string, attempt uint64) (*rosterReceipt, bool, error) {
	var snapshot json.RawMessage
	if err := c.do(http.MethodGet, "/api/trajectories/"+url.PathEscape(trajectoryID), nil, &snapshot); err == nil {
		var doc map[string]any
		if json.Unmarshal(snapshot, &doc) == nil {
			receipt.RunIDs["trajectory"] = doc
		}
	}
	var evidence json.RawMessage
	path := fmt.Sprintf("/api/trajectories/%s/capsule-evidence/%s?attempt=%d", url.PathEscape(trajectoryID), url.PathEscape(assignmentID), attempt)
	if err := c.do(http.MethodGet, path, nil, &evidence); err == nil {
		var doc map[string]any
		if json.Unmarshal(evidence, &doc) == nil {
			receipt.RunIDs["capsule_evidence"] = doc
		}
	}
	var costs json.RawMessage
	if err := c.do(http.MethodGet, "/api/costs?detail=1", nil, &costs); err == nil {
		var doc map[string]any
		if json.Unmarshal(costs, &doc) == nil {
			receipt.RunIDs["costs"] = doc
		}
	}
	receipt.NeedsHumanClassify = true
	receipt.Notes = "terminal disposition observed; model-behavior classification is human-adjudicated (see P5-review)"
	return receipt, true, nil
}

// rosterWriteArtifact writes the receipt atomically: temp file plus rename,
// so a killed collect never leaves a half-written receipt as spend authority.
func rosterWriteArtifact(path string, receipt *rosterReceipt) error {
	raw, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal receipt: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("write receipt temp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("publish receipt: %w", err)
	}
	return nil
}
