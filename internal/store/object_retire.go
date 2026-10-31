package store

// Telemetry retirement is a tape operation
// (docs/design/object-graph-redesign-2026-10-30.md sections 11.3 and 11.6).
//
// A ProjectionOpObjectRetire op removes the aged choir.event rows of named
// event kinds from the working store (og_objects, and choir_events for the
// stamped rows). The rows stay on the sealed tape, in the projection batch
// that recorded each of them: only the working-store copy goes. The op names
// a half-open time slice [updated_from, updated_before) of at most 24 hours,
// so live apply and replay remove the same rows at the same tape position,
// and the apply reads only its own slice: the select uses the partial index
// idx_og_objects_choir_event_updated (objectgraph.ChoirEventUpdatedIndex,
// created with the schema at open), which
// needs the literal predicate object_kind = 'choir.event' in the SQL (a bound
// parameter cannot match a partial index predicate). Validation restricts
// object_kind to that value, so the literal is the op's meaning.
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

// retireOGTable is the table clause of every slice statement. Without
// statistics SQLite estimates an equality on idx_og_objects_kind_id as a few
// rows and prefers it to the partial index's range, which would read all 2.5
// million events per op and sort them; INDEXED BY pins the partial index. It
// exists whenever the store is open (objectgraph EnsureSchema creates it), and
// every statement that names it carries the literal object_kind predicate the
// index requires.
const retireOGTable = `og_objects INDEXED BY ` + objectgraph.ChoirEventUpdatedIndex

// retireEventPhaseSQL is the phase of the event whose body is body and whose
// metadata is meta: the payload's phase, else the metadata's, else ”. A body
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

// retireKeepCitedPredicate keeps the rows whose event a stored run acceptance
// cites in its evidence_refs. A body or metadata that is not JSON neither
// cites nor is cited (a statement that errors on one row would wedge replay),
// and the cited set excludes NULL, which would make NOT IN match nothing.
func retireKeepCitedPredicate(p computerevent.ObjectRetireProjection, meta string) string {
	if p.KeepCited != computerevent.ObjectRetireKeepRunAcceptance {
		return ""
	}
	return ` AND COALESCE(CASE WHEN json_valid(` + meta + `) THEN CAST(json_extract(` + meta + `, '$.event_id') AS TEXT) END, '') NOT IN (
		SELECT CAST(json_extract(e.value, '$.event_id') AS TEXT)
		FROM og_objects ra, json_each(CASE WHEN json_valid(CAST(ra.body AS TEXT)) THEN CAST(ra.body AS TEXT) ELSE '{}' END, '$.evidence_refs') e
		WHERE ra.object_kind = '` + string(ogKindRunAccept) + `' AND json_extract(e.value, '$.event_id') IS NOT NULL)`
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
// The object kind is the literal 'choir.event', never a parameter, so the
// planner can use the partial index (Validate admits no other kind).
func retireOGWhere(p computerevent.ObjectRetireProjection, from, before time.Time) (string, []any) {
	marks, kindArgs := retireKindPlaceholders(p)
	where := `object_kind = 'choir.event' AND updated_at >= ? AND updated_at < ? AND json_valid(metadata) AND json_extract(metadata, '$.kind') IN (` + marks + `)` +
		retirePhasePredicate(p, "body", "metadata") + retireKeepCitedPredicate(p, "metadata")
	args := append([]any{from, before}, kindArgs...)
	return where, args
}

// retireTableWhere is the choir_events selection of a retire op.
func retireTableWhere(p computerevent.ObjectRetireProjection, from, before time.Time) (string, []any) {
	marks, kindArgs := retireKindPlaceholders(p)
	where := `updated_at >= ? AND updated_at < ? AND event_kind IN (` + marks + `)` + retirePhasePredicate(p, "body", "metadata") + retireKeepCitedPredicate(p, "metadata")
	args := append([]any{from, before}, kindArgs...)
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
	from, before, err := p.Slice()
	if err != nil {
		return 0, fmt.Errorf("%w: %v", computerevent.ErrProjectionBatchInvalid, err)
	}
	var removed int64

	where, args := retireOGWhere(p, from, before)
	if derived.Keyed(objectgraph.ObjectKind(p.ObjectKind)) {
		ids, err := retireSelectIDs(ctx, tx, `SELECT canonical_id FROM `+retireOGTable+` WHERE `+where, args)
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
		res, err := tx.ExecContext(ctx, `DELETE FROM `+retireOGTable+` WHERE `+where, args...)
		if err != nil {
			return 0, fmt.Errorf("object retire: delete: %w", err)
		}
		n, _ := res.RowsAffected()
		removed += n
	}

	twhere, targs := retireTableWhere(p, from, before)
	res, err := tx.ExecContext(ctx, `DELETE FROM `+choirEventsTable+` WHERE `+twhere, targs...)
	if err != nil {
		return 0, fmt.Errorf("object retire: delete %s: %w", choirEventsTable, err)
	}
	n, _ := res.RowsAffected()
	removed += n
	if removed > 0 {
		log.Printf("store: object retire: removed %d %s rows (kinds=%v slice=[%s,%s) phase=%q reason=%q)",
			removed, p.ObjectKind, p.EventKinds, p.UpdatedFrom, p.UpdatedBefore, p.Phase, p.Reason)
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
// emitter's read-only probe on the read pool, so a slice with nothing to
// retire appends nothing.
func (s *Store) retireMatchExists(ctx context.Context, p computerevent.ObjectRetireProjection) (bool, error) {
	from, before, err := p.Slice()
	if err != nil {
		return false, err
	}
	where, args := retireOGWhere(p, from, before)
	var one int
	err = s.retireReader().QueryRowContext(ctx, `SELECT 1 FROM `+retireOGTable+` WHERE `+where+` LIMIT 1`, args...).Scan(&one)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	twhere, targs := retireTableWhere(p, from, before)
	err = s.retireReader().QueryRowContext(ctx, `SELECT 1 FROM `+choirEventsTable+` WHERE `+twhere+` LIMIT 1`, targs...).Scan(&one)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

// retireOldest is the oldest updated_at of any choir.event row before cutoff
// (the kind is not filtered: an over-approximation that keeps the query on
// the partial index), or the zero time when there is none.
func (s *Store) retireOldest(ctx context.Context, cutoff time.Time) (time.Time, error) {
	var oldest time.Time
	for _, q := range []string{
		`SELECT MIN(updated_at) FROM ` + retireOGTable + ` WHERE object_kind = 'choir.event' AND updated_at < ?`,
		`SELECT MIN(updated_at) FROM ` + choirEventsTable + ` WHERE updated_at < ?`,
	} {
		var raw any
		if err := s.retireReader().QueryRowContext(ctx, q, cutoff).Scan(&raw); err != nil {
			return time.Time{}, err
		}
		t, err := retireParseTime(raw)
		if err != nil {
			return time.Time{}, err
		}
		if !t.IsZero() && (oldest.IsZero() || t.Before(oldest)) {
			oldest = t
		}
	}
	return oldest, nil
}

// retireReader is the read pool, or the write handle where there is none: the
// emitter's probes run outside any write transaction.
func (s *Store) retireReader() *sql.DB {
	if s.readDB != nil {
		return s.readDB
	}
	return s.db
}

// retireParseTime reads a MIN(updated_at) result: nil for no row, a time, or
// the driver's text form.
func retireParseTime(raw any) (time.Time, error) {
	switch v := raw.(type) {
	case nil:
		return time.Time{}, nil
	case time.Time:
		return v.UTC(), nil
	case string:
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05.999999999"} {
			if t, err := time.Parse(layout, v); err == nil {
				return t.UTC(), nil
			}
		}
		return time.Time{}, fmt.Errorf("store: unreadable updated_at %q", v)
	case []byte:
		return retireParseTime(string(v))
	}
	return time.Time{}, fmt.Errorf("store: unreadable updated_at of type %T", raw)
}
