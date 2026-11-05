package agentcore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// MA3.8 and MA3.9, end to end through the reducer and the host's egress.
//
// Failure modes the tests hold (written before the code):
//   - a cell supplies a writer: the intent has no field for one, and the
//     writer is the run record's desk, so a desk naming another package's kind,
//     a built-in kind or an unregistered kind is refused;
//   - a built-in desk puts a package object;
//   - a body breaks the kind's schema or the size bound, or an id is malformed;
//   - a Put replaces nothing it should not, a Retract leaves the object readable;
//   - a desk reads another package's objects through Get or List;
//   - a desk past its object quota, its storage quota or its write rate is
//     committed anyway (the whole cell must stay uncommitted);
//   - a desk wakes past its wake rate;
//   - a manifest whose schedules wake the desk past the bound installs.

const (
	objNote   = "choir.app.p2btest.note"
	objLedger = "choir.app.p2btest.ledger"
)

func registerObjectsPackage(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile("../apppackage/testdata/p2btest.choir-package.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := apppackage.ParseManifest(raw, apppackage.OriginFirstParty)
	if err != nil {
		t.Fatal(err)
	}
	if err := apppackage.RegisterManifest(m); err != nil {
		t.Fatal(err)
	}
	for slug, uid := range map[string]uint32{"p2btest": 62961, "otherpkg": 62962} {
		if _, ok := deskmanifest.Lookup("app:" + slug); ok {
			continue
		}
		d, err := deskmanifest.NewAppDesk(deskmanifest.AppDeskSpec{
			Name: "app:" + slug, Verbs: []string{"Put", "Get", "List", "Retract", "Done"},
			Taint: deskmanifest.TaintUntrustedInput, UID: uid, WorkerMaxBytes: 64 << 20,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := deskmanifest.RegisterApp(d); err != nil {
			t.Fatal(err)
		}
		slug := slug
		t.Cleanup(func() { _ = deskmanifest.RetireApp("app:" + slug) })
	}
}

func objectReduction(rt *Runtime, role, owner string) (*rlmCallReduction, context.Context) {
	run := types.RunRecord{RunID: "run-obj-" + role, AgentID: "agent-" + role, AgentProfile: role, OwnerID: owner, ComputerID: "autoputer-test"}
	scope := ReductionScope{
		FromAgentID: run.AgentID, DeskAgentID: run.AgentID, FromRole: role, ChannelID: run.AgentID,
		RunID: run.RunID, OwnerID: owner, ComputerID: run.ComputerID, CellID: "cell-1",
	}
	ctx := toolregistry.WithExecutionContext(context.Background(), toolregistry.ExecutionContext{
		RunID: run.RunID, AgentID: run.AgentID, OwnerID: owner, ComputerID: run.ComputerID, RunRecord: &run, Profile: role, Role: role,
	})
	return &rlmCallReduction{active: true, mb: rt, st: rt.store, ledger: rt.store, scope: scope, rec: &run}, ctx
}

// taint makes the run hold a label, as a read of the owner's capture would.
func taint(rt *Runtime, r *rlmCallReduction) {
	rt.taintRun(r.scope.RunID, types.NewLabelSet(types.OwnerPrivateLabel("capture")))
}

func put(local, kind, id, body string) yaegikernel.StagedIntent {
	return yaegikernel.StagedIntent{LocalID: local, Kind: yaegikernel.IntentObjectPut, ObjectKind: kind, ObjectID: id, Body: body}
}

func retract(local, kind, id string) yaegikernel.StagedIntent {
	return yaegikernel.StagedIntent{LocalID: local, Kind: yaegikernel.IntentObjectRetract, ObjectKind: kind, ObjectID: id}
}

func deskCall(t *testing.T, rt *Runtime, role, owner string, action yaegikernel.BrokerAction, payload any) (json.RawMessage, error) {
	t.Helper()
	raw, _ := json.Marshal(payload)
	exec := toolregistry.ExecutionContext{RunID: "run-call", OwnerID: owner, ComputerID: "autoputer-test", AgentID: "agent-call"}
	return rt.deskEgress(context.Background(), role, exec, action, raw)
}

func TestPackageDeskPutsGetsListsAndRetractsItsOwnObjects(t *testing.T) {
	registerObjectsPackage(t)
	rt, _ := testRuntime(t)
	r, ctx := objectReduction(rt, "app:p2btest", "owner-objs")
	taint(rt, r)

	if err := r.commit(ctx, []yaegikernel.StagedIntent{
		put("t1", objNote, "n1", `{"title":"first","priority":2}`),
		put("t2", objNote, "n2", `{"title":"second","priority":3,"tags":["x"]}`),
	}); err != nil {
		t.Fatalf("commit: %v", err)
	}

	raw, err := deskCall(t, rt, "app:p2btest", "owner-objs", yaegikernel.ActionObjectGet, yaegikernel.ObjectGetPayload{Kind: objNote, ID: "n1"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	var got struct {
		ID            string          `json:"id"`
		Body          json.RawMessage `json:"body"`
		SchemaVersion int             `json:"schema_version"`
		Labels        []string        `json:"labels"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "n1" || got.SchemaVersion != 1 || !strings.Contains(string(got.Body), `"first"`) {
		t.Fatalf("Get = %s", raw)
	}
	if len(got.Labels) != 1 || got.Labels[0] != types.OwnerPrivateLabel("capture") {
		t.Fatalf("the object lost the writing run's labels: %v", got.Labels)
	}

	// The object was written as the package's desk.
	obj, err := rt.store.GetPackageObject(objectgraph.WithReader(context.Background(), objectgraph.WriterOwner), objNote, "owner-objs", "n1")
	if err != nil || obj.SchemaVersion != 1 {
		t.Fatalf("stored object = %+v, %v", obj, err)
	}

	list, err := deskCall(t, rt, "app:p2btest", "owner-objs", yaegikernel.ActionObjectList, yaegikernel.ObjectListPayload{Kind: objNote})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	var page struct {
		Objects []struct{ ID string } `json:"objects"`
	}
	_ = json.Unmarshal(list, &page)
	if len(page.Objects) != 2 {
		t.Fatalf("List = %s", list)
	}

	// Put again replaces.
	if err := r.commit(ctx, []yaegikernel.StagedIntent{put("t3", objNote, "n1", `{"title":"changed","priority":1}`)}); err != nil {
		t.Fatal(err)
	}
	raw, _ = deskCall(t, rt, "app:p2btest", "owner-objs", yaegikernel.ActionObjectGet, yaegikernel.ObjectGetPayload{Kind: objNote, ID: "n1"})
	if !strings.Contains(string(raw), `"changed"`) {
		t.Fatalf("Put did not replace: %s", raw)
	}

	// Retract withdraws.
	if err := r.commit(ctx, []yaegikernel.StagedIntent{retract("t4", objNote, "n1")}); err != nil {
		t.Fatalf("retract: %v", err)
	}
	if _, err := deskCall(t, rt, "app:p2btest", "owner-objs", yaegikernel.ActionObjectGet, yaegikernel.ObjectGetPayload{Kind: objNote, ID: "n1"}); err == nil {
		t.Fatal("a retracted object is still readable")
	}
	list, _ = deskCall(t, rt, "app:p2btest", "owner-objs", yaegikernel.ActionObjectList, yaegikernel.ObjectListPayload{Kind: objNote})
	_ = json.Unmarshal(list, &page)
	if len(page.Objects) != 1 {
		t.Fatalf("List after retract = %s", list)
	}
	// Another retract of the same object is the cell's error.
	if err := r.commit(ctx, []yaegikernel.StagedIntent{retract("t5", objNote, "n1")}); err == nil {
		t.Fatal("retracting a missing object succeeded")
	}
}

func TestObjectWritesAreRefusedForTheWrongDeskKindBodyOrId(t *testing.T) {
	registerObjectsPackage(t)
	rt, _ := testRuntime(t)
	big := `{"title":"a","priority":1,"meta":{"by":"` + strings.Repeat("x", yaegikernel.MaxObjectBody) + `"}}`
	cases := []struct {
		name   string
		role   string
		intent yaegikernel.StagedIntent
		want   string
	}{
		{"another package's kind", "app:otherpkg", put("a", objNote, "n1", `{"title":"a","priority":1}`), "not a kind of package otherpkg"},
		{"a built-in kind", "app:p2btest", put("b", "choir.texture_document", "n1", `{}`), "not a kind of package p2btest"},
		{"an unregistered kind in its namespace", "app:p2btest", put("c", "choir.app.p2btest.nothing", "n1", `{}`), "not registered"},
		{"a built-in desk", agentprofile.Management, put("d", objNote, "n1", `{"title":"a","priority":1}`), "not a package desk"},
		{"a body that breaks the schema", "app:p2btest", put("e", objNote, "n1", `{"title":"a"}`), "breaks the schema"},
		{"a wrong type", "app:p2btest", put("f", objNote, "n1", `{"title":1,"priority":1}`), "breaks the schema"},
		{"an oversize body", "app:p2btest", put("g", objNote, "n1", big), "exceeds"},
		{"a malformed id", "app:p2btest", put("h", objNote, "../x", `{"title":"a","priority":1}`), "object id"},
		{"a retract of another package's kind", "app:otherpkg", retract("i", objNote, "n1"), "not a kind of package otherpkg"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, ctx := objectReduction(rt, tc.role, "owner-refused")
			err := r.commit(ctx, []yaegikernel.StagedIntent{tc.intent})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
	// Nothing landed.
	if _, err := rt.store.GetPackageObject(objectgraph.WithReader(context.Background(), objectgraph.WriterOwner), objNote, "owner-refused", "n1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a refused write left an object: %v", err)
	}
}

// A cell cannot supply a writer: the intent decodes with no such field, and the
// store refuses a writer that is not the kind's even when the reducer is
// bypassed.
func TestACellCannotSupplyTheWriter(t *testing.T) {
	registerObjectsPackage(t)
	rt, _ := testRuntime(t)
	var in yaegikernel.StagedIntent
	raw := `{"local_id":"x","kind":"object_put","object_kind":"` + objNote + `","object_id":"n9","body":"{\"title\":\"a\",\"priority\":1}","writer":"desk:management","desk":"management"}`
	if err := json.Unmarshal([]byte(raw), &in); err != nil {
		t.Fatal(err)
	}
	r, ctx := objectReduction(rt, "app:otherpkg", "owner-writer")
	if err := r.commit(ctx, []yaegikernel.StagedIntent{in}); err == nil {
		t.Fatal("another package's desk wrote a kind it does not own")
	}
	_, err := rt.store.PutDeskObject(ctx, objectgraph.WriterManagement, objNote, "owner-writer", "n9", json.RawMessage(`{"title":"a","priority":1}`), 1, "", nil)
	if !errors.Is(err, objectgraph.ErrWrongWriter) {
		t.Fatalf("store accepted a wrong writer: %v", err)
	}
}

func TestADeskReadsOnlyItsOwnPackagesObjects(t *testing.T) {
	registerObjectsPackage(t)
	rt, _ := testRuntime(t)
	r, ctx := objectReduction(rt, "app:p2btest", "owner-read")
	taint(rt, r)
	if err := r.commit(ctx, []yaegikernel.StagedIntent{put("a", objNote, "n1", `{"title":"a","priority":1}`)}); err != nil {
		t.Fatal(err)
	}
	if _, err := deskCall(t, rt, "app:otherpkg", "owner-read", yaegikernel.ActionObjectGet, yaegikernel.ObjectGetPayload{Kind: objNote, ID: "n1"}); err == nil {
		t.Fatal("another package's desk read the object")
	}
	if _, err := deskCall(t, rt, "app:otherpkg", "owner-read", yaegikernel.ActionObjectList, yaegikernel.ObjectListPayload{Kind: objNote}); err == nil {
		t.Fatal("another package's desk listed the kind")
	}
	if _, err := deskCall(t, rt, agentprofile.Management, "owner-read", yaegikernel.ActionObjectGet, yaegikernel.ObjectGetPayload{Kind: objNote, ID: "n1"}); err == nil {
		t.Fatal("a built-in desk read a package object")
	}
	if _, err := deskCall(t, rt, "app:p2btest", "owner-read", yaegikernel.ActionObjectGet, yaegikernel.ObjectGetPayload{Kind: "choir.texture_document", ID: "n1"}); err == nil {
		t.Fatal("a package desk read a built-in kind through Get")
	}
	// The object is the owner's: another owner's desk finds nothing.
	if _, err := deskCall(t, rt, "app:p2btest", "owner-other", yaegikernel.ActionObjectGet, yaegikernel.ObjectGetPayload{Kind: objNote, ID: "n1"}); err == nil {
		t.Fatal("a desk read another owner's object")
	}
	// The read taints the run with the owner's package data.
	raw, _ := deskCall(t, rt, "app:p2btest", "owner-read", yaegikernel.ActionObjectGet, yaegikernel.ObjectGetPayload{Kind: objNote, ID: "n1"})
	labels, ok := egressReadLabels(yaegikernel.ActionObjectGet, nil, raw)
	if !ok || !labels.Has(types.OwnerPrivateLabel("app_object")) || !labels.Has(types.OwnerPrivateLabel("capture")) {
		t.Fatalf("read labels = %v, %v", labels, ok)
	}
}
