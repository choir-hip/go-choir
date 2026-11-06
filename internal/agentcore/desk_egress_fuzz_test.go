package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	contentowner "github.com/yusefmosiah/go-choir/internal/content"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/events"
	"github.com/yusefmosiah/go-choir/internal/provider"
	"github.com/yusefmosiah/go-choir/internal/provideriface"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA6.3: each frame handler's payload, fuzzed through the real host handler
// (deskEgress), and the confused-deputy properties checked on every input: a
// verb acts with the authority of the session's desk, never with the run's,
// and never on a cell-supplied reference outside the caller's scope.
//
// Ways this could fail, written first:
//  1. a handler panics or hangs on a payload admission accepted;
//  2. a research-only verb (System1, Relate, RelateStories) serves another
//     desk;
//  3. Stories or ProductAPI serve a desk that lacks the grant because the
//     run's role holds it (the run's context is the cell's, not the frame's
//     authority);
//  4. ProductAPI serves a path the router reads differently from the route
//     check (a dot segment, an empty segment, an escaped slash), reaching a
//     route the check refuses;
//  5. Get or List read a kind the session's desk is not the writer of;
//  6. read_evidence returns another owner's record, or get_run_memory_entry
//     another run's entry, by a cell-supplied id;
//  7. list_evidence returns a page past its clamp.
//
// The research verb is not driven here: a valid call runs child model loops.
// Its payload is fuzzed by yaegikernel FuzzHandlerPayloads through the same
// ResearchChildrenBounds the handler runs.

// The fuzzed verbs, and the desks they are called from.
var (
	fuzzEgressActions = []yaegikernel.BrokerAction{
		yaegikernel.ActionSystem1, yaegikernel.ActionRelateStories, yaegikernel.ActionRelate, yaegikernel.ActionRequestLease,
		yaegikernel.ActionObjectGet, yaegikernel.ActionObjectList, yaegikernel.ActionProductAPI, yaegikernel.ActionStories,
		yaegikernel.ActionTextureSearch, yaegikernel.ActionSaveEvidence, yaegikernel.ActionReadEvidence,
		yaegikernel.ActionListEvidence, yaegikernel.ActionGetRunMemoryEntry, yaegikernel.ActionRead, yaegikernel.ActionPollSource,
		yaegikernel.ActionSourceAPI, yaegikernel.ActionReadCapture, yaegikernel.ActionSearchWireCorpus,
	}
	fuzzEgressDesks = []string{agentprofile.Management, agentprofile.ManagementTask, agentprofile.Research,
		agentprofile.Texture, "app:mail", "nonexistent", ""}
)

const (
	fuzzOwner        = "owner-fuzz"
	fuzzRun          = "run-fuzz"
	fuzzOtherEvID    = "ev-other-owner"
	fuzzOwnEvID      = "ev-own"
	fuzzOtherEntryID = "rme-other-run"
)

// fuzzEgressRuntime is one runtime for the whole fuzz run: a real store
// seeded with another owner's evidence and another run's memory entry, and a
// product API handler that records the raw path it is served.
func fuzzEgressRuntime(f *testing.F) (*Runtime, *servedPaths) {
	dir := f.TempDir()
	dbPath := filepath.Join(dir, "fuzz.db")
	s, err := openTestStore(dbPath)
	if err != nil {
		f.Fatalf("open store: %v", err)
	}
	bus := events.NewEventBus()
	rt := New(provideriface.Config{ComputerID: "autoputer-fuzz", StorePath: dbPath, PromptRoot: filepath.Join(dir, "prompts"),
		ProviderTimeout: time.Second}, s, bus, provider.NewStubProvider(0), WithContentService(contentowner.NewService(s, bus)))
	setTestDispatch(rt, s)
	f.Cleanup(func() {
		rt.Stop()
		_ = s.Close()
		_ = os.RemoveAll(dir)
	})
	ctx := context.Background()
	for _, ev := range []types.EvidenceRecord{
		{EvidenceID: fuzzOtherEvID, OwnerID: "owner-other", AgentID: "agent-other", Kind: "note", Content: "not yours", CreatedAt: time.Now().UTC()},
		{EvidenceID: fuzzOwnEvID, OwnerID: fuzzOwner, AgentID: "agent-fuzz", Kind: "note", Content: "yours", CreatedAt: time.Now().UTC()},
	} {
		if err := s.CreateEvidence(ctx, ev); err != nil {
			f.Fatalf("seed evidence: %v", err)
		}
	}
	if _, err := s.AppendRunMemoryEntry(ctx, types.RunMemoryEntry{EntryID: fuzzOtherEntryID, RunID: "run-other", OwnerID: fuzzOwner,
		Kind: types.RunMemoryEntryMessage, Role: "user", Message: json.RawMessage(`{"role":"user","content":"x"}`), CreatedAt: time.Now().UTC()}); err != nil {
		f.Fatalf("seed run memory: %v", err)
	}
	// Turn on the owner settings the verbs check before their decode, so the
	// fuzzer reaches the decode and the scope checks.
	for _, key := range []string{store.SettingIntakeLabels, store.SettingResearchRelatedStory} {
		if err := s.PutComputerSetting(ctx, store.ComputerSetting{OwnerID: fuzzOwner, ComputerID: "autoputer-fuzz", Key: key, Value: true}); err != nil {
			f.Fatalf("seed setting %s: %v", key, err)
		}
	}
	served := &servedPaths{}
	rt.SetProductAPIHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.add(r.URL.Path, r.URL.EscapedPath())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}), "autoputer-fuzz")
	return rt, served
}

type servedPaths struct {
	mu    sync.Mutex
	paths [][2]string
}

func (s *servedPaths) add(p, escaped string) {
	s.mu.Lock()
	s.paths = append(s.paths, [2]string{p, escaped})
	s.mu.Unlock()
}

func (s *servedPaths) take() [][2]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.paths
	s.paths = nil
	return out
}

func FuzzDeskEgressHandlers(f *testing.F) {
	rt, served := fuzzEgressRuntime(f)
	seed := func(action yaegikernel.BrokerAction, desk int, payload string) {
		for i, a := range fuzzEgressActions {
			if a == action {
				f.Add(uint8(i), uint8(desk), []byte(payload))
			}
		}
	}
	for i := range fuzzEgressActions {
		f.Add(uint8(i), uint8(0), []byte(`{}`))
		f.Add(uint8(i), uint8(2), []byte(`{"query":"q","limit":3}`))
	}
	seed(yaegikernel.ActionSystem1, 0, `{"questions":[{"id":"q1","text":"Is it?","choices":["yes","no"],"context_refs":["choir.web_capture:abc"]}]}`)
	seed(yaegikernel.ActionSystem1, 2, `{"questions":[{"id":"q1","text":"Is it?","choices":["yes","no"],"context_refs":["host:request"]}]}`)
	seed(yaegikernel.ActionRelate, 2, `{"story":"story:head","entities":[{"name":"Ada","role":"subject","type":"person"}]}`)
	seed(yaegikernel.ActionProductAPI, 0, `{"method":"GET","path":"/api/x/../leases"}`)
	seed(yaegikernel.ActionProductAPI, 0, `{"method":"GET","path":"/api//leases"}`)
	seed(yaegikernel.ActionProductAPI, 0, `{"method":"GET","path":"/api/texture%2fdocuments/../../leases"}`)
	seed(yaegikernel.ActionProductAPI, 0, `{"method":"POST","path":"/api/autopaper/sources/schedules/./"}`)
	seed(yaegikernel.ActionProductAPI, 0, `{"method":"GET","path":"/api/autopaper/../leases"}`)
	seed(yaegikernel.ActionProductAPI, 0, `{"method":"POST","path":"/api/run-acceptances/../texture/documents/d1"}`)
	seed(yaegikernel.ActionProductAPI, 0, `{"method":"GET","path":"/api/computers/x%2f..%2f..%2fagent/runs"}`)
	seed(yaegikernel.ActionProductAPI, 0, `{"method":"GET","path":"/api/texture/documents"}`)
	seed(yaegikernel.ActionProductAPI, 2, `{"method":"GET","path":"/api/texture/documents"}`)
	seed(yaegikernel.ActionStories, 2, `{"limit":5}`)
	seed(yaegikernel.ActionStories, 4, `{}`)
	seed(yaegikernel.ActionObjectGet, 4, `{"kind":"choir.texture_document","id":"x"}`)
	seed(yaegikernel.ActionObjectList, 3, `{"kind":"choir.source_schedule"}`)
	seed(yaegikernel.ActionReadEvidence, 2, `{"evidence_id":"`+fuzzOtherEvID+`"}`)
	seed(yaegikernel.ActionReadEvidence, 2, `{"evidence_id":"`+fuzzOwnEvID+`"}`)
	seed(yaegikernel.ActionListEvidence, 2, `{"agent_id":"agent-other","limit":100000}`)
	seed(yaegikernel.ActionGetRunMemoryEntry, 2, `{"entry_id":"`+fuzzOtherEntryID+`"}`)
	seed(yaegikernel.ActionSaveEvidence, 2, `{"kind":"note","content":"c","metadata":{"labels":["world"]}}`)
	seed(yaegikernel.ActionRequestLease, 3, `{"kinds":["choir.web_capture"],"purpose":"p","req":null}`)

	f.Fuzz(func(t *testing.T, which, deskIdx uint8, payload []byte) {
		action := fuzzEgressActions[int(which)%len(fuzzEgressActions)]
		desk := fuzzEgressDesks[int(deskIdx)%len(fuzzEgressDesks)]
		// Admission's bound check runs first, as in the session's Egress.
		if yaegikernel.ValidatePayload(action, payload) != nil {
			return
		}
		// The run's role is management's (it holds every grant a desk can
		// hold here): any grant check that reads the run instead of the
		// session's desk serves the wrong desk and is caught below.
		exec := toolregistry.ExecutionContext{RunID: fuzzRun, AgentID: "agent-fuzz", OwnerID: fuzzOwner, ComputerID: "autoputer-fuzz",
			Role: agentprofile.Management, Profile: agentprofile.Management}
		ctx, cancel := context.WithTimeout(toolregistry.WithExecutionContext(context.Background(), exec), 5*time.Second)
		defer cancel()
		_ = served.take()
		type answer struct {
			out json.RawMessage
			err error
		}
		done := make(chan answer, 1)
		go func() {
			out, err := rt.deskEgress(ctx, desk, exec, action, payload)
			done <- answer{out, err}
		}()
		var got answer
		select {
		case got = <-done:
		case <-time.After(15 * time.Second):
			t.Fatalf("%s from %q did not return", action, desk)
		}
		paths := served.take()
		for _, p := range paths {
			if err := checkServedProductPath(p[0], p[1]); err != "" {
				t.Fatalf("product_api from %q served %q (escaped %q): %s; payload %s", desk, p[0], p[1], err, payload)
			}
		}
		if got.err != nil {
			return
		}
		switch action {
		case yaegikernel.ActionSystem1, yaegikernel.ActionRelate, yaegikernel.ActionRelateStories:
			if desk != agentprofile.Research {
				t.Fatalf("%s served desk %q", action, desk)
			}
		case yaegikernel.ActionProductAPI:
			if !deskmanifest.Holds(desk, "product.api") {
				t.Fatalf("product_api served desk %q, which lacks the grant", desk)
			}
		case yaegikernel.ActionStories:
			if !deskmanifest.Holds(desk, "intake.read") {
				t.Fatalf("stories served desk %q, which lacks the grant (the run's role holds it)", desk)
			}
		case yaegikernel.ActionObjectGet, yaegikernel.ActionObjectList:
			var req struct {
				Kind string `json:"kind"`
			}
			_ = json.Unmarshal(payload, &req)
			if _, err := resolveDeskObject(desk, req.Kind); err != nil {
				t.Fatalf("%s served desk %q kind %q, which it does not write: %v", action, desk, req.Kind, err)
			}
		case yaegikernel.ActionReadEvidence:
			var rec struct {
				OwnerID string `json:"owner_id"`
			}
			_ = json.Unmarshal(got.out, &rec)
			if rec.OwnerID != fuzzOwner {
				t.Fatalf("read_evidence returned a record of owner %q", rec.OwnerID)
			}
		case yaegikernel.ActionGetRunMemoryEntry:
			var entry struct {
				RunID string `json:"loop_id"`
			}
			_ = json.Unmarshal(got.out, &entry)
			if entry.RunID != fuzzRun {
				t.Fatalf("get_run_memory_entry returned an entry of run %q", entry.RunID)
			}
		case yaegikernel.ActionListEvidence:
			var page struct {
				Items []json.RawMessage `json:"items"`
			}
			_ = json.Unmarshal(got.out, &page)
			if len(page.Items) > maxListEvidenceLimit {
				t.Fatalf("list_evidence returned %d items", len(page.Items))
			}
		}
	})
}

// checkServedProductPath names why a path the product API handler was served
// breaks the route check, or "".
func checkServedProductPath(p, escaped string) string {
	clean := pathpkg.Clean(p)
	if strings.HasSuffix(p, "/") && clean != "/" {
		clean += "/"
	}
	if clean != p {
		return "not canonical"
	}
	if strings.Contains(strings.ToLower(escaped), "%2f") {
		return "an escaped slash"
	}
	for _, blocked := range []string{"/internal/", "/api/agent/", "/api/prompts", "/api/test/", "/api/leases", "/api/disclosures"} {
		if p == strings.TrimSuffix(blocked, "/") || strings.HasPrefix(p, blocked) {
			return "a blocked route"
		}
	}
	return ""
}
