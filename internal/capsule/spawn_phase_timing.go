package capsule

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// spawnPhaseTimer logs one line per capsule spawn phase with its duration,
// under the capsule id, as each phase ends, so a deployed log shows where a
// cold start's seconds go and a hang shows the last phase that finished
// (docs/problems/capsule-coldstart-18s-2026-10-22.md). Log lines only: a nil
// timer is silent, and no phase changes what the spawn does.
type spawnPhaseTimer struct {
	capsuleID   string
	start, last time.Time
}

func newSpawnPhaseTimer(capsuleID string) *spawnPhaseTimer {
	now := time.Now()
	return &spawnPhaseTimer{capsuleID: capsuleID, start: now, last: now}
}

// phase logs the time since the previous phase (or the start) as name.
func (t *spawnPhaseTimer) phase(name string, detail ...string) {
	if t == nil {
		return
	}
	now := time.Now()
	suffix := ""
	if len(detail) > 0 {
		suffix = " " + strings.Join(detail, " ")
	}
	log.Printf("capsule %s spawn phase=%s took=%s%s", t.capsuleID, name, now.Sub(t.last).Round(time.Millisecond), suffix)
	t.last = now
}

// done logs the whole spawn's duration and whether it succeeded.
func (t *spawnPhaseTimer) done(err error) {
	if t == nil {
		return
	}
	total := time.Since(t.start).Round(time.Millisecond)
	if err != nil {
		log.Printf("capsule %s spawn total=%s ok=false err=%v", t.capsuleID, total, err)
		return
	}
	log.Printf("capsule %s spawn total=%s ok=true", t.capsuleID, total)
}

func gitProcsDetail(files int) []string {
	return []string{fmt.Sprintf("files=%d", files), fmt.Sprintf("git_procs=%d", files)}
}
