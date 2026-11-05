package agentcore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/store"
	"github.com/yusefmosiah/go-choir/internal/toolregistry"
	"github.com/yusefmosiah/go-choir/internal/types"
	"github.com/yusefmosiah/go-choir/internal/yaegikernel"
)

// Package desks' object verbs (MA3.8). A package desk keeps its data as
// objects of its own package's kinds (choir.app.<pkg>.<kind>). Put and Retract
// stage tray intents the reducer commits; Get and List are host reads. In all
// four the desk is the run record's desk, never anything the cell says: the
// intent has no writer field, the reducer declares the writer from the record
// (objectgraph.DeskWriter), and the host reads as the session's desk.

const (
	deskObjectDefaultPage = 50
	deskObjectMaxPage     = 200
)

// appDeskPackage returns the package an app desk belongs to. role is the
// desk's registered name (app:<pkg>) or the bare package name; a built-in
// desk is no package's.
func appDeskPackage(role string) (string, bool) {
	role = strings.TrimSpace(role)
	d, ok := deskmanifest.Lookup(role)
	if !ok {
		d, ok = deskmanifest.Lookup("app:" + role)
	}
	if !ok || d.Kind != deskmanifest.KindApp {
		return "", false
	}
	return strings.TrimPrefix(d.Name, "app:"), true
}

// deskObjectTarget is a Put, Get, List or Retract resolved against the desk:
// the writer the run's desk holds and the kind's registration.
type deskObjectTarget struct {
	pkg     string
	writer  objectgraph.Writer
	kind    objectgraph.ObjectKind
	reg     objectgraph.KindRegistration
	version int
}

// resolveDeskObject checks that role is a package desk and that kind is one of
// its package's registered kinds. It does not look at an id or a body.
func resolveDeskObject(role, kind string) (deskObjectTarget, error) {
	pkg, ok := appDeskPackage(role)
	if !ok {
		return deskObjectTarget{}, fmt.Errorf("desk %q is not a package desk; only a package desk keeps package objects", strings.TrimSpace(role))
	}
	writer, ok := objectgraph.DeskWriter(role)
	if !ok {
		return deskObjectTarget{}, fmt.Errorf("desk %q has no writer", strings.TrimSpace(role))
	}
	k := objectgraph.ObjectKind(strings.TrimSpace(kind))
	if owner, ok := objectgraph.PackageOfKind(k); !ok || owner != pkg {
		return deskObjectTarget{}, fmt.Errorf("kind %q is not a kind of package %s (%s<kind>)", kind, pkg, objectgraph.PackageKindNamespace(pkg))
	}
	reg, ok := objectgraph.LookupPackageKind(k)
	if !ok {
		return deskObjectTarget{}, fmt.Errorf("kind %q is not registered: the package %s declares no such kind", kind, pkg)
	}
	if reg.Writer != writer {
		return deskObjectTarget{}, fmt.Errorf("kind %q is written by %q, not by %q", kind, reg.Writer, writer)
	}
	version := 0
	if _, v, ok := strings.Cut(reg.Schema, "@"); ok {
		version, _ = strconv.Atoi(v)
	}
	return deskObjectTarget{pkg: pkg, writer: writer, kind: k, reg: reg, version: version}, nil
}

// validateObjectIntent is the reducer's pure check of a Put or Retract: the
// desk is a package desk, the kind is its own, the id is well formed and a
// Put's body is within the size bound and satisfies the kind's schema.
func validateObjectIntent(role string, in yaegikernel.StagedIntent) error {
	t, err := resolveDeskObject(role, in.ObjectKind)
	if err != nil {
		return fmt.Errorf("reduce: %s %s: %w", in.Kind, in.LocalID, err)
	}
	if !objectgraph.ValidPackageObjectID(in.ObjectID) {
		return fmt.Errorf("reduce: %s %s: object id %q must match %s", in.Kind, in.LocalID, in.ObjectID, "[A-Za-z0-9][A-Za-z0-9._-]{0,127}")
	}
	if in.Kind != yaegikernel.IntentObjectPut {
		return nil
	}
	if len(in.Body) > yaegikernel.MaxObjectBody {
		return fmt.Errorf("reduce: put %s: body %d bytes exceeds %d", in.LocalID, len(in.Body), yaegikernel.MaxObjectBody)
	}
	schema, ok := apppackage.SchemaFor(t.kind)
	if !ok {
		return fmt.Errorf("reduce: put %s: kind %q has no schema", in.LocalID, t.kind)
	}
	if err := schema.Validate([]byte(in.Body)); err != nil {
		return fmt.Errorf("reduce: put %s: body breaks the schema of %s (version %d): %w", in.LocalID, t.kind, t.version, err)
	}
	return nil
}

// objectRole is the desk the reduction's run belongs to: the run record's, and
// the scope's only when the record is absent.
func (r *rlmCallReduction) objectRole() string {
	if r.rec != nil && strings.TrimSpace(r.rec.AgentProfile) != "" {
		return strings.TrimSpace(r.rec.AgentProfile)
	}
	return strings.TrimSpace(r.scope.FromRole)
}

func objectIntent(in yaegikernel.StagedIntent) bool {
	return in.Kind == yaegikernel.IntentObjectPut || in.Kind == yaegikernel.IntentObjectRetract
}

// preflightObjectIntents holds a cell's object writes against the desk's
// quotas before anything commits (MA3.9): the live objects and stored bytes the
// owner's package kinds would hold afterwards, and the writes in the rolling
// hour. A refusal leaves the whole cell uncommitted.
func (r *rlmCallReduction) preflightObjectIntents(ctx context.Context, intents []yaegikernel.StagedIntent) error {
	var writes []yaegikernel.StagedIntent
	for _, in := range intents {
		if objectIntent(in) {
			writes = append(writes, in)
		}
	}
	if len(writes) == 0 {
		return nil
	}
	role := r.objectRole()
	if r.ledger == nil {
		return errors.New("reduce: object writes need the store")
	}
	first, err := resolveDeskObject(role, writes[0].ObjectKind)
	if err != nil {
		return fmt.Errorf("reduce: %s %s: %w", writes[0].Kind, writes[0].LocalID, err)
	}
	limits := r.rt().limitsForDesks()
	readCtx := objectgraph.WithReader(ctx, first.writer)
	var kinds []objectgraph.ObjectKind
	for _, reg := range objectgraph.PackageKindsOf(first.pkg) {
		kinds = append(kinds, reg.Kind)
	}
	objects, bytes, err := r.ledger.DeskObjectUsage(readCtx, r.scope.OwnerID, kinds)
	if err != nil {
		return fmt.Errorf("reduce: read %s's object usage: %w", role, err)
	}
	// The cell's writes in order against what the owner holds: a Put adds an
	// object or replaces one, a Retract frees one, and a Retract then a Put in
	// one cell nets out.
	held := map[string]int64{} // kind|id -> body bytes, -1 when none is live
	size := func(in yaegikernel.StagedIntent) (int64, error) {
		key := in.ObjectKind + "|" + in.ObjectID
		if n, ok := held[key]; ok {
			return n, nil
		}
		prior, err := r.ledger.GetPackageObject(readCtx, objectgraph.ObjectKind(in.ObjectKind), r.scope.OwnerID, in.ObjectID)
		switch {
		case err == nil:
			held[key] = int64(len(prior.Body))
		case errors.Is(err, store.ErrNotFound):
			held[key] = -1
		default:
			return 0, fmt.Errorf("reduce: read %s %s: %w", in.ObjectKind, in.ObjectID, err)
		}
		return held[key], nil
	}
	for _, in := range writes {
		key := in.ObjectKind + "|" + in.ObjectID
		prior, err := size(in)
		if err != nil {
			return err
		}
		switch {
		case in.Kind == yaegikernel.IntentObjectPut && prior >= 0:
			bytes += int64(len(in.Body)) - prior
			held[key] = int64(len(in.Body))
		case in.Kind == yaegikernel.IntentObjectPut:
			objects++
			bytes += int64(len(in.Body))
			held[key] = int64(len(in.Body))
		case prior >= 0:
			objects--
			bytes -= prior
			held[key] = -1
		}
	}
	if objects > limits.Objects {
		return fmt.Errorf("reduce: %w: desk %s would keep %d objects (at most %d); Retract some first", ErrDeskQuota, role, objects, limits.Objects)
	}
	if bytes > limits.Bytes {
		return fmt.Errorf("reduce: %w: desk %s would keep %d bytes of objects (at most %d); Retract or shrink some first", ErrDeskQuota, role, bytes, limits.Bytes)
	}
	if held, ok := r.rt().deskRates.take("write:"+role, len(writes), limits.WritesPerHour, time.Now()); !ok {
		return fmt.Errorf("reduce: %w: desk %s made %d object writes in the last hour (at most %d)", ErrDeskQuota, role, held, limits.WritesPerHour)
	}
	return nil
}

// commitObjectIntent writes one Put or Retract as the run's desk. The writer
// is read from the run record (objectRole), the kind and body were checked by
// validateObjectIntent, and the object carries the schema version of the kind
// as registered, the run and the run's labels.
func (r *rlmCallReduction) commitObjectIntent(ctx context.Context, in yaegikernel.StagedIntent) error {
	if r.ledger == nil {
		return errors.New("object writes need the store")
	}
	t, err := resolveDeskObject(r.objectRole(), in.ObjectKind)
	if err != nil {
		return err
	}
	ctx = objectgraph.WithWriter(objectgraph.WithReader(ctx, t.writer), t.writer)
	if in.Kind == yaegikernel.IntentObjectRetract {
		err := r.ledger.RetractDeskObject(ctx, t.writer, t.kind, r.scope.OwnerID, in.ObjectID)
		if errors.Is(err, store.ErrNotFound) {
			return fmt.Errorf("no live object %s of %s to retract", in.ObjectID, t.kind)
		}
		return err
	}
	_, err = r.ledger.PutDeskObject(ctx, t.writer, t.kind, r.scope.OwnerID, in.ObjectID, json.RawMessage(in.Body), t.version, r.scope.RunID, r.scope.Labels)
	return err
}

// deskObjectJSON is what Get and List return to the cell.
func deskObjectJSON(o store.PackageObject) map[string]any {
	out := map[string]any{
		"kind": o.Kind, "id": o.ID, "body": json.RawMessage(o.Body), "schema_version": o.SchemaVersion,
		"created_at": o.CreatedAt, "updated_at": o.UpdatedAt,
	}
	return out
}

// objectGetEgress serves choir.Get: one live object of the session's package
// desk, read as that desk.
func (rt *Runtime) objectGetEgress(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	var req yaegikernel.ObjectGetPayload
	if err := decodeStrict(payload, &req); err != nil {
		return nil, fmt.Errorf("choir.Get: decode the call: %w", err)
	}
	t, err := resolveDeskObject(deskRole, req.Kind)
	if err != nil {
		return nil, fmt.Errorf("choir.Get: %w", err)
	}
	if !objectgraph.ValidPackageObjectID(req.ID) {
		return nil, fmt.Errorf("choir.Get: object id %q is not a valid id", req.ID)
	}
	if rt == nil || rt.store == nil {
		return nil, errors.New("choir.Get: no store on this computer")
	}
	obj, err := rt.store.GetPackageObject(objectgraph.WithReader(ctx, t.writer), t.kind, exec.OwnerID, req.ID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("choir.Get: no object %s of %s", req.ID, t.kind)
	}
	if err != nil {
		return nil, fmt.Errorf("choir.Get: %w", err)
	}
	out := deskObjectJSON(obj)
	out["labels"] = obj.Labels
	return json.Marshal(out)
}

// objectListEgress serves choir.List: the session's package desk's live objects
// of one of its kinds, newest first.
func (rt *Runtime) objectListEgress(ctx context.Context, deskRole string, exec toolregistry.ExecutionContext, payload json.RawMessage) (json.RawMessage, error) {
	var req yaegikernel.ObjectListPayload
	if err := decodeStrict(payload, &req); err != nil {
		return nil, fmt.Errorf("choir.List: decode the call: %w", err)
	}
	t, err := resolveDeskObject(deskRole, req.Kind)
	if err != nil {
		return nil, fmt.Errorf("choir.List: %w", err)
	}
	limit := req.Limit
	if limit <= 0 {
		limit = deskObjectDefaultPage
	}
	if limit > deskObjectMaxPage {
		limit = deskObjectMaxPage
	}
	if rt == nil || rt.store == nil {
		return nil, errors.New("choir.List: no store on this computer")
	}
	objs, err := rt.store.ListPackageObjects(objectgraph.WithReader(ctx, t.writer), t.kind, exec.OwnerID, limit)
	if err != nil {
		return nil, fmt.Errorf("choir.List: %w", err)
	}
	items := make([]map[string]any, 0, len(objs))
	labels := types.LabelSet{}
	for _, o := range objs {
		items = append(items, deskObjectJSON(o))
		labels = labels.Union(types.NewLabelSet(o.Labels...))
	}
	return json.Marshal(map[string]any{"objects": items, "labels": labels.Strings()})
}

// objectReadLabels labels a Get or List result: the owner's package data, and
// what the writing runs held.
func objectReadLabels(result json.RawMessage) types.LabelSet {
	var body struct {
		Labels []string `json:"labels"`
	}
	_ = json.Unmarshal(result, &body)
	return types.NewLabelSet(types.OwnerPrivateLabel("app_object")).Union(types.NewLabelSet(body.Labels...))
}

func decodeStrict(raw json.RawMessage, into any) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	return dec.Decode(into)
}
