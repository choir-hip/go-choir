package agentcore

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/jevrecord"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
)

// The desk on a Jev call (first design, F0). Failure list, written before the code:
//  1. a host pipeline's call (intake labels, event layer, front importance, oped gate) is not host;
//  2. a call made outside any desk's run is not host, so it logs unknown;
//  3. a call inside a desk's run is made host, hiding the desk;
//  4. a host call drops the record's run id.
type callerJudge struct {
	mu      sync.Mutex
	callers []provideriface.Caller
	execs   []toolregistry.ExecutionContext
}

func (j *callerJudge) Judge(ctx context.Context, state json.RawMessage, q map[string]json.RawMessage) (json.RawMessage, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.callers = append(j.callers, provideriface.CallerFrom(ctx))
	j.execs = append(j.execs, toolregistry.ExecutionContextFrom(ctx))
	return json.RawMessage(`{}`), nil
}

func TestJudgeCallNamesHostCodeAndLeavesADesksRunItsOwnDesk(t *testing.T) {
	for _, site := range []string{"intake_labels", "event_layer", "front_importance", "oped_gate"} {
		j := &callerJudge{}
		rt := &Runtime{candidateJudge: j}
		_, _ = rt.judgeCall(context.Background(), jevrecord.Meta{Site: site, RunID: "run-h"}, time.Second, json.RawMessage(`{}`), nil)
		if len(j.callers) != 1 || j.callers[0].Desk != provideriface.DeskHost || j.callers[0].RunID != "run-h" {
			t.Fatalf("(1)(4) site %s: caller = %+v", site, j.callers)
		}
	}
	// (2) any other site outside a run is host code's.
	j := &callerJudge{}
	rt := &Runtime{candidateJudge: j}
	_, _ = rt.judgeCall(context.Background(), jevrecord.Meta{Site: "story_analysis"}, time.Second, json.RawMessage(`{}`), nil)
	if len(j.callers) != 1 || j.callers[0].Desk != provideriface.DeskHost {
		t.Fatalf("(2) a call outside a run: %+v", j.callers)
	}
	// (3) inside a desk's run: no caller override; the execution context names the desk.
	j = &callerJudge{}
	rt = &Runtime{candidateJudge: j}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{RunID: "run-m", Role: "management"})
	_, _ = rt.judgeCall(ctx, jevrecord.Meta{Site: "subcall_judge"}, time.Second, json.RawMessage(`{}`), nil)
	if len(j.callers) != 1 || j.callers[0].Desk != "" || j.execs[0].Role != "management" {
		t.Fatalf("(3) a desk's call was made host: %+v %+v", j.callers, j.execs)
	}
}
