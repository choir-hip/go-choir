package store

// Failure modes (MA7.10 wiring), written first:
//  S1 the seed is regenerated or differs between reads: the sampler would
//     draw a different sample after a restart, so the log would not replay.
//  S2 the seed is shared across computers: two computers' samples correlate.
//  S3 a desk can read the seed or the anchors: every reader but the trust
//     service and the owner is refused by the object graph's gate.
//  S4 an anchor is written half-way: one row carries the item whole, so a
//     reader sees an anchor with its truth and its scorer answers or none.
//  S5 an anchor is rewritten under a second truth: the item id is the
//     identity, and the first settled truth stands.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/trust"
)

func TestTrustSeedIsCreatedOnceAndPerComputer(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	a, err := s.TrustAnchors().Seed(ctx, "owner-1", "computer-a")
	if err != nil || len(a) != 32 {
		t.Fatalf("seed %d bytes, err %v", len(a), err)
	}
	again, err := s.TrustAnchors().Seed(ctx, "owner-1", "computer-a")
	if err != nil || string(again) != string(a) {
		t.Fatalf("S1: the seed changed between reads (err %v)", err)
	}
	b, err := s.TrustAnchors().Seed(ctx, "owner-1", "computer-b")
	if err != nil || string(b) == string(a) {
		t.Fatalf("S2: two computers share a seed (err %v)", err)
	}
}

func TestTrustKindsAreTheTrustServicesAndNoDeskReadsThem(t *testing.T) {
	t.Cleanup(objectgraph.SetReaderGateMode(objectgraph.WriterGateEnforce))
	for _, kind := range []objectgraph.ObjectKind{"choir.trust_seed", "choir.trust_anchor"} {
		c, ok := objectgraph.CustodianOf(kind)
		if !ok || c != objectgraph.WriterTrust {
			t.Fatalf("%s custodian %q %v", kind, c, ok)
		}
		for _, desk := range []objectgraph.Writer{objectgraph.WriterTexture, objectgraph.WriterManagement, objectgraph.WriterEngineering, objectgraph.WriterResearch} {
			if err := objectgraph.CheckReadAs(objectgraph.WithReader(context.Background(), desk), kind); !errors.Is(err, objectgraph.ErrWrongReader) {
				t.Errorf("S3: %s read %s: %v", desk, kind, err)
			}
		}
		if err := objectgraph.CheckReadAs(objectgraph.WithReader(context.Background(), objectgraph.WriterTrust), kind); err != nil {
			t.Errorf("the trust service cannot read %s: %v", kind, err)
		}
	}
}

func TestTrustAnchorIsOneRowAndTheFirstTruthStands(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	item := trust.AnchorItem{ItemID: "cast-1#q0", Agent: "research:d", QuestionType: "research_child", Choices: []string{"yes", "no"},
		Answers: []trust.ScorerAnswer{{Scorer: "system:judge", Answer: "yes"}, {Scorer: "self:research:d", Answer: "no"}},
		Truth:   "yes", Kind: trust.AnchorSpotCheck, Judges: []string{"deepseek"}, Agreement: 1, SettledAt: at, SeedID: "s", SampledAt: at}
	if err := s.TrustAnchors().PutAnchor(ctx, "owner-1", "computer-a", item); err != nil {
		t.Fatal(err)
	}
	other := item
	other.Truth = "no"
	if err := s.TrustAnchors().PutAnchor(ctx, "owner-1", "computer-a", other); err != nil {
		t.Fatal(err)
	}
	got, err := s.TrustAnchors().ListAnchors(ctx, "owner-1", "computer-a")
	if err != nil || len(got) != 1 {
		t.Fatalf("%d anchors, err %v", len(got), err)
	}
	if got[0].Truth != "yes" || len(got[0].Answers) != 2 || got[0].Agent != "research:d" {
		t.Fatalf("S4/S5: %+v", got[0])
	}
	// another computer's anchors are not this one's
	if other, _ := s.TrustAnchors().ListAnchors(ctx, "owner-1", "computer-b"); len(other) != 0 {
		t.Fatalf("computer-b sees %d anchors", len(other))
	}
	// an item with no truth is not an anchor
	bad := item
	bad.ItemID, bad.Truth = "cast-2#q0", ""
	if err := s.TrustAnchors().PutAnchor(ctx, "owner-1", "computer-a", bad); err == nil {
		t.Fatal("an anchor with no truth was stored")
	}
}

// R8: custody holds in the default (observe) mode too: a read that is not
// the trust service's returns no trust object.
func TestNoNonTrustReaderReadsATrustObject(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	if _, err := s.TrustAnchors().Seed(ctx, "owner-1", "computer-a"); err != nil {
		t.Fatal(err)
	}
	if err := s.TrustAnchors().PutAnchor(ctx, "owner-1", "computer-a", trust.AnchorItem{ItemID: "c#q0", Truth: "yes", Kind: trust.AnchorSpotCheck, SettledAt: at}); err != nil {
		t.Fatal(err)
	}
	for _, reader := range []objectgraph.Writer{objectgraph.WriterTexture, objectgraph.WriterManagement, objectgraph.WriterEngineering, objectgraph.WriterResearch, "desk:app-x"} {
		rctx := objectgraph.WithReader(ctx, reader)
		for _, kind := range []objectgraph.ObjectKind{ogKindTrustAnchor, ogKindTrustSeed} {
			objs, err := s.ogListByOwnerAndBody(rctx, kind, "owner-1", nil, 100)
			if err == nil && len(objs) != 0 {
				t.Fatalf("R8: %s read %d %s objects", reader, len(objs), kind)
			}
		}
		// a whole-scope read drops them too
		all, err := s.ogListByOwnerAndBody(rctx, "", "owner-1", nil, 1000)
		if err == nil {
			for _, o := range all {
				if o.ObjectKind == ogKindTrustAnchor || o.ObjectKind == ogKindTrustSeed {
					t.Fatalf("R8: %s read a %s in a whole-scope list", reader, o.ObjectKind)
				}
			}
		}
	}
	if got, err := s.TrustAnchors().ListAnchors(ctx, "owner-1", "computer-a"); err != nil || len(got) != 1 {
		t.Fatalf("the trust service lost its own read: %d %v", len(got), err)
	}
}

// With the reader gate off, a whole-scope read still drops the trust kinds.
func TestTrustKindsAreDroppedFromAWholeScopeReadEvenWithTheGateOff(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	if _, err := s.TrustAnchors().Seed(ctx, "owner-1", "computer-a"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(objectgraph.SetReaderGateMode(objectgraph.WriterGateOff))
	rctx := objectgraph.WithReader(ctx, objectgraph.WriterResearch)
	all, err := s.ogListByOwnerAndBody(rctx, "", "owner-1", nil, 1000)
	if err != nil {
		return
	}
	for _, o := range all {
		if o.ObjectKind == ogKindTrustSeed || o.ObjectKind == ogKindTrustAnchor {
			t.Fatalf("a %s survived a whole-scope read with the gate off", o.ObjectKind)
		}
	}
}
