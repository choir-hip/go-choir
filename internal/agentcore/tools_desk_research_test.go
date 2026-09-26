package agentcore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
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

func TestR3rResearchEgressChargedOnNetworkTools(t *testing.T) {
	rt, _ := testRuntime(t)
	if err := rt.InstallDefaultAgentTools(t.TempDir()); err != nil {
		t.Fatalf("install tools: %v", err)
	}
	reg := rt.ToolRegistryForProfile("research")
	if reg == nil {
		t.Fatal("research registry missing")
	}
	ledger := rt.researchEgress
	ledger.MaxCalls = 1 // tighten for the test: one network call allowed

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("research evidence body"))
	}))
	defer server.Close()

	runCtx := toolregistry.WithExecutionContext(t.Context(), toolregistry.ExecutionContext{
		RunID:     "run-r3r-egress",
		Profile:   "research",
		RunRecord: &types.RunRecord{RunID: "run-r3r-egress"},
	})

	fetch, ok := reg.Lookup("fetch_url")
	if !ok {
		t.Fatal("fetch_url must be on the research cell registry")
	}
	args, _ := json.Marshal(map[string]any{"url": server.URL})
	out, err := fetch.Func(runCtx, args)
	if err != nil {
		t.Fatalf("first fetch_url under budget: %v", err)
	}
	if !strings.Contains(out, "research evidence body") {
		t.Fatalf("fetch_url output missing body: %s", out)
	}
	calls, fetched := ledger.Usage(toolregistry.ExecutionContextFrom(runCtx))
	if calls != 1 || fetched <= 0 {
		t.Fatalf("egress usage = %d calls / %d bytes, want 1 + >0", calls, fetched)
	}

	// Second network call refuses at cap — the budget binds the activation,
	// not the single tool.
	_, err = fetch.Func(runCtx, args)
	if err == nil || !strings.Contains(err.Error(), "egress budget exhausted") {
		t.Fatalf("fetch_url beyond cap must refuse with budget error, got %v", err)
	}
}
