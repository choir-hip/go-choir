package agentcore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// Enforce flip S1, D3 (sections 3, 6.5), FM7 and FM8, written first.
//
//	FM7 A lease admitted after its expiry, outside its run, for another owner,
//	    or to a desk that is not the recipient.
//	FM8 A label atom the host mints from the registry maps to no custodian.

func registerLeaseKindApp(t *testing.T, slug string, uid uint32) {
	t.Helper()
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{
		Name: "app:" + slug, Verbs: []string{"Cast"}, Taint: deskmanifest.TaintUntrustedInput,
		UID: uid, WorkerMaxBytes: 64 << 20,
		LeaseKinds: []string{"choir.texture_document", "choir.texture_revision"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:" + slug) })
}

// The lease is held by the recipient's run; the sender's run id is not the
// lease's run. The check matches the recipient, the owner, the expiry and the
// run-or-durable rule.
func TestFM7TheLeaseCheckMatchesTheRecipientOwnerExpiryAndRun(t *testing.T) {
	rt, _ := testRuntime(t)
	registerLeaseKindApp(t, "recip", 62821)
	registerLeaseKindApp(t, "bystander", 62822)
	doc := types.OwnerPrivateLabel("document")
	run := leaseRunRecord("recip", "run-recip")
	if _, err := rt.RequestLease(context.Background(), run, lease.Scope{Kinds: []string{"choir.texture_document", "choir.texture_revision"}}, "read the draft", 10*time.Minute, leaseFields()); err != nil {
		t.Fatal(err)
	}
	const sender = "run-the-senders"
	if !rt.messageAdmitted(run.OwnerID, sender, "app:recip", doc) {
		t.Fatal("a live lease of the recipient's run did not admit a message from another run")
	}
	// The grant-path run binding is unchanged: a sender run id never widens it.
	if rt.messageAdmitted("owner-other", sender, "app:recip", doc) {
		t.Error("another owner's message was admitted by this owner's lease")
	}
	if rt.messageAdmitted(run.OwnerID, sender, "app:bystander", doc) {
		t.Error("a desk that holds no lease was admitted")
	}
	if rt.messageAdmitted(run.OwnerID, sender, "app:recip", types.OwnerPrivateLabel("mail")) {
		t.Error("a label outside the lease's kinds was admitted")
	}
	if len(rt.leases.Live(run.OwnerID)) != 1 {
		t.Fatal("setup: the lease is not live")
	}
	// The use is counted.
	for _, rec := range rt.leases.Live(run.OwnerID) {
		if rec.Uses == 0 {
			t.Errorf("an admitting check counted no use: %+v", rec)
		}
	}
	// Expiry: the lease is live in the index until something sweeps it, and the
	// check must not admit it.
	later := time.Now().UTC().Add(2 * lease.MaxDuration)
	rt.leases.SetClock(func() time.Time { return later })
	if rt.messageAdmitted(run.OwnerID, sender, "app:recip", doc) {
		t.Fatal("a lease was admitted after its expiry")
	}
	rt.leases.SetClock(func() time.Time { return time.Now().UTC() })
	// Outside its run: the run ends and the lease ends with it.
	run2 := leaseRunRecord("recip", "run-recip-2")
	if _, err := rt.RequestLease(context.Background(), run2, lease.Scope{Kinds: []string{"choir.texture_document"}}, "again", 0, leaseFields()); err != nil {
		t.Fatal(err)
	}
	rt.endRunLeases(context.Background(), run2.RunID, lease.ReasonRunEnded)
	rt.endRunLeases(context.Background(), run.RunID, lease.ReasonRunEnded)
	if rt.messageAdmitted(run.OwnerID, sender, "app:recip", doc) {
		t.Fatal("a lease outlived its run")
	}
}

// FM8: every owner_private atom the host mints from the registry resolves to
// a kind that has a custodian. The atoms are the object-kind spellings the
// registry yields (run_labels.go, texture, capture) and the product's words.
func TestFM8EveryRegistryMintedLabelAtomMapsToACustodian(t *testing.T) {
	for _, atom := range []string{"document", "texture", "capture", "evidence"} {
		kinds := labelObjectKinds(atom)
		if len(kinds) == 0 {
			t.Errorf("atom %s maps to no kind", atom)
			continue
		}
		for _, k := range kinds {
			if _, ok := objectgraph.CustodianOf(k); !ok {
				t.Errorf("atom %s maps to %s, which has no custodian in the registry", atom, k)
			}
		}
	}
	if got := labelObjectKinds("evidence"); len(got) != 1 || got[0] != "choir.agent_evidence" {
		t.Errorf("evidence = %v, want choir.agent_evidence", got)
	}
	if got := labelObjectKinds("capture"); len(got) != 1 || got[0] != objectgraph.WebCaptureObjectKind {
		t.Errorf("capture = %v", got)
	}
	// Atoms that name no registered kind have no custodian, so custody never
	// admits a message carrying one: it falls to the table and asks.
	for _, atom := range []string{"story", "run_memory", "app_object", "mail", "model-policy", "computers", "prompt-bar"} {
		for _, k := range labelObjectKinds(atom) {
			if w, ok := objectgraph.CustodianOf(k); ok {
				t.Errorf("atom %s now maps to %s (custodian %s): custody would admit it; give it a ruling first", atom, k, w)
			}
		}
	}
}

// The OwnerFlow atoms the host mints from the registry (not the path-segment
// atoms, which are product API words that fall to the table) all resolve.
func TestFM8OwnerFlowRegistryAtomsHaveCustodians(t *testing.T) {
	for _, atom := range []string{"capture", "evidence", "document", "texture"} {
		for _, k := range labelObjectKinds(atom) {
			if _, ok := objectgraph.CustodianOf(k); !ok {
				t.Errorf("owner flow atom %s -> %s has no custodian", atom, k)
			}
		}
	}
	if !strings.HasPrefix(types.OwnerPrivateLabel("x"), types.LabelOwnerPrivatePrefix) {
		t.Fatal("label prefix changed")
	}
}
