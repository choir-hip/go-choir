//go:build linux

package capsule

import (
	"context"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// StageGrantedDeliverables stages the files the frozen assignment capsule
// left in DeliverablesDir (stageDeliverableFiles) under a new directory of stageRoot and returns it
// with the accepted files and the refusals. Files are opened beneath the capsule root
// without following symlinks. It needs the capsule's capability and a frozen
// capsule, as StageGrantedRelease does, and changes nothing in the capsule.
func (e *Executor) StageGrantedDeliverables(ctx context.Context, agentRunID, handle, stageRoot string) (*StagedDeliverables, error) {
	capability, err := e.ResolveCapability(agentRunID, handle)
	if err != nil || !capability.AgentRole.MutatesCapsule() {
		return nil, fmt.Errorf("capsule deliverables staging unavailable")
	}
	e.mu.RLock()
	caps := e.capsules[capability.TargetCapsule]
	e.mu.RUnlock()
	if caps == nil {
		return nil, fmt.Errorf("capsule deliverables staging unavailable")
	}
	caps.mu.RLock()
	state := caps.State
	caps.mu.RUnlock()
	if state != StateFrozen {
		return nil, fmt.Errorf("capsule deliverables staging requires frozen capsule")
	}
	changes, err := caps.Diff(ctx)
	if err != nil {
		return nil, err
	}
	names, links := deliverableNames(changes)
	if len(names) == 0 {
		return &StagedDeliverables{}, nil
	}
	rootFD, err := unix.Open(caps.MergedDir, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("capsule root is unavailable: %w", err)
	}
	defer unix.Close(rootFD)
	if err := os.MkdirAll(stageRoot, 0o700); err != nil {
		return nil, err
	}
	destDir, err := os.MkdirTemp(stageRoot, ".deliverables-")
	if err != nil {
		return nil, err
	}
	files, refused, err := stageDeliverableFiles(ctx, names, links, func(name string) (screenInput, error) {
		in, err := openBeneath(rootFD, DeliverablesPrefix+name)
		if err != nil {
			return nil, err
		}
		return in, nil
	}, destDir)
	if err != nil {
		_ = os.RemoveAll(destDir)
		return nil, err
	}
	return &StagedDeliverables{Dir: destDir, Files: files, Refused: refused}, nil
}
