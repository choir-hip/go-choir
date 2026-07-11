package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/types"
)

const ogMetadataValueScanPageSize = 256

// backfillOGFromSQL reads all migrated records from the relational SQL tables
// and writes them into the object graph. This is called on every Open to
// ensure all legacy SQL data is visible through OG-only read paths.
// Each record is written with put-if-absent semantics: if the object already
// exists in OG (e.g. from a newer OG-only write), it is skipped to avoid
// replaying stale SQL state.
func (s *Store) backfillOGFromSQL(ctx context.Context) error {
	if s.og == nil {
		return nil
	}
	steps := []struct {
		name string
		kind objectgraph.ObjectKind
		run  func(context.Context) error
	}{
		{"agents", ogKindAgent, s.backfillAgentsOG},
		{"runs", ogKindRun, s.backfillRunsOG},
		{"events", ogKindEvent, s.backfillEventsOG},
		{"channel-messages", ogKindChannelMsg, s.backfillChannelMessagesOG},
		{"worker-updates", ogKindWorkerUpdate, s.backfillWorkerUpdatesOG},
		{"run-acceptances", ogKindRunAccept, s.backfillRunAcceptancesOG},
		{"run-continuations", ogKindRunContin, s.backfillRunContinuationsOG},
		{"browser-sessions", ogKindBrowserSess, s.backfillBrowserSessionsOG},
		{"trajectories", ogKindTrajectory, s.backfillTrajectoriesOG},
		{"work-items", ogKindWorkItem, s.backfillWorkItemsOG},
		{"texture-tables", "", s.backfillTextureTablesOG},
	}
	for _, step := range steps {
		if err := s.runOGBackfillStep(ctx, step.name, step.kind, step.run); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) runOGBackfillStep(ctx context.Context, name string, kind objectgraph.ObjectKind, run func(context.Context) error) error {
	if kind != "" {
		complete, err := s.ogBackfillMigrationComplete(ctx, kind)
		if err != nil {
			return fmt.Errorf("backfill OG %s: inspect completion: %w", name, err)
		}
		if complete {
			log.Printf("store: objectgraph backfill kind=%s status=skipped reason=migration-complete", name)
			return nil
		}
	}
	log.Printf("store: objectgraph backfill kind=%s status=starting", name)
	if err := run(ctx); err != nil {
		return err
	}
	if kind != "" {
		if err := s.markOGBackfillMigrationComplete(ctx, kind); err != nil {
			return fmt.Errorf("backfill OG %s: mark completion: %w", name, err)
		}
	}
	log.Printf("store: objectgraph backfill kind=%s status=complete", name)
	return nil
}

func ogBackfillMigrationID(kind objectgraph.ObjectKind) string {
	return "sql-to-objectgraph-v1:" + string(kind)
}

func (s *Store) ogBackfillMigrationComplete(ctx context.Context, kind objectgraph.ObjectKind) (bool, error) {
	if s == nil || s.textureHandle() == nil {
		return false, fmt.Errorf("store: object graph database not initialized")
	}
	var count int
	if err := s.textureHandle().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM og_migrations WHERE migration_id = ?`,
		ogBackfillMigrationID(kind),
	).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) markOGBackfillMigrationComplete(ctx context.Context, kind objectgraph.ObjectKind) error {
	if s == nil || s.textureHandle() == nil {
		return fmt.Errorf("store: object graph database not initialized")
	}
	_, err := s.textureHandle().ExecContext(ctx,
		`INSERT INTO og_migrations (migration_id, completed_at) VALUES (?, ?)
		 ON DUPLICATE KEY UPDATE completed_at = VALUES(completed_at)`,
		ogBackfillMigrationID(kind), time.Now().UTC(),
	)
	return err
}

// ogMetadataValueSet scans one object kind in bounded keyset pages and returns
// its non-empty metadata values. The embedded Dolt handle intentionally has one
// connection, so closing rows between pages lets foreground runtime work make
// progress while a large resumable migration runs in the background.
func (s *Store) ogMetadataValueSet(ctx context.Context, kind objectgraph.ObjectKind, jsonPath string) (map[string]struct{}, error) {
	if s == nil || s.textureHandle() == nil {
		return nil, fmt.Errorf("store: object graph database not initialized")
	}
	values := make(map[string]struct{})
	cursor := ""
	for {
		rows, err := s.textureHandle().QueryContext(ctx,
			`SELECT canonical_id, JSON_UNQUOTE(JSON_EXTRACT(CAST(metadata AS JSON), ?))
			 FROM og_objects
			 WHERE object_kind = ? AND canonical_id > ?
			 ORDER BY canonical_id
			 LIMIT ?`,
			jsonPath, string(kind), cursor, ogMetadataValueScanPageSize,
		)
		if err != nil {
			return nil, err
		}
		pageCount := 0
		for rows.Next() {
			var canonicalID string
			var value sql.NullString
			if err := rows.Scan(&canonicalID, &value); err != nil {
				_ = rows.Close()
				return nil, err
			}
			cursor = canonicalID
			pageCount++
			if value.Valid && strings.TrimSpace(value.String) != "" {
				values[value.String] = struct{}{}
			}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, err
		}
		if err := rows.Close(); err != nil {
			return nil, err
		}
		if pageCount < ogMetadataValueScanPageSize {
			return values, nil
		}
		// Give already-waiting foreground queries a scheduling opportunity
		// before the migration acquires the sole connection for another page.
		runtime.Gosched()
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Millisecond):
		}
	}
}

func (s *Store) backfillAgentsOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT agent_id, owner_id, sandbox_id, profile, role, channel_id, created_at, updated_at FROM agents`)
	if err != nil {
		return fmt.Errorf("backfill OG agents: query: %w", err)
	}
	var records []types.AgentRecord
	for rows.Next() {
		var rec struct {
			AgentID   string
			OwnerID   string
			SandboxID string
			Profile   string
			Role      string
			ChannelID string
			CreatedAt string
			UpdatedAt string
		}
		if err := rows.Scan(&rec.AgentID, &rec.OwnerID, &rec.SandboxID, &rec.Profile, &rec.Role, &rec.ChannelID, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill OG agents: scan: %w", err)
		}
		createdAt, _ := time.Parse(time.RFC3339Nano, rec.CreatedAt)
		updatedAt, _ := time.Parse(time.RFC3339Nano, rec.UpdatedAt)
		records = append(records, types.AgentRecord{
			AgentID:   rec.AgentID,
			OwnerID:   rec.OwnerID,
			SandboxID: rec.SandboxID,
			Profile:   rec.Profile,
			Role:      rec.Role,
			ChannelID: rec.ChannelID,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
		})
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill OG agents: iterate: %w", err)
	}
	for _, rec := range records {
		// Put-if-absent: skip if OG already has this agent.
		if _, err := s.GetAgentOG(ctx, rec.AgentID); err == nil {
			continue
		} else if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("backfill OG agents: check %s: %w", rec.AgentID, err)
		}
		if err := s.UpsertAgentOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG agents: put %s: %w", rec.AgentID, err)
		}
	}
	return nil
}

func (s *Store) backfillRunsOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT loop_id, agent_id, channel_id, requested_by_run_id, trajectory_id, agent_profile, agent_role, owner_id, sandbox_id, state, prompt, result, error, created_at, updated_at, finished_at, metadata_json FROM runs`)
	if err != nil {
		return fmt.Errorf("backfill OG runs: query: %w", err)
	}
	var records []types.RunRecord
	for rows.Next() {
		rec, err := scanRun(rows)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill OG runs: scan: %w", err)
		}
		records = append(records, rec)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill OG runs: iterate: %w", err)
	}
	for _, rec := range records {
		// Put-if-absent: skip if OG already has this run (e.g. newer state).
		if _, err := s.GetRunOG(ctx, rec.RunID); err == nil {
			continue
		} else if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("backfill OG runs: check %s: %w", rec.RunID, err)
		}
		if err := s.CreateRunOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG runs: put %s: %w", rec.RunID, err)
		}
	}
	return nil
}

func (s *Store) backfillEventsOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT event_id, loop_id, agent_id, channel_id, owner_id, trajectory_id, seq, stream_seq, ts, kind, phase, payload_json FROM events`)
	if err != nil {
		return fmt.Errorf("backfill OG events: query: %w", err)
	}
	var records []types.EventRecord
	for rows.Next() {
		rec, err := scanEvent(rows)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill OG events: scan: %w", err)
		}
		records = append(records, rec)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill OG events: iterate: %w", err)
	}
	existingEventIDs, err := s.ogMetadataValueSet(ctx, ogKindEvent, "$.event_id")
	if err != nil {
		return fmt.Errorf("backfill OG events: load existing event ids: %w", err)
	}
	for i := range records {
		// Put-if-absent: skip if OG already has this event.
		if _, exists := existingEventIDs[records[i].EventID]; exists {
			continue
		}
		// OG requires a non-empty owner_id. Synthesize a system owner
		// for ownerless legacy events (e.g. health/degraded events).
		if records[i].OwnerID == "" {
			records[i].OwnerID = "__system__"
		}
		if err := s.AppendEventOG(ctx, &records[i]); err != nil {
			return fmt.Errorf("backfill OG events: put %s: %w", records[i].EventID, err)
		}
		existingEventIDs[records[i].EventID] = struct{}{}
	}
	return nil
}

func (s *Store) backfillChannelMessagesOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT channel_id, seq, from_agent_id, from_loop_id, to_agent_id, to_loop_id, trajectory_id, from_name, role, content, created_at, owner_id FROM channel_messages`)
	if err != nil {
		return fmt.Errorf("backfill OG channel messages: query: %w", err)
	}
	type msgWithOwner struct {
		msg     types.ChannelMessage
		ownerID string
	}
	var records []msgWithOwner
	for rows.Next() {
		var msg types.ChannelMessage
		var createdAt, ownerID string
		if err := rows.Scan(
			&msg.ChannelID, &msg.Seq, &msg.FromAgentID, &msg.FromRunID,
			&msg.ToAgentID, &msg.ToRunID, &msg.TrajectoryID,
			&msg.From, &msg.Role, &msg.Content, &createdAt, &ownerID,
		); err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill OG channel messages: scan: %w", err)
		}
		msg.Timestamp, _ = time.Parse(time.RFC3339Nano, createdAt)
		records = append(records, msgWithOwner{msg: msg, ownerID: ownerID})
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill OG channel messages: iterate: %w", err)
	}
	for _, r := range records {
		// OG requires a non-empty owner_id. Synthesize a system owner
		// for ownerless legacy channel messages.
		ownerID := r.ownerID
		if ownerID == "" {
			ownerID = "__system__"
		}
		// Channel messages use content-hash identity, so re-backfilling
		// the same message is an upsert to the same object — always safe.
		if err := s.AppendChannelMessageOG(ctx, &r.msg, ownerID); err != nil {
			return fmt.Errorf("backfill OG channel messages: put: %w", err)
		}
	}
	return nil
}

func (s *Store) backfillWorkerUpdatesOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, workerUpdateSelectSQL())
	if err != nil {
		return fmt.Errorf("backfill OG worker updates: query: %w", err)
	}
	var records []types.CoagentSourcePacket
	for rows.Next() {
		rec, err := scanWorkerUpdate(rows)
		if err != nil {
			// Legacy worker_updates rows without canonical packet_json
			// are audit-only historical data and must not block store
			// open. Skip them during backfill.
			continue
		}
		records = append(records, rec)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill OG worker updates: iterate: %w", err)
	}
	for _, rec := range records {
		// Put-if-absent: skip if OG already has this worker update
		// (delivery state may have changed since SQL was frozen).
		if _, err := s.GetWorkerUpdateOG(ctx, rec.OwnerID, rec.UpdateID); err == nil {
			continue
		} else if !errors.Is(err, ErrNotFound) {
			return fmt.Errorf("backfill OG worker updates: check %s: %w", rec.UpdateID, err)
		}
		if err := s.CreateWorkerUpdateOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG worker updates: put %s: %w", rec.UpdateID, err)
		}
	}
	return nil
}

func (s *Store) backfillRunAcceptancesOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, runAcceptanceSelectSQL())
	if err != nil {
		return fmt.Errorf("backfill OG run acceptances: query: %w", err)
	}
	var records []types.RunAcceptanceRecord
	for rows.Next() {
		rec, err := scanRunAcceptance(rows)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill OG run acceptances: scan: %w", err)
		}
		records = append(records, rec)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill OG run acceptances: iterate: %w", err)
	}
	for _, rec := range records {
		// Put-if-absent: skip if OG already has this acceptance.
		exists, err := s.ogExistsByKey(ctx, ogKindRunAccept, "acceptance_id", rec.AcceptanceID)
		if err != nil {
			return fmt.Errorf("backfill OG run acceptances: check %s: %w", rec.AcceptanceID, err)
		}
		if exists {
			continue
		}
		if err := s.CreateRunAcceptanceOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG run acceptances: put %s: %w", rec.AcceptanceID, err)
		}
	}
	return nil
}

func (s *Store) backfillRunContinuationsOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT continuation_id, owner_id, source_loop_id, next_loop_id, objective, reason, authority_profile, lease_seconds, status, details_json, created_at, updated_at FROM run_continuations`)
	if err != nil {
		return fmt.Errorf("backfill OG run continuations: query: %w", err)
	}
	var records []types.RunContinuationRecord
	for rows.Next() {
		rec, err := scanRunContinuation(rows)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill OG run continuations: scan: %w", err)
		}
		records = append(records, rec)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill OG run continuations: iterate: %w", err)
	}
	for _, rec := range records {
		// Put-if-absent: skip if OG already has this continuation.
		exists, err := s.ogExistsByKey(ctx, ogKindRunContin, "continuation_id", rec.ContinuationID)
		if err != nil {
			return fmt.Errorf("backfill OG run continuations: check %s: %w", rec.ContinuationID, err)
		}
		if exists {
			continue
		}
		if err := s.CreateRunContinuationOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG run continuations: put %s: %w", rec.ContinuationID, err)
		}
	}
	return nil
}

func (s *Store) backfillBrowserSessionsOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT session_id, owner_id, provider, mode, execution_scope, backend_session_id, world_kind, vm_id, snapshot_id, source_loop_id, candidate_trace_id, state, current_url, title, text_snapshot, html_snapshot, links_json, screenshot_png_base64, error, created_at, updated_at FROM browser_sessions`)
	if err != nil {
		return fmt.Errorf("backfill OG browser sessions: query: %w", err)
	}
	var records []types.BrowserSessionRecord
	for rows.Next() {
		rec, err := scanBrowserSession(rows)
		if err != nil {
			_ = rows.Close()
			return fmt.Errorf("backfill OG browser sessions: scan: %w", err)
		}
		records = append(records, rec)
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		return fmt.Errorf("backfill OG browser sessions: iterate: %w", err)
	}
	for _, rec := range records {
		// Put-if-absent: skip if OG already has this browser session.
		exists, err := s.ogExistsByKey(ctx, ogKindBrowserSess, "session_id", rec.SessionID)
		if err != nil {
			return fmt.Errorf("backfill OG browser sessions: check %s: %w", rec.SessionID, err)
		}
		if exists {
			continue
		}
		if err := s.CreateBrowserSessionOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG browser sessions: put %s: %w", rec.SessionID, err)
		}
	}
	return nil
}

func (s *Store) backfillTrajectoriesOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT trajectory_id, owner_id, kind, subject_refs_json, status, settlement_rule_json, created_at, updated_at, settled_at FROM trajectories`)
	if err != nil {
		return fmt.Errorf("backfill OG trajectories: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			rec            types.TrajectoryRecord
			subjectRefs    string
			settlementRule string
			settledAt      sql.NullString
			createdAt      string
			updatedAt      string
		)
		if err := rows.Scan(&rec.TrajectoryID, &rec.OwnerID, &rec.Kind, &subjectRefs, &rec.Status, &settlementRule, &createdAt, &updatedAt, &settledAt); err != nil {
			return fmt.Errorf("backfill OG trajectories: scan: %w", err)
		}
		if subjectRefs != "" {
			_ = json.Unmarshal([]byte(subjectRefs), &rec.SubjectRefs)
		}
		if settlementRule != "" {
			_ = json.Unmarshal([]byte(settlementRule), &rec.SettlementRule)
		}
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		rec.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		if settledAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, settledAt.String)
			rec.SettledAt = &t
		}
		if _, err := s.CreateTrajectoryIfAbsentOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG trajectories: put %s: %w", rec.TrajectoryID, err)
		}
	}
	return rows.Err()
}

func (s *Store) backfillWorkItemsOG(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `SELECT work_item_id, trajectory_id, owner_id, objective, reason, authority_profile, step_budget, token_budget, objective_fingerprint, status, assigned_agent_id, created_by_loop_id, details_json, created_at, updated_at FROM work_items`)
	if err != nil {
		return fmt.Errorf("backfill OG work items: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			rec         types.WorkItemRecord
			detailsJSON string
			createdAt   string
			updatedAt   string
		)
		if err := rows.Scan(&rec.WorkItemID, &rec.TrajectoryID, &rec.OwnerID, &rec.Objective, &rec.Reason, &rec.AuthorityProfile, &rec.StepBudget, &rec.TokenBudget, &rec.ObjectiveFingerprint, &rec.Status, &rec.AssignedAgentID, &rec.CreatedByRunID, &detailsJSON, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("backfill OG work items: scan: %w", err)
		}
		if detailsJSON != "" {
			_ = json.Unmarshal([]byte(detailsJSON), &rec.Details)
		}
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		rec.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		// Put-if-absent: skip if OG already has this work item.
		exists, err := s.ogExistsByKey(ctx, ogKindWorkItem, "work_item_id", rec.WorkItemID)
		if err != nil {
			return fmt.Errorf("backfill OG work items: check %s: %w", rec.WorkItemID, err)
		}
		if exists {
			continue
		}
		if _, err := s.CreateWorkItemOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG work items: put %s: %w", rec.WorkItemID, err)
		}
	}
	return rows.Err()
}

func (s *Store) backfillTextureDocumentsOG(ctx context.Context) error {
	rows, err := s.textureHandle().QueryContext(ctx, `SELECT doc_id, owner_id, title, current_revision_id, created_at, updated_at FROM texture_documents`)
	if err != nil {
		return fmt.Errorf("backfill OG texture documents: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			doc       types.Document
			createdAt string
			updatedAt string
		)
		if err := rows.Scan(&doc.DocID, &doc.OwnerID, &doc.Title, &doc.CurrentRevisionID, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("backfill OG texture documents: scan: %w", err)
		}
		doc.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		doc.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		// Put-if-absent: skip if OG already has this document
		// (head/updated_at may have changed since SQL was frozen).
		exists, err := s.ogExistsByKey(ctx, ogKindTexDoc, "doc_id", doc.DocID)
		if err != nil {
			return fmt.Errorf("backfill OG texture documents: check %s: %w", doc.DocID, err)
		}
		if exists {
			continue
		}
		if err := s.CreateTextureDocumentOG(ctx, doc); err != nil {
			return fmt.Errorf("backfill OG texture documents: put %s: %w", doc.DocID, err)
		}
	}
	return rows.Err()
}

func (s *Store) backfillTextureRevisionsOG(ctx context.Context) error {
	rows, err := s.textureHandle().QueryContext(ctx, `SELECT revision_id, doc_id, owner_id, author_kind, author_label, version_number, content, body_doc_json, source_entities_json, citations_json, metadata_json, provenance_json, revision_hash, parent_revision_id, created_at FROM texture_revisions`)
	if err != nil {
		return fmt.Errorf("backfill OG texture revisions: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			rev            types.Revision
			bodyDocJSON    string
			sourceEntities string
			citationsJSON  string
			metadataJSON   string
			provenanceJSON string
			createdAt      string
		)
		if err := rows.Scan(&rev.RevisionID, &rev.DocID, &rev.OwnerID, &rev.AuthorKind, &rev.AuthorLabel, &rev.VersionNumber, &rev.Content, &bodyDocJSON, &sourceEntities, &citationsJSON, &metadataJSON, &provenanceJSON, &rev.RevisionHash, &rev.ParentRevisionID, &createdAt); err != nil {
			return fmt.Errorf("backfill OG texture revisions: scan: %w", err)
		}
		rev.BodyDoc = []byte(bodyDocJSON)
		rev.SourceEntities = []byte(sourceEntities)
		rev.Citations = []byte(citationsJSON)
		rev.Metadata = []byte(metadataJSON)
		rev.Provenance = []byte(provenanceJSON)
		rev.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		// Put-if-absent: skip if OG already has this revision.
		exists, err := s.ogExistsByKey(ctx, ogKindTexRev, "revision_id", rev.RevisionID)
		if err != nil {
			return fmt.Errorf("backfill OG texture revisions: check %s: %w", rev.RevisionID, err)
		}
		if exists {
			continue
		}
		if err := s.CreateTextureRevisionOG(ctx, rev); err != nil {
			return fmt.Errorf("backfill OG texture revisions: put %s: %w", rev.RevisionID, err)
		}
	}
	return rows.Err()
}

func (s *Store) backfillTextureDecisionsOG(ctx context.Context) error {
	rows, err := s.textureHandle().QueryContext(ctx, `SELECT decision_id, owner_id, doc_id, loop_id, trajectory_id, actor_id, decision_kind, reason, evidence_refs_json, next_action, created_at FROM texture_decisions`)
	if err != nil {
		return fmt.Errorf("backfill OG texture decisions: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			dec          types.TextureDecisionRecord
			evidenceRefs string
			createdAt    string
		)
		if err := rows.Scan(&dec.DecisionID, &dec.OwnerID, &dec.DocID, &dec.RunID, &dec.TrajectoryID, &dec.ActorID, &dec.DecisionKind, &dec.Reason, &evidenceRefs, &dec.NextAction, &createdAt); err != nil {
			return fmt.Errorf("backfill OG texture decisions: scan: %w", err)
		}
		if evidenceRefs != "" {
			_ = json.Unmarshal([]byte(evidenceRefs), &dec.EvidenceRefs)
		}
		dec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		// Put-if-absent: skip if OG already has this decision.
		exists, err := s.ogExistsByKey(ctx, ogKindTexDecision, "decision_id", dec.DecisionID)
		if err != nil {
			return fmt.Errorf("backfill OG texture decisions: check %s: %w", dec.DecisionID, err)
		}
		if exists {
			continue
		}
		if err := s.CreateTextureDecisionOG(ctx, dec); err != nil {
			return fmt.Errorf("backfill OG texture decisions: put %s: %w", dec.DecisionID, err)
		}
	}
	return rows.Err()
}

func (s *Store) backfillContentItemsOG(ctx context.Context) error {
	rows, err := s.textureHandle().QueryContext(ctx, `SELECT content_id, owner_id, source_type, media_type, app_hint, title, source_url, canonical_url, file_path, text_content, content_hash, metadata_json, provenance_json, created_at, updated_at FROM content_items`)
	if err != nil {
		return fmt.Errorf("backfill OG content items: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			rec        types.ContentItem
			metadata   string
			provenance string
			createdAt  string
			updatedAt  string
		)
		if err := rows.Scan(&rec.ContentID, &rec.OwnerID, &rec.SourceType, &rec.MediaType, &rec.AppHint, &rec.Title, &rec.SourceURL, &rec.CanonicalURL, &rec.FilePath, &rec.TextContent, &rec.ContentHash, &metadata, &provenance, &createdAt, &updatedAt); err != nil {
			return fmt.Errorf("backfill OG content items: scan: %w", err)
		}
		rec.Metadata = []byte(metadata)
		rec.Provenance = []byte(provenance)
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		rec.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
		// Put-if-absent: skip if OG already has this content item.
		exists, err := s.ogExistsByKey(ctx, ogKindContentItem, "content_id", rec.ContentID)
		if err != nil {
			return fmt.Errorf("backfill OG content items: check %s: %w", rec.ContentID, err)
		}
		if exists {
			continue
		}
		if err := s.CreateContentItemOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG content items: put %s: %w", rec.ContentID, err)
		}
	}
	return rows.Err()
}

func (s *Store) backfillPodcastSubscriptionsOG(ctx context.Context) error {
	rows, err := s.textureHandle().QueryContext(ctx, `SELECT subscription_id, owner_id, feed_url, content_id, title, author, artwork_url, last_fetched_at, created_at, updated_at FROM podcast_subscriptions`)
	if err != nil {
		return fmt.Errorf("backfill OG podcast subscriptions: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		rec, err := scanPodcastSubscription(rows)
		if err != nil {
			return fmt.Errorf("backfill OG podcast subscriptions: scan: %w", err)
		}
		// Put-if-absent: skip if OG already has this podcast subscription.
		exists, err := s.ogExistsByKey(ctx, ogKindPodcastSub, "subscription_id", rec.SubscriptionID)
		if err != nil {
			return fmt.Errorf("backfill OG podcast subscriptions: check %s: %w", rec.SubscriptionID, err)
		}
		if exists {
			continue
		}
		if err := s.CreatePodcastSubscriptionOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG podcast subscriptions: put %s: %w", rec.SubscriptionID, err)
		}
	}
	return rows.Err()
}

func (s *Store) backfillTextureSourceEntitiesOG(ctx context.Context) error {
	rows, err := s.textureHandle().QueryContext(ctx, `SELECT canonical_id, version_id, owner_id, computer_id, content_hash, body, metadata_json, legacy_source_entity_id, created_at FROM texture_source_entities`)
	if err != nil {
		return fmt.Errorf("backfill OG texture source entities: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			rec       TextureSourceEntityGraphRecord
			metadata  string
			createdAt string
		)
		if err := rows.Scan(&rec.CanonicalID, &rec.VersionID, &rec.OwnerID, &rec.ComputerID, &rec.ContentHash, &rec.Body, &metadata, &rec.LegacySourceEntityID, &createdAt); err != nil {
			return fmt.Errorf("backfill OG texture source entities: scan: %w", err)
		}
		rec.Metadata = json.RawMessage(metadata)
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		// Put-if-absent: skip if OG already has this source entity version.
		exists, err := s.TextureSourceEntityVersionExistsOG(ctx, rec.CanonicalID, rec.VersionID)
		if err != nil {
			return fmt.Errorf("backfill OG texture source entities: check %s/%s: %w", rec.CanonicalID, rec.VersionID, err)
		}
		if exists {
			continue
		}
		if err := s.PutTextureSourceEntityOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG texture source entities: put %s/%s: %w", rec.CanonicalID, rec.VersionID, err)
		}
	}
	return rows.Err()
}

func (s *Store) backfillTextureSourceRefsOG(ctx context.Context) error {
	rows, err := s.textureHandle().QueryContext(ctx, `SELECT canonical_id, version_id, owner_id, computer_id, content_hash, doc_id, texture_revision_id, body_node_id, body_node_path_hash, legacy_source_entity_id, source_entity_canonical_id, source_entity_version_id, display_mode, citation_state, metadata_json, created_at FROM texture_source_refs`)
	if err != nil {
		return fmt.Errorf("backfill OG texture source refs: query: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			rec       TextureSourceRefGraphRecord
			metadata  string
			createdAt string
		)
		if err := rows.Scan(&rec.CanonicalID, &rec.VersionID, &rec.OwnerID, &rec.ComputerID, &rec.ContentHash, &rec.DocID, &rec.TextureRevisionID, &rec.BodyNodeID, &rec.BodyNodePathHash, &rec.LegacySourceEntityID, &rec.SourceEntityCanonicalID, &rec.SourceEntityVersionID, &rec.DisplayMode, &rec.CitationState, &metadata, &createdAt); err != nil {
			return fmt.Errorf("backfill OG texture source refs: scan: %w", err)
		}
		rec.Metadata = json.RawMessage(metadata)
		rec.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		// Put-if-absent: skip if OG already has this source ref version.
		// Check by composite (canonical_id, version_id) since the SQL
		// primary key is (canonical_id, version_id).
		exists, err := s.TextureSourceRefVersionExistsOG(ctx, rec.CanonicalID, rec.VersionID)
		if err != nil {
			return fmt.Errorf("backfill OG texture source refs: check %s/%s: %w", rec.CanonicalID, rec.VersionID, err)
		}
		if exists {
			continue
		}
		if err := s.PutTextureSourceRefOG(ctx, rec); err != nil {
			return fmt.Errorf("backfill OG texture source refs: put %s/%s: %w", rec.CanonicalID, rec.VersionID, err)
		}
	}
	return rows.Err()
}

var runtimeTables = []string{
	"agents",
	"runs",
	"events",
	"channel_messages",
	"inbox_deliveries",
	"run_memory_entries",
	"run_acceptances",
	"run_continuations",
	"browser_sessions",
	"worker_updates",
	"media_progress",
	"media_recents",
	"user_preferences",
	"desktop_state",
	"desktop_workspaces",
}

func (s *Store) importLegacySQLiteRuntime(dbPath string) error {
	if strings.TrimSpace(dbPath) == "" {
		return nil
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat legacy sqlite path: %w", err)
	}
	if info.IsDir() || info.Size() == 0 {
		return nil
	}
	empty, err := s.runtimeTablesEmpty(context.Background())
	if err != nil {
		return err
	}
	if !empty {
		return nil
	}

	source, err := sql.Open("sqlite", dbPath+"?_busy_timeout=60000")
	if err != nil {
		return fmt.Errorf("open legacy sqlite %s: %w", dbPath, err)
	}
	defer func() { _ = source.Close() }()

	for _, table := range runtimeTables {
		exists, err := sqliteTableExists(source, table)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		if err := s.importSQLiteTable(context.Background(), source, table); err != nil {
			return err
		}
	}
	if err := s.backfillDerivedRuntimeState(); err != nil {
		return err
	}
	return nil
}

func (s *Store) runtimeTablesEmpty(ctx context.Context) (bool, error) {
	for _, table := range runtimeTables {
		if err := validateIdentifier(table); err != nil {
			return false, err
		}
		var count int
		if err := s.db.QueryRowContext(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count); err != nil {
			return false, fmt.Errorf("count runtime table %s: %w", table, err)
		}
		if count > 0 {
			return false, nil
		}
	}
	return true, nil
}

func sqliteTableExists(db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect legacy sqlite table %s: %w", table, err)
	}
	return name == table, nil
}

func (s *Store) importSQLiteTable(ctx context.Context, source *sql.DB, table string) error {
	if err := validateIdentifier(table); err != nil {
		return err
	}
	sourceCols, err := sqliteTableColumns(source, table)
	if err != nil {
		return err
	}
	if len(sourceCols) == 0 {
		return nil
	}
	destinationCols, err := s.destinationTableColumns(ctx, table)
	if err != nil {
		return err
	}
	cols := intersectColumns(sourceCols, destinationCols)
	if len(cols) == 0 {
		return nil
	}

	selectSQL := fmt.Sprintf("SELECT %s FROM %s", joinIdentifiers(cols), table)
	rows, err := source.QueryContext(ctx, selectSQL)
	if err != nil {
		return fmt.Errorf("query legacy sqlite table %s: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	placeholders := make([]string, len(cols))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		joinIdentifiers(cols),
		strings.Join(placeholders, ", "),
	)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin import table %s: %w", table, err)
	}
	defer func() { _ = tx.Rollback() }()

	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return fmt.Errorf("scan legacy sqlite table %s: %w", table, err)
		}
		args := make([]any, len(values))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				args[i] = string(b)
			} else {
				args[i] = v
			}
		}
		if _, err := tx.ExecContext(ctx, insertSQL, args...); err != nil {
			return fmt.Errorf("insert migrated row into %s: %w", table, err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate legacy sqlite table %s: %w", table, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit import table %s: %w", table, err)
	}
	return nil
}

func sqliteTableColumns(db *sql.DB, table string) ([]string, error) {
	if err := validateIdentifier(table); err != nil {
		return nil, err
	}
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return nil, fmt.Errorf("pragma legacy sqlite table_info(%s): %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	var cols []string
	for rows.Next() {
		var (
			cid      int
			name     string
			colType  string
			notNull  int
			defaultV sql.NullString
			primaryK int
		)
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defaultV, &primaryK); err != nil {
			return nil, fmt.Errorf("scan legacy sqlite table_info(%s): %w", table, err)
		}
		if err := validateIdentifier(name); err != nil {
			return nil, err
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate legacy sqlite table_info(%s): %w", table, err)
	}
	return cols, nil
}

func (s *Store) destinationTableColumns(ctx context.Context, table string) (map[string]bool, error) {
	if err := validateIdentifier(table); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT column_name
FROM information_schema.columns
WHERE table_schema = DATABASE()
  AND table_name = ?`, table)
	if err != nil {
		return nil, fmt.Errorf("inspect destination table %s columns: %w", table, err)
	}
	defer func() { _ = rows.Close() }()

	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan destination table %s column: %w", table, err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate destination table %s columns: %w", table, err)
	}
	return cols, nil
}

func intersectColumns(source []string, destination map[string]bool) []string {
	cols := make([]string, 0, len(source))
	for _, col := range source {
		if destination[col] {
			cols = append(cols, col)
		}
	}
	return cols
}

func joinIdentifiers(cols []string) string {
	quoted := make([]string, len(cols))
	for i, col := range cols {
		quoted[i] = "`" + col + "`"
	}
	return strings.Join(quoted, ", ")
}
