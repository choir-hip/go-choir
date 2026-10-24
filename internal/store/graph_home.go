package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

// One home for agent evidence and engineering slot claims
// (docs/problems/data-has-more-than-one-home-2026-10-24.md, step 3; doctrine
// I27). Both were direct-SQL tables off the tape, which the replay manifest
// admits only while empty, so one evidence row or one claimed slot made a
// computer checkpoint-ineligible. With the computer setting on, writes go to
// graph objects (on the tape through the projection interceptor) and the old
// table is emptied by an import. Reads union both homes whatever the setting
// says, so turning a setting on or off never hides a record.

const ogKindEngineeringSlot = objectgraph.ObjectKind("choir.co_super_slot")

// graphHomeOn reports whether a graph-home setting is on for this computer.
// The setting is read live at each write (no restart, no deploy). An unbound
// store (the test seam, no tape) and any read failure are off: the graph is
// only a home once it is on a tape.
func (s *Store) graphHomeOn(ctx context.Context, ownerID, key string) bool {
	if s == nil || s.projectionTape == nil || s.og == nil || strings.TrimSpace(ownerID) == "" {
		return false
	}
	setting, err := s.GetComputerSetting(ctx, ownerID, s.projectionTape.computerID, key)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.Printf("graph home: read %s: %v; treating it as off", key, err)
		}
		return false
	}
	return setting.Value
}

func (s *Store) tableHasRows(ctx context.Context, db *sql.DB, table string) (bool, error) {
	var one int
	err := db.QueryRowContext(ctx, "SELECT 1 FROM "+table+" LIMIT 1").Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check %s: %w", table, err)
	}
	return true, nil
}

// ----- Evidence -----

// CreateEvidence stores a durable evidence record. Evidence is owner-scoped
// and associated with the capturing agent. With evidence.graph on it is a
// choir.agent_evidence object on the tape (the legacy table's rows move first);
// otherwise it is a row of agent_evidence.
func (s *Store) CreateEvidence(ctx context.Context, rec types.EvidenceRecord) error {
	if !s.graphHomeOn(ctx, rec.OwnerID, SettingEvidenceGraph) {
		return s.createEvidenceTable(ctx, rec)
	}
	if has, err := s.tableHasRows(ctx, s.textureHandle(), "agent_evidence"); err != nil {
		return err
	} else if has {
		if _, err := s.ImportEvidenceToGraph(ctx); err != nil {
			return err
		}
	}
	return s.createEvidenceGraph(ctx, rec)
}

func (s *Store) createEvidenceGraph(ctx context.Context, rec types.EvidenceRecord) error {
	if strings.TrimSpace(rec.EvidenceID) == "" {
		return fmt.Errorf("create agent evidence: evidence_id is required")
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now().UTC()
	}
	// The store writes on the research desk's behalf: the desk is the sole
	// writer of the kind.
	if err := s.CreateEvidenceOG(objectgraph.WithWriter(ctx, objectgraph.WriterResearch), rec); err != nil {
		return fmt.Errorf("create agent evidence %s: %w", rec.EvidenceID, err)
	}
	return nil
}

// GetEvidence returns one evidence record scoped to the owner, from the graph
// or, failing that, the legacy table.
func (s *Store) GetEvidence(ctx context.Context, evidenceID, ownerID string) (types.EvidenceRecord, error) {
	if s.og != nil {
		rec, err := s.GetEvidenceOG(ctx, ownerID, evidenceID)
		if err == nil {
			return rec, nil
		}
		// Only an absent record falls through to the table: a graph read
		// failure is not "not found".
		if !errors.Is(err, ErrNotFound) && !errors.Is(err, objectgraph.ErrNotFound) {
			return types.EvidenceRecord{}, err
		}
	}
	return s.getEvidenceTable(ctx, evidenceID, ownerID)
}

// ListEvidenceByAgent returns recent evidence captured by an agent and scoped
// to the owner, newest first, from both homes. If agentID is empty it returns
// recent evidence across all of the owner's agents.
func (s *Store) ListEvidenceByAgent(ctx context.Context, ownerID, agentID string, limit int) ([]types.EvidenceRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	table, err := s.listEvidenceByAgentTable(ctx, ownerID, agentID, limit)
	if err != nil {
		return nil, err
	}
	var graph []types.EvidenceRecord
	if s.og != nil {
		var objs []objectgraph.Object
		if strings.TrimSpace(agentID) == "" {
			objs, err = s.ogListByOwnerAndBody(ctx, ogKindEvidence, ownerID, nil, limit)
		} else {
			objs, err = s.ogListByMetadata(ctx, ogKindEvidence, "agent_id", agentID, limit)
		}
		if err != nil {
			return nil, err
		}
		for _, obj := range objs {
			var rec types.EvidenceRecord
			if err := ogDecode(obj, &rec); err != nil {
				return nil, err
			}
			if rec.OwnerID == ownerID {
				graph = append(graph, rec)
			}
		}
	}
	if len(graph) == 0 {
		return table, nil
	}
	seen := make(map[string]bool, len(graph)+len(table))
	merged := make([]types.EvidenceRecord, 0, len(graph)+len(table))
	for _, rec := range append(graph, table...) { // the graph copy wins a duplicate
		if seen[rec.EvidenceID] {
			continue
		}
		seen[rec.EvidenceID] = true
		merged = append(merged, rec)
	}
	sort.SliceStable(merged, func(i, j int) bool { return merged[i].CreatedAt.After(merged[j].CreatedAt) })
	if len(merged) > limit {
		merged = merged[:limit]
	}
	return merged, nil
}

const graphHomeImportPage = 50

// ImportEvidenceToGraph moves every agent_evidence row into the graph and
// deletes it, in pages: a one-time import that empties the old home (I27). The
// object is written before the row is deleted and is keyed by evidence_id, so
// a crash between the two re-runs idempotently. A row the graph refuses (for
// size) stays in the table and the error names it. It returns the rows moved.
func (s *Store) ImportEvidenceToGraph(ctx context.Context) (int, error) {
	s.graphHomeMu.Lock()
	defer s.graphHomeMu.Unlock()
	if s.og == nil {
		return 0, fmt.Errorf("store: object graph not initialized")
	}
	moved := 0
	for {
		rows, err := s.textureHandle().QueryContext(ctx,
			`SELECT evidence_id, owner_id, agent_id, kind, source_uri, title, content, metadata_json, created_at
			   FROM agent_evidence ORDER BY created_at, evidence_id LIMIT ?`, graphHomeImportPage)
		if err != nil {
			return moved, fmt.Errorf("import agent evidence: %w", err)
		}
		var page []types.EvidenceRecord
		for rows.Next() {
			rec, err := scanEvidence(rows)
			if err != nil {
				_ = rows.Close()
				return moved, err
			}
			page = append(page, rec)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return moved, fmt.Errorf("import agent evidence: %w", err)
		}
		if len(page) == 0 {
			return moved, nil
		}
		for _, rec := range page {
			if err := s.createEvidenceGraph(ctx, rec); err != nil {
				return moved, err
			}
			if _, err := s.textureHandle().ExecContext(ctx, `DELETE FROM agent_evidence WHERE evidence_id = ?`, rec.EvidenceID); err != nil {
				return moved, fmt.Errorf("import agent evidence %s: delete the moved row: %w", rec.EvidenceID, err)
			}
			moved++
		}
	}
}

// ----- Engineering slots -----

func engineeringSlotKey(trajectoryID, slot string) string {
	return strings.TrimSpace(trajectoryID) + "\x00" + strings.TrimSpace(slot)
}

func engineeringSlotCanonicalID(ownerID, trajectoryID, slot string) (string, error) {
	return objectgraph.BuildCanonicalID(ogKindEngineeringSlot, strings.TrimSpace(ownerID),
		objectgraph.StableSuffixFromKey(engineeringSlotKey(trajectoryID, slot)))
}

func engineeringSlotObject(rec EngineeringSlotRecord, createdAt time.Time) (objectgraph.Object, error) {
	id, err := engineeringSlotCanonicalID(rec.OwnerID, rec.TrajectoryID, rec.Slot)
	if err != nil {
		return objectgraph.Object{}, err
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return objectgraph.Object{}, fmt.Errorf("marshal engineering slot: %w", err)
	}
	meta, err := objectgraph.NormalizeMetadata(map[string]any{
		"trajectory_id": rec.TrajectoryID,
		"slot":          rec.Slot,
		"run_id":        rec.RunID,
		"agent_id":      rec.AgentID,
		"updated_at":    rec.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return objectgraph.Object{}, err
	}
	if createdAt.IsZero() {
		createdAt = rec.UpdatedAt
	}
	return objectgraph.Object{
		CanonicalID: id,
		ObjectKind:  ogKindEngineeringSlot,
		OwnerID:     strings.TrimSpace(rec.OwnerID),
		ContentHash: objectgraph.ContentHash(ogKindEngineeringSlot, body, meta),
		Body:        body,
		Metadata:    meta,
		CreatedAt:   createdAt.UTC(),
		UpdatedAt:   rec.UpdatedAt.UTC(),
	}, nil
}

// engineeringSlotGraphObject reads the slot object, or (zero, false) if none.
func (s *Store) engineeringSlotGraphObject(ctx context.Context, ownerID, trajectoryID, slot string) (objectgraph.Object, EngineeringSlotRecord, bool, error) {
	if s.og == nil {
		return objectgraph.Object{}, EngineeringSlotRecord{}, false, nil
	}
	id, err := engineeringSlotCanonicalID(ownerID, trajectoryID, slot)
	if err != nil {
		return objectgraph.Object{}, EngineeringSlotRecord{}, false, err
	}
	obj, err := s.og.GetObject(ctx, id)
	if errors.Is(err, objectgraph.ErrNotFound) {
		return objectgraph.Object{}, EngineeringSlotRecord{}, false, nil
	}
	if err != nil {
		return objectgraph.Object{}, EngineeringSlotRecord{}, false, err
	}
	var rec EngineeringSlotRecord
	if err := ogDecode(obj, &rec); err != nil {
		return objectgraph.Object{}, EngineeringSlotRecord{}, false, err
	}
	return obj, rec, true, nil
}

// putEngineeringSlotGraph writes the slot object only if the stored version is
// still the one read (or still absent). The caller holds graphHomeMu. The
// store's conditional check and its tape write are not one atomic step, so
// graphHomeMu (one live store per computer) is what makes the claim safe, not
// the condition alone.
func (s *Store) putEngineeringSlotGraph(ctx context.Context, prior *objectgraph.Object, rec EngineeringSlotRecord) error {
	createdAt := rec.ClaimedAt
	cond := objectgraph.ObjectCondition{}
	obj, err := engineeringSlotObject(rec, createdAt)
	if err != nil {
		return err
	}
	cond.CanonicalID = obj.CanonicalID
	if prior != nil {
		cond.Exists, cond.ExpectedContentHash = true, prior.ContentHash
		obj.CreatedAt = prior.CreatedAt
	}
	return s.ogStore.PutBatchConditional(ctx, []objectgraph.ObjectCondition{cond}, objectgraph.Batch{Objects: []objectgraph.Object{obj}})
}

// claimEngineeringSlotGraph is ClaimEngineeringSlot with engineering.slots_graph
// on: the same compare-and-set over one object per (trajectory, slot). The
// caller holds graphHomeMu, which also covers the table check and the import
// of the legacy rows, so no table claim can land between the check and the
// write.
func (s *Store) claimEngineeringSlotGraph(ctx context.Context, ownerID, trajectoryID, slot, runID, agentID, requesterRunID string) (types.RunRecord, bool, error) {
	ownerID, trajectoryID = strings.TrimSpace(ownerID), strings.TrimSpace(trajectoryID)
	slot, runID = strings.TrimSpace(slot), strings.TrimSpace(runID)
	if ownerID == "" || trajectoryID == "" || slot == "" || runID == "" {
		return types.RunRecord{}, false, fmt.Errorf("claim co-super slot: owner_id, trajectory_id, slot, and run_id are required")
	}
	if s.og == nil || s.ogStore == nil {
		return types.RunRecord{}, false, fmt.Errorf("claim co-super slot (graph): object graph not initialized")
	}
	if has, err := s.tableHasRows(ctx, s.db, "co_super_slots"); err != nil {
		return types.RunRecord{}, false, err
	} else if has {
		if _, err := s.importEngineeringSlotsLocked(ctx); err != nil {
			return types.RunRecord{}, false, err
		}
	}
	if hook := s.slotClaimHook; hook != nil {
		hook()
	}
	prior, current, found, err := s.engineeringSlotGraphObject(ctx, ownerID, trajectoryID, slot)
	if err != nil {
		return types.RunRecord{}, false, err
	}
	if found && current.RunID != "" {
		existing, err := s.GetRun(ctx, current.RunID)
		if err == nil && existing.State.Active() {
			return existing, false, nil
		}
		if err != nil && !errors.Is(err, ErrNotFound) {
			return types.RunRecord{}, false, err
		}
	}
	now := time.Now().UTC()
	rec := EngineeringSlotRecord{
		OwnerID: ownerID, TrajectoryID: trajectoryID, Slot: slot, RunID: runID,
		AgentID: agentID, RequestedByRunID: requesterRunID, ClaimedAt: now, UpdatedAt: now,
	}
	var priorPtr *objectgraph.Object
	if found {
		priorPtr = &prior
	}
	if err := s.putEngineeringSlotGraph(ctx, priorPtr, rec); err != nil {
		if errors.Is(err, objectgraph.ErrConflict) {
			// Another writer moved the slot between the read and the write
			// (the mutex covers this store only): report the holder.
			_, now2, ok, rerr := s.engineeringSlotGraphObject(ctx, ownerID, trajectoryID, slot)
			if rerr == nil && ok && now2.RunID != "" {
				if existing, gerr := s.GetRun(ctx, now2.RunID); gerr == nil {
					return existing, false, nil
				}
			}
		}
		return types.RunRecord{}, false, fmt.Errorf("claim co-super slot (graph): %w", err)
	}
	return types.RunRecord{}, true, nil
}

// releaseEngineeringSlotGraph releases the graph home's claim only if it is
// still owned by runID. A release is recorded as a slot with no run, which
// reads as absent and is claimable (no delete exists on the tape). The caller
// holds graphHomeMu.
func (s *Store) releaseEngineeringSlotGraph(ctx context.Context, ownerID, trajectoryID, slot, runID string) error {
	if s.og == nil || s.ogStore == nil {
		return nil
	}
	prior, current, found, err := s.engineeringSlotGraphObject(ctx, ownerID, trajectoryID, slot)
	if err != nil {
		return fmt.Errorf("release co-super slot claim: %w", err)
	}
	if !found || current.RunID != runID {
		return nil
	}
	// A newer claim in the table (a rolled-back setting) holds the slot now:
	// an empty graph marker would win the newer-record rule and free it.
	if union, ok, err := s.engineeringSlotRecord(ctx, ownerID, trajectoryID, slot); err != nil {
		return fmt.Errorf("release co-super slot claim: %w", err)
	} else if !ok || union.RunID != runID {
		return nil
	}
	current.RunID, current.AgentID, current.RequestedByRunID = "", "", ""
	current.UpdatedAt = time.Now().UTC()
	if err := s.putEngineeringSlotGraph(ctx, &prior, current); err != nil {
		return fmt.Errorf("release co-super slot claim (graph): %w", err)
	}
	return nil
}

// engineeringSlotGraphHolder reports the live run the graph holds a slot for,
// so the legacy table path (a rolled-back setting) never hands out a held slot.
func (s *Store) engineeringSlotGraphHolder(ctx context.Context, ownerID, trajectoryID, slot string) (types.RunRecord, bool, error) {
	_, rec, found, err := s.engineeringSlotGraphObject(ctx, ownerID, trajectoryID, slot)
	if err != nil || !found || rec.RunID == "" {
		return types.RunRecord{}, false, err
	}
	existing, err := s.GetRun(ctx, rec.RunID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return types.RunRecord{}, false, nil
		}
		return types.RunRecord{}, false, err
	}
	if existing.State.Active() {
		return existing, true, nil
	}
	return types.RunRecord{}, false, nil
}

// ImportEngineeringSlotsToGraph moves every co_super_slots row into the graph
// and deletes it (I27). A row older than the graph's object for the same slot
// is dropped, not written: the newer record wins, as the reads decide.
func (s *Store) ImportEngineeringSlotsToGraph(ctx context.Context) (int, error) {
	s.graphHomeMu.Lock()
	defer s.graphHomeMu.Unlock()
	return s.importEngineeringSlotsLocked(ctx)
}

// importEngineeringSlotsLocked is ImportEngineeringSlotsToGraph for a caller
// that holds graphHomeMu.
func (s *Store) importEngineeringSlotsLocked(ctx context.Context) (int, error) {
	if s.og == nil || s.ogStore == nil {
		return 0, fmt.Errorf("store: object graph not initialized")
	}
	moved := 0
	for {
		rows, err := s.db.QueryContext(ctx,
			`SELECT owner_id, trajectory_id, slot, run_id, agent_id, requested_by_run_id, claimed_at, updated_at
			   FROM co_super_slots ORDER BY owner_id, trajectory_id, slot LIMIT ?`, graphHomeImportPage)
		if err != nil {
			return moved, fmt.Errorf("import co-super slots: %w", err)
		}
		var page []EngineeringSlotRecord
		for rows.Next() {
			var rec EngineeringSlotRecord
			if err := rows.Scan(&rec.OwnerID, &rec.TrajectoryID, &rec.Slot, &rec.RunID, &rec.AgentID, &rec.RequestedByRunID, &rec.ClaimedAt, &rec.UpdatedAt); err != nil {
				_ = rows.Close()
				return moved, fmt.Errorf("import co-super slots: scan: %w", err)
			}
			rec.ClaimedAt, rec.UpdatedAt = rec.ClaimedAt.UTC(), rec.UpdatedAt.UTC()
			page = append(page, rec)
		}
		err = rows.Err()
		_ = rows.Close()
		if err != nil {
			return moved, fmt.Errorf("import co-super slots: %w", err)
		}
		if len(page) == 0 {
			return moved, nil
		}
		for _, rec := range page {
			prior, current, found, err := s.engineeringSlotGraphObject(ctx, rec.OwnerID, rec.TrajectoryID, rec.Slot)
			if err != nil {
				return moved, err
			}
			// The table's clock has one-second resolution: a tie goes to the table.
			if !found || !current.UpdatedAt.Truncate(time.Second).After(rec.UpdatedAt.Truncate(time.Second)) {
				var priorPtr *objectgraph.Object
				if found {
					priorPtr = &prior
				}
				if err := s.putEngineeringSlotGraph(ctx, priorPtr, rec); err != nil {
					return moved, fmt.Errorf("import co-super slot %s/%s: %w", rec.TrajectoryID, rec.Slot, err)
				}
				moved++
			}
			// Delete only the row that was read: a row advanced since (the lock
			// makes that unreachable today) is read again by the next page.
			if _, err := s.db.ExecContext(ctx,
				`DELETE FROM co_super_slots WHERE owner_id = ? AND trajectory_id = ? AND slot = ? AND run_id = ?`,
				rec.OwnerID, rec.TrajectoryID, rec.Slot, rec.RunID); err != nil {
				return moved, fmt.Errorf("import co-super slot %s/%s: delete the moved row: %w", rec.TrajectoryID, rec.Slot, err)
			}
		}
	}
}

// ----- Slot reads: both homes, the newer record per slot -----

// newerSlot reports whether a is the record to keep over b. The table's clock
// is DATETIME (one second), so a tie goes to the table record, which is the
// later write in a rollback.
func newerSlot(a, b EngineeringSlotRecord, aFromTable bool) bool {
	at, bt := a.UpdatedAt.Truncate(time.Second), b.UpdatedAt.Truncate(time.Second)
	if at.Equal(bt) {
		return aFromTable
	}
	return at.After(bt)
}

// engineeringSlotRecord returns the record of one (trajectory, slot) from
// whichever home holds the newer one. A released slot reads as absent.
func (s *Store) engineeringSlotRecord(ctx context.Context, ownerID, trajectoryID, slot string) (EngineeringSlotRecord, bool, error) {
	var tableRec EngineeringSlotRecord
	tableFound := false
	err := s.db.QueryRowContext(ctx,
		`SELECT owner_id, trajectory_id, slot, run_id, agent_id, requested_by_run_id, claimed_at, updated_at
		   FROM co_super_slots WHERE owner_id = ? AND trajectory_id = ? AND slot = ?`,
		ownerID, trajectoryID, slot,
	).Scan(&tableRec.OwnerID, &tableRec.TrajectoryID, &tableRec.Slot, &tableRec.RunID, &tableRec.AgentID,
		&tableRec.RequestedByRunID, &tableRec.ClaimedAt, &tableRec.UpdatedAt)
	switch {
	case err == nil:
		tableFound = true
	case errors.Is(err, sql.ErrNoRows):
	default:
		return EngineeringSlotRecord{}, false, fmt.Errorf("query co-super slot: %w", err)
	}
	_, graphRec, graphFound, err := s.engineeringSlotGraphObject(ctx, ownerID, trajectoryID, slot)
	if err != nil {
		return EngineeringSlotRecord{}, false, err
	}
	var rec EngineeringSlotRecord
	switch {
	case tableFound && graphFound:
		rec = graphRec
		if newerSlot(tableRec, graphRec, true) {
			rec = tableRec
		}
	case tableFound:
		rec = tableRec
	case graphFound:
		rec = graphRec
	default:
		return EngineeringSlotRecord{}, false, nil
	}
	if rec.RunID == "" {
		return EngineeringSlotRecord{}, false, nil
	}
	return rec, true, nil
}

func (s *Store) engineeringSlotRunID(ctx context.Context, ownerID, trajectoryID, slot string) (string, error) {
	rec, found, err := s.engineeringSlotRecord(ctx, ownerID, trajectoryID, slot)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrNotFound
	}
	return rec.RunID, nil
}

// graphSlotsBy lists the graph home's slot records for one metadata field.
func (s *Store) graphSlotsBy(ctx context.Context, ownerID, field, value string) ([]EngineeringSlotRecord, error) {
	if s.og == nil {
		return nil, nil
	}
	objs, err := s.ogListAllByMetadata(ctx, ogKindEngineeringSlot, field, value)
	if err != nil {
		return nil, err
	}
	var out []EngineeringSlotRecord
	for _, obj := range objs {
		var rec EngineeringSlotRecord
		if err := ogDecode(obj, &rec); err != nil {
			return nil, err
		}
		if rec.OwnerID == ownerID {
			out = append(out, rec)
		}
	}
	return out, nil
}

// slotAgentBefore is the legacy ordering: updated_at, claimed_at, then the key,
// newest first.
func slotAgentBefore(a, b EngineeringSlotRecord, byTrajectory bool) bool {
	if !a.UpdatedAt.Truncate(time.Second).Equal(b.UpdatedAt.Truncate(time.Second)) {
		return a.UpdatedAt.Truncate(time.Second).After(b.UpdatedAt.Truncate(time.Second))
	}
	if !a.ClaimedAt.Truncate(time.Second).Equal(b.ClaimedAt.Truncate(time.Second)) {
		return a.ClaimedAt.Truncate(time.Second).After(b.ClaimedAt.Truncate(time.Second))
	}
	if !byTrajectory && a.TrajectoryID != b.TrajectoryID {
		return a.TrajectoryID > b.TrajectoryID
	}
	return a.Slot > b.Slot
}

// EngineeringSlotByAgent returns the most recent trajectory slot record for a
// Engineering agent, from both homes. Authority callers use this instead of
// active-run parentage.
func (s *Store) EngineeringSlotByAgent(ctx context.Context, ownerID, agentID string) (EngineeringSlotRecord, bool, error) {
	ownerID, agentID = strings.TrimSpace(ownerID), strings.TrimSpace(agentID)
	if ownerID == "" || agentID == "" {
		return EngineeringSlotRecord{}, false, nil
	}
	best, found, err := s.engineeringSlotByAgentTable(ctx, ownerID, agentID)
	if err != nil {
		return EngineeringSlotRecord{}, false, err
	}
	graph, err := s.graphSlotsBy(ctx, ownerID, "agent_id", agentID)
	if err != nil {
		return EngineeringSlotRecord{}, false, err
	}
	return s.bestSlot(ctx, best, found, graph, false)
}

// EngineeringSlotByAgentAndTrajectory returns the trajectory slot record for a
// Engineering agent on a specific trajectory, from both homes.
func (s *Store) EngineeringSlotByAgentAndTrajectory(ctx context.Context, ownerID, trajectoryID, agentID string) (EngineeringSlotRecord, bool, error) {
	ownerID, trajectoryID, agentID = strings.TrimSpace(ownerID), strings.TrimSpace(trajectoryID), strings.TrimSpace(agentID)
	if ownerID == "" || trajectoryID == "" || agentID == "" {
		return EngineeringSlotRecord{}, false, nil
	}
	best, found, err := s.engineeringSlotByAgentAndTrajectoryTable(ctx, ownerID, trajectoryID, agentID)
	if err != nil {
		return EngineeringSlotRecord{}, false, err
	}
	graph, err := s.graphSlotsBy(ctx, ownerID, "trajectory_id", trajectoryID)
	if err != nil {
		return EngineeringSlotRecord{}, false, err
	}
	filtered := graph[:0]
	for _, rec := range graph {
		if rec.AgentID == agentID {
			filtered = append(filtered, rec)
		}
	}
	return s.bestSlot(ctx, best, found, filtered, true)
}

// bestSlot picks the first record in the legacy ordering among the table's
// best and the graph's candidates. A record is dropped when the other home
// holds a newer record for the same slot (a stale copy must not answer).
func (s *Store) bestSlot(ctx context.Context, table EngineeringSlotRecord, tableFound bool, graph []EngineeringSlotRecord, byTrajectory bool) (EngineeringSlotRecord, bool, error) {
	var cands []EngineeringSlotRecord
	cands = append(cands, graph...)
	if tableFound {
		cands = append(cands, table)
	}
	var best EngineeringSlotRecord
	found := false
	for _, c := range cands {
		if c.RunID == "" {
			continue
		}
		current, ok, err := s.engineeringSlotRecord(ctx, c.OwnerID, c.TrajectoryID, c.Slot)
		if err != nil {
			return EngineeringSlotRecord{}, false, err
		}
		if !ok || current.RunID != c.RunID || current.AgentID != c.AgentID {
			continue
		}
		if !found || slotAgentBefore(c, best, byTrajectory) {
			best, found = c, true
		}
	}
	return best, found, nil
}

// engineeringSlotsOfTrajectory lists the held slots of a trajectory from both
// homes, the newer record per slot.
func (s *Store) engineeringSlotsOfTrajectory(ctx context.Context, ownerID, trajectoryID string) ([]EngineeringSlotRecord, error) {
	slots := map[string]bool{}
	rows, err := s.db.QueryContext(ctx,
		`SELECT slot FROM co_super_slots WHERE owner_id = ? AND trajectory_id = ?`, ownerID, trajectoryID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var slot string
		if err := rows.Scan(&slot); err != nil {
			_ = rows.Close()
			return nil, err
		}
		slots[slot] = true
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	graph, err := s.graphSlotsBy(ctx, ownerID, "trajectory_id", trajectoryID)
	if err != nil {
		return nil, err
	}
	for _, rec := range graph {
		slots[rec.Slot] = true
	}
	var out []EngineeringSlotRecord
	for slot := range slots {
		rec, ok, err := s.engineeringSlotRecord(ctx, ownerID, trajectoryID, slot)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, rec)
		}
	}
	return out, nil
}
