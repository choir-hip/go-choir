package objectgraph

import (
	"context"
	"fmt"
	"strings"
)

// JSONPrefix is a body predicate on the text of one JSON field.
type JSONPrefix struct {
	JSONPath string
	// LowerPrefix is compared with the field's text, trimmed of spaces and
	// lower-cased (ASCII). It must already be lower case.
	LowerPrefix string
}

// JSONArrayHas is a body predicate: the JSON array at JSONPath has an
// element equal to Value (an exact string match, never a prefix).
type JSONArrayHas struct {
	JSONPath string
	Value    string
}

// BodyPageQuery is a keyset-paged read of one owner's and one computer's
// live objects of a kind by predicates on the persisted JSON body.
// ListObjectsByOwnerAndBody has only exact field matches and a count limit;
// this adds the two predicates the calibration reads need (a field's
// negated prefix, an array element) and a cursor that is stable under
// appends: the row id, which only grows.
type BodyPageQuery struct {
	Kind       string
	OwnerID    string
	ComputerID string
	// Matches are exact field predicates, as in ListObjectsByOwnerAndBody.
	Matches []JSONFieldMatch
	// ExcludePrefix leaves out rows whose field text begins with a prefix. A
	// row without the field is kept.
	ExcludePrefix []JSONPrefix
	// ArrayHas keeps rows whose array field has the element.
	ArrayHas []JSONArrayHas
	// AfterObjID is the cursor: only rows with a greater row id (ascending) or
	// a smaller one (descending, when non-zero) are returned.
	AfterObjID int64
	// Descending returns the newest rows first.
	Descending bool
	// Limit is the page size, at least 1.
	Limit int
}

// BodyPageRow is one object and its row id, the next cursor.
type BodyPageRow struct {
	Object Object
	ObjID  int64
}

// extraScan scans one more column after the object's.
type extraScan struct {
	row   rowScanner
	extra *int64
}

func (e extraScan) Scan(dest ...any) error { return e.row.Scan(append(dest, e.extra)...) }

func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func bodyPageSQL(q BodyPageQuery) (string, []any, error) {
	if q.Kind == "" || q.OwnerID == "" || q.ComputerID == "" {
		return "", nil, fmt.Errorf("objectgraph sql: body page requires kind, owner and computer")
	}
	if q.Limit < 1 {
		return "", nil, fmt.Errorf("objectgraph sql: body page requires a limit of at least 1")
	}
	query := `SELECT ` + objectColumnsSQL + `, h.obj_id` + objectFromSQL + `
		 WHERE h.object_kind = ? AND h.owner_id = ? AND h.computer_id = ? AND h.tombstone = 0`
	args := []any{q.Kind, q.OwnerID, q.ComputerID}
	for _, match := range q.Matches {
		if match.JSONPath == "" {
			return "", nil, fmt.Errorf("objectgraph sql: body JSON path is required")
		}
		if match.MissingMatchesEmpty {
			return "", nil, fmt.Errorf("objectgraph sql: body page does not support missing-matches-empty")
		}
		query += ` AND ` + jsonTextSQL(bodyTextSQL, `?`) + ` = ?`
		args = append(args, jsonTextArgs(match.JSONPath)...)
		args = append(args, match.Value)
	}
	for _, ex := range q.ExcludePrefix {
		if ex.JSONPath == "" || ex.LowerPrefix == "" {
			return "", nil, fmt.Errorf("objectgraph sql: body page prefix requires a path and a prefix")
		}
		query += ` AND (json_extract(` + bodyTextSQL + `, ?) IS NULL OR lower(trim(CAST(json_extract(` + bodyTextSQL + `, ?) AS TEXT))) NOT LIKE ? ESCAPE '\')`
		args = append(args, ex.JSONPath, ex.JSONPath, likeEscape(ex.LowerPrefix)+`%`)
	}
	for _, has := range q.ArrayHas {
		if has.JSONPath == "" {
			return "", nil, fmt.Errorf("objectgraph sql: body JSON path is required")
		}
		query += ` AND EXISTS (SELECT 1 FROM json_each(` + bodyTextSQL + `, ?) WHERE json_each.value = ?)`
		args = append(args, has.JSONPath, has.Value)
	}
	switch {
	case q.Descending && q.AfterObjID > 0:
		query += ` AND h.obj_id < ?`
		args = append(args, q.AfterObjID)
	case !q.Descending && q.AfterObjID > 0:
		query += ` AND h.obj_id > ?`
		args = append(args, q.AfterObjID)
	}
	if q.Descending {
		query += ` ORDER BY h.obj_id DESC LIMIT ?`
	} else {
		query += ` ORDER BY h.obj_id ASC LIMIT ?`
	}
	args = append(args, q.Limit)
	return query, args, nil
}

// ListObjectsByOwnerBodyPage runs a BodyPageQuery.
func (s *SQLStore) ListObjectsByOwnerBodyPage(ctx context.Context, q BodyPageQuery) ([]BodyPageRow, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("objectgraph sql: nil store")
	}
	if err := checkReadKind(ctx, ObjectKind(q.Kind)); err != nil {
		return nil, err
	}
	query, args, err := bodyPageSQL(q)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("objectgraph sql: body page: %w", err)
	}
	defer rows.Close()
	var out []BodyPageRow
	for rows.Next() {
		var id int64
		obj, err := scanObject(extraScan{row: rows, extra: &id})
		if err != nil {
			return nil, err
		}
		out = append(out, BodyPageRow{Object: obj, ObjID: id})
	}
	return out, rows.Err()
}
