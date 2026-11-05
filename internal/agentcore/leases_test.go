package agentcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/lease"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/researchtools"
	"github.com/yusefmosiah/go-choir/internal/sourcegraph"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA3.4 in the runtime: the capture frame handler checks leases for an
// installed package desk, the boot pass ends the leases a crash left open, and
// the owner's routes list and revoke.

func registerLeaseApp(t *testing.T, slug string, uid uint32, captures bool) {
	t.Helper()
	d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{
		Name: "app:" + slug, Verbs: []string{"Cast", "ReadCapture"}, Taint: deskmanifest.TaintUntrustedInput,
		UID: uid, WorkerMaxBytes: 64 << 20, LeaseCaptures: captures,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := deskmanifest.RegisterApp(d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = deskmanifest.RetireApp("app:" + slug) })
}

func leaseRunRecord(slug, runID string) *types.RunRecord {
	return &types.RunRecord{RunID: runID, OwnerID: "owner-ma34", ComputerID: "autoputer-test", AgentID: "agent-" + runID, AgentProfile: "app:" + slug}
}

func leaseFields() *types.RequestFields {
	p, w := 0.7, 0.4
	return &types.RequestFields{PSuccess: &p, Weight: &w, ImpactClass: types.ImpactI1, Settlement: types.SettlementRecipient}
}

// (g) An app desk calling choir.ReadCapture with the id of an owner-private
// capture it holds no lease for is refused; with a world capture's id it is
// served; a lease by id or by label serves the private one.
func TestAppDeskReadCaptureNeedsALeaseForAnOwnerPrivateCapture(t *testing.T) {
	rt, _ := testRuntime(t)
	registerLeaseApp(t, "capreader", 62811, true)
	world := json.RawMessage(`{"capture_id":"cap-world","labels":["world_untrusted"]}`)
	private := json.RawMessage(`{"capture_id":"cap-private","labels":["owner_private:texture_document","world_untrusted"]}`)
	other := json.RawMessage(`{"capture_id":"cap-other","labels":["owner_private:texture_document"]}`)
	role := "app:capreader"
	run := leaseRunRecord("capreader", "run-g")
	ctx := objectgraph.WithLeases(context.Background(), rt.leaseAuthority(), run.RunID)

	if err := gateCaptureRead(ctx, role, world); err != nil {
		t.Fatalf("a world capture was refused: %v", err)
	}
	if err := gateCaptureRead(ctx, role, private); err == nil {
		t.Fatal("an owner-private capture was served to an app desk with no lease")
	}
	// A built-in desk only observes until MA3.7.
	if err := gateCaptureRead(ctx, agentprofile.Research, private); err != nil {
		t.Fatalf("a built-in desk was refused (it only observes): %v", err)
	}

	if _, err := rt.RequestLease(context.Background(), run, lease.Scope{CaptureIDs: []string{"cap-private"}}, "cut the clip", 0, leaseFields()); err != nil {
		t.Fatal(err)
	}
	if err := gateCaptureRead(ctx, role, private); err != nil {
		t.Fatalf("a leased capture was refused: %v", err)
	}
	if err := gateCaptureRead(ctx, role, other); err == nil {
		t.Fatal("a lease for one capture served another")
	}
	// A lease by label.
	if _, err := rt.RequestLease(context.Background(), run, lease.Scope{CaptureLabels: []string{"owner_private:texture_document"}}, "all private", 0, leaseFields()); err != nil {
		t.Fatal(err)
	}
	if err := gateCaptureRead(ctx, role, other); err != nil {
		t.Fatalf("a label lease did not serve the capture: %v", err)
	}
	// The run ends: the lease ends with it.
	rt.endRunLeases(context.Background(), run.RunID, lease.ReasonRunEnded)
	if err := gateCaptureRead(ctx, role, private); err == nil {
		t.Fatal("a lease outlived its run")
	}
	// A desk whose ceiling does not name captures cannot lease them.
	registerLeaseApp(t, "nocaps", 62812, false)
	if _, err := rt.RequestLease(context.Background(), leaseRunRecord("nocaps", "run-g2"), lease.Scope{CaptureIDs: []string{"cap-private"}}, "x", 0, leaseFields()); err == nil {
		t.Fatal("a desk with no capture ceiling was leased a capture")
	}
}

// (c) The runtime's boot pass ends the leases the last process left open, and
// does not trust them in between.
func TestBootEndsTheLeasesACrashLeftOpen(t *testing.T) {
	rt, st := testRuntime(t)
	registerLeaseApp(t, "bootlease", 62813, true)
	rt.selfdevRouteOwnerID = "owner-ma34"
	run := leaseRunRecord("bootlease", "run-boot")
	if _, err := rt.RequestLease(context.Background(), run, lease.Scope{CaptureIDs: []string{"c1"}}, "x", 0, leaseFields()); err != nil {
		t.Fatal(err)
	}
	// The process restarts: a new service over the same tape.
	rt.leases = lease.New(st.Leases())
	defer rt.leases.Close()
	ctx := objectgraph.WithLeases(context.Background(), rt.leaseAuthority(), run.RunID)
	private := json.RawMessage(`{"capture_id":"c1","labels":["owner_private:texture_document"]}`)
	if err := gateCaptureRead(ctx, "app:bootlease", private); err == nil {
		t.Fatal("a fresh process trusted an unloaded lease")
	}
	rt.endLeasesAtBoot(context.Background())
	recs, err := st.Leases().ListLeases(context.Background(), "owner-ma34", rt.TextureComputerID())
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 1 || recs[0].State != lease.StateEnded || recs[0].EndReason != lease.ReasonRestart {
		t.Fatalf("tape after boot = %+v", recs)
	}
}

// (d) The owner's routes: list shows the sentence, revoke ends the lease and
// the next capture read fails.
func TestOwnerListsAndRevokesLeases(t *testing.T) {
	rt, _ := testRuntime(t)
	registerLeaseApp(t, "revokee", 62814, true)
	run := leaseRunRecord("revokee", "run-d")
	rec, err := rt.RequestLease(context.Background(), run, lease.Scope{CaptureIDs: []string{"c1"}}, "cut the clip", 0, leaseFields())
	if err != nil {
		t.Fatal(err)
	}
	h := &APIHandler{rt: rt}
	call := func(method, path, user string, handler http.HandlerFunc) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if user != "" {
			req.Header.Set("X-Authenticated-User", user)
		}
		rec := httptest.NewRecorder()
		handler(rec, req)
		return rec
	}
	if got := call(http.MethodGet, "/api/leases", "", h.HandleLeases); got.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated list = %d", got.Code)
	}
	list := call(http.MethodGet, "/api/leases", "owner-ma34", h.HandleLeases)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "cut the clip") || !strings.Contains(list.Body.String(), rec.LeaseID) {
		t.Fatalf("list = %d %s", list.Code, list.Body.String())
	}
	// Another owner cannot revoke it.
	if got := call(http.MethodPost, "/api/leases/"+rec.LeaseID+"/revoke", "owner-intruder", h.HandleLeaseRouter); got.Code != http.StatusNotFound {
		t.Fatalf("another owner's revoke = %d", got.Code)
	}
	ctx := objectgraph.WithLeases(context.Background(), rt.leaseAuthority(), run.RunID)
	private := json.RawMessage(`{"capture_id":"c1","labels":["owner_private:texture_document"]}`)
	if err := gateCaptureRead(ctx, "app:revokee", private); err != nil {
		t.Fatalf("the lease did not serve before the revoke: %v", err)
	}
	if got := call(http.MethodPost, "/api/leases/"+rec.LeaseID+"/revoke", "owner-ma34", h.HandleLeaseRouter); got.Code != http.StatusOK {
		t.Fatalf("revoke = %d %s", got.Code, got.Body.String())
	}
	if err := gateCaptureRead(ctx, "app:revokee", private); err == nil {
		t.Fatal("the read after the owner's revoke succeeded")
	}
	// A desk never reaches the owner's lease routes through ProductAPI.
	if err := validateProductAPIRoute(http.MethodPost, "/api/leases/"+rec.LeaseID+"/revoke"); err == nil {
		t.Fatal("ProductAPI allows the owner's revoke route")
	}
}

// B2: a capture lease names owner-private labels only, and admits a capture by
// label only when every owner-private label the record carries is covered.
func TestCaptureLabelLeaseIsNotAWildcard(t *testing.T) {
	rt, _ := testRuntime(t)
	registerLeaseApp(t, "labeler", 62815, true)
	run := leaseRunRecord("labeler", "run-b2")
	ctx := objectgraph.WithLeases(context.Background(), rt.leaseAuthority(), run.RunID)

	// A ubiquitous label is refused outright.
	if _, err := rt.RequestLease(context.Background(), run, lease.Scope{CaptureLabels: []string{"world_untrusted"}}, "everything", 0, leaseFields()); err == nil {
		t.Fatal("a lease for world_untrusted was issued")
	}
	// A lease for one private label.
	if _, err := rt.RequestLease(context.Background(), run, lease.Scope{CaptureLabels: []string{"owner_private:texture_document"}}, "texture captures", 0, leaseFields()); err != nil {
		t.Fatal(err)
	}
	covered := json.RawMessage(`{"capture_id":"cap-1","labels":["owner_private:texture_document","world_untrusted"]}`)
	if err := gateCaptureRead(ctx, "app:labeler", covered); err != nil {
		t.Fatalf("a capture whose private labels are all covered was refused: %v", err)
	}
	// A private capture that also carries world_untrusted and a second private
	// label the lease does not cover is refused.
	partly := json.RawMessage(`{"capture_id":"cap-2","labels":["owner_private:texture_document","owner_private:mail_message","world_untrusted"]}`)
	if err := gateCaptureRead(ctx, "app:labeler", partly); err == nil {
		t.Fatal("a lease for one private label served a capture that carries another")
	}
}

type leaseTestSink struct{ captures map[string]string }

func (s leaseTestSink) WriteCaptures(context.Context, string, string, []sourcegraph.CaptureItem) (sourcegraph.CaptureWriteResult, error) {
	return sourcegraph.CaptureWriteResult{}, nil
}
func (s leaseTestSink) ReadCapture(_ context.Context, id string, _, _ int) (json.RawMessage, error) {
	if body, ok := s.captures[id]; ok {
		return json.RawMessage(body), nil
	}
	return nil, http.ErrMissingFile
}
func (s leaseTestSink) ListCaptures(context.Context, string, string, int) ([]objectgraph.Object, error) {
	return nil, nil
}

// B3, end to end: an app desk cell calls choir.RequestLease for an
// owner-private capture inside its ceiling, then choir.ReadCapture succeeds;
// outside the ceiling it is refused; after the run ends the next read is
// refused. The calls go through the host's egress path (deskEgress), where the
// frame layer binds the desk and the run.
func TestAppDeskRequestsALeaseThenReadsTheCapture(t *testing.T) {
	rt, _ := testRuntime(t)
	registerLeaseApp(t, "cellcaller", 62816, true)
	rt.researchDeps = &researchtools.Dependencies{Captures: leaseTestSink{captures: map[string]string{
		"cap-private": `{"capture_id":"cap-private","labels":["owner_private:texture_document"],"blocks":[]}`,
		"cap-world":   `{"capture_id":"cap-world","labels":["world_untrusted"],"blocks":[]}`,
	}}}
	run := leaseRunRecord("cellcaller", "run-b3")
	exec := toolregistry.ExecutionContext{RunID: run.RunID, OwnerID: run.OwnerID, ComputerID: run.ComputerID, AgentID: run.AgentID, RunRecord: run}
	ctx := objectgraph.WithLeases(objectgraph.WithReader(context.Background(), mustDeskWriter(t, "app:cellcaller")), rt.leaseAuthority(), run.RunID)
	role := "app:cellcaller"
	egress := func(action yaegikernel.BrokerAction, payload any) (json.RawMessage, error) {
		raw, _ := json.Marshal(payload)
		return rt.deskEgress(ctx, role, exec, action, raw)
	}
	read := func(id string) error {
		_, err := egress(yaegikernel.ActionReadCapture, yaegikernel.ReadCapturePayload{CaptureID: id})
		return err
	}
	p, w := 0.8, 0.3
	ask := func(scope yaegikernel.RequestLeasePayload) error {
		scope.Purpose = "cut the clip"
		scope.Req = &types.RequestFields{PSuccess: &p, Weight: &w, ImpactClass: types.ImpactI1, Settlement: types.SettlementRecipient}
		_, err := egress(yaegikernel.ActionRequestLease, scope)
		return err
	}

	if err := read("cap-world"); err != nil {
		t.Fatalf("a world capture was refused: %v", err)
	}
	if err := read("cap-private"); err == nil {
		t.Fatal("an owner-private capture was served before any lease")
	}
	if err := ask(yaegikernel.RequestLeasePayload{CaptureIDs: []string{"cap-private"}}); err != nil {
		t.Fatalf("a lease inside the ceiling was refused: %v", err)
	}
	if err := read("cap-private"); err != nil {
		t.Fatalf("the leased read failed: %v", err)
	}
	// Outside the ceiling: a kind the manifest does not name.
	if err := ask(yaegikernel.RequestLeasePayload{Kinds: []string{"choir.texture_document"}}); err == nil {
		t.Fatal("a kind outside the ceiling was leased")
	}
	// The holder is the session's desk and run, not anything the cell says: a
	// run record naming another desk still leases for the session's desk.
	leases, _ := rt.leases.All(context.Background(), run.OwnerID, run.ComputerID)
	if len(leases) != 1 || leases[0].Holder != "app:cellcaller" || leases[0].HolderRunID != run.RunID {
		t.Fatalf("lease holder = %+v", leases)
	}
	// The run ends: the store's terminal observer ends the lease.
	now := time.Now().UTC()
	running := *run
	running.State, running.CreatedAt, running.UpdatedAt = types.RunRunning, now, now
	if err := rt.store.CreateRun(context.Background(), running); err != nil {
		t.Fatal(err)
	}
	done := running
	done.State, done.FinishedAt = types.RunCompleted, &now
	if err := rt.store.UpdateRun(context.Background(), done); err != nil {
		t.Fatal(err)
	}
	if err := read("cap-private"); err == nil {
		t.Fatal("the read after the run ended succeeded")
	}
	if got := rt.leases.Live(run.OwnerID); len(got) != 0 {
		t.Fatalf("live leases after the run ended: %+v", got)
	}
}

func mustDeskWriter(t *testing.T, desk string) objectgraph.Writer {
	t.Helper()
	w, ok := objectgraph.DeskWriter(desk)
	if !ok {
		t.Fatalf("no writer for %s", desk)
	}
	return w
}
