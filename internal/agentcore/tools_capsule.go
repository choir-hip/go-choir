package agentcore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// CapsuleToolCtx holds the capsule executor and current agent context
// for capsule-related tools. This is set up by the runtime when capsule
// mode is enabled.
type CapsuleToolCtx struct {
	Executor      *capsule.Executor
	AgentRunID    string
	Role          capsule.AgentRole
	CapsuleHandle string // opaque handle for the current capsule

	// TransactionTape is the tamper-evident append-only log of capsule
	// transaction records. Each commit_transaction call appends one entry.
	// The tape models the candidate branch's transaction history in the
	// TLA+ promotion protocol spec (capsuleTxns variable).
	TransactionTape *transaction.Tape

	// TransactionBuilder classifies capsule diffs and builds structured
	// transaction records for the tape. If nil, commit_transaction falls
	// back to returning raw changes without classification or tape append.
	TransactionBuilder *transaction.TransactionBuilder
}

type capsuleCtxKey struct{}

// WithCapsuleCtx returns a context with the capsule tool context attached.
func WithCapsuleCtx(ctx context.Context, ctc *CapsuleToolCtx) context.Context {
	return context.WithValue(ctx, capsuleCtxKey{}, ctc)
}

// capsuleCtxFromCtx extracts the capsule tool context from the context.
func capsuleCtxFromCtx(ctx context.Context) *CapsuleToolCtx {
	v, _ := ctx.Value(capsuleCtxKey{}).(*CapsuleToolCtx)
	return v
}

// RegisterCapsuleTools registers capsule-related tools for the super role:
// spawn_capsule, destroy_capsule, mint_capability, list_capsules,
// commit_transaction, inspect_capsule.
func RegisterCapsuleTools(registry *toolregistry.ToolRegistry) error {
	for _, tool := range []toolregistry.Tool{
		newSpawnCapsuleTool(),
		newDestroyCapsuleTool(),
		newListCapsulesTool(),
		newMintCapabilityTool(),
		newCommitTransactionTool(),
		newInspectCapsuleTool(),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

// RegisterCapsuleExecTools registers tools that operate inside a capsule
// for the cosuper role: capsule_exec, capsule_read_file, capsule_write_file,
// capsule_list_dir. These route through the broker via capability-verified RPCs.
func RegisterCapsuleExecTools(registry *toolregistry.ToolRegistry) error {
	for _, tool := range []toolregistry.Tool{
		newCapsuleExecTool(),
		newCapsuleReadFileTool(),
		newCapsuleWriteFileTool(),
		newCapsuleListDirTool(),
	} {
		if err := registry.Register(tool); err != nil {
			return err
		}
	}
	return nil
}

func newSpawnCapsuleTool() toolregistry.Tool {
	type args struct {
		MemoryMaxMB int64  `json:"memory_max_mb"`
		CpuQuota    int64  `json:"cpu_quota"`
		PidsMax     int64  `json:"pids_max"`
		WorkingDir  string `json:"working_dir"`
	}
	return toolregistry.Tool{Name: "spawn_capsule",
		Description: "Spawn a new isolated capsule (lightweight container) with the specified resource limits. Returns a capsule handle.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"memory_max_mb": map[string]any{"type": "integer", "description": "Memory limit in MB (default 1024)"},
			"cpu_quota":     map[string]any{"type": "integer", "description": "CPU quota in microseconds per period (default 100000 = 1 CPU)"},
			"pids_max":      map[string]any{"type": "integer", "description": "Max processes (default 256)"},
			"working_dir":   map[string]any{"type": "string", "description": "Initial working directory"},
		}, nil, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}
			if ctc.Role != capsule.RoleSuper {
				return "", fmt.Errorf("only super role can spawn capsules")
			}

			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode spawn_capsule args: %w", err)
			}

			// Apply defaults.
			memoryMax := in.MemoryMaxMB * 1024 * 1024
			if memoryMax == 0 {
				memoryMax = 1024 * 1024 * 1024 // 1GB default
			}
			cpuQuota := in.CpuQuota
			if cpuQuota == 0 {
				cpuQuota = 100000 // 1 CPU default
			}
			pidsMax := in.PidsMax
			if pidsMax == 0 {
				pidsMax = 256
			}

			capsuleID := generateCapsuleID()
			spec := capsule.SpawnSpec{
				CapsuleID:  capsuleID,
				MemoryMax:  memoryMax,
				CpuQuota:   cpuQuota,
				CpuPeriod:  100000,
				PidsMax:    pidsMax,
				WorkingDir: in.WorkingDir,
				OwnerRunID: ctc.AgentRunID,
			}

			caps, err := ctc.Executor.Spawn(ctx, spec)
			if err != nil {
				return "", fmt.Errorf("failed to spawn capsule: %w", err)
			}

			return toolregistry.ResultJSON(map[string]any{
				"capsule_id": caps.ID,
				"state":      caps.State.String(),
				"memory_max": spec.MemoryMax,
				"cpu_quota":  spec.CpuQuota,
				"pids_max":   spec.PidsMax,
			})
		}}
}

func newDestroyCapsuleTool() toolregistry.Tool {
	type args struct {
		CapsuleID string `json:"capsule_id"`
		Force     bool   `json:"force"`
	}
	return toolregistry.Tool{Name: "destroy_capsule",
		Description: "Destroy a capsule, cleaning up all resources (processes, overlayfs, cgroups).",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"capsule_id": map[string]any{"type": "string"},
			"force":      map[string]any{"type": "boolean", "description": "Force destroy (SIGKILL)"},
		}, []string{"capsule_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}
			if ctc.Role != capsule.RoleSuper {
				return "", fmt.Errorf("only super role can destroy capsules")
			}

			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode destroy_capsule args: %w", err)
			}

			var err error
			if in.Force {
				err = ctc.Executor.ForceDestroy(ctx, in.CapsuleID)
			} else {
				err = ctc.Executor.Destroy(ctx, in.CapsuleID)
			}
			if err != nil {
				return "", fmt.Errorf("failed to destroy capsule: %w", err)
			}

			return toolregistry.ResultJSON(map[string]any{
				"capsule_id": in.CapsuleID,
				"destroyed":  true,
			})
		}}
}

func newListCapsulesTool() toolregistry.Tool {
	return toolregistry.Tool{Name: "list_capsules",
		Description: "List all active capsules with their state and resource usage.",
		Parameters:  toolregistry.JSONSchemaObject(map[string]any{}, nil, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}

			summaries := ctc.Executor.ListCapsules()
			return toolregistry.ResultJSON(map[string]any{
				"capsules": summaries,
				"count":    len(summaries),
			})
		}}
}

func newMintCapabilityTool() toolregistry.Tool {
	type args struct {
		AgentRunID string `json:"agent_run_id"`
		Role       string `json:"role"`
		CapsuleID  string `json:"capsule_id"`
		TTLHours   int    `json:"ttl_hours"`
	}
	return toolregistry.Tool{Name: "mint_capability",
		Description: "Mint an Ed25519-signed capability for an agent to access a capsule. Super only.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"agent_run_id": map[string]any{"type": "string"},
			"role":         map[string]any{"type": "string", "enum": []string{"cosuper", "researcher"}},
			"capsule_id":   map[string]any{"type": "string"},
			"ttl_hours":    map[string]any{"type": "integer", "description": "TTL in hours (max 24)"},
		}, []string{"agent_run_id", "role", "capsule_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}
			if ctc.Role != capsule.RoleSuper {
				return "", fmt.Errorf("only super role can mint capabilities")
			}

			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode mint_capability args: %w", err)
			}

			ttl := time.Duration(in.TTLHours) * time.Hour
			if ttl == 0 {
				ttl = 4 * time.Hour // default 4h
			}
			if ttl > 24*time.Hour {
				return "", fmt.Errorf("TTL exceeds 24h maximum")
			}

			role := capsule.AgentRole(in.Role)
			cap, err := ctc.Executor.MintCapability(in.AgentRunID, role, in.CapsuleID, ttl)
			if err != nil {
				return "", fmt.Errorf("failed to mint capability: %w", err)
			}

			return toolregistry.ResultJSON(map[string]any{
				"handle":         cap.Handle,
				"capability_id":  cap.CapabilityID,
				"role":           cap.AgentRole,
				"target_capsule": cap.TargetCapsule,
				"expires_at":     cap.ExpiresAt,
			})
		}}
}

func newCommitTransactionTool() toolregistry.Tool {
	type args struct {
		CapsuleID string `json:"capsule_id"`
	}
	return toolregistry.Tool{Name: "commit_transaction",
		Description: "Extract the capsule diff, classify it, and commit to the tape. Super only.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"capsule_id": map[string]any{"type": "string"},
		}, []string{"capsule_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}
			if ctc.Role != capsule.RoleSuper {
				return "", fmt.Errorf("only super role can commit transactions")
			}

			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode commit_transaction args: %w", err)
			}

			changes, err := ctc.Executor.ExtractDiff(in.CapsuleID)
			if err != nil {
				return "", fmt.Errorf("failed to extract diff: %w", err)
			}

			// If no transaction builder is configured, fall back to raw changes.
			// This preserves backward compatibility with existing callers.
			if ctc.TransactionBuilder == nil {
				return toolregistry.ResultJSON(map[string]any{
					"capsule_id":   in.CapsuleID,
					"change_count": len(changes),
					"changes":      changes,
					"tape_append":  false,
				})
			}

			// Classify the diff and build a transaction record.
			record, err := ctc.TransactionBuilder.BuildTransactionFromDiff(in.CapsuleID, changes)
			if err != nil {
				return "", fmt.Errorf("build transaction: %w", err)
			}

			// If the record is rejected (unknown paths), do not append to tape.
			if record.Rejected {
				return toolregistry.ResultJSON(map[string]any{
					"capsule_id":    in.CapsuleID,
					"change_count":  len(changes),
					"rejected":      true,
					"reject_reason": record.RejectReason,
					"tape_append":   false,
				})
			}

			// Append to the tamper-evident tape.
			var tapeHash string
			var tapeLen int
			if ctc.TransactionTape != nil {
				tapeHash, err = ctc.TransactionTape.Append(record)
				if err != nil {
					return "", fmt.Errorf("tape append: %w", err)
				}
				tapeLen = ctc.TransactionTape.Len()
			}

			return toolregistry.ResultJSON(map[string]any{
				"capsule_id":         in.CapsuleID,
				"change_count":       len(changes),
				"classifier_version": record.ClassifierV,
				"classifier_digest":  record.ClassifierDigest,
				"groups":             record.Groups,
				"ignored_count":      len(record.Ignored),
				"tape_append":        ctc.TransactionTape != nil,
				"tape_hash":          tapeHash,
				"tape_length":        tapeLen,
			})
		}}
}

func newInspectCapsuleTool() toolregistry.Tool {
	type args struct {
		CapsuleID string `json:"capsule_id"`
	}
	return toolregistry.Tool{Name: "inspect_capsule",
		Description: "Inspect a capsule's state, resource usage, and diagnostics. Bypasses broker.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"capsule_id": map[string]any{"type": "string"},
		}, []string{"capsule_id"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}

			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode inspect_capsule args: %w", err)
			}

			diag, err := ctc.Executor.InspectCapsuleRaw(in.CapsuleID)
			if err != nil {
				return "", fmt.Errorf("failed to inspect capsule: %w", err)
			}

			return toolregistry.ResultJSON(map[string]any{
				"id":           diag.ID,
				"state":        diag.State.String(),
				"pid":          diag.PID,
				"memory_usage": diag.MemoryUsage,
				"memory_max":   diag.MemoryMax,
				"uptime":       diag.Uptime,
			})
		}}
}

// Capsule exec tools (cosuper role — route through broker)

func newCapsuleExecTool() toolregistry.Tool {
	type args struct {
		Command   string `json:"command"`
		Cwd       string `json:"cwd"`
		TimeoutMS int    `json:"timeout_ms"`
	}
	return toolregistry.Tool{Name: "capsule_exec",
		Description: "Execute a command inside the capsule via the broker. Requires cosuper capability.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"command":    map[string]any{"type": "string"},
			"cwd":        map[string]any{"type": "string"},
			"timeout_ms": map[string]any{"type": "integer"},
		}, []string{"command"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}

			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode capsule_exec args: %w", err)
			}

			cap, err := ctc.Executor.ResolveCapability(ctc.AgentRunID, ctc.CapsuleHandle)
			if err != nil {
				return "", fmt.Errorf("failed to resolve capability: %w", err)
			}

			// Resolve the capsule from the capability.
			targets, err := ctc.Executor.ResolveTarget(cap)
			if err != nil || len(targets) == 0 {
				return "", fmt.Errorf("no target capsule for capability")
			}

			// TODO: Get the capsule and call Exec.
			// This requires the Executor to expose a method to get a capsule
			// and call Exec on it with the capability.
			return toolregistry.ResultJSON(map[string]any{
				"command": in.Command,
				"status":  "not_implemented",
				"message": "capsule exec routing requires broker connection setup",
			})
		}}
}

func newCapsuleReadFileTool() toolregistry.Tool {
	type args struct {
		Path string `json:"path"`
	}
	return toolregistry.Tool{Name: "capsule_read_file",
		Description: "Read a file from inside the capsule via the broker.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"path": map[string]any{"type": "string"},
		}, []string{"path"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}

			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode capsule_read_file args: %w", err)
			}

			// TODO: Route through broker.
			return toolregistry.ResultJSON(map[string]any{
				"path":   in.Path,
				"status": "not_implemented",
			})
		}}
}

func newCapsuleWriteFileTool() toolregistry.Tool {
	type args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	return toolregistry.Tool{Name: "capsule_write_file",
		Description: "Write a file inside the capsule via the broker.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"path":    map[string]any{"type": "string"},
			"content": map[string]any{"type": "string"},
		}, []string{"path", "content"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}

			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode capsule_write_file args: %w", err)
			}

			// TODO: Route through broker.
			return toolregistry.ResultJSON(map[string]any{
				"path":   in.Path,
				"status": "not_implemented",
			})
		}}
}

func newCapsuleListDirTool() toolregistry.Tool {
	type args struct {
		Path string `json:"path"`
	}
	return toolregistry.Tool{Name: "capsule_list_dir",
		Description: "List directory contents inside the capsule via the broker.",
		Parameters: toolregistry.JSONSchemaObject(map[string]any{
			"path": map[string]any{"type": "string"},
		}, []string{"path"}, false),
		Func: func(ctx context.Context, raw json.RawMessage) (string, error) {
			ctc := capsuleCtxFromCtx(ctx)
			if ctc == nil || ctc.Executor == nil {
				return "", fmt.Errorf("capsule executor not configured")
			}

			var in args
			if err := json.Unmarshal(raw, &in); err != nil {
				return "", fmt.Errorf("decode capsule_list_dir args: %w", err)
			}

			// TODO: Route through broker.
			return toolregistry.ResultJSON(map[string]any{
				"path":   in.Path,
				"status": "not_implemented",
			})
		}}
}

// generateCapsuleID generates a unique capsule ID using crypto/rand.
func generateCapsuleID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// On entropy failure, fall back to timestamp (should never happen).
		return fmt.Sprintf("capsule-%d", time.Now().UnixNano())
	}
	return "capsule-" + hex.EncodeToString(b)
}
