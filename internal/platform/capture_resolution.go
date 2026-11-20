package platform

// proj_capture_resolution: which captures the entity layer has answered
// (docs/design/entity-layer-2026-11-20.md sections 3.3 and 4.1). One row per
// capture the layer has either resolved (its entity links are written) or
// refused (the vendor will not answer for it alone, so it is not asked
// again). The ready list (capture_ready.go) is the captures that are ready
// and have no row here, so corpusd knows what is still waiting without a
// queue in memory.
//
// The table is derived and rebuildable: the authority is the computer's
// tape record (record_id names it), and the rows are re-exported from it.
// Nothing in station E2 writes it but tests; station E3's flush does.
// Rollback: DROP TABLE proj_capture_resolution (every capture is then
// waiting again, which only means the next flush asks again).

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const captureResolutionTable = "proj_capture_resolution"

const (
	// ResolutionResolved: the capture's entity links were decided and exported.
	ResolutionResolved = "resolved"
	// ResolutionRefused: the vendor refused the capture asked alone; not asked again.
	ResolutionRefused = "refused"
)

const captureResolutionDDL = `
CREATE TABLE IF NOT EXISTS proj_capture_resolution (
	capture_id  TEXT NOT NULL PRIMARY KEY,
	owner_id    TEXT NOT NULL,
	computer_id TEXT NOT NULL,
	outcome     TEXT NOT NULL,
	record_id   TEXT NOT NULL DEFAULT '',
	resolved_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pcr_scope ON proj_capture_resolution(owner_id, computer_id, resolved_at DESC);
`

// ensureCaptureResolutionTable creates the table (idempotent, once a process).
func (s *Store) ensureCaptureResolutionTable(ctx context.Context) error {
	if s == nil {
		return fmt.Errorf("capture resolution: nil store")
	}
	if s.retrievalState.captureResolutionReady.Load() {
		return nil
	}
	if _, err := s.corpus().ExecContext(ctx, captureResolutionDDL); err != nil {
		return fmt.Errorf("create capture resolution table: %w", err)
	}
	s.retrievalState.captureResolutionReady.Store(true)
	return nil
}

// CaptureResolution is one answered capture.
type CaptureResolution struct {
	CaptureID  string
	OwnerID    string
	ComputerID string
	Outcome    string
	RecordID   string
	At         time.Time
}

// RecordCaptureResolutions writes the rows, replacing a capture's earlier row
// (a re-export of the same tape record is the same row).
func (s *Store) RecordCaptureResolutions(ctx context.Context, rows []CaptureResolution) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.ensureCaptureResolutionTable(ctx); err != nil {
		return err
	}
	for start := 0; start < len(rows); start += 500 {
		chunk := rows[start:min(start+500, len(rows))]
		vals := make([]string, 0, len(chunk))
		args := make([]any, 0, 6*len(chunk))
		for _, r := range chunk {
			if strings.TrimSpace(r.CaptureID) == "" || strings.TrimSpace(r.OwnerID) == "" || strings.TrimSpace(r.ComputerID) == "" {
				return fmt.Errorf("capture resolution: capture, owner and computer are required")
			}
			if r.Outcome != ResolutionResolved && r.Outcome != ResolutionRefused {
				return fmt.Errorf("capture resolution: outcome %q is not %s or %s", r.Outcome, ResolutionResolved, ResolutionRefused)
			}
			at := r.At
			if at.IsZero() {
				at = time.Now()
			}
			vals = append(vals, "(?,?,?,?,?,?)")
			args = append(args, r.CaptureID, r.OwnerID, r.ComputerID, r.Outcome, r.RecordID, at.UTC())
		}
		if _, err := s.corpus().ExecContext(ctx, `INSERT INTO proj_capture_resolution (capture_id, owner_id, computer_id, outcome, record_id, resolved_at) VALUES `+strings.Join(vals, ",")+`
			ON CONFLICT (capture_id) DO UPDATE SET owner_id = EXCLUDED.owner_id, computer_id = EXCLUDED.computer_id, outcome = EXCLUDED.outcome, record_id = EXCLUDED.record_id, resolved_at = EXCLUDED.resolved_at`, args...); err != nil {
			return fmt.Errorf("write capture resolutions: %w", err)
		}
	}
	return nil
}
