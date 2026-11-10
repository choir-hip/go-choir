package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/jevrecord"
)

// Failure modes of the record at the call sites (docs/evidence/
// jev-measurement-2026-11-10.md, "Build: failure list", Recording): a failed
// call leaves no record, a site's call skips the record (every call goes
// through judgeCall), the record write holds the hot path, and the decision a
// call fed is lost so no later outcome can join it.

type failingJudge struct{}

func (failingJudge) Judge(context.Context, json.RawMessage, map[string]json.RawMessage) (json.RawMessage, error) {
	return nil, errors.New("gateway: 502 Bad Gateway")
}

func jevCallsFile(rt *Runtime) string {
	return filepath.Join(filepath.Dir(rt.store.StatePath()), "jev-calls", "calls.jsonl")
}

func TestJudgeCallRecordsSuccessAndFailureWithTheDecision(t *testing.T) {
	rt, _ := testRuntime(t)
	questions := map[string]json.RawMessage{"q": json.RawMessage(`{"type":"choice","instructions":"Same event?","choices":["yes","no"]}`)}
	rt.SetCandidateJudge(&scopeJudge{})
	if _, err := rt.judgeCall(context.Background(), jevrecord.Meta{Site: "support_judge", DecisionID: "pair-1", RunID: "run-1"}, time.Second, json.RawMessage(`{}`), questions); err != nil {
		t.Fatal(err)
	}
	rt.SetCandidateJudge(failingJudge{})
	if _, err := rt.judgeCall(context.Background(), jevrecord.Meta{Site: "intake_labels", DecisionID: "cap-9", Scope: judgmentScopeIntakeLabels}, time.Second, json.RawMessage(`{}`), questions); err == nil {
		t.Fatal("the failing judge should fail the call")
	}
	rt.jevRecorder().Close()
	raw, err := os.ReadFile(jevCallsFile(rt))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{`"site":"support_judge"`, `"decision_id":"pair-1"`, `"run_id":"run-1"`, `"ok":true`,
		`"site":"intake_labels"`, `"decision_id":"cap-9"`, `"ok":false`, `502`, `"scope":"intake_labels"`, `"options":["yes","no"]`} {
		if !strings.Contains(text, want) {
			t.Fatalf("records lack %s:\n%s", want, text)
		}
	}
}

// Every site's call goes through judgeCall: no call site names callJudgeWithin
// directly any more, so none can skip the record.
func TestNoCallSiteSkipsTheRecord(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.Contains(line, "callJudgeWithin(") && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "func callJudgeWithin") {
				if name != "intake_labels.go" {
					t.Errorf("%s calls callJudgeWithin directly: %s", name, trimmed)
				}
			}
		}
	}
}
