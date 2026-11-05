package capsule

import (
	"context"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/ownerfiles"
)

// The computer-state layer of an engineering capsule
// (docs/design/capsule-overlay-computer-state-2026-11-03.md, slice S2): a
// read-only, point-in-time copy of the computer's files, as data. It is a
// hardlink snapshot of the files root taken under the files write lock,
// mounted between the source lower and the guest root. A copy is not
// access: no credential, no store handle, no write path, no live view.
const (
	// ComputerStateDir is where the layer appears inside the capsule.
	ComputerStateDir = "/computer"
	// ComputerFilesDir is the files snapshot inside the capsule.
	ComputerFilesDir = "/computer/files"
	// ComputerReceiptPath is the layer's receipt inside the capsule.
	ComputerReceiptPath = "/computer/receipt.json"
	// ComputerStorePath is the store copy inside the capsule; an autoputer
	// started over it takes RUNTIME_STORE_PATH=/computer/state.
	ComputerStorePath = "/computer/state.db"
	// computerStoreAllowance is the upper room the charge keeps for the
	// capsule's own writes under /computer (the copy's WAL, a scratch file).
	computerStoreAllowance = int64(512 << 20)
	// computerStoreCopyFactor is the free space a copy needs, as a multiple
	// of the estimate (VACUUM INTO writes a compact file, about the live
	// pages; the second VACUUM after a blanking briefly needs another).
	computerStoreCopyFactor = 2.0
	// computerLowerName is the layer root under the capsule's state dir;
	// it holds computer/files and computer/receipt.json.
	computerLowerName = "computer-lower"
	// maxNamedLeaveOuts bounds the receipt's list of named leave-outs; the
	// counts by reason are always complete.
	maxNamedLeaveOuts = 500
)

// Leave-out reasons, named in the receipt.
const (
	LeaveOutSource          = "source_lower"     // the computer's source is the capsule's /workspace/platform
	LeaveOutMail            = "mail"             // the owner's emails need an owner decision
	LeaveOutNotReadable     = "not_o+r"          // a mode the owner or a tool set; never raised, counted, never named
	LeaveOutSymlink         = "symlink"          // never followed
	LeaveOutSpecial         = "special_file"     // socket, FIFO or device
	LeaveOutTemporary       = "temporary"        // a writer's in-flight temporary file
	LeaveOutOtherFilesystem = "other_filesystem" // a mount inside the files root
	LeaveOutChanged         = "changed_during_walk"
)

// ComputerStateReceipt records what one spawn's computer-state layer holds
// and what it left out. It is written read-only at /computer/receipt.json
// and kept on the Capsule.
type ComputerStateReceipt struct {
	// ComputerState is "provided", or why the spawn got no /computer layers
	// at all (ComputerStateWithheldRawCredential).
	ComputerState string `json:"computer_state"`
	// Files is true when /computer/files was built; Refused names why not.
	Files   bool   `json:"files"`
	Refused string `json:"refused,omitempty"`
	// LockedAt is the instant the files snapshot holds (the files write
	// lock was taken then); LockHeldMS is how long writes waited.
	LockedAt    time.Time `json:"locked_at"`
	LockWaitMS  int64     `json:"lock_wait_ms"`
	LockHeldMS  int64     `json:"lock_held_ms"`
	LinkedFiles int       `json:"linked_files"`
	LinkedBytes int64     `json:"linked_bytes"`
	Directories int       `json:"directories"`
	// LeftOut names leave-outs (paths relative to the files root, at most
	// maxNamedLeaveOuts); LeftOutCounts counts every one by reason.
	LeftOut       []ComputerStateLeaveOut `json:"left_out,omitempty"`
	LeftOutCounts map[string]int          `json:"left_out_counts,omitempty"`
	LeftOutMore   int                     `json:"left_out_unnamed,omitempty"`
	// Store is the store copy at /computer/state.db; StoreRefused names why
	// there is none.
	Store        *ComputerStoreCopy `json:"store,omitempty"`
	StoreRefused string             `json:"store_refused,omitempty"`
}

// ComputerStoreCopy is what one store copy holds: the event head (the
// second instant of the layer, beside the files lock time), sizes, the
// secret-shape scan's work and blankings, and the sidecar parts left out.
type ComputerStoreCopy struct {
	ComputerID   string         `json:"computer_id,omitempty"`
	Sequence     uint64         `json:"sequence,omitempty"`
	Head         string         `json:"head,omitempty"`
	Bytes        int64          `json:"bytes"`
	CopyMS       int64          `json:"copy_ms"`
	ScanMS       int64          `json:"scan_ms"`
	ScannedBytes int64          `json:"scanned_bytes"`
	Blanked      map[string]int `json:"blanked,omitempty"`
	LeftOut      []string       `json:"left_out,omitempty"`
	// LeaseID names the lease that chose the kinds the copy holds (MA3.4);
	// Withheld counts the objects of the kinds it does not name, by kind.
	LeaseID  string         `json:"lease_id,omitempty"`
	Withheld map[string]int `json:"withheld,omitempty"`
}

// ComputerStoreSource takes the store copy for a capsule: the guest store
// (store.Store.SnapshotToKinds, adapted by the guest core). kinds are the
// object kinds the assignment's lease names; the copy holds those and no
// others. The capsule never gets a store handle, only the copy's file.
type ComputerStoreSource interface {
	SnapshotEstimate(ctx context.Context) (int64, error)
	SnapshotForCapsule(ctx context.Context, dir string, kinds []string) (ComputerStoreCopy, error)
}

// ComputerStateProvided marks a spawn that got the layers (each may still
// be refused on its own, as Refused and StoreRefused say).
const ComputerStateProvided = "provided"

// ComputerStateWithheldRawCredential is the refusal for a capsule that holds
// the owner's raw model credential: it never gets the computer's state beside
// a credential that reaches outside the computer. The claude mode's relay
// secret is not one: the owner's token stays in the host gateway, and the
// secret works only at the capsule's own egress proxy (problem 26,
// docs/problems/delegate-capsules-see-no-computer-state-2026-11-03.md).
const ComputerStateWithheldRawCredential = "withheld: raw model credential present"

// rawModelCredentialEnv are the environment keys that carry a model
// credential to a broker or a Claude Code process. A relay secret under one
// of them is gateway-scoped; any other non-empty value is a raw credential.
var rawModelCredentialEnv = []string{ClaudeDelegateBrokerTokenEnv, "ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN", "CLAUDE_CODE_OAUTH_TOKEN"}

// anthropicKeyShape is the prefix of every Anthropic key and OAuth token.
const anthropicKeyShape = "sk-ant-"

// rawModelCredential reports a value that is a credential and not a relay
// secret.
func rawModelCredential(value string) bool {
	value = strings.TrimSpace(value)
	return value != "" && !IsClaudeRelaySecret(value)
}

// computerStateWithheld says why a spawn that asked for the computer's state
// gets none: a delegate token about to be handed to its broker that is not a
// relay secret (a claude-mode spawn always carries one, so a claude-mode
// spawn without it fails closed), or a credential in its environment under a
// credential key or in an Anthropic key's shape anywhere in the entry. The
// claude mode with its relay secret does not withhold it. The broker's base
// environment is an allowlist (BrokerProcessEnv), so spec.Env and the token
// are every credential-bearing input. A relay secret is recognized by shape,
// not by identity: no Anthropic credential has that shape, and a
// relay-shaped value from elsewhere works at no proxy but its own capsule's.
func computerStateWithheld(spec SpawnSpec, delegateToken string) string {
	if rawModelCredential(delegateToken) || (spec.ClaudeDelegate && !IsClaudeRelaySecret(strings.TrimSpace(delegateToken))) {
		return ComputerStateWithheldRawCredential
	}
	for _, kv := range spec.Env {
		key, value, _ := strings.Cut(kv, "=")
		if strings.Contains(kv, anthropicKeyShape) {
			return ComputerStateWithheldRawCredential
		}
		for _, credKey := range rawModelCredentialEnv {
			if key == credKey && rawModelCredential(value) {
				return ComputerStateWithheldRawCredential
			}
		}
	}
	return ""
}

// withheldComputerState is the receipt of a withheld spawn: no files layer,
// no store copy, the reason on all three.
func withheldComputerState(reason string) *ComputerStateReceipt {
	return &ComputerStateReceipt{ComputerState: reason, Refused: reason, StoreRefused: reason}
}

// ComputerStateLeaveOut is one named leave-out.
type ComputerStateLeaveOut struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

func (r *ComputerStateReceipt) countLeaveOut(reason string) {
	if r.LeftOutCounts == nil {
		r.LeftOutCounts = map[string]int{}
	}
	r.LeftOutCounts[reason]++
}

func (r *ComputerStateReceipt) leaveOut(rel, reason string) {
	r.countLeaveOut(reason)
	if len(r.LeftOut) >= maxNamedLeaveOuts {
		r.LeftOutMore++
		return
	}
	r.LeftOut = append(r.LeftOut, ComputerStateLeaveOut{Path: rel, Reason: reason})
}

// computerFilesTemporary reports a writer's in-flight temporary name: the
// files write helper's ".<name>.tmp-*" and the older screens and hydration
// prefixes. A temporary is renamed into place or removed; it is never the
// owner's file.
func computerFilesTemporary(name string) bool {
	if strings.HasPrefix(name, ".") && strings.Contains(name, ownerfiles.TempInfix) {
		return true
	}
	return strings.HasPrefix(name, ".screen-") || strings.HasPrefix(name, ".choir-hydrate-")
}
