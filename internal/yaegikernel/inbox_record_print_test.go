package yaegikernel

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// B13: a cell that prints its records must see their bodies. The default
// print of a pointer field is an address, so a capture arrival printed as
// Captures:0x1daabebf and management concluded the items were lost.
func TestInboxRecordPrintsItsBodies(t *testing.T) {
	rs := []InboxRecord{{RecordID: "capture-arrival:rss:x:1", Kind: "captures", From: "source:rss:x",
		Captures: &types.CommitmentCaptures{SourceID: "rss:x", Items: []types.CaptureArrival{{CaptureID: "cap-1", Title: "A headline", URL: "https://example.com/a"}}}}}
	for _, verb := range []string{"%v", "%+v"} {
		out := fmt.Sprintf(verb, rs)
		if !strings.Contains(out, "cap-1") || !strings.Contains(out, "A headline") || strings.Contains(out, "0x") {
			t.Fatalf("%s print of records hides their captures: %s", verb, out)
		}
	}
}

// Engineering reads the Texture request in the Cast that opened its
// assignment: the request survives the frame's JSON hop to the worker and is
// readable in a cell as r.Request.Objective / r.Request.Body, and a printed
// record shows it.
func TestInboxRecordCarriesTheTextureRequestIntoACell(t *testing.T) {
	in := SessionFrame{ID: "cell", Source: `for _, r := range choir.Records() {
	fmt.Println(r.Request.From, "|", r.Request.Objective, "|", r.Request.Body, "|", r.Request.Truncated)
}
fmt.Println(choir.Records()[0].RequestError == "")`,
		Records: []InboxRecord{{Seq: 1, RecordID: "rec-1", Kind: "cast", From: "management:o",
			Request: &InboxRequest{RecordID: "texture-cast", From: "texture:doc", Objective: "write the probe line", Body: `{"path":"docs/probes/x.md"}`, Digest: "sha256:x"}}}}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var frame SessionFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatal(err)
	}
	_, _, scope, _ := testChoirFixture(t)
	scope.desk = "engineering"
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	out, err := serveCell(sess, frame, nil, &hooks)
	want := "texture:doc | write the probe line | {\"path\":\"docs/probes/x.md\"} | false\ntrue\n"
	if err != nil || out.Error != "" || out.Stdout != want {
		t.Fatalf("cell read of the request: %v %q stdout %q", err, out.Error, out.Stdout)
	}
	if s := frame.Records[0].String(); !strings.Contains(s, "write the probe line") || !strings.Contains(s, `"request"`) {
		t.Fatalf("a printed record hides the request: %s", s)
	}
}
