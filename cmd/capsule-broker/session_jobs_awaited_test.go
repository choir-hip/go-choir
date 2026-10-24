//go:build linux

package main

import (
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsulejob"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// The jobs a cell's Poll returned unfinished ride the go_eval result to
// guest core, which parks the turn on them
// (problems/engineering-busy-polls-its-own-job-2026-10-24.md); dropping
// them here would leave the busy poll in place.
func TestSessionGoEvalResultCarriesJobsAwaited(t *testing.T) {
	got := sessionGoEvalResult(yaegikernel.SessionResult{Stdout: "running", RecordsShown: 7, JobsAwaited: []string{"job-build"}}, time.Second)
	if strings.Join(got.JobsAwaited, ",") != "job-build" || got.RecordsShown != 7 || got.Stdout != "running" || got.Duration != time.Second {
		t.Fatalf("result = %+v", got)
	}
}

// End to end through the real worker binary: a Poll of a running job in an
// engineering cell comes back as awaited.
func TestSessionWorkerReportsAwaitedJobsEndToEnd(t *testing.T) {
	bin := buildBrokerBinary(t)
	w, err := spawnSessionWorker(bin, workerSessionConfig{
		computerID: "test-capsule", epoch: 1, activation: "run-jobs",
		allowedRoot: t.TempDir(), timeout: 60 * time.Second, role: "engineering",
	})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer w.close()
	if res := evalCell(t, w, `import "choir"`); res.Error != "" {
		t.Fatalf("import choir: %s", res.Error)
	}
	res, err := w.eval(`st, _ := choir.Poll("job-build")
println(st.State)`, nil, nil, nil, []yaegikernel.JobView{{ID: "job-build", State: capsulejob.StateRunning}}, 60*time.Second)
	if err != nil || res.Error != "" {
		t.Fatalf("eval: %v %s", err, res.Error)
	}
	if strings.Join(res.JobsAwaited, ",") != "job-build" {
		t.Fatalf("awaited = %v, want [job-build]", res.JobsAwaited)
	}
}
