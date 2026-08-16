package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

var (
	ErrResidueImportUnbound = errors.New("store: residue import requires a bound projection tape")
	ErrResidueImportSplit   = errors.New("store: desktop and OG residue must be imported together")
)

// ResidueImportResult is the observed snapshot that was appended. It is not
// an eligibility receipt and does not reclassify EmptyUntilSupported.
type ResidueImportResult struct {
	Desktops int
	Sessions int
	Objects  int
	Edges    int
	Appended bool
}

// ImportResidueSnapshot appends one projection_batch_recorded event whose
// payload is the current desktop+OG SQL residue. That event is "state as of
// now," not fabricated history of heads 1–26. Presence fields are omitted.
// Live staging must not call this until current main is deployed. This method
// does not reclassify replay eligibility.
func (s *Store) ImportResidueSnapshot(ctx context.Context) (ResidueImportResult, error) {
	if s == nil || s.projectionTape == nil {
		return ResidueImportResult{}, ErrResidueImportUnbound
	}
	desktops, err := s.snapshotResidueDesktops(ctx)
	if err != nil {
		return ResidueImportResult{}, err
	}
	objects, err := s.snapshotResidueObjects(ctx)
	if err != nil {
		return ResidueImportResult{}, err
	}
	edges, err := s.snapshotResidueEdges(ctx)
	if err != nil {
		return ResidueImportResult{}, err
	}
	sessionCount := 0
	for _, desktop := range desktops {
		sessionCount += len(desktop.Sessions)
	}
	result := ResidueImportResult{
		Desktops: len(desktops),
		Sessions: sessionCount,
		Objects:  len(objects),
		Edges:    len(edges),
	}
	desktopResidue := result.Desktops > 0 || result.Sessions > 0
	ogResidue := result.Objects > 0 || result.Edges > 0
	if desktopResidue != ogResidue {
		return result, ErrResidueImportSplit
	}
	if !desktopResidue && !ogResidue {
		return result, nil
	}
	ops := make([]computerevent.ProjectionOp, 0, result.Desktops+result.Objects+result.Edges)
	for _, desktop := range desktops {
		body, err := json.Marshal(desktop)
		if err != nil {
			return result, fmt.Errorf("store: marshal residue desktop: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{Kind: computerevent.ProjectionOpDesktopState, Body: body})
	}
	for _, obj := range objects {
		body, err := json.Marshal(obj)
		if err != nil {
			return result, fmt.Errorf("store: marshal residue object: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{
			Kind:        computerevent.ProjectionOpObject,
			CanonicalID: obj.CanonicalID,
			Body:        body,
		})
	}
	for _, edge := range edges {
		body, err := json.Marshal(edge)
		if err != nil {
			return result, fmt.Errorf("store: marshal residue edge: %w", err)
		}
		ops = append(ops, computerevent.ProjectionOp{
			Kind:        computerevent.ProjectionOpObjectEdge,
			CanonicalID: edge.EdgeID,
			Body:        body,
		})
	}
	if err := s.projectionTape.appendOps(ctx, ops); err != nil {
		return result, err
	}
	result.Appended = true
	return result, nil
}

type desktopKey struct {
	ownerID   string
	desktopID string
}

func (s *Store) snapshotResidueDesktops(ctx context.Context) ([]projectedDesktopState, error) {
	keys := map[desktopKey]struct{}{}
	if err := s.collectDesktopKeys(ctx, `SELECT DISTINCT owner_id, desktop_id FROM desktop_workspaces`, keys); err != nil {
		return nil, err
	}
	if err := s.collectDesktopKeys(ctx, `SELECT DISTINCT owner_id, desktop_id FROM desktop_app_instances`, keys); err != nil {
		return nil, err
	}
	if err := s.collectDesktopKeys(ctx, `SELECT DISTINCT owner_id, desktop_id FROM desktop_window_placements`, keys); err != nil {
		return nil, err
	}
	if err := s.collectDesktopKeys(ctx, `SELECT DISTINCT owner_id, desktop_id FROM desktop_sessions`, keys); err != nil {
		return nil, err
	}
	sessions, err := s.snapshotResidueSessionIdentities(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]projectedDesktopState, 0, len(keys))
	for key := range keys {
		state, err := s.GetDesktopStateForSession(ctx, key.ownerID, key.desktopID, "projected")
		if err != nil {
			return nil, err
		}
		projected := desktopStateProjection(state, key.desktopID, "projected", state.UpdatedAt)
		projected.Sessions = sessions[key]
		out = append(out, projected)
	}
	return out, nil
}

func (s *Store) collectDesktopKeys(ctx context.Context, query string, keys map[desktopKey]struct{}) error {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("store: list residue desktops: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var ownerID, desktopID string
		if err := rows.Scan(&ownerID, &desktopID); err != nil {
			return fmt.Errorf("store: scan residue desktop: %w", err)
		}
		ownerID = strings.TrimSpace(ownerID)
		desktopID = normalizeDesktopID(desktopID)
		if ownerID == "" {
			continue
		}
		keys[desktopKey{ownerID: ownerID, desktopID: desktopID}] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: iterate residue desktops: %w", err)
	}
	return nil
}

func (s *Store) snapshotResidueSessionIdentities(ctx context.Context) (map[desktopKey][]projectedSessionIdentity, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT owner_id, desktop_id, session_id, device_id, viewport_profile
		   FROM desktop_sessions
		  ORDER BY owner_id, desktop_id, session_id`)
	if err != nil {
		return nil, fmt.Errorf("store: list residue sessions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := map[desktopKey][]projectedSessionIdentity{}
	for rows.Next() {
		var ownerID, desktopID, sessionID, deviceID, viewport string
		if err := rows.Scan(&ownerID, &desktopID, &sessionID, &deviceID, &viewport); err != nil {
			return nil, fmt.Errorf("store: scan residue session: %w", err)
		}
		key := desktopKey{ownerID: strings.TrimSpace(ownerID), desktopID: normalizeDesktopID(desktopID)}
		out[key] = append(out[key], projectedSessionIdentity{
			SessionID:       strings.TrimSpace(sessionID),
			DeviceID:        strings.TrimSpace(deviceID),
			ViewportProfile: strings.TrimSpace(viewport),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate residue sessions: %w", err)
	}
	return out, nil
}

func (s *Store) snapshotResidueObjects(ctx context.Context) ([]objectgraph.Object, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT canonical_id, object_kind, owner_id, computer_id, version_id, content_hash, body, metadata, created_at, updated_at, tombstone, superseded_by
		   FROM og_objects
		  ORDER BY canonical_id`)
	if err != nil {
		return nil, fmt.Errorf("store: list residue objects: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []objectgraph.Object
	for rows.Next() {
		obj, err := scanResidueObject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate residue objects: %w", err)
	}
	return out, nil
}

func (s *Store) snapshotResidueEdges(ctx context.Context) ([]objectgraph.Edge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT edge_id, from_id, to_id, kind, metadata, created_at, tombstone
		   FROM og_edges
		  ORDER BY edge_id`)
	if err != nil {
		return nil, fmt.Errorf("store: list residue edges: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []objectgraph.Edge
	for rows.Next() {
		edge, err := scanResidueEdge(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, edge)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: iterate residue edges: %w", err)
	}
	return out, nil
}

func scanResidueObject(row interface{ Scan(...any) error }) (objectgraph.Object, error) {
	var obj objectgraph.Object
	var metadata string
	if err := row.Scan(
		&obj.CanonicalID, &obj.ObjectKind, &obj.OwnerID, &obj.ComputerID,
		&obj.VersionID, &obj.ContentHash, &obj.Body, &metadata,
		&obj.CreatedAt, &obj.UpdatedAt, &obj.Tombstone, &obj.SupersededBy,
	); err != nil {
		return objectgraph.Object{}, fmt.Errorf("store: scan residue object: %w", err)
	}
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	obj.Metadata = json.RawMessage(metadata)
	obj.CreatedAt = obj.CreatedAt.UTC()
	obj.UpdatedAt = obj.UpdatedAt.UTC()
	return obj, nil
}

func scanResidueEdge(row interface{ Scan(...any) error }) (objectgraph.Edge, error) {
	var edge objectgraph.Edge
	var metadata string
	if err := row.Scan(
		&edge.EdgeID, &edge.FromID, &edge.ToID, &edge.Kind, &metadata,
		&edge.CreatedAt, &edge.Tombstone,
	); err != nil {
		return objectgraph.Edge{}, fmt.Errorf("store: scan residue edge: %w", err)
	}
	if strings.TrimSpace(metadata) == "" {
		metadata = "{}"
	}
	edge.Metadata = json.RawMessage(metadata)
	edge.CreatedAt = edge.CreatedAt.UTC()
	return edge, nil
}
