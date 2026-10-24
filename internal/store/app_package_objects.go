package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
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

// SettingAppPackageObjects turns on the owner's route to package kinds
// (/api/app-packages/{package}/objects/{kind}). Read live on every request, so
// the owner turns it on for one computer, and off again, with no deploy and no
// restart. Off by default; off is the rollback: the route answers 404, objects
// already written stay on the tape as inert rows that replay without a registry
// lookup.
const SettingAppPackageObjects = "app_packages.objects"

func init() { KnownComputerSettings[SettingAppPackageObjects] = false }

// PackageObject is one stored object of a package kind.
type PackageObject struct {
	Kind      string          `json:"kind"`
	ID        string          `json:"id"`
	Body      json.RawMessage `json:"body"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func packageObjectFrom(obj objectgraph.Object) PackageObject {
	var meta struct {
		ID string `json:"object_id"`
	}
	_ = json.Unmarshal(obj.Metadata, &meta)
	return PackageObject{Kind: string(obj.ObjectKind), ID: meta.ID, Body: json.RawMessage(obj.Body), CreatedAt: obj.CreatedAt, UpdatedAt: obj.UpdatedAt}
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
