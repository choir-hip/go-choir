package yaegikernel

import (
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
