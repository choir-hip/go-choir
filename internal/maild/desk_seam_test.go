package maild

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/maildesk"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	guest "github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The Mail desk proof (modular architecture plan MA4.7): a video-editing-style
// fixture desk, a test-only first-party fixture, asks Mail through a
// precommitment-record request for one email by ref, gets a per-task lease,
// and is refused a second email. The mail stays in maild: the Mail custodian
// reads this maild store through its seam.
//
// Failure modes, written before the code:
//
//  1. a desk with no lease reads an email through the Mail service;
//  2. a lease on email A serves email B (the second email);
//  3. a second request in the same task, for another email, is granted;
//  4. one request naming two emails is granted;
//  5. a desk whose ceiling does not name mail.read is granted;
//  6. another run of the same desk uses the lease;
//  7. the lease outlives its run;
//  8. the request is not a scored precommitment addressed to the Mail desk, or
//     a refused request leaves no record;
//  9. a ref that is not a message in the owner's mailbox is leased;
//  10. a mail.read lease admits through the ref-less grant check;
//  11. a custodian other than Mail issues mail.read;
//  12. the holder comes from anything but the trusted reader label.

const (
	seamOwner    = "owner-mail-desk"
	seamComputer = "computer-mail-desk"
	fixturePkg   = "vidfixture"
	plainPkg     = "vidplain"
)

type mailDeskFixture struct {
	t       *testing.T
	maild   *Store
	guest   *guest.Store
	leases  *lease.Service
	mail    *maildesk.Service
	fixture objectgraph.Writer
	plain   objectgraph.Writer
}

func registerFixtureDesk(t *testing.T, slug string, uid uint32, grants []string) objectgraph.Writer {
	t.Helper()
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{
		Name: "app:" + slug, Verbs: []string{"Cast", "RequestLease"}, Taint: deskmanifest.TaintUntrustedInput,
		UID: uid, WorkerMaxBytes: 64 << 20, LeaseGrants: grants,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:" + slug) })
	w, ok := objectgraph.DeskWriter("app:" + slug)
	if !ok {
		t.Fatalf("desk app:%s has no writer", slug)
	}
	return w
}

func newMailDeskFixture(t *testing.T) *mailDeskFixture {
	t.Helper()
	ms, _ := newTestStore(t)
	seedMessage(t, ms, seamOwner, "msg-a", "trusted")
	seedMessage(t, ms, seamOwner, "msg-b", "trusted")
	gs, err := guest.Open(filepath.Join(t.TempDir(), "runtime.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gs.Close() })
	leases := lease.New(gs.Leases())
	t.Cleanup(leases.Close)
	return &mailDeskFixture{
		t: t, maild: ms, guest: gs, leases: leases,
		mail:    maildesk.New(ms.DeskMailbox(), leases),
		fixture: registerFixtureDesk(t, fixturePkg, 62851, []string{lease.GrantMailRead}),
		plain:   registerFixtureDesk(t, plainPkg, 62852, nil),
	}
}

func mailFields() *types.RequestFields {
	p, w := 0.8, 0.3
	return &types.RequestFields{PSuccess: &p, Weight: &w, ImpactClass: types.ImpactI1, Settlement: types.SettlementRecipient}
}

func (f *mailDeskFixture) ask(slug, runID string, refs ...string) (lease.Record, error) {
	return f.mail.RequestRead(context.Background(), maildesk.ReadRequest{
		OwnerID: seamOwner, ComputerID: seamComputer, Desk: "app:" + slug, RunID: runID, AgentID: slug + ":" + runID,
		Refs: refs, Purpose: "cut the clip the owner's email describes", Fields: mailFields(),
	})
}

// runCtx is the context trusted code builds from the run record: the run's
// desk as the reader and the lease authority for the run.
func (f *mailDeskFixture) runCtx(holder objectgraph.Writer, runID string) context.Context {
	return objectgraph.WithLeases(objectgraph.WithReader(context.Background(), holder), f.leases, runID)
}

func (f *mailDeskFixture) read(holder objectgraph.Writer, runID, ref string) (maildesk.Message, error) {
	return f.mail.Read(f.runCtx(holder, runID), seamOwner, ref)
}

func TestMailDeskLeasesOneEmailPerTaskAndRefusesASecond(t *testing.T) {
	f := newMailDeskFixture(t)

	// (1) No lease, no mail.
	if _, err := f.read(f.fixture, "run-1", "msg-a"); !errors.Is(err, maildesk.ErrNoLease) {
		t.Fatalf("a desk with no lease read an email: %v", err)
	}

	rec, err := f.ask(fixturePkg, "run-1", "msg-a")
	if err != nil {
		t.Fatalf("the request for one email was refused: %v", err)
	}
	if rec.Holder != "app:"+fixturePkg || rec.HolderRunID != "run-1" || rec.State != lease.StateActive ||
		len(rec.Custodians) != 1 || rec.Custodians[0] != string(lease.WriterMail) ||
		len(rec.Scope.Refs) != 1 || rec.Scope.Refs[0] != "msg-a" || !rec.ExpiresAt.IsZero() {
		t.Fatalf("the lease is not Mail's per-task lease on msg-a: %+v", rec)
	}

	// (8) The request is a scored precommitment addressed to the Mail desk.
	got, err := f.guest.GetCommitmentRecord(context.Background(), seamOwner, seamComputer, rec.RequestRef)
	if err != nil || got == nil {
		t.Fatalf("request record: %v", err)
	}
	if got.Kind != types.CommitmentKindPrecommit || got.Precommit == nil || got.Addressee != string(lease.WriterMail) ||
		got.Precommit.Resolver != string(lease.WriterMail) || got.PSuccess == nil || *got.PSuccess != 0.8 ||
		got.MaterialityProcedure == nil || got.MaterialityProcedure.Kind != types.RequestLease ||
		got.Provenance.Desk != "app:"+fixturePkg || got.Provenance.RunID != "run-1" || got.Provenance.Origin != types.OriginDeskRun {
		t.Fatalf("the request is not a stamped precommitment to Mail: %+v", got)
	}

	// The leased email is served from maild, labelled.
	m, err := f.read(f.fixture, "run-1", "msg-a")
	if err != nil {
		t.Fatalf("the leased read was refused: %v", err)
	}
	if m.Ref != "msg-a" || m.Subject != "Project update" || m.From != "sender@example.com" || m.Label != types.OwnerPrivateLabel("mail") {
		t.Fatalf("served message: %+v", m)
	}

	// (2) The second email is refused at the read.
	if _, err := f.read(f.fixture, "run-1", "msg-b"); !errors.Is(err, maildesk.ErrNoLease) {
		t.Fatalf("the lease on msg-a served msg-b: %v", err)
	}
	// (3) and at the request: one email per task. The refused ask stays.
	before, _ := f.guest.ListCommitmentRecords(context.Background(), seamOwner, seamComputer, string(lease.WriterMail), 50)
	if _, err := f.ask(fixturePkg, "run-1", "msg-b"); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("a second email in the same task was granted: %v", err)
	}
	after, _ := f.guest.ListCommitmentRecords(context.Background(), seamOwner, seamComputer, string(lease.WriterMail), 50)
	if len(after) != len(before)+1 {
		t.Fatalf("a refused request left %d new records, want 1", len(after)-len(before))
	}
	if _, err := f.read(f.fixture, "run-1", "msg-b"); !errors.Is(err, maildesk.ErrNoLease) {
		t.Fatalf("the refused request leased msg-b: %v", err)
	}

	// (6) Another run of the same desk holds nothing.
	if _, err := f.read(f.fixture, "run-other", "msg-a"); !errors.Is(err, maildesk.ErrNoLease) {
		t.Fatalf("another run used the lease: %v", err)
	}
	// (12) Another desk presenting the run's id holds nothing.
	if _, err := f.read(f.plain, "run-1", "msg-a"); !errors.Is(err, maildesk.ErrNoLease) {
		t.Fatalf("another desk used the lease: %v", err)
	}
	if _, err := f.mail.Read(objectgraph.WithLeases(context.Background(), f.leases, "run-1"), seamOwner, "msg-a"); !errors.Is(err, maildesk.ErrNoLease) {
		t.Fatalf("a read with no declared reader was served: %v", err)
	}
	// (10) The ref-less grant check never admits a by-ref grant.
	if _, ok := f.leases.AdmitsGrant(f.fixture, "run-1", lease.GrantMailRead); ok {
		t.Fatal("mail.read admitted without a ref")
	}

	// (7) The lease ends with its run.
	if n := f.leases.EndRun(context.Background(), "run-1", lease.ReasonRunEnded); n != 1 {
		t.Fatalf("ended %d leases, want 1", n)
	}
	if _, err := f.read(f.fixture, "run-1", "msg-a"); !errors.Is(err, maildesk.ErrNoLease) {
		t.Fatalf("the lease outlived its run: %v", err)
	}
}

func TestMailDeskRefusesWhatItsRuleDoesNotAdmit(t *testing.T) {
	f := newMailDeskFixture(t)
	// (4) Two emails in one request.
	if _, err := f.ask(fixturePkg, "run-2", "msg-a", "msg-b"); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("a request for two emails was granted: %v", err)
	}
	// (5) A desk whose ceiling does not name mail.read.
	if _, err := f.ask(plainPkg, "run-3", "msg-a"); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("a desk without mail.read in its ceiling was granted: %v", err)
	}
	// (9) A ref that is not in the owner's mailbox, or another owner's.
	if _, err := f.ask(fixturePkg, "run-4", "msg-missing"); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("a missing message was leased: %v", err)
	}
	if _, err := f.mail.RequestRead(context.Background(), maildesk.ReadRequest{
		OwnerID: "another-owner", ComputerID: seamComputer, Desk: "app:" + fixturePkg, RunID: "run-5", AgentID: "a",
		Refs: []string{"msg-a"}, Purpose: "p", Fields: mailFields(),
	}); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("another owner's ref was leased: %v", err)
	}
	// No request may leave a request without p_success and weight.
	if _, err := f.mail.RequestRead(context.Background(), maildesk.ReadRequest{
		OwnerID: seamOwner, ComputerID: seamComputer, Desk: "app:" + fixturePkg, RunID: "run-6", AgentID: "a",
		Refs: []string{"msg-a"}, Purpose: "p",
	}); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("an unscored request was granted: %v", err)
	}
	// (11) Only the Mail custodian issues mail.read, and Mail issues only it.
	p, w := 0.8, 0.3
	req := lease.Request{OwnerID: seamOwner, ComputerID: seamComputer, Desk: "app:" + fixturePkg, RunID: "run-7", AgentID: "a",
		Scope:   lease.Scope{Grants: []string{lease.GrantMailRead}, Refs: []string{"msg-a"}},
		Purpose: "p", Fields: &types.RequestFields{PSuccess: &p, Weight: &w, ImpactClass: types.ImpactI1, Settlement: types.SettlementRecipient}}
	if _, err := f.leases.IssueGrant(context.Background(), objectgraph.WriterTexture, req, nil); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("Texture issued mail.read: %v", err)
	}
	if _, err := f.leases.Request(context.Background(), req); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("a desk took mail.read without the custodian: %v", err)
	}
	req.Scope = lease.Scope{Grants: []string{lease.GrantTextureSearch}}
	if _, err := f.leases.IssueGrant(context.Background(), lease.WriterMail, req, nil); !errors.Is(err, lease.ErrRefused) {
		t.Fatalf("Mail issued texture.search: %v", err)
	}
	for _, run := range []string{"run-2", "run-3", "run-4", "run-5", "run-6", "run-7"} {
		if _, err := f.read(f.fixture, run, "msg-a"); !errors.Is(err, maildesk.ErrNoLease) {
			t.Fatalf("a refused request in %s leased msg-a: %v", run, err)
		}
	}
}
