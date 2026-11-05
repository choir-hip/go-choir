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

// A package desk's objects (MA3.8). The desk is the writer of its package's
// kinds; the writer is the one the reducer read from the run record, passed
// here as an argument and checked against the kind's registration. A cell
// supplies a kind, an id and a body, never a writer.

// ErrDeskObjectRefused refuses a desk object write that the store itself
// checks: an unregistered kind, a kind of another writer, a bad id.
var ErrDeskObjectRefused = errors.New("store: desk object refused")

func deskKind(writer objectgraph.Writer, kind objectgraph.ObjectKind) (objectgraph.KindRegistration, error) {
	reg, ok := objectgraph.LookupPackageKind(kind)
	if !ok {
		return objectgraph.KindRegistration{}, fmt.Errorf("%w: %q is not a registered package kind", ErrDeskObjectRefused, kind)
	}
	if reg.Writer != writer {
		return objectgraph.KindRegistration{}, fmt.Errorf("%w: %w: kind %q is written by %q, not %q", ErrDeskObjectRefused, objectgraph.ErrWrongWriter, kind, reg.Writer, writer)
	}
	return reg, nil
}

// PutDeskObject writes one object of a package kind as that package's desk.
// schemaVersion is the version of the kind's schema the caller validated the
// body against; it is stamped on the object. labels are the run's labels at
// the write (MA3.5). An object with the same (kind, owner, id) is replaced.
func (s *Store) PutDeskObject(ctx context.Context, writer objectgraph.Writer, kind objectgraph.ObjectKind, ownerID, id string, body json.RawMessage, schemaVersion int, runID string, labels types.LabelSet) (PackageObject, error) {
	if _, err := deskKind(writer, kind); err != nil {
		return PackageObject{}, err
	}
	if strings.TrimSpace(ownerID) == "" || !objectgraph.ValidPackageObjectID(id) {
		return PackageObject{}, fmt.Errorf("%w: an owner and a valid object id are required", ErrDeskObjectRefused)
	}
	meta := map[string]any{"object_id": id, "schema_version": schemaVersion, "written_by": string(writer)}
	if runID != "" {
		meta["run_id"] = runID
	}
	if len(labels) > 0 {
		raw, _ := json.Marshal(meta)
		stamped := types.StampMetadataLabels(raw, labels)
		meta = map[string]any{}
		if err := json.Unmarshal(stamped, &meta); err != nil {
			return PackageObject{}, err
		}
	}
	ctx = objectgraph.WithWriter(ctx, writer)
	obj, err := s.ogPut(ctx, kind, ownerID, id, body, meta, time.Now().UTC())
	if err != nil {
		return PackageObject{}, err
	}
	return packageObjectFrom(obj), nil
}

// RetractDeskObject withdraws one object of a package kind as that package's
// desk: the object is tombstoned, so a read no longer finds it and the tape
// keeps the history. It returns ErrNotFound when no live object has the id.
func (s *Store) RetractDeskObject(ctx context.Context, writer objectgraph.Writer, kind objectgraph.ObjectKind, ownerID, id string) error {
	if _, err := deskKind(writer, kind); err != nil {
		return err
	}
	if s.ogStore == nil {
		return errors.New("store: object graph not initialized")
	}
	ctx = objectgraph.WithWriter(ctx, writer)
	obj, err := s.ogGetByIdentityKey(ctx, kind, ownerID, "", id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if obj.Tombstone {
		return ErrNotFound
	}
	obj.Tombstone = true
	obj.UpdatedAt = time.Now().UTC()
	return s.ogStore.PutObject(ctx, obj)
}

// DeskObjectUsage is the live object count and stored bytes the owner's
// objects of the given package kinds hold: the figure the per-desk storage
// quota is held against (MA3.9).
func (s *Store) DeskObjectUsage(ctx context.Context, ownerID string, kinds []objectgraph.ObjectKind) (objects int, bytes int64, err error) {
	if s.ogStore == nil {
		return 0, 0, errors.New("store: object graph not initialized")
	}
	for _, kind := range kinds {
		n, b, err := s.ogStore.KindUsage(ctx, kind, ownerID)
		if err != nil {
			return 0, 0, err
		}
		objects += n
		bytes += b
	}
	return objects, bytes, nil
}
