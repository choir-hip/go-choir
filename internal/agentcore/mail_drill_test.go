package agentcore

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/maildesk"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Drill P4, the Mail half (modular architecture definition): a fixture desk
// asks Mail for one email by ref through a record, gets a per-task lease, and
// is refused a second. This is the host path a deployed computer runs: the
// cell's RequestLease names the grant and the ref, requestLeaseEgress routes
// it to the Mail custodian service, and ReadMail serves only the leased ref.
//
// Failure modes, written before the code:
//
//  1. a mail.read request is decided by the generic lease path (no custodian
//     rule), so a second email, or two at once, is granted;
//  2. ReadMail serves an email with no lease, or one the lease does not name;
//  3. a computer with no mailbox grants a lease it cannot serve;
//  4. a desk whose ceiling does not name mail.read is granted;
//  5. a cell asks for a grant other than mail.read (texture.search) and gets it;
//  6. the mail read does not taint the run with the mail label.

type drillMailbox map[string]maildesk.Message

func (m drillMailbox) Message(_ context.Context, ownerID, ref string) (maildesk.Message, error) {
	if ownerID != "owner-ma34" {
		return maildesk.Message{}, maildesk.ErrNoMessage
	}
	msg, ok := m[ref]
	if !ok {
		return maildesk.Message{}, maildesk.ErrNoMessage
	}
	return msg, nil
}

func registerMailDrill(t *testing.T, slug string, uid uint32, verbs, grants []string, firstParty bool) {
	t.Helper()
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{
		Name: "app:" + slug, Verbs: verbs, Taint: deskmanifest.TaintUntrustedInput,
		UID: uid, WorkerMaxBytes: 64 << 20, LeaseGrants: grants, FirstParty: firstParty,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:" + slug) })
}

func TestFixtureDeskLeasesOneEmailAndIsRefusedASecond(t *testing.T) {
	rt, _ := testRuntime(t)
	registerMailDrill(t, "mail-reader-drill", 62861, []string{"RequestLease", "ReadMail", "Done"}, []string{lease.GrantMailRead}, true)
	registerMailDrill(t, "mailnoceil", 62862, []string{"RequestLease", "Done"}, nil, false)
	if _, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{Name: "app:mailthief", Verbs: []string{"ReadMail"},
		Taint: deskmanifest.TaintUntrustedInput, UID: 62863, WorkerMaxBytes: 64 << 20}); err == nil {
		t.Fatal("a non-first-party desk was given ReadMail")
	}
	rt.mailbox = drillMailbox{
		"msg-a": {From: "a@example.com", Subject: "first", TextBody: "body a"},
		"msg-b": {From: "b@example.com", Subject: "second", TextBody: "body b"},
	}
	run := leaseRunRecord("mail-reader-drill", "run-p4")
	exec := toolregistry.ExecutionContext{RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, AgentID: run.AgentID, RunRecord: run}
	role := "app:mail-reader-drill"
	ctx := objectgraph.WithLeases(objectgraph.WithReader(context.Background(), mustDeskWriter(t, role)), rt.leaseAuthority(), run.RunID)
	egress := func(action yaegikernel.BrokerAction, payload any) (json.RawMessage, error) {
		raw, _ := json.Marshal(payload)
		return rt.deskEgress(ctx, role, exec, action, raw)
	}
	ask := func(grants []string, refs ...string) error {
		_, err := egress(yaegikernel.ActionRequestLease, yaegikernel.RequestLeasePayload{
			Grants: grants, Refs: refs, Purpose: "summarize the one email the owner named", Req: leaseFields()})
		return err
	}
	readMail := func(ref string) (maildesk.Message, error) {
		out, err := egress(yaegikernel.ActionReadMail, yaegikernel.ReadMailPayload{Ref: ref})
		if err != nil {
			return maildesk.Message{}, err
		}
		var m maildesk.Message
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatal(err)
		}
		return m, nil
	}

	// (2) No lease, no mail.
	if _, err := readMail("msg-a"); err == nil {
		t.Fatal("ReadMail served an email with no lease")
	}
	// (1) Two emails in one request, and an unknown ref, are refused.
	if err := ask([]string{lease.GrantMailRead}, "msg-a", "msg-b"); err == nil {
		t.Fatal("one request for two emails was granted")
	}
	if err := ask([]string{lease.GrantMailRead}, "msg-nope"); err == nil {
		t.Fatal("a ref that is not in the mailbox was leased")
	}
	// (5) A grant a cell may not ask for.
	if err := ask([]string{lease.GrantTextureSearch}, "msg-a"); err == nil {
		t.Fatal("a cell asked for texture.search and got it")
	}
	// One email, by ref: granted. The ref is stored trimmed.
	if err := ask([]string{lease.GrantMailRead}, "  msg-a "); err != nil {
		t.Fatalf("one email inside the ceiling was refused: %v", err)
	}
	m, err := readMail("msg-a")
	if err != nil {
		t.Fatalf("the leased read failed: %v", err)
	}
	if m.Subject != "first" || m.Label != maildesk.MailLabel {
		t.Fatalf("the leased email: %+v", m)
	}
	// (2) The lease names msg-a only.
	if _, err := readMail("msg-b"); err == nil {
		t.Fatal("a lease on msg-a served msg-b")
	}
	// The second email is refused, and the refusal is on the tape.
	if err := ask([]string{lease.GrantMailRead}, "msg-b"); err == nil || !strings.Contains(err.Error(), "one email per task") {
		t.Fatalf("the second email was not refused as one per task: %v", err)
	}
	all, _ := rt.leases.All(context.Background(), run.OwnerID, run.ComputerID)
	granted := 0
	for _, l := range all {
		for _, g := range l.Scope.Grants {
			if l.Holder == role && g == lease.GrantMailRead {
				granted++
			}
		}
	}
	for _, l := range all {
		for _, ref := range l.Scope.Refs {
			if ref != strings.TrimSpace(ref) {
				t.Fatalf("a lease stores the untrimmed ref %q", ref)
			}
		}
	}
	if granted != 1 {
		t.Fatalf("the ledger holds %d mail.read leases, want 1: %+v", granted, all)
	}
	// (6) The read tainted the run with the mail label.
	if !rt.runLabelSet(run.RunID).Has(types.OwnerPrivateLabel("mail")) {
		t.Fatalf("the run is not labelled owner_private:mail after a mail read: %v", rt.runLabelSet(run.RunID))
	}

	// (4) A desk whose ceiling does not name mail.read is refused.
	other := leaseRunRecord("mailnoceil", "run-p4-b")
	otherExec := toolregistry.ExecutionContext{RunID: other.RunID, OwnerID: other.OwnerID, ComputerID: other.ComputerID, AgentID: other.AgentID, RunRecord: other}
	raw, _ := json.Marshal(yaegikernel.RequestLeasePayload{Grants: []string{lease.GrantMailRead}, Refs: []string{"msg-a"}, Purpose: "x", Req: leaseFields()})
	if _, err := rt.deskEgress(ctx, "app:mailnoceil", otherExec, yaegikernel.ActionRequestLease, raw); err == nil {
		t.Fatal("a desk with no mail.read ceiling was granted")
	}

	// (3) A computer with no mailbox grants nothing.
	rt.mailbox = nil
	run2 := leaseRunRecord("mail-reader-drill", "run-p4-c")
	exec2 := toolregistry.ExecutionContext{RunID: run2.RunID, OwnerID: run2.OwnerID, ComputerID: run2.ComputerID, AgentID: run2.AgentID, RunRecord: run2}
	if _, err := rt.deskEgress(ctx, role, exec2, yaegikernel.ActionRequestLease, raw); err == nil {
		t.Fatal("a lease was granted on a computer with no mailbox")
	}
}
