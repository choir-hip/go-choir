package agentcore

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/installreview"
	"github.com/yusefmosiah/go-choir/internal/trust"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The reputation route (MA7.6), failure modes written before the code:
//
//	L1  the block's answer carries a score, stars or a mean, or a sentence
//	    with no n;
//	L2  a publisher with nothing settled reads as a record (n > 0 or a rate);
//	L3  a forged verdict on the ledger counts as a review of the publisher;
//	L4  the signed bundle does not verify against this computer's key, or
//	    carries another publisher's outcomes;
//	L5  another publisher's review is counted.

func reputationOf(t *testing.T, e *installEnv, publisher string) (reputationResponse, string) {
	t.Helper()
	w := packagesCall(t, e.h, packagesReputationPath+"?publisher="+publisher, e.owner)
	if w.Code != http.StatusOK {
		t.Fatalf("reputation = %d %s", w.Code, w.Body.String())
	}
	var body reputationResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body, w.Body.String()
}

func TestReputationIsSentencesWithN(t *testing.T) {
	f := newNotesReviewFixture(t)
	e, signer := f.e, f.signer

	// L2: nothing settled.
	body, raw := reputationOf(t, e, "publisher-x")
	if body.N != 0 || len(body.Sentences) != 1 || body.Sentences[0].N != 0 || body.Signed != nil {
		t.Fatalf("L2: %s", raw)
	}
	// L3: a forged verdict for the publisher is not a review.
	subject := installreview.Subject{ReleaseID: strings.Repeat("a", 64), PatchSHA256: strings.Repeat("b", 64), Package: installName, Publisher: "publisher-x"}
	forged := installreview.VerdictRecord(subject, installreview.Plan{Risk: types.ImpactI1}, nil, nil, true, e.computer, time.Now().UTC())
	if _, err := e.rt.store.AppendCommitmentRecord(context.Background(), e.owner, e.computer, forged); err != nil {
		t.Fatal(err)
	}
	if body, raw = reputationOf(t, e, "publisher-x"); body.N != 0 {
		t.Fatalf("L3: a forged verdict counted: %s", raw)
	}

	// A reviewed, accepted install of the publisher's release: management owes
	// the verdict, judges it, and the stored request installs.
	if code, _, reason := f.post(t); code != http.StatusAccepted {
		t.Fatalf("install = %d %s", code, reason)
	}
	f.judge(t, true)
	body, raw = reputationOf(t, e, "publisher-x")
	if body.N != 1 || len(body.Sentences) != 1 || body.Sentences[0].N != 1 || body.Sentences[0].Text != "Reviewed: 1 of 1 accepted." {
		t.Fatalf("reviewed: %s", raw)
	}
	// L1: no score anywhere in the answer.
	for _, bad := range []string{`"score`, `"rating`, `"stars`, `"mean`} {
		if strings.Contains(strings.ToLower(raw), bad) {
			t.Fatalf("L1: the answer carries %s: %s", bad, raw)
		}
	}
	// L4: the signed bundle verifies against this computer's key.
	if body.Signed == nil {
		t.Fatal("L4: no signed bundle")
	}
	pub := signer.key.PrivateKey.Public().(ed25519.PublicKey)
	got, rejected := trust.Aggregate([]trust.SignedBundle{*body.Signed}, "publisher-x", func(c string) (ed25519.PublicKey, bool) { return pub, c == e.computer })
	if len(got) != 1 || len(rejected) != 0 {
		t.Fatalf("L4: aggregate %d rejected %v", len(got), rejected)
	}
	// L5: another publisher has none of it.
	if body, raw = reputationOf(t, e, "publisher-y"); body.N != 0 {
		t.Fatalf("L5: %s", raw)
	}
}
