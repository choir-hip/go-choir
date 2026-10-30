package store

// Telemetry retirement is a tape operation
// (docs/design/object-graph-redesign-2026-10-30.md sections 11.3 and 11.6).
//
// A ProjectionOpObjectRetire op removes the aged choir.event rows of named
// event kinds from the working store (og_objects, and choir_events for the
// stamped rows). The rows stay on the sealed tape, in the projection batch
// that recorded each of them: only the working-store copy goes. The op names
// its cutoff, so live apply and replay remove the same rows at the same tape
// position; a bounded op (MaxRows) removes the oldest first by
// (updated_at, canonical_id), which is a total order.
//
// An event line's kind is in its metadata ("kind", written by
// buildEventObject), and choir_events keeps it in event_kind. The phase of a
// progress line is in the payload ("payload.phase"; the store keeps the
// payload, not EventRecord.Phase, so runEventEmitter writes it there), with
// metadata.phase as the fallback for a row that carries it only there.
//
// Never touched: the tape, the CAS and the payload stores. The agent traces
// are the owner's data and stay sealed.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
)

// retireDropBatch bounds the ids one derived-key drop and delete handles.
const retireDropBatch = 500

// retireEventPhaseSQL is the phase of the event whose body is body and whose
// metadata is meta: the payload's phase, else the metadata's, else ''. A body
// or metadata that is not JSON reads as phaseless rather than failing the
// whole statement (a projection that errors on one row would wedge replay).
func retireEventPhaseSQL(body, meta string) string {
	return `COALESCE(
		CASE WHEN json_valid(CAST(` + body + ` AS TEXT)) THEN NULLIF(CAST(json_extract(CAST(` + body + ` AS TEXT), '$.payload.phase') AS TEXT), '') END,
		CASE WHEN json_valid(` + meta + `) THEN NULLIF(CAST(json_extract(` + meta + `, '$.phase') AS TEXT), '') END,
		'')`
}

func retirePhasePredicate(p computerevent.ObjectRetireProjection, body, meta string) string {
	switch p.Phase {
	case computerevent.ObjectRetirePhaseNone:
		return ` AND ` + retireEventPhaseSQL(body, meta) + ` = ''`
	case computerevent.ObjectRetirePhaseSome:
		return ` AND ` + retireEventPhaseSQL(body, meta) + ` <> ''`
	}
	return ""
}

func retireKindPlaceholders(p computerevent.ObjectRetireProjection) (string, []any) {
	marks := make([]string, len(p.EventKinds))
	args := make([]any, len(p.EventKinds))
	for i, k := range p.EventKinds {
		marks[i], args[i] = "?", k
	}
	return strings.Join(marks, ","), args
}

// retireOGWhere is the og_objects selection of a retire op, with its args.
func retireOGWhere(p computerevent.ObjectRetireProjection, cutoff time.Time) (string, []any) {
	marks, kindArgs := retireKindPlaceholders(p)
	where := `object_kind = ? AND updated_at < ? AND json_valid(metadata) AND json_extract(metadata, '$.kind') IN (` + marks + `)` +
		retirePhasePredicate(p, "body", "metadata")
	args := append([]any{p.ObjectKind, cutoff}, kindArgs...)
	return where, args
}

// retireTableWhere is the choir_events selection of a retire op.
func retireTableWhere(p computerevent.ObjectRetireProjection, cutoff time.Time) (string, []any) {
	marks, kindArgs := retireKindPlaceholders(p)
	where := `updated_at < ? AND event_kind IN (` + marks + `)` + retirePhasePredicate(p, "body", "metadata")
	args := append([]any{cutoff}, kindArgs...)
	return where, args
}

// projectObjectRetire applies a retire op in the batch transaction and
// returns the rows it removed. derived keeps the og_key_index side table in
// step (nil on replay, which withdraws the families and lets the online pass
// fill them; a choir.event has no derived-key family today, so there is
// nothing to drop, but the drop is written for the day it has one).
func projectObjectRetire(ctx context.Context, tx *sql.Tx, body []byte, derived *objectgraph.DerivedKeys) (int64, error) {
	p, err := computerevent.DecodeObjectRetire(body)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", computerevent.ErrProjectionBatchInvalid, err)
	}
	cutoff, err := p.Cutoff()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", computerevent.ErrProjectionBatchInvalid, err)
	}
	var removed int64

	// og_objects first, then choir_events: a bounded op spends its bound in
	// that order, which is part of the op's meaning.
	where, args := retireOGWhere(p, cutoff)
	limit := ""
	if p.MaxRows > 0 {
		limit = fmt.Sprintf(` ORDER BY updated_at, canonical_id LIMIT %d`, p.MaxRows)
	}
	if derived.Keyed(objectgraph.ObjectKind(p.ObjectKind)) {
		ids, err := retireSelectIDs(ctx, tx, `SELECT canonical_id FROM og_objects WHERE `+where+limit, args)
		if err != nil {
			return 0, fmt.Errorf("object retire: select: %w", err)
		}
		for start := 0; start < len(ids); start += retireDropBatch {
			chunk := ids[start:min(start+retireDropBatch, len(ids))]
			marks := make([]string, len(chunk))
			chunkArgs := make([]any, len(chunk))
			for i, id := range chunk {
				derived.Drop(ctx, tx, id)
				marks[i], chunkArgs[i] = "?", id
			}
			res, err := tx.ExecContext(ctx, `DELETE FROM og_objects WHERE canonical_id IN (`+strings.Join(marks, ",")+`)`, chunkArgs...)
			if err != nil {
				return 0, fmt.Errorf("object retire: delete: %w", err)
			}
			n, _ := res.RowsAffected()
			removed += n
		}
	} else {
		var res sql.Result
		if p.MaxRows > 0 {
			res, err = tx.ExecContext(ctx, `DELETE FROM og_objects WHERE canonical_id IN (SELECT canonical_id FROM og_objects WHERE `+where+limit+`)`, args...)
		} else {
			res, err = tx.ExecContext(ctx, `DELETE FROM og_objects WHERE `+where, args...)
		}
		if err != nil {
			return 0, fmt.Errorf("object retire: delete: %w", err)
		}
		n, _ := res.RowsAffected()
		removed += n
	}

	room := int64(p.MaxRows) - removed
	if p.MaxRows == 0 || room > 0 {
		twhere, targs := retireTableWhere(p, cutoff)
		var res sql.Result
		if p.MaxRows > 0 {
			res, err = tx.ExecContext(ctx, `DELETE FROM `+choirEventsTable+` WHERE rowid IN (SELECT rowid FROM `+choirEventsTable+` WHERE `+twhere+
				fmt.Sprintf(` ORDER BY updated_at, tape_seq, op_index LIMIT %d)`, room), targs...)
		} else {
			res, err = tx.ExecContext(ctx, `DELETE FROM `+choirEventsTable+` WHERE `+twhere, targs...)
		}
		if err != nil {
			return 0, fmt.Errorf("object retire: delete %s: %w", choirEventsTable, err)
		}
		n, _ := res.RowsAffected()
		removed += n
	}
	if removed > 0 {
		log.Printf("store: object retire: removed %d %s rows (kinds=%v before=%s phase=%q max_rows=%d reason=%q)",
			removed, p.ObjectKind, p.EventKinds, p.UpdatedBefore, p.Phase, p.MaxRows, p.Reason)
	}
	return removed, nil
}

func retireSelectIDs(ctx context.Context, tx *sql.Tx, query string, args []any) ([]string, error) {
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// retireMatchExists reports whether any row would be removed by the op: the
// emitter's read-only probe, so a day with nothing to retire appends nothing.
func (s *Store) retireMatchExists(ctx context.Context, p computerevent.ObjectRetireProjection) (bool, error) {
	cutoff, err := p.Cutoff()
	if err != nil {
		return false, err
	}
	where, args := retireOGWhere(p, cutoff)
	var one int
	err = s.db.QueryRowContext(ctx, `SELECT 1 FROM og_objects WHERE `+where+` LIMIT 1`, args...).Scan(&one)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	twhere, targs := retireTableWhere(p, cutoff)
	err = s.db.QueryRowContext(ctx, `SELECT 1 FROM `+choirEventsTable+` WHERE `+twhere+` LIMIT 1`, targs...).Scan(&one)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}
