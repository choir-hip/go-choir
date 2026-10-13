package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// R3r — the research desk is live on the cell carrier: a research cell's
// staged choir.Report/ReportPacket mints a commitment_record (the plan's
// acceptance proof).

func TestR3rResearchCellReportMintsCommitmentUnderCap(t *testing.T) {
	rt, s := testRuntime(t)
	scope := testReductionScope()
	scope.FromRole = "research"
	scope.FromAgentID = "research:probe"
	scope.CellID = "cell-r3r-report"
	ctx := testReductionCtx(scope)

	reduction := &rlmCallReduction{active: true, mb: rt, st: rt.store, scope: scope, ledger: rt.store}

	// A thin Report mints a commitment_record on the ledger.
	report := yaegikernel.StagedIntent{
		LocalID: "rep-1", Kind: yaegikernel.IntentReport,
		Claim: "indexed provider coverage differs across regions",
	}
	if _, err := reduction.commitActIntent(ctx, report); err != nil {
		t.Fatalf("research cell Report reduce: %v", err)
	}
	if id := commitmentCanonicalID(t, s, scope, report); id == "" {
		t.Fatal("research cell Report absent from the commitment ledger")
	}

	// A packet-bodied ReportPacket mints too — the findings-report surface
	// research cells actually use.
	packetReport := yaegikernel.StagedIntent{
		LocalID: "rep-2", Kind: yaegikernel.IntentReport,
		Packet: `{"schema_version":"coagent_source_packet.v1","kind":"evidence_update","summary":"provider coverage differs across indexed regions","claims":[{"text":"provider coverage differs"}],"sources":[{"kind":"url","source_id":"src-1","target":{"uri":"https://example.com/coverage"}}],"questions":[],"notes":[]}`,
	}
	if _, err := reduction.commitActIntent(ctx, packetReport); err != nil {
		t.Fatalf("research cell ReportPacket reduce: %v", err)
	}
	if id := commitmentCanonicalID(t, s, scope, packetReport); id == "" {
		t.Fatal("research cell ReportPacket absent from the commitment ledger")
	}
}
