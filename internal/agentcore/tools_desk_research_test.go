package agentcore

import (
	"testing"

	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// R3r — the research desk is live on the cell carrier with the D2 cap
// boundary resolved: a research cell's staged choir.Report/ReportPacket
// mints a commitment_record under the activation's cap accounting (the
// plan's acceptance proof), and every host-mediated network tool charges
// the shared per-activation egress ledger.

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

func TestR3rEgressLedgerInstalledOnRuntime(t *testing.T) {
	rt, _ := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	if rt.researchEgress == nil {
		t.Fatal("research egress ledger must be installed — D2 cap boundary is not optional")
	}
	if rt.researchEgress.MaxCalls <= 0 || rt.researchEgress.MaxFetchedBytes <= 0 {
		t.Fatalf("egress ledger installed without limits: %+v", rt.researchEgress)
	}
}

