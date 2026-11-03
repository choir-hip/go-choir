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
}

// ComputerStoreSource takes the store copy for a capsule: the guest store
// (store.Store.SnapshotTo, adapted by the guest core). The capsule never gets
// a store handle, only the copy's file.
type ComputerStoreSource interface {
	SnapshotEstimate(ctx context.Context) (int64, error)
	SnapshotForCapsule(ctx context.Context, dir string) (ComputerStoreCopy, error)
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
