package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// TestDeskGoEvalOwnTimeoutNamesItself reproduces R2 (desk-cell-ergonomics
// 2026-10-10): research set timeout_ms=2000 on a cell whose web search took
// longer, and the result said only "context deadline exceeded" with duration
// 0. The result must name the cell's own limit, the time spent, the lost
// session, and the calls that take time.
func TestDeskGoEvalOwnTimeoutNamesItself(t *testing.T) {
	deskTestWorkerBin(t)
	workers := newDeskSessionWorkers()
	tool := newDeskGoEvalTool(&Runtime{}, workers, agentprofile.Research)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile = agentprofile.Research
	execCtx.Role = agentprofile.Research
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)

	src := `import "time"
time.Sleep(3 * time.Second)`
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+cellSource(src)+`,"timeout_ms":500}`))
	if err != nil {
		t.Fatalf("desk_go_eval returned error: %v", err)
	}
	var got struct {
		Error    string `json:"error"`
		Duration int64  `json:"duration"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("decode %s: %v", out, err)
	}
	for _, want := range []string{"timeout_ms=500", "session was killed", "choir.Read", "choir.SearchWait"} {
		if !strings.Contains(got.Error, want) {
			t.Fatalf("timeout error %q does not name %q", got.Error, want)
		}
	}
	if got.Duration < 500 {
		t.Fatalf("duration = %d ms, want the time the cell ran", got.Duration)
	}
}
