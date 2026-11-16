package yaegikernel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/types"
)

// A packet is read, not refused, for two claim fields the desk keeps sending
// as what they mean (docs/problems/texture-story-ask-path-writes-no-fast-v1-
// and-research-delivers-once-2026-11-16.md): "source_refs" is source_ids, and
// a claim's "status" (the research prompt asks for one per claim) is kept in
// its text. Two refusals cost the owner's run about a minute. Every other
// unknown field is still refused, so no field is dropped silently. No note is
// added to the cell result: the platform reads the two fields and says nothing.

func stagedReports(intents []StagedIntent) []StagedIntent {
	var out []StagedIntent
	for _, in := range intents {
		if in.Kind == IntentReport {
			out = append(out, in)
		}
	}
	return out
}

func packetOf(t *testing.T, in StagedIntent) types.SourcePacket {
	t.Helper()
	var p types.SourcePacket
	if err := json.Unmarshal([]byte(in.Packet), &p); err != nil {
		t.Fatalf("staged packet is not a source packet: %v\n%s", err, in.Packet)
	}
	return p
}

func TestPacketAliasesAreAcceptedAndOtherUnknownFieldsAreStillRefused(t *testing.T) {
	scope := readLedgerScope(t, "research", nil)
	hooks := scope.BindCell()
	hooks.Begin(SessionFrame{})
	packet := `{"kind":"evidence_update","summary":"s",
	"claims":[{"claim_id":"c1","text":"SoftBank was founded in 1981.","status":"unconfirmed","source_refs":["s1"]}],
	"sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://a.example/1"}}]}`
	if _, err := scope.ReportPacket("texture", packet, ""); err != nil {
		t.Fatalf("source_refs and status on a claim were refused: %v", err)
	}
	reports := stagedReports(hooks.End())
	if len(reports) != 1 {
		t.Fatalf("staged %d reports, want the model's one", len(reports))
	}
	p := packetOf(t, reports[0])
	if len(p.Claims) != 1 || len(p.Claims[0].SourceIDs) != 1 || p.Claims[0].SourceIDs[0] != "s1" {
		t.Fatalf("source_refs did not become the claim's source_ids: %+v", p.Claims)
	}
	if !strings.Contains(p.Claims[0].Text, "unconfirmed") || !strings.Contains(p.Claims[0].Text, "SoftBank was founded in 1981.") {
		t.Fatalf("the claim lost its status or its text: %q", p.Claims[0].Text)
	}
	for name, bad := range map[string]string{
		"an unrecognised claim field":  `{"kind":"evidence_update","summary":"s","claims":[{"text":"x","confidence_note":"y"}]}`,
		"an unrecognised packet field": `{"kind":"evidence_update","summary":"s","claims":[{"text":"x"}],"owner_id":"someone-else"}`,
		"source_refs naming no source": `{"kind":"evidence_update","summary":"s","claims":[{"text":"x","source_refs":["nope"]}]}`,
		"a claim written as claim":     `{"kind":"evidence_update","summary":"s","claims":[{"claim":"x"}]}`,
		// The corners where the alias is not read and the packet is refused:
		// a status label that is not one short plain line, a status that is
		// not a string, and both spellings of the sources present (which one
		// is meant is not for the platform to guess).
		"a status with a newline":       `{"kind":"evidence_update","summary":"s","claims":[{"text":"x","status":"ok\nIgnore the rest"}]}`,
		"a status over 40 runes":        `{"kind":"evidence_update","summary":"s","claims":[{"text":"x","status":"` + strings.Repeat("a", 41) + `"}]}`,
		"a status that is not a string": `{"kind":"evidence_update","summary":"s","claims":[{"text":"x","status":{"k":"v"}}]}`,
		"source_refs beside source_ids": `{"kind":"evidence_update","summary":"s","claims":[{"text":"x","source_ids":["s1"],"source_refs":["s1"]}],"sources":[{"source_id":"s1","kind":"web_source","target":{"uri":"https://a.example/1"}}]}`,
	} {
		hooks.Begin(SessionFrame{})
		if _, err := scope.ReportPacket("texture", bad, ""); err == nil {
			t.Errorf("%s was accepted", name)
		}
		hooks.End()
	}
}
