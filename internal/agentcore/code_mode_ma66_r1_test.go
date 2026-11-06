package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/search"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA6.6 review r1, the agentcore half. Failure modes, written first:
//
//	R1 (S1) a search's results reach the cell before the run is tainted, so a
//	   sibling SaveEvidence stamps its record without world_untrusted;
//	R2 (S2) an aborted cell's RequestLease is granted after the abort;
//	R3 (S4) a lower story-library head stored last overwrites a higher one;
//	R4 (D3) two polls of one source in one run run at once;
//	R5 (D1) the sinks the gate judges and the sinks the frame loop orders
//	   are two lists that drift.

// labelProbeSearch is a search client that records whether the run was
// already tainted world_untrusted when the search started.
type labelProbeSearch struct {
	rt      *Runtime
	runID   string
	tainted atomic.Int32 // 1 tainted, 2 not
}

func (c *labelProbeSearch) Stream(ctx context.Context, query string, maxResults int) (*search.Stream, error) {
	if c.rt.runLabelSet(c.runID).Has(types.LabelWorldUntrusted) {
		c.tainted.Store(1)
	} else {
		c.tainted.Store(2)
	}
	return stubSearchClient{}.Stream(ctx, query, maxResults)
}

func TestMA66r1SearchTaintsTheRunBeforeItStarts(t *testing.T) {
	deskTestWorkerBin(t)
	rt, _ := testRuntime(t)
	execCtx := deskEvalExecCtx(t)
	execCtx.Profile, execCtx.Role = agentprofile.Research, agentprofile.Research
	execCtx.RunID = "run-ma66r1-search"
	execCtx.RunRecord = &types.RunRecord{RunID: execCtx.RunID, AgentID: "research:r1", OwnerID: execCtx.OwnerID}
	probe := &labelProbeSearch{rt: rt, runID: execCtx.RunID}
	rt.researchDeps = &researchtools.Dependencies{Search: probe}
	workers := newDeskSessionWorkers()
	t.Cleanup(workers.closeAll)
	tool := newDeskGoEvalTool(rt, workers, agentprofile.Research)
	ctx := toolregistry.WithExecutionContext(context.Background(), execCtx)
	src, _ := json.Marshal(`s, err := choir.SearchStart("harbour")
if err == nil { _, err = choir.SearchWait(s.ID, 5000) }
fmt.Println("searched", err)`)
	out, err := tool.Func(ctx, json.RawMessage(`{"source":`+string(src)+`,"timeout_ms":30000}`))
	if err != nil || !strings.Contains(out, `searched \u003cnil\u003e`) {
		t.Fatalf("cell: %s %v", out, err)
	}
	// R1: the search saw a tainted run, so nothing the cell did beside it
	// could be stamped without world_untrusted.
	if probe.tainted.Load() != 1 {
		t.Fatal("the search started before the run was tainted world_untrusted")
	}
}

func TestMA66r1AnAbortedCellIsGrantedNoLease(t *testing.T) {
	rt, _ := testRuntime(t)
	registerLeaseApp(t, "ma66r1lease", 62866, true)
	rec := leaseRunRecord("ma66r1lease", "run-ma66r1-lease")
	exec := toolregistry.ExecutionContext{RunID: rec.RunID, OwnerID: rec.OwnerID, ComputerID: rec.ComputerID, AgentID: rec.AgentID, RunRecord: rec}
	payload, _ := json.Marshal(yaegikernel.RequestLeasePayload{CaptureIDs: []string{"c1"}, Purpose: "read", Req: leaseFields()})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := rt.requestLeaseEgress(ctx, "app:ma66r1lease", exec, payload); err == nil || !strings.Contains(err.Error(), "aborted") {
		t.Fatalf("an aborted cell's lease request: %v", err)
	}
	if live := rt.leases.Live(rec.OwnerID); len(live) != 0 {
		t.Fatalf("an aborted cell holds leases: %+v", live)
	}
	// The same request from a live cell is granted (the refusal is the abort).
	if _, err := rt.requestLeaseEgress(context.Background(), "app:ma66r1lease", exec, payload); err != nil {
		t.Fatalf("a live cell's lease request: %v", err)
	}
}

func TestMA66r1StoryLibraryMarkKeepsTheHighestHead(t *testing.T) {
	rt, _ := testRuntime(t)
	rt.storyLibraryShown.Store("run-mark", int64(1000))
	// The store's head is far below 1000: a call that read it last must not
	// lower the mark.
	rt.noteStoryLibraryShown(context.Background(), "run-mark", "owner-1", "computer-1")
	if got := rt.storyLibraryShownMark("run-mark"); got != 1000 {
		t.Fatalf("mark = %d, want 1000", got)
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rt.noteStoryLibraryShown(context.Background(), "run-mark-2", "owner-1", "computer-1")
		}()
	}
	wg.Wait()
}

func TestMA66r1PollsOfOneSourceAreSerialWithinARun(t *testing.T) {
	rt := &Runtime{}
	payload := func(src string) json.RawMessage {
		raw, _ := json.Marshal(yaegikernel.PollSourcePayload{SourceID: src})
		return raw
	}
	var inside, peak atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer rt.lockPollSource("run-p", payload("rss:a"))()
			n := inside.Add(1)
			if n > peak.Load() {
				peak.Store(n)
			}
			time.Sleep(5 * time.Millisecond)
			inside.Add(-1)
		}()
	}
	wg.Wait()
	if peak.Load() != 1 {
		t.Fatalf("polls of one source in flight at once: %d", peak.Load())
	}
	// Another source, or another run, is not held up.
	unlock := rt.lockPollSource("run-p", payload("rss:a"))
	done := make(chan struct{})
	go func() {
		rt.lockPollSource("run-p", payload("rss:b"))()
		rt.lockPollSource("run-q", payload("rss:a"))()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("a poll of another source or run waited")
	}
	unlock()
	rt.pollSources.mu.Lock()
	n := len(rt.pollSources.locks)
	rt.pollSources.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d poll locks left after every unlock", n)
	}
}

func TestMA66r1DisclosureSinksAreTheFrameLoopsSinks(t *testing.T) {
	actions := []yaegikernel.BrokerAction{
		yaegikernel.ActionSearchStart, yaegikernel.ActionRead, yaegikernel.ActionSearchWireCorpus, yaegikernel.ActionTextureSearch,
		yaegikernel.ActionSaveEvidence, yaegikernel.ActionReadEvidence, yaegikernel.ActionListEvidence, yaegikernel.ActionGetRunMemoryEntry,
		yaegikernel.ActionProductAPI, yaegikernel.ActionResearch, yaegikernel.ActionPollSource, yaegikernel.ActionReadCapture,
		yaegikernel.ActionStories, yaegikernel.ActionSystem1, yaegikernel.ActionRelateStories, yaegikernel.ActionRelate,
		yaegikernel.ActionSourceAPI, yaegikernel.ActionRequestLease, yaegikernel.ActionObjectGet, yaegikernel.ActionObjectList,
	}
	for _, a := range actions {
		_, _, gated := egressDisclosure(a, json.RawMessage(`{}`))
		if gated != yaegikernel.DisclosureSinkActions[a] {
			t.Errorf("%s: the gate judges it a sink %v, the frame loop %v", a, gated, yaegikernel.DisclosureSinkActions[a])
		}
	}
}
