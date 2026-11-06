package agentcore

import (
	"encoding/json"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA6.3 confused-deputy fixes, one test each. Ways these could fail, written
// first: a path the route check reads one way reaches the router another way;
// a canonical path is refused (a real call breaks); a System1 call that makes
// the judge read a capture leaves the run untainted, or one that names only
// the cell's own passages taints it.

func TestProductAPIPathIsCheckedAsItIsServed(t *testing.T) {
	for _, p := range []string{
		"/api/autopaper/../leases",
		"/api/run-acceptances/../texture/documents/d1",
		"/api//leases",
		"/api/autopaper/./sources/schedules",
		"/api/computers/x%2f..%2fagent",
		"/api/computers/x%2Fy",
		`/api/autopaper/a\b`,
	} {
		if norm, err := normalizeProductAPIPath(p); err == nil {
			t.Errorf("%q accepted as %q", p, norm)
		}
	}
	for _, p := range []string{
		"/api/texture/documents",
		"/api/autopaper/sources/schedules",
		"/api/autopaper/sources/schedules/",
		"/api/computers/c1/operations?limit=5&cursor=a%2Fb",
		"https://choir.news/api/run-acceptances/r1",
	} {
		if _, err := normalizeProductAPIPath(p); err != nil {
			t.Errorf("canonical %q refused: %v", p, err)
		}
	}
}

func TestSystem1OverACaptureTaintsTheRun(t *testing.T) {
	payload := func(refs []string, passages []string) json.RawMessage {
		b, _ := json.Marshal(yaegikernel.System1Payload{Questions: []yaegikernel.System1Question{{
			ID: "q1", Text: "Is it?", Choices: []string{"yes", "no"}, ContextRefs: refs, Passages: passages,
		}}})
		return b
	}
	labels, reads := egressReadLabels(yaegikernel.ActionSystem1, payload([]string{"choir.web_capture:abc"}, nil), nil)
	if !reads || !labels.Has(types.OwnerPrivateLabel("capture")) || !labels.Has(types.LabelWorldUntrusted) {
		t.Fatalf("a System1 call over a capture: labels %v reads %v, want the capture read floor", labels, reads)
	}
	if _, reads := egressReadLabels(yaegikernel.ActionSystem1, payload(nil, []string{"the cell's own text"}), nil); reads {
		t.Fatal("a System1 call over the cell's own passages taints the run")
	}
	if _, reads := egressReadLabels(yaegikernel.ActionSystem1, payload([]string{"host:request", "text:00"}, nil), nil); reads {
		t.Fatal("host and passage refs read no capture")
	}
}
