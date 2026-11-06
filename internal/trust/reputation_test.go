package trust

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Publisher reputation in the Library (MA7.6, trust draft section 8), failure
// modes written before the code:
//
//	P1  the block shows a bare score (one number, stars, a mean) instead of
//	    sentences with their n;
//	P2  n = 0 reads as a perfect record (or as any rate at all);
//	P3  an outcome about another publisher, or an unsettled prediction, is
//	    counted;
//	P4  a forged bundle (no signature, another key, edited outcomes, another
//	    computer's name) is aggregated;
//	P5  the same bundle counted twice doubles the record;
//	P6  a bundle carries content (a record's text) rather than settled
//	    outcomes only.

func TestSentencesCarryNNotAScore(t *testing.T) {
	// P2: nothing settled.
	s := Sentences(nil)
	if len(s) != 1 || s[0].N != 0 || strings.ContainsAny(s[0].Text, "%0123456789") {
		t.Fatalf("P2: n=0 sentences %+v", s)
	}
	at := "2026-12-03T10:00:00Z"
	mail := ActionClass{Grant: "desk.mail.read", Impact: types.ImpactI1}
	var outs []SettledOutcome
	for i := 0; i < 5; i++ {
		outs = append(outs, SettledOutcome{Kind: OutcomeAct, Publisher: "p", Class: mail, Evidence: E1, Within: i != 4, SettledAt: at})
	}
	outs = append(outs,
		SettledOutcome{Kind: OutcomeAct, Publisher: "p", Class: mail, Evidence: E3, Fidelity: true, SettledAt: at},
		SettledOutcome{Kind: OutcomeReview, Publisher: "p", ReviewVerdict: "accept", SettledAt: at},
		SettledOutcome{Kind: OutcomeReview, Publisher: "p", ReviewVerdict: "reject", SettledAt: at},
		SettledOutcome{Kind: OutcomeReviewPrediction, Publisher: "p", Within: false, SettledAt: at},
	)
	s = Sentences(outs)
	text := ""
	for _, x := range s {
		if x.N <= 0 {
			t.Fatalf("P1: a sentence without its n: %+v", x)
		}
		text += x.Text + "\n"
	}
	for _, want := range []string{"4 of 6 settled acts within commitment", "One fidelity event (December 2026)", "Reviewed: 1 of 2 accepted", "1 of 1 review predictions contradicted"} {
		if !strings.Contains(text, want) {
			t.Fatalf("P1: %q missing from\n%s", want, text)
		}
	}
	// P1: the JSON of a sentence has a text and an n, nothing like a score.
	raw, _ := json.Marshal(s)
	for _, bad := range []string{"score", "rating", "stars", "mean"} {
		if strings.Contains(strings.ToLower(string(raw)), `"`+bad) {
			t.Fatalf("P1: sentence JSON carries %q: %s", bad, raw)
		}
	}
}

func TestPublisherOutcomesCountOnlyThatPublishersSettledEvidence(t *testing.T) {
	pre := func(id, desk string) types.CommitmentRecord {
		return types.CommitmentRecord{RecordID: id, Kind: types.CommitmentKindPrecommit,
			Provenance: types.CommitmentProvenance{AgentID: desk + ":r", Desk: desk, ModelID: "m", CommittedAt: ts(0)},
			Precommit:  &types.CommitmentPrecommit{Question: "q", Distribution: map[string]float64{"yes": 0.9, "no": 0.1}, ImpactClass: types.ImpactI1}}
	}
	res := func(parent, verdict string) types.CommitmentRecord {
		return types.CommitmentRecord{RecordID: parent + ":resolve", Kind: types.CommitmentKindResolve, ParentID: parent,
			Provenance: types.CommitmentProvenance{AgentID: "system:rule", CommittedAt: ts(time.Hour)}, Resolve: &types.CommitmentResolve{Verdict: verdict}}
	}
	in := Inputs{
		Records: []types.CommitmentRecord{pre("a", "app:notes"), res("a", "yes"), pre("b", "app:other"), res("b", "no"), pre("c", "app:notes")},
		Resolver: func(desk string) SubjectInfo {
			switch desk {
			case "app:notes":
				return SubjectInfo{Package: "notes", Publisher: "p"}
			case "app:other":
				return SubjectInfo{Package: "other", Publisher: "q"}
			}
			return SubjectInfo{}
		},
	}
	outs := PublisherOutcomes(in, baseParams(), "p")
	if len(outs) != 1 || outs[0].Package != "notes" || !outs[0].Within || outs[0].Kind != OutcomeAct {
		t.Fatalf("P3: outcomes %+v", outs)
	}
}

type bundleKey struct {
	key computerevent.SigningKey
}

func newBundleKey(t *testing.T) *bundleKey {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &bundleKey{key: computerevent.SigningKey{SignerRef: computerevent.SignerRef{SignerDomain: "guest-core", KeyID: "k"}, PrivateKey: priv}}
}

func (k *bundleKey) SignReceipt(_ context.Context, kind, issuer string, fields map[string]any, at time.Time) (computerevent.Receipt, error) {
	return computerevent.NewSignedReceipt(kind, issuer, fields, []computerevent.SigningKey{k.key}, at)
}

func (k *bundleKey) pub() ed25519.PublicKey { return k.key.PrivateKey.Public().(ed25519.PublicKey) }

func TestSignedBundlesAggregateOnlyWhenTheyVerify(t *testing.T) {
	at := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	mail := ActionClass{Grant: "desk.mail.read", Impact: types.ImpactI1}
	outs := []SettledOutcome{{Kind: OutcomeAct, RecordID: "r1", Publisher: "p", Class: mail, Evidence: E1, Within: true, SettledAt: ts(0)}}
	a, b := newBundleKey(t), newBundleKey(t)
	sa, err := SignBundle(context.Background(), a, OutcomeBundle{ComputerID: "computer-a", Publisher: "p", Outcomes: outs}, at)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := SignBundle(context.Background(), b, OutcomeBundle{ComputerID: "computer-b", Publisher: "p", Outcomes: outs}, at)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]ed25519.PublicKey{"computer-a": a.pub(), "computer-b": b.pub()}
	resolve := func(c string) (ed25519.PublicKey, bool) { k, ok := keys[c]; return k, ok }

	// P5: the same bundle twice counts once; two computers count twice.
	got, rejected := Aggregate([]SignedBundle{sa, sa, sb}, "p", resolve)
	if len(got) != 2 || len(rejected) != 0 {
		t.Fatalf("P5: %d outcomes, rejected %v", len(got), rejected)
	}
	// P4: forgeries.
	edited := sa
	edited.Bundle.Outcomes = append([]SettledOutcome(nil), sa.Bundle.Outcomes...)
	edited.Bundle.Outcomes[0].Within = false
	renamed := sa
	renamed.Bundle.ComputerID = "computer-b"
	unsigned := sa
	unsigned.Receipt = nil
	otherKey, _ := SignBundle(context.Background(), b, OutcomeBundle{ComputerID: "computer-a", Publisher: "p", Outcomes: outs}, at)
	for name, forged := range map[string]SignedBundle{"edited": edited, "renamed": renamed, "unsigned": unsigned, "other key": otherKey} {
		got, rejected := Aggregate([]SignedBundle{forged}, "p", resolve)
		if len(got) != 0 || len(rejected) != 1 {
			t.Fatalf("P4 %s: aggregated %d, rejected %v", name, len(got), rejected)
		}
	}
	// P3: a bundle about another publisher is not this one's record.
	if got, _ := Aggregate([]SignedBundle{sa}, "q", resolve); len(got) != 0 {
		t.Fatalf("P3: another publisher's bundle counted: %+v", got)
	}
}

func TestOutcomeCarriesNoContent(t *testing.T) {
	// P6: the outcome's fields are the draft's list (class, impact, quality,
	// severity, release, timestamps) and opaque ids, no free text.
	raw, _ := json.Marshal(SettledOutcome{})
	var fields map[string]any
	_ = json.Unmarshal(raw, &fields)
	for name := range fields {
		switch name {
		case "kind", "record_id", "publisher", "package", "release", "class", "evidence", "within", "severity", "fidelity", "review_verdict", "settled_at":
		default:
			t.Fatalf("P6: outcome field %q", name)
		}
	}
}
