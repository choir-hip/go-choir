package agentcore

import (
	"bytes"
	"log"
	"regexp"
	"testing"
	"time"
)

// The saga phase line is log only (docs/problems/capsule-coldstart-18s-2026-10-22.md).
// It fails its purpose if it lacks the capsule id the executor's spawn lines
// use, the phase, a duration or the assignment, or if it does not hand back
// the time the next phase starts from.
func TestAssignmentSagaPhaseLogsUnderTheCapsule(t *testing.T) {
	var buf bytes.Buffer
	previous, flags := log.Writer(), log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() { log.SetOutput(previous); log.SetFlags(flags) })

	started := time.Now().Add(-1500 * time.Millisecond)
	next := logAssignmentSagaPhase("capsule-abc", "assignment-1", "resume_preflight", started)
	if !next.After(started) {
		t.Fatalf("next phase start %v is not after %v", next, started)
	}
	if !regexp.MustCompile(`^capsule capsule-abc saga phase=resume_preflight took=1\.5\d*s assignment=assignment-1\n$`).MatchString(buf.String()) {
		t.Fatalf("line = %q", buf.String())
	}
}
