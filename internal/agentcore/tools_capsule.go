package agentcore

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/selfdev"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// CapsuleToolCtx is injected by guest core. Opaque handles are bound to the
// current run; signed capabilities, raw capsule IDs, paths, sockets, and keys
// never enter model-visible arguments or results.
type CapsuleToolCtx struct {
	Executor           *capsule.Executor
	AgentRunID         string
	ComputerID         string
	Role               capsule.AgentRole
	UpdaterRoot        string
	CapsuleHandle      string
	EventAppender      *computerevent.ComputerEventAppender
	TransactionBuilder *transaction.TransactionBuilder
	OperationStore     *selfdev.Store
	EventProjection    interface {
		Head(context.Context, string) (*computerevent.Head, error)
		EventByIdempotency(context.Context, string, string) (computerevent.Event, bool, error)
	}
}

type capsuleCtxKey struct{}

func WithCapsuleCtx(ctx context.Context, value *CapsuleToolCtx) context.Context {
	return context.WithValue(ctx, capsuleCtxKey{}, value)
}

func capsuleCtxFromCtx(ctx context.Context) *CapsuleToolCtx {
	value, _ := ctx.Value(capsuleCtxKey{}).(*CapsuleToolCtx)
	return value
}

// RegisterCapsuleTools installs conductor-only lifecycle tools. Grant minting
// is deliberately absent: guest core grants processor handles while creating a
// child run, never through a model-callable tool.
func RegisterCapsuleTools(registry *toolregistry.ToolRegistry) error {
	for _, tool := range []toolregistry.Tool{
		newSpawnCapsuleTool(), newDestroyCapsuleTool(), newListCapsulesTool(), newInspectCapsuleTool(),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func RegisterCapsuleExecTools(registry *toolregistry.ToolRegistry) error {
	for _, tool := range []toolregistry.Tool{
		newCapsuleExecTool(), newCapsuleReadFileTool(), newCapsuleWriteFileTool(), newCapsuleListDirTool(),
		newCommitTransactionTool(), newInspectSelfDevelopmentBundleTool(), newRecordSelfDevelopmentVerificationTool(),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func requireCapsuleRole(ctx context.Context, role capsule.AgentRole) (*CapsuleToolCtx, error) {
	value := capsuleCtxFromCtx(ctx)
	if value == nil || value.Executor == nil || value.AgentRunID == "" {
		return nil, fmt.Errorf("capsule authority unavailable")
	}
	if value.Role != role {
		return nil, fmt.Errorf("capsule operation refused for role %q", value.Role)
	}
	return value, nil
}

func newSpawnCapsuleTool() toolregistry.Tool {
	type args struct {
		MemoryMaxMB int64 `json:"memory_max_mb"`
		CPUQuota    int64 `json:"cpu_quota"`
		PidsMax     int64 `json:"pids_max"`
	}
	return toolregistry.Tool{
		Name:        "spawn_capsule",
		Description: "Create an isolated, networkless work capsule and return an opaque lifecycle handle.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"memory_max_mb": map[string]any{"type": "integer"},
			"cpu_quota":     map[string]any{"type": "integer"},
			"pids_max":      map[string]any{"type": "integer"},
		}, nil, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleSuper)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			if input.MemoryMaxMB == 0 {
				input.MemoryMaxMB = 1024
			}
			if input.CPUQuota == 0 {
				input.CPUQuota = 100000
			}
			if input.PidsMax == 0 {
				input.PidsMax = 256
			}
			id, err := randomCapsuleID()
			if err != nil {
				return "", err
			}
			created, err := toolCtx.Executor.Spawn(ctx, capsule.SpawnSpec{
				CapsuleID: id, OwnerRunID: toolCtx.AgentRunID,
				MemoryMax: input.MemoryMaxMB * 1024 * 1024,
				CpuQuota:  input.CPUQuota, CpuPeriod: 100000, PidsMax: input.PidsMax,
			})
			if err != nil {
				return "", err
			}
			handle, err := toolCtx.Executor.ControlHandle(toolCtx.AgentRunID, created.ID)
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{"handle": handle, "state": created.State.String(), "source_snapshot_digest": created.SourceSnapshotDigest})
		},
	}
}

func newDestroyCapsuleTool() toolregistry.Tool {
	type args struct {
		Handle string `json:"handle"`
		Force  bool   `json:"force"`
	}
	return toolregistry.Tool{
		Name:        "destroy_capsule",
		Description: "Destroy the capsule identified by an opaque lifecycle handle.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"handle": map[string]any{"type": "string"}, "force": map[string]any{"type": "boolean"},
		}, []string{"handle"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleSuper)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			if err := toolCtx.Executor.DestroyOwned(ctx, toolCtx.AgentRunID, input.Handle, input.Force); err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{"handle": input.Handle, "destroyed": true})
		},
	}
}

func newListCapsulesTool() toolregistry.Tool {
	return toolregistry.Tool{
		Name: "list_capsules", Description: "List this conductor run's capsules by opaque handle.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{}, nil, false),
		Func: func(ctx context.Context, _ json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleSuper)
			if err != nil {
				return "", err
			}
			items := toolCtx.Executor.ListOwned(toolCtx.AgentRunID)
			return toolregistry.ResultJSON(map[string]any{"capsules": items, "count": len(items)})
		},
	}
}

func newCommitTransactionTool() toolregistry.Tool {
	type args struct {
		Handle string `json:"handle"`
	}
	return toolregistry.Tool{
		Name: "commit_transaction", Description: "Classify and freeze the capsule diff for audited proposal construction.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{"handle": map[string]any{"type": "string"}}, []string{"handle"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleCoSuper)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			changes, err := toolCtx.Executor.ExtractGranted(toolCtx.AgentRunID, input.Handle)
			if err != nil {
				return "", err
			}
			if toolCtx.TransactionBuilder == nil || toolCtx.EventAppender == nil || toolCtx.ComputerID == "" {
				return "", fmt.Errorf("capsule event authority unavailable")
			}
			capsuleID, err := toolCtx.Executor.ResolveGrantedCapsuleID(toolCtx.AgentRunID, input.Handle)
			if err != nil {
				return "", err
			}
			record, err := toolCtx.TransactionBuilder.BuildTransactionFromDiff(capsuleID, changes)
			if err != nil {
				return "", err
			}
			if record.Rejected {
				return toolregistry.ResultJSON(map[string]any{"handle": input.Handle, "rejected": true, "reject_reason": record.RejectReason})
			}
			execution := toolregistry.ExecutionContextFrom(ctx)
			trajectoryID := trajectoryIDForRun(execution.RunRecord)
			if toolCtx.OperationStore == nil || toolCtx.EventProjection == nil || strings.TrimSpace(toolCtx.UpdaterRoot) == "" || trajectoryID == "" {
				return "", fmt.Errorf("self-development freeze authority unavailable")
			}
			operation, err := toolCtx.OperationStore.GetByTrajectory(ctx, toolCtx.ComputerID, trajectoryID)
			if err != nil {
				return "", fmt.Errorf("resolve self-development operation: %w", err)
			}
			if operation.State != selfdev.StateExecuting {
				if operation.BundleDigest != "" && (operation.State == selfdev.StateFrozen || operation.State == selfdev.StateVerified || operation.State == selfdev.StateAwaitingApproval) {
					return toolregistry.ResultJSON(map[string]any{
						"handle": input.Handle, "bundle_digest": operation.BundleDigest,
						"operation_id": operation.OperationID, "state": operation.State,
					})
				}
				return "", fmt.Errorf("self-development operation is %s, expected %s", operation.State, selfdev.StateExecuting)
			}
			headBefore, err := toolCtx.EventProjection.Head(ctx, toolCtx.ComputerID)
			if err != nil || headBefore == nil || headBefore.PendingTransitionRef != "" {
				return "", fmt.Errorf("self-development base head unavailable or pending")
			}
			files, temporary, err := toolCtx.Executor.StageGrantedRelease(toolCtx.AgentRunID, input.Handle, filepath.Join(toolCtx.UpdaterRoot, "incoming"))
			if err != nil {
				return "", err
			}
			defer os.RemoveAll(temporary)
			sourceIntent, err := computerevent.CanonicalJSON(record.Groups)
			if err != nil {
				return "", err
			}
			runtimeIntent, err := computerevent.CanonicalJSON(files)
			if err != nil {
				return "", err
			}
			record.SourceTreeDigest = computerevent.DigestBytes(sourceIntent)
			record.RuntimeArtifactDigest = computerevent.DigestBytes(runtimeIntent)
			record.BaseEffectiveEventHead = headBefore.EffectiveEventHead
			record.RuntimeFiles = files
			bundle, err := computerevent.CanonicalJSON(record)
			if err != nil {
				return "", fmt.Errorf("canonical capsule effect bundle: %w", err)
			}
			bundleDigest := computerevent.DigestBytes(bundle)
			if err := os.WriteFile(filepath.Join(temporary, "bundle.json"), bundle, 0o400); err != nil {
				return "", err
			}
			frozenRoot := filepath.Join(toolCtx.UpdaterRoot, "incoming", bundleDigest)
			if err := os.Rename(temporary, frozenRoot); err != nil {
				existing, readErr := os.ReadFile(filepath.Join(frozenRoot, "bundle.json"))
				if readErr != nil || !bytes.Equal(existing, bundle) {
					return "", fmt.Errorf("freeze immutable release: %w", err)
				}
			}
			eventIdempotency := computerevent.DigestBytes([]byte("capsule-effect-v1\x00" + toolCtx.AgentRunID + "\x00" + capsuleID + "\x00" + bundleDigest))
			var receiptID string
			pinnedDigest := bundleDigest
			existingEvent := false
			if toolCtx.EventProjection != nil {
				projected, found, lookupErr := toolCtx.EventProjection.EventByIdempotency(ctx, toolCtx.ComputerID, eventIdempotency)
				if lookupErr != nil {
					return "", lookupErr
				}
				if found {
					if projected.ProposedEffectRef != bundleDigest || projected.TrajectoryID != trajectoryID || projected.CapsuleID != capsuleID {
						return "", fmt.Errorf("durable capsule effect event binding mismatch")
					}
					existingEvent = true
				}
			}
			if !existingEvent {
				eventID, eventErr := computerevent.NewEventID()
				if eventErr != nil {
					return "", eventErr
				}
				event := computerevent.Event{
					SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: toolCtx.ComputerID,
					EventKind: computerevent.EventEffectProposed, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
					IdempotencyKey: eventIdempotency, TrajectoryID: trajectoryID, CapsuleID: capsuleID, ActorProfile: agentprofile.CoSuper,
					AuthorityRef: "guest-core:capsule-control", PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
				}
				receipt, digest, appendErr := toolCtx.EventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, bundle, "application/vnd.choir.capsule-effect+json", "public")
				if appendErr != nil {
					return "", appendErr
				}
				receiptID, pinnedDigest = receipt.ReceiptID, digest
			}
			head, headErr := toolCtx.EventProjection.Head(ctx, toolCtx.ComputerID)
			if headErr != nil || head == nil {
				return "", fmt.Errorf("resolve frozen operation head: %w", headErr)
			}
			operation, err = toolCtx.OperationStore.Transition(ctx, toolCtx.ComputerID, operation.OperationID, selfdev.StateExecuting, selfdev.StateFrozen, func(next *selfdev.Operation) error {
				next.CapsuleID = capsuleID
				next.BundleDigest = pinnedDigest
				next.DesiredHead = head.DesiredEventHead
				next.EffectiveHead = head.EffectiveEventHead
				return nil
			})
			if err != nil {
				return "", err
			}
			result := map[string]any{
				"handle": input.Handle, "change_count": len(changes), "classifier_version": record.ClassifierV,
				"classifier_digest": record.ClassifierDigest, "groups": record.Groups,
				"bundle_digest": pinnedDigest, "event_head_receipt_id": receiptID,
			}
			result["operation_id"], result["state"] = operation.OperationID, operation.State
			return toolregistry.ResultJSON(result)
		},
	}
}

func newInspectSelfDevelopmentBundleTool() toolregistry.Tool {
	type args struct {
		OperationID  string `json:"operation_id"`
		BundleDigest string `json:"bundle_digest"`
	}
	return toolregistry.Tool{
		Name:        "inspect_self_development_bundle",
		Description: "Verify the immutable staged release and classifier metadata for an exact frozen self-development bundle.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"operation_id":  map[string]any{"type": "string"},
			"bundle_digest": map[string]any{"type": "string"},
		}, []string{"operation_id", "bundle_digest"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleCoSuper)
			if err != nil {
				return "", err
			}
			execution := toolregistry.ExecutionContextFrom(ctx)
			if execution.RunRecord == nil || normalizeCoSuperSlot(metadataStringValue(execution.RunRecord.Metadata, runMetadataCoSuperSlot)) != "verifier" {
				return "", fmt.Errorf("bundle inspection is restricted to the co-super verifier slot")
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			input.OperationID, input.BundleDigest = strings.TrimSpace(input.OperationID), strings.TrimSpace(input.BundleDigest)
			if input.OperationID == "" || !computerevent.IsSHA256(input.BundleDigest) || toolCtx.OperationStore == nil || strings.TrimSpace(toolCtx.UpdaterRoot) == "" {
				return "", fmt.Errorf("exact frozen bundle binding is required")
			}
			operation, err := toolCtx.OperationStore.Get(ctx, toolCtx.ComputerID, input.OperationID)
			if err != nil || operation.BundleDigest != input.BundleDigest || operation.TrajectoryID != trajectoryIDForRun(execution.RunRecord) {
				return "", fmt.Errorf("frozen operation binding mismatch")
			}
			root := filepath.Join(toolCtx.UpdaterRoot, "incoming", input.BundleDigest)
			rawBundle, err := os.ReadFile(filepath.Join(root, "bundle.json"))
			if err != nil || computerevent.DigestBytes(rawBundle) != input.BundleDigest {
				return "", fmt.Errorf("immutable bundle digest mismatch")
			}
			var record transaction.TransactionRecord
			decoder := json.NewDecoder(bytes.NewReader(rawBundle))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&record); err != nil || record.Rejected || record.BaseEffectiveEventHead != operation.EffectiveHead {
				return "", fmt.Errorf("invalid frozen bundle")
			}
			for _, file := range record.RuntimeFiles {
				path := filepath.Join(root, filepath.FromSlash(file.Path))
				inputFile, err := os.Open(path)
				if err != nil {
					return "", fmt.Errorf("frozen runtime file unavailable: %s", file.Path)
				}
				hash := sha256.New()
				_, copyErr := io.Copy(hash, inputFile)
				closeErr := inputFile.Close()
				if copyErr != nil || closeErr != nil || hex.EncodeToString(hash.Sum(nil)) != file.SHA256 {
					return "", fmt.Errorf("frozen runtime file digest mismatch: %s", file.Path)
				}
			}
			return toolregistry.ResultJSON(map[string]any{
				"operation_id": operation.OperationID, "bundle_digest": operation.BundleDigest,
				"source_tree_digest": record.SourceTreeDigest, "runtime_artifact_digest": record.RuntimeArtifactDigest,
				"base_effective_event_head": record.BaseEffectiveEventHead, "runtime_files": record.RuntimeFiles,
				"classifier_version": record.ClassifierV, "classifier_digest": record.ClassifierDigest, "groups": record.Groups,
			})
		},
	}
}

func newRecordSelfDevelopmentVerificationTool() toolregistry.Tool {
	type args struct {
		OperationID  string   `json:"operation_id"`
		BundleDigest string   `json:"bundle_digest"`
		Decision     string   `json:"decision"`
		VerifierRefs []string `json:"verifier_refs"`
	}
	return toolregistry.Tool{
		Name:        "record_self_development_verification",
		Description: "Record an independent verifier decision for the exact frozen self-development bundle.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"operation_id":  map[string]any{"type": "string"},
			"bundle_digest": map[string]any{"type": "string"},
			"decision":      map[string]any{"type": "string", "enum": []string{"pass", "fail"}},
			"verifier_refs": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		}, []string{"operation_id", "bundle_digest", "decision", "verifier_refs"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleCoSuper)
			if err != nil {
				return "", err
			}
			execution := toolregistry.ExecutionContextFrom(ctx)
			if normalizeCoSuperSlot(metadataStringValue(execution.RunRecord.Metadata, runMetadataCoSuperSlot)) != "verifier" {
				return "", fmt.Errorf("verification recording is restricted to the co-super verifier slot")
			}
			if toolCtx.OperationStore == nil || toolCtx.EventAppender == nil || toolCtx.EventProjection == nil {
				return "", fmt.Errorf("self-development verification authority unavailable")
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			input.OperationID, input.BundleDigest, input.Decision = strings.TrimSpace(input.OperationID), strings.TrimSpace(input.BundleDigest), strings.TrimSpace(input.Decision)
			input.VerifierRefs = trimNonEmptyStrings(input.VerifierRefs)
			if input.OperationID == "" || !computerevent.IsSHA256(input.BundleDigest) || (input.Decision != "pass" && input.Decision != "fail") || len(input.VerifierRefs) == 0 {
				return "", fmt.Errorf("complete exact verification binding is required")
			}
			operation, err := toolCtx.OperationStore.Get(ctx, toolCtx.ComputerID, input.OperationID)
			if err != nil {
				return "", err
			}
			if operation.BundleDigest != input.BundleDigest || operation.TrajectoryID != trajectoryIDForRun(execution.RunRecord) {
				return "", fmt.Errorf("verification does not bind the frozen operation")
			}
			if (operation.State == selfdev.StateAwaitingApproval && input.Decision == "pass") || (operation.State == selfdev.StateFailed && input.Decision == "fail") {
				return toolregistry.ResultJSON(map[string]any{"operation_id": operation.OperationID, "state": operation.State, "bundle_digest": operation.BundleDigest, "verifier_ref": firstString(operation.VerifierRefs)})
			}
			if operation.State != selfdev.StateFrozen && operation.State != selfdev.StateVerified {
				return "", fmt.Errorf("self-development operation is %s, expected frozen verification state", operation.State)
			}
			record := map[string]any{
				"schema_version": 1, "operation_id": operation.OperationID, "bundle_digest": operation.BundleDigest,
				"decision": input.Decision, "verifier_refs": input.VerifierRefs, "verifier_run_id": toolCtx.AgentRunID,
			}
			payload, err := computerevent.CanonicalJSON(record)
			if err != nil {
				return "", err
			}
			eventIdempotency := computerevent.DigestBytes(append([]byte("selfdev-verification-v1\x00"), payload...))
			verificationEvent, found, lookupErr := toolCtx.EventProjection.EventByIdempotency(ctx, toolCtx.ComputerID, eventIdempotency)
			if lookupErr != nil {
				return "", lookupErr
			}
			if !found {
				eventID, eventErr := computerevent.NewEventID()
				if eventErr != nil {
					return "", eventErr
				}
				event := computerevent.Event{
					SchemaVersion: computerevent.SchemaVersionV1, EventID: eventID, ComputerID: toolCtx.ComputerID,
					EventKind: computerevent.EventVerificationRecorded, OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
					IdempotencyKey: eventIdempotency, TrajectoryID: operation.TrajectoryID, CapsuleID: operation.CapsuleID,
					ActorProfile: agentprofile.CoSuper, AuthorityRef: "guest-core:self-development-verifier",
					PrivacyClass: "public", ReducerVersion: computerevent.ReducerVersionV1,
				}
				if _, _, appendErr := toolCtx.EventAppender.AppendNewPayload(ctx, event, computerevent.TransitionInput{}, payload, "application/vnd.choir.self-development-verification+json", "public"); appendErr != nil {
					return "", appendErr
				}
				verificationEvent, found, lookupErr = toolCtx.EventProjection.EventByIdempotency(ctx, toolCtx.ComputerID, eventIdempotency)
				if lookupErr != nil || !found {
					return "", fmt.Errorf("verification event projection unavailable: %w", lookupErr)
				}
			}
			verifierRef, err := verificationEvent.Digest()
			if err != nil {
				return "", err
			}
			if input.Decision == "fail" {
				operation, err = toolCtx.OperationStore.Transition(ctx, toolCtx.ComputerID, operation.OperationID, selfdev.StateFrozen, selfdev.StateFailed, func(next *selfdev.Operation) error {
					next.VerifierRefs = []string{verifierRef}
					next.TerminalError = "independent verifier rejected frozen bundle"
					return nil
				})
			} else {
				if operation.State == selfdev.StateFrozen {
					operation, err = toolCtx.OperationStore.Transition(ctx, toolCtx.ComputerID, operation.OperationID, selfdev.StateFrozen, selfdev.StateVerified, func(next *selfdev.Operation) error {
						next.VerifierRefs = []string{verifierRef}
						return nil
					})
					if err != nil {
						return "", err
					}
				}
				operation, err = toolCtx.OperationStore.Transition(ctx, toolCtx.ComputerID, operation.OperationID, selfdev.StateVerified, selfdev.StateAwaitingApproval, nil)
			}
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{"operation_id": operation.OperationID, "state": operation.State, "bundle_digest": operation.BundleDigest, "decision": input.Decision, "verifier_ref": verifierRef})
		},
	}
}

func newInspectCapsuleTool() toolregistry.Tool {
	type args struct {
		Handle string `json:"handle"`
	}
	return toolregistry.Tool{
		Name: "inspect_capsule", Description: "Inspect the safe lifecycle projection for an opaque capsule handle.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{"handle": map[string]any{"type": "string"}}, []string{"handle"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleSuper)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			summary, err := toolCtx.Executor.InspectOwned(toolCtx.AgentRunID, input.Handle)
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(summary)
		},
	}
}

func newCapsuleExecTool() toolregistry.Tool {
	type args struct {
		Command   string `json:"command"`
		Cwd       string `json:"cwd"`
		TimeoutMS int    `json:"timeout_ms"`
	}
	return toolregistry.Tool{
		Name: "capsule_exec", Description: "Execute a command inside the assigned isolated capsule.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"command": map[string]any{"type": "string"}, "cwd": map[string]any{"type": "string"}, "timeout_ms": map[string]any{"type": "integer"},
		}, []string{"command"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleCoSuper)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			result, err := toolCtx.Executor.Exec(ctx, toolCtx.AgentRunID, toolCtx.CapsuleHandle, capsule.ExecRequest{Command: input.Command, Cwd: input.Cwd, TimeoutMS: input.TimeoutMS})
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(result)
		},
	}
}

func newCapsuleReadFileTool() toolregistry.Tool {
	type args struct {
		Path string `json:"path"`
	}
	return toolregistry.Tool{Name: "capsule_read_file", Description: "Read a file inside the assigned capsule.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{"path": map[string]any{"type": "string"}}, []string{"path"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleCoSuper)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			content, err := toolCtx.Executor.ReadFile(ctx, toolCtx.AgentRunID, toolCtx.CapsuleHandle, input.Path)
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{"path": input.Path, "content": content})
		},
	}
}

func newCapsuleWriteFileTool() toolregistry.Tool {
	type args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		Mode    uint32 `json:"mode"`
	}
	return toolregistry.Tool{Name: "capsule_write_file", Description: "Write a file inside the assigned capsule.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"path": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}, "mode": map[string]any{"type": "integer"},
		}, []string{"path", "content"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleCoSuper)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			if err := toolCtx.Executor.WriteFile(ctx, toolCtx.AgentRunID, toolCtx.CapsuleHandle, input.Path, []byte(input.Content), input.Mode); err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{"path": input.Path, "written": true})
		},
	}
}

func newCapsuleListDirTool() toolregistry.Tool {
	type args struct {
		Path string `json:"path"`
	}
	return toolregistry.Tool{Name: "capsule_list_dir", Description: "List a directory inside the assigned capsule.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{"path": map[string]any{"type": "string"}}, []string{"path"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			toolCtx, err := requireCapsuleRole(ctx, capsule.RoleCoSuper)
			if err != nil {
				return "", err
			}
			var input args
			if err := json.Unmarshal(raw, &input); err != nil {
				return "", err
			}
			entries, err := toolCtx.Executor.ListDir(ctx, toolCtx.AgentRunID, toolCtx.CapsuleHandle, input.Path)
			if err != nil {
				return "", err
			}
			return toolregistry.ResultJSON(map[string]any{"path": input.Path, "entries": entries})
		},
	}
}

func randomCapsuleID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("capsule identity: %w", err)
	}
	return "capsule-" + hex.EncodeToString(value[:]), nil
}

func trimNonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func firstString(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
