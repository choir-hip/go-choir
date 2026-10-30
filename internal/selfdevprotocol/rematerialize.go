package selfdevprotocol

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

const (
	RematerializeMethodTape        = "tape_reconstruct"
	RematerializeMethodPinCheckout = "pin_checkout"
)

// RematerializeRequest is the destructive tape-only restore procedure.
// Pin checkout is evidence-only and is refused as a completion method.
// Original realization paths must be distinct from the staged reconstruction
// so the restore implementation cannot read surviving local state.
type RematerializeRequest struct {
	ComputerID            string     `json:"computer_id"`
	Checkpoint            Checkpoint `json:"checkpoint"`
	Method                string     `json:"method"`
	OriginalMarkerPath    string     `json:"original_marker_path"`
	OriginalWorkspacePath string     `json:"original_workspace_path"`
	StagedMarkerPath      string     `json:"staged_marker_path"`
	StagedWorkspacePath   string     `json:"staged_workspace_path"`
}

func RematerializeFromRequest(request RematerializeRequest) error {
	if strings.TrimSpace(request.Method) == RematerializeMethodPinCheckout {
		return fmt.Errorf("rematerialize: pin checkout is evidence-only and cannot complete restore")
	}
	if strings.TrimSpace(request.Method) != RematerializeMethodTape {
		return fmt.Errorf("rematerialize: tape reconstruction is required")
	}
	rebuilt, _, err := CheckpointFromRequest(request.Checkpoint.Request)
	if err != nil {
		return fmt.Errorf("rematerialize: checkpoint refused: %w", err)
	}
	if !computerevent.IsSHA256(request.Checkpoint.Digest) || rebuilt.Digest != request.Checkpoint.Digest {
		return fmt.Errorf("rematerialize: checkpoint digest does not match request")
	}
	if strings.TrimSpace(request.ComputerID) == "" || request.ComputerID != request.Checkpoint.Request.ComputerID {
		return fmt.Errorf("rematerialize: computer binding does not match checkpoint")
	}
	originalMarker := filepath.Clean(strings.TrimSpace(request.OriginalMarkerPath))
	originalWorkspace := filepath.Clean(strings.TrimSpace(request.OriginalWorkspacePath))
	stagedMarker := filepath.Clean(strings.TrimSpace(request.StagedMarkerPath))
	stagedWorkspace := filepath.Clean(strings.TrimSpace(request.StagedWorkspacePath))
	if originalMarker == "." || originalWorkspace == "." || stagedMarker == "." || stagedWorkspace == "." {
		return fmt.Errorf("rematerialize: original and staged realization paths are required")
	}
	if originalMarker == originalWorkspace || stagedMarker == stagedWorkspace {
		return fmt.Errorf("rematerialize: marker and workspace paths must be distinct")
	}
	if overlapsPath(stagedMarker, originalMarker) || overlapsPath(stagedMarker, originalWorkspace) ||
		overlapsPath(stagedWorkspace, originalMarker) || overlapsPath(stagedWorkspace, originalWorkspace) {
		return fmt.Errorf("rematerialize: staged realization must not read the original data.img or workspace")
	}
	return nil
}

// WitnessContentMatches compares the verification commitment of a
// reconstructed store with a checkpoint's. The content root commits to the
// schema hash and every table digest, so a match is a match of all of them.
func WitnessContentMatches(got, want VMLocalContentWitness) error {
	if err := got.Validate(); err != nil {
		return fmt.Errorf("rematerialize: reconstructed witness refused: %w", err)
	}
	if err := want.Validate(); err != nil {
		return fmt.Errorf("rematerialize: checkpoint witness refused: %w", err)
	}
	if got.Database != want.Database {
		return fmt.Errorf("rematerialize: reconstructed database does not match checkpoint")
	}
	if tables := differingTables(got, want); len(tables) > 0 {
		return fmt.Errorf("rematerialize: reconstructed %s does not match checkpoint", strings.Join(tables, ", "))
	}
	if got.ContentRoot != want.ContentRoot || got.DerivabilityDigest != want.DerivabilityDigest {
		return fmt.Errorf("rematerialize: reconstructed content root does not match checkpoint")
	}
	return nil
}

func overlapsPath(left, right string) bool {
	left, right = filepath.Clean(left), filepath.Clean(right)
	if left == right {
		return true
	}
	sep := string(filepath.Separator)
	return strings.HasPrefix(left, right+sep) || strings.HasPrefix(right, left+sep)
}
