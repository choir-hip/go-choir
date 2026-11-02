package agentcore

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

const delegateStartCell = `spec := map[string]any{"objective": "x"}
raw, _ := json.Marshal(spec)
r, err := choir.Exec("choir-delegate", []string{"start", "--spec-json", string(raw)})`

const delegateDoneOut = `{
  "id": "run-1",
  "state": "done",
  "result": {"subtype": "success", "num_turns": 7}
}`

func TestDelegateCellCountsStartsAndSuccessesByRunID(t *testing.T) {
	if s, d := delegateCellCalls(`choir.Exec("go", []string{"build"})`, ""); s != 0 || len(d) != 0 {
		t.Fatalf("plain cell = %d %v", s, d)
	}
	if s, _ := delegateCellCalls(delegateStartCell, `{"ok": true, "model": "claude-sonnet-5-5"}`); s != 1 {
		t.Fatalf("start cell starts = %d", s)
	}
	_, done := delegateCellCalls(`r, _ := choir.Exec("choir-delegate", []string{"wait", "--id", "run-1"})`, delegateDoneOut)
	if len(done) != 1 || done[0] != "run-1" {
		t.Fatalf("done = %v", done)
	}
	failed := strings.Replace(delegateDoneOut, `"subtype": "success"`, `"subtype": "error_max_turns"`, 1)
	if _, done := delegateCellCalls(`choir.Exec("choir-delegate", []string{"wait"})`, failed); len(done) != 0 {
		t.Fatalf("a non-success run counted: %v", done)
	}
}

func TestDelegateUseRecordsPatchAndLogsPatchWithoutDelegateOnce(t *testing.T) {
	rt, _ := testRuntime(t)
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	// Delegate available, a frozen patch, zero delegate calls.
	rt.noteDelegateCell("a1", 1, `choir.Exec("go", []string{"test"})`, "", false)
	rt.noteDelegateCell("a1", 1, "", "", true)
	use := rt.delegateUseFor("a1", 1, true)
	if use == nil || !use.Available || !use.SourcePatch || use.Calls != 0 || use.Succeeded != 0 {
		t.Fatalf("use = %+v", use)
	}
	if got := strings.Count(buf.String(), "engineering: source patch without delegate"); got != 1 {
		t.Fatalf("log line count = %d in %q", got, buf.String())
	}

	// A patch made through the delegate logs nothing and names the model.
	buf.Reset()
	rt.noteDelegateCell("a2", 1, delegateStartCell, `{"ok": true}`, false)
	rt.noteDelegateCell("a2", 1, `choir.Exec("choir-delegate", []string{"wait"})`, delegateDoneOut, false)
	rt.noteDelegateCell("a2", 1, "", "", true)
	use = rt.delegateUseFor("a2", 1, true)
	if use == nil || use.Calls != 1 || use.Succeeded != 1 || use.Model != "claude-sonnet-5-5" || !use.SourcePatch {
		t.Fatalf("use = %+v", use)
	}
	if buf.Len() != 0 {
		t.Fatalf("unexpected log %q", buf.String())
	}

	// No patch, no delegate call: nothing to log. Unavailable: no record at all.
	rt.noteDelegateCell("a3", 1, `choir.Exec("curl", []string{"x"})`, "", false)
	if use = rt.delegateUseFor("a3", 1, true); use == nil || use.SourcePatch || buf.Len() != 0 {
		t.Fatalf("api-call assignment: %+v %q", use, buf.String())
	}
	rt.noteDelegateCell("a4", 1, "", "", true)
	if use = rt.delegateUseFor("a4", 1, false); use != nil || buf.Len() != 0 {
		t.Fatalf("delegate unavailable must leave behavior unchanged: %+v %q", use, buf.String())
	}
}
