package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/apppackage"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// App packages (S7-0a, docs/design/s7-0-app-packages-2026-10-14.md §5).
//
// A computer's installed packages are the fold of its package lifecycle
// events. Each event is one immutable choir.app_package_event object; each
// package datum is one choir.app_package_datum object. Both reach the tape
// as ordinary object-graph projection ops, so a pinned-head restore replays
// package presence and data, and a binary that predates packages replays
// them as inert rows (design D2). There is no cached inventory beside the
// events: the events are the single authority.
const (
	ogKindAppPackageEvent = objectgraph.ObjectKind("choir.app_package_event")
	ogKindAppPackageDatum = objectgraph.ObjectKind("choir.app_package_datum")
)

func appPackageScope(computerID, pkg string) string {
	return strings.TrimSpace(computerID) + "/" + strings.TrimSpace(pkg)
}

// AppPackageEvents returns a computer's package lifecycle events in seq
// order.
func (s *Store) AppPackageEvents(ctx context.Context, computerID string) ([]apppackage.Event, error) {
	computerID = strings.TrimSpace(computerID)
	if computerID == "" {
		return nil, fmt.Errorf("app packages: computer_id is required")
	}
	objs, err := s.ogListAllByMetadata(ctx, ogKindAppPackageEvent, "computer_id", computerID)
	if err != nil {
		return nil, fmt.Errorf("app packages: list events: %w", err)
	}
	events := make([]apppackage.Event, 0, len(objs))
	for _, obj := range objs {
		if obj.Tombstone {
			continue
		}
		var ev apppackage.Event
		if err := ogDecode(obj, &ev); err != nil {
			return nil, fmt.Errorf("app packages: decode event %s: %w", obj.CanonicalID, err)
		}
		events = append(events, ev)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Seq < events[j].Seq })
	return events, nil
}

// AppPackageInventory folds a computer's package events. A broken chain is
// an error, never a silently partial inventory.
func (s *Store) AppPackageInventory(ctx context.Context, computerID string) (apppackage.Inventory, error) {
	events, err := s.AppPackageEvents(ctx, computerID)
	if err != nil {
		return apppackage.Inventory{}, err
	}
	return apppackage.Reduce(events)
}

// RecordAppPackageEvent applies one lifecycle act: it chains ev onto the
// current fold, runs the pure reducer, and appends the event (and, for a
// delete_data purge, the tombstones of the package's data) as one batch. A
// refused act appends nothing. It returns the new inventory and the
// recorded event.
func (s *Store) RecordAppPackageEvent(ctx context.Context, ownerID, computerID string, ev apppackage.Event) (apppackage.Inventory, apppackage.Event, error) {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return apppackage.Inventory{}, apppackage.Event{}, fmt.Errorf("app packages: owner_id and computer_id are required")
	}
	if s == nil || s.ogStore == nil {
		return apppackage.Inventory{}, apppackage.Event{}, fmt.Errorf("app packages: object graph not initialized")
	}
	s.appPackageMu.Lock()
	defer s.appPackageMu.Unlock()

	inv, err := s.AppPackageInventory(ctx, computerID)
	if err != nil {
		return apppackage.Inventory{}, apppackage.Event{}, err
	}
	if ev.Op == apppackage.OpPurge && ev.RetainedDataSchemaVersion == 0 {
		ev = apppackage.PurgeEvent(inv, ev.Package)
	}
	now := time.Now().UTC()
	ev.Actor = "owner:" + ownerID
	ev.At = now.Format(time.RFC3339Nano)
	chained, err := apppackage.Chain(inv, ev)
	if err != nil {
		return apppackage.Inventory{}, apppackage.Event{}, err
	}
	next, effect, err := apppackage.Apply(inv, chained)
	if err != nil {
		return apppackage.Inventory{}, apppackage.Event{}, err
	}
	digest, err := chained.Digest()
	if err != nil {
		return apppackage.Inventory{}, apppackage.Event{}, err
	}
	metadata := map[string]any{
		"computer_id":  computerID,
		"package":      chained.Package,
		"op":           string(chained.Op),
		"seq":          strconv.FormatUint(chained.Seq, 10),
		"event_digest": digest,
		"created_at":   ev.At,
	}
	obj, err := lifecycleObject(ogKindAppPackageEvent, ownerID, computerID, "app-package-event:"+strconv.FormatUint(chained.Seq, 10), chained, metadata, now, now)
	if err != nil {
		return apppackage.Inventory{}, apppackage.Event{}, err
	}
	objects := []objectgraph.Object{obj}
	if effect.DeleteData {
		tombstones, err := s.appPackageDataTombstones(ctx, computerID, chained.Package, now)
		if err != nil {
			return apppackage.Inventory{}, apppackage.Event{}, err
		}
		objects = append(objects, tombstones...)
	}
	// The seq-derived id must be new: a second writer that raced past the
	// mutex (another process) cannot fork the chain.
	condition := objectgraph.ObjectCondition{CanonicalID: obj.CanonicalID, Exists: false}
	if err := s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{condition}, objectgraph.Batch{Objects: objects}); err != nil {
		return apppackage.Inventory{}, apppackage.Event{}, fmt.Errorf("app packages: record %s %s: %w", chained.Op, chained.Package, err)
	}
	return next, chained, nil
}

// appPackageDataTombstones returns a tombstone for every live datum of one
// package. A tombstone drops the value from the current state; the tape
// still holds it, so a restore to an earlier head brings it back (design
// §5.4, residual R3).
func (s *Store) appPackageDataTombstones(ctx context.Context, computerID, pkg string, now time.Time) ([]objectgraph.Object, error) {
	objs, err := s.ogListAllByMetadata(ctx, ogKindAppPackageDatum, "package_scope", appPackageScope(computerID, pkg))
	if err != nil {
		return nil, fmt.Errorf("app packages: list data of %s: %w", pkg, err)
	}
	out := make([]objectgraph.Object, 0, len(objs))
	for _, obj := range objs {
		if obj.Tombstone {
			continue
		}
		body, err := json.Marshal(map[string]any{"package": pkg, "purged": true})
		if err != nil {
			return nil, err
		}
		tomb := obj
		tomb.Body = body
		tomb.Tombstone = true
		tomb.UpdatedAt = now
		tomb.ContentHash = objectgraph.ContentHash(tomb.ObjectKind, tomb.Body, tomb.Metadata)
		out = append(out, tomb)
	}
	return out, nil
}

func appPackageDatumObject(ownerID, computerID string, d apppackage.Datum, now time.Time) (objectgraph.Object, error) {
	metadata := map[string]any{
		"computer_id":    computerID,
		"package_scope":  appPackageScope(computerID, d.Package),
		"package":        d.Package,
		"key":            d.Key,
		"schema_version": strconv.Itoa(d.SchemaVersion),
		"updated_at":     now.Format(time.RFC3339Nano),
	}
	return lifecycleObject(ogKindAppPackageDatum, ownerID, computerID, appPackageDatumKey(d.Package, d.Key), d, metadata, now, now)
}

func appPackageDatumKey(pkg, key string) string {
	return "app-package-datum:" + pkg + "\x00" + key
}

// PutAppPackageDatum writes one package_kv/v1 value. The package must be
// enabled and declare package data; the value is stamped with the package's
// data schema version.
func (s *Store) PutAppPackageDatum(ctx context.Context, ownerID, computerID string, d apppackage.Datum) error {
	ownerID, computerID = strings.TrimSpace(ownerID), strings.TrimSpace(computerID)
	if ownerID == "" || computerID == "" {
		return fmt.Errorf("app packages: owner_id and computer_id are required")
	}
	if s == nil || s.ogStore == nil {
		return fmt.Errorf("app packages: object graph not initialized")
	}
	s.appPackageMu.Lock()
	defer s.appPackageMu.Unlock()
	inv, err := s.AppPackageInventory(ctx, computerID)
	if err != nil {
		return err
	}
	stamped, err := apppackage.PrepareDatum(inv, d)
	if err != nil {
		return err
	}
	obj, err := appPackageDatumObject(ownerID, computerID, stamped, time.Now().UTC())
	if err != nil {
		return err
	}
	if err := s.ogStore.PutBatch(ctx, objectgraph.Batch{Objects: []objectgraph.Object{obj}}); err != nil {
		return fmt.Errorf("app packages: put datum %s/%s: %w", d.Package, d.Key, err)
	}
	return nil
}

// GetAppPackageDatum reads one value of an installed package. An absent or
// purged value, or a package that is not installed, is ErrNotFound; a value
// whose schema the installed version cannot read is refused.
func (s *Store) GetAppPackageDatum(ctx context.Context, computerID, pkg, key string) (apppackage.Datum, error) {
	computerID = strings.TrimSpace(computerID)
	if err := apppackage.ValidateDatumKey(key); err != nil {
		return apppackage.Datum{}, err
	}
	if s == nil || s.ogStore == nil {
		return apppackage.Datum{}, fmt.Errorf("app packages: object graph not initialized")
	}
	inv, err := s.AppPackageInventory(ctx, computerID)
	if err != nil {
		return apppackage.Datum{}, err
	}
	if _, installed := inv.Packages[pkg]; !installed {
		return apppackage.Datum{}, ErrNotFound
	}
	objs, err := s.ogListAllByMetadata(ctx, ogKindAppPackageDatum, "package_scope", appPackageScope(computerID, pkg))
	if err != nil {
		return apppackage.Datum{}, fmt.Errorf("app packages: read datum %s/%s: %w", pkg, key, err)
	}
	for _, obj := range objs {
		if obj.Tombstone {
			continue
		}
		var d apppackage.Datum
		if err := ogDecode(obj, &d); err != nil {
			return apppackage.Datum{}, err
		}
		if d.Package != pkg || d.Key != key {
			continue
		}
		if err := apppackage.ReadableDatum(inv, d); err != nil {
			return apppackage.Datum{}, err
		}
		return d, nil
	}
	return apppackage.Datum{}, ErrNotFound
}
