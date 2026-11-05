package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// The generic object seam for app package kinds (doctrine I27; P2b of
// docs/design/app-packages-replan-2026-10-24.md). A package's kinds are
// choir.app.<package>.<kind>, registered through objectgraph.RegisterPackageKind;
// the store holds no typed record for them, only bodies in the object graph, so
// replay and restore carry them like every other kind.
//
// The only caller is the owner-act route in internal/autoputer. The writer is
// declared here, and it is always the owner: a package desk writes its kinds
// through its own seam when P4 lands, never through this one.

// PackageObject is one stored object of a package kind.
type PackageObject struct {
	Kind string          `json:"kind"`
	ID   string          `json:"id"`
	Body json.RawMessage `json:"body"`
	// SchemaVersion is the schema version of the kind the body was written
	// under (0 for an object the owner wrote before versions were stamped).
	SchemaVersion int `json:"schema_version,omitempty"`
	// Labels are the labels the writing run held (MA3.5); a reader's run
	// inherits them.
	Labels    []string  `json:"labels,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func packageObjectFrom(obj objectgraph.Object) PackageObject {
	var meta struct {
		ID            string `json:"object_id"`
		SchemaVersion int    `json:"schema_version"`
	}
	_ = json.Unmarshal(obj.Metadata, &meta)
	return PackageObject{Kind: string(obj.ObjectKind), ID: meta.ID, Body: json.RawMessage(obj.Body), SchemaVersion: meta.SchemaVersion, Labels: types.LabelsFromMetadata(obj.Metadata).Strings(), CreatedAt: obj.CreatedAt, UpdatedAt: obj.UpdatedAt}
}

// PutPackageObject writes one object of a registered package kind as the
// owner. The kind must be registered through the package seam and admit owner
// edits; the caller has already validated the body against the kind's schema.
// An object with the same (kind, owner, id) is replaced.
func (s *Store) PutPackageObject(ctx context.Context, kind objectgraph.ObjectKind, ownerID, id string, body json.RawMessage) (PackageObject, error) {
	reg, ok := objectgraph.LookupPackageKind(kind)
	if !ok {
		return PackageObject{}, fmt.Errorf("store: %q is not a registered package kind", kind)
	}
	if !reg.OwnerEdits {
		return PackageObject{}, fmt.Errorf("store: package kind %q does not admit owner edits", kind)
	}
	if strings.TrimSpace(ownerID) == "" || strings.TrimSpace(id) == "" {
		return PackageObject{}, errors.New("store: package object needs an owner and an id")
	}
	ctx = objectgraph.WithWriter(ctx, objectgraph.WriterOwner)
	obj, err := s.ogPut(ctx, kind, ownerID, id, body, map[string]any{"object_id": id}, time.Now().UTC())
	if err != nil {
		return PackageObject{}, err
	}
	return packageObjectFrom(obj), nil
}

// GetPackageObject returns one object of a package kind, or ErrNotFound.
func (s *Store) GetPackageObject(ctx context.Context, kind objectgraph.ObjectKind, ownerID, id string) (PackageObject, error) {
	if _, ok := objectgraph.LookupPackageKind(kind); !ok {
		return PackageObject{}, fmt.Errorf("store: %q is not a registered package kind", kind)
	}
	obj, err := s.ogGetByIdentityKey(ctx, kind, ownerID, "", id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return PackageObject{}, ErrNotFound
	}
	if err != nil {
		return PackageObject{}, err
	}
	if obj.Tombstone {
		return PackageObject{}, ErrNotFound
	}
	return packageObjectFrom(obj), nil
}

// ListPackageObjects lists up to limit of the owner's objects of a package
// kind, newest first.
func (s *Store) ListPackageObjects(ctx context.Context, kind objectgraph.ObjectKind, ownerID string, limit int) ([]PackageObject, error) {
	if _, ok := objectgraph.LookupPackageKind(kind); !ok {
		return nil, fmt.Errorf("store: %q is not a registered package kind", kind)
	}
	objs, err := s.ogListByOwnerAndBody(ctx, kind, ownerID, nil, limit)
	if err != nil {
		return nil, err
	}
	out := make([]PackageObject, 0, len(objs))
	for _, obj := range objs {
		if !obj.Tombstone {
			out = append(out, packageObjectFrom(obj))
		}
	}
	return out, nil
}
