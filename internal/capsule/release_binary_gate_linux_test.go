//go:build linux

package capsule

import "testing"

// The same gate through the executor's own differ path: Freeze reads
// reuseDiffer(), so a gate installed at construction and turned on later must
// reach it.
func TestExecutorReuseDifferFollowsALiveGate(t *testing.T) {
	repo, base, frontend, goCommit := gateTestRepo(t)
	executor := &Executor{sourceDir: repo}
	if executor.reuseDiffer() != nil {
		t.Fatal("an executor with no gate must build no differ")
	}
	checkGateGoesLive(t, base, frontend, goCommit, executor.SetBinaryReuseGate, executor.reuseDiffer)
}
