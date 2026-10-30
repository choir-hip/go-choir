package agentcore

import (
	"errors"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/capsule/transaction"
)

// Verification binds a reused release binary to this computer's running
// build (docs/archive/problems/selfdev-drill-capsule-cannot-build-autoputer-2026-10-14.md, A5).
// Ways it could fail: a deploy restarted the computer between Freeze and
// Verify, so the reused binary is not the running build and a pass would
// approve a release that names the wrong executable; the running identity is
// unreadable and the check passes by default; a capsule-built or legacy
// bundle is wrongly held to the running build.
func TestVerifyPassBindsReusedReleaseBinaryToTheRunningBuild(t *testing.T) {
	commit := "0123456789abcdef0123456789abcdef01234567"
	digest := strings.Repeat("a", 64)
	reused := transaction.CapsuleEffectBundle{ReleaseBinary: &capsule.ReleaseBinaryProvenance{
		Origin: capsule.ReleaseBinaryReusedRunning, SHA256: digest, RunningCommit: commit, SourceBaseCommit: commit,
	}}
	running := func(sha, c string) *CapsuleToolCtx {
		return &CapsuleToolCtx{RunningBinary: func() (capsule.RunningBinary, error) {
			return capsule.RunningBinary{Path: "/proc/self/exe", SHA256: sha, Commit: c}, nil
		}}
	}
	if err := releaseBinaryStillRunning(running(digest, commit), reused); err != nil {
		t.Fatalf("matching running build refused: %v", err)
	}
	if err := releaseBinaryStillRunning(running(strings.Repeat("b", 64), strings.Repeat("9", 40)), reused); err == nil {
		t.Fatal("a reused binary that is no longer the running build passed")
	}
	unreadable := &CapsuleToolCtx{RunningBinary: func() (capsule.RunningBinary, error) {
		return capsule.RunningBinary{}, errors.New("open /proc/self/exe: permission denied")
	}}
	if err := releaseBinaryStillRunning(unreadable, reused); err == nil {
		t.Fatal("an unreadable running identity passed a reused binary")
	}
	built := transaction.CapsuleEffectBundle{ReleaseBinary: &capsule.ReleaseBinaryProvenance{Origin: capsule.ReleaseBinaryCapsuleBuilt, SHA256: digest}}
	if err := releaseBinaryStillRunning(unreadable, built); err != nil {
		t.Fatalf("capsule-built binary held to the running build: %v", err)
	}
	if err := releaseBinaryStillRunning(unreadable, transaction.CapsuleEffectBundle{}); err != nil {
		t.Fatalf("legacy bundle held to the running build: %v", err)
	}
}
