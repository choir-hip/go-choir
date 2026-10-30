package yaegikernel

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/texturewrite"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Self-scores and the calibration digest in the cell
// (docs/archive/reports/icrl-calibration-experiment-2026-10-14.md). Ways it can
// fail, written first: P8 a pSupported outside [0,1] stages; a Write
// without one is refused (it is optional); the interpreter cannot call
// the variadic Write; choir.Pack().Calibration is unreadable in a cell.

func TestWriteSelfScore(t *testing.T) {
	scope, hooks := textureTurnScope(t)
	for _, bad := range [][]float64{{1.5}, {-0.1}, {0.5, 0.6}} {
		if _, err := scope.Write("@insert end\nA.", "x", bad...); err == nil {
			t.Fatalf("pSupported %v staged", bad)
		}
	}
	if got := hooks.End(); len(got) != 0 {
		t.Fatalf("refused Writes staged %d intents", len(got))
	}
	scope, hooks = textureTurnScope(t)
	if _, err := scope.Write("@insert end\nA {{cite r1.c1 \"q\"}}.", "add", 0.8); err != nil {
		t.Fatalf("Write with pSupported: %v", err)
	}
	var turn TextureTurn
	if err := json.Unmarshal([]byte(hooks.End()[0].Body), &turn); err != nil {
		t.Fatal(err)
	}
	if turn.Write == nil || turn.Write.PSupported == nil || *turn.Write.PSupported != 0.8 {
		t.Fatalf("turn write = %+v, want pSupported 0.8", turn.Write)
	}
	scope, hooks = textureTurnScope(t)
	if _, err := scope.Write("@insert end\nA.", "add"); err != nil {
		t.Fatalf("Write without pSupported: %v", err)
	}
	var plain TextureTurn
	_ = json.Unmarshal([]byte(hooks.End()[0].Body), &plain)
	if plain.Write == nil || plain.Write.PSupported != nil {
		t.Fatalf("a Write without pSupported = %+v", plain.Write)
	}
}

// A research Report's p_complete passes the packet check at the call, and
// a value outside [0,1] is refused there, with the packet shape (P8).
func TestReportPacketSelfScore(t *testing.T) {
	base := `{"kind":"evidence_update","summary":"s","claims":[{"text":"c"}],"work_disposition":"completed"`
	raw, err := checkReportPacket(base + `,"p_complete":0.65}`)
	if err != nil || !strings.Contains(raw, `"p_complete":0.65`) {
		t.Fatalf("p_complete 0.65 = %q, %v", raw, err)
	}
	if _, err := checkReportPacket(base + `,"p_complete":1.5}`); err == nil || !strings.Contains(err.Error(), "p_complete") {
		t.Fatalf("p_complete 1.5 = %v, want refused", err)
	}
	if raw, err := checkReportPacket(base + `}`); err != nil || strings.Contains(raw, "p_complete") {
		t.Fatalf("no p_complete = %q, %v", raw, err)
	}
}

func TestCellCallsWriteWithSelfScoreAndReadsCalibration(t *testing.T) {
	broker, issuer, _, _ := testChoirFixture(t)
	scope, err := NewChoirScope(broker, issuer, "computer-choir", "activation-texture", 1, "texture", "")
	if err != nil {
		t.Fatal(err)
	}
	sess, err := NewSession(NewAllowlist(DefaultSafeStdlibPackagesList()...), scope.ChoirExports())
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	hooks := scope.BindCell()
	frame := SessionFrame{ID: "cell",
		Source: "fmt.Println(choir.Pack().Calibration.N, choir.Pack().Calibration.Direction)\n_, err := choir.Write(\"@insert end\\nA.\", \"add\", 0.9)\nfmt.Println(err)",
		Doc:    &DocSnapshot{DocID: "doc-1", RevisionID: "rev-1", Blocks: []texturewrite.Block{{Alias: "b1", ID: "p1", Kind: "paragraph", Text: "Body."}}},
		Pack: &types.ActingPack{AgentID: "texture:doc-1", Items: []types.ActingPackItem{},
			Calibration: &types.CalibrationDigest{Desk: "texture", N: 40, Direction: "over-confident", Text: "t"}},
	}
	out, err := serveCell(sess, frame, nil, &hooks)
	if err != nil || out.Error != "" {
		t.Fatalf("cell: %v %s", err, out.Error)
	}
	if !strings.Contains(out.Stdout, "40 over-confident") || !strings.Contains(out.Stdout, "<nil>") {
		t.Fatalf("stdout = %q, want the digest fields and a staged Write", out.Stdout)
	}
}

// docs/problems/texture-done-after-refused-write-stalls-story-2026-10-16.md:
// a cell whose Write was refused must not close the turn with Done, which
// would complete the story at its seed for good. A successful Write's Done
// still completes, and Wait (a turn without a revision) stays open.
func TestDoneAfterARefusedWriteIsRefused(t *testing.T) {
	scope, hooks := textureTurnScope(t)
	_, werr := scope.Write("@insert end\nA {{cite r1.c1 \"q\"}}.", "add", 0.98, 0.97)
	if werr == nil || !strings.Contains(werr.Error(), "one number for the whole Write") {
		t.Fatalf("per-citation pSupported refusal = %v; want it to say one number for the whole Write", werr)
	}
	if _, err := scope.Done("integrated the reports"); err == nil || !strings.Contains(err.Error(), "Write was refused") {
		t.Fatalf("Done after a refused Write = %v, want refused", err)
	}
	if got := hooks.End(); len(got) != 0 {
		t.Fatalf("a refused Write and its Done staged %d intents", len(got))
	}

	scope, hooks = textureTurnScope(t)
	if _, err := scope.Write("@insert end\nA.", "add", 0.9); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.Done("written"); err != nil {
		t.Fatalf("Done after a Write that staged: %v", err)
	}
	if got := hooks.End(); len(got) != 1 {
		t.Fatalf("Write and Done staged %d intents, want one turn", len(got))
	}

	scope, hooks = textureTurnScope(t)
	_, _ = scope.Write("@insert end\nA.", "add", 2)
	if _, err := scope.Wait("waiting for a second outlet"); err != nil {
		t.Fatalf("Wait after a refused Write: %v", err)
	}
	if got := hooks.End(); len(got) != 1 {
		t.Fatalf("Wait staged %d intents, want one", len(got))
	}
}
