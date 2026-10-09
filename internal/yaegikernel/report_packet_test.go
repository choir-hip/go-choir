package yaegikernel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// docs/problems/research-report-packets-rejected-by-schema-2026-10-09.md.
// Failure modes (each seen on staging, run 5): a claim written as "claim"
// instead of "text"; a source carrying "title"/"date" outside target;
// selectors as strings. Each was rejected only at reduce time, after the
// cell, so the desk lost a research round per attempt. A well-formed packet
// without schema_version must pass.

func TestReportPacketRejectsRunFiveShapesAtTheCall(t *testing.T) {
	cases := map[string]struct {
		packet string
		hint   string
	}{
		"claim not text":             {`{"kind":"evidence_update","summary":"s","claims":[{"claim":"x"}]}`, `"text"`},
		"title on source":            {`{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","kind":"web_source","title":"T","target":{"uri":"https://e.x"}}]}`, `"target"`},
		"date on source":             {`{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","kind":"web_source","date":"2026","target":{"uri":"https://e.x"}}]}`, `"target"`},
		"selector string":            {`{"kind":"evidence_update","summary":"s","sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://e.x"},"selectors":["quote"]}]}`, `"selectors"`},
		"claim cites missing source": {`{"kind":"evidence_update","summary":"s","claims":[{"text":"x","source_ids":["nope"]}]}`, "source"},
	}
	for name, tc := range cases {
		_, err := checkReportPacket(tc.packet)
		if err == nil {
			t.Fatalf("%s: accepted at the call", name)
		}
		if !strings.Contains(err.Error(), tc.hint) {
			t.Fatalf("%s: error does not show the expected shape (%s): %v", name, tc.hint, err)
		}
	}
}

func TestReportPacketAcceptsMinimalPacketAndDefaultsSchemaVersion(t *testing.T) {
	out, err := checkReportPacket(`{"kind":"evidence_update","summary":"s",
		"claims":[{"text":"x","source_ids":["s1"]}],
		"sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://e.x","title":"T"},
			"selectors":[{"kind":"text_quote","quote":"q"}],"excerpt":"e"}]}`)
	if err != nil {
		t.Fatalf("well-formed packet refused: %v", err)
	}
	var p types.CoagentSourcePacketPayload
	if err := json.Unmarshal([]byte(out), &p); err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != types.CoagentSourcePacketSchemaV1 || len(p.Sources) != 1 || len(p.Claims) != 1 {
		t.Fatalf("packet not carried through: %+v", p)
	}
}
