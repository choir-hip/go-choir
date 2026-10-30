package rowcopy

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// column is one column of an introspected table.
type column struct {
	Name    string
	Class   class
	NotNull bool
}

// table is an introspected table. PK is only filled for the source.
type table struct {
	Name    string
	Columns []column
	PK      []string
}

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func quoteMySQL(id string) string { return "`" + strings.ReplaceAll(id, "`", "``") + "`" }
func quoteANSI(id string) string  { return `"` + strings.ReplaceAll(id, `"`, `""`) + `"` }

// introspectSource reads the tables of the current MySQL/Dolt database.
func introspectSource(ctx context.Context, db *sql.DB) (map[string]*table, error) {
	tables := map[string]*table{}
	rows, err := db.QueryContext(ctx, `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE'`)
	if err != nil {
		return nil, fmt.Errorf("rowcopy: list source tables: %w", err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		tables[name] = &table{Name: name}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `SELECT table_name, column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = DATABASE() ORDER BY table_name, ordinal_position`)
	if err != nil {
		return nil, fmt.Errorf("rowcopy: list source columns: %w", err)
	}
	for rows.Next() {
		var tn, cn, dt, nullable string
		if err := rows.Scan(&tn, &cn, &dt, &nullable); err != nil {
			rows.Close()
			return nil, err
		}
		if t := tables[tn]; t != nil {
			t.Columns = append(t.Columns, column{Name: cn, Class: sourceClass(dt), NotNull: strings.EqualFold(nullable, "NO")})
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = db.QueryContext(ctx, `SELECT table_name, column_name FROM information_schema.key_column_usage WHERE table_schema = DATABASE() AND constraint_name = 'PRIMARY' ORDER BY table_name, ordinal_position`)
	if err != nil {
		return nil, fmt.Errorf("rowcopy: list source primary keys: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tn, cn string
		if err := rows.Scan(&tn, &cn); err != nil {
			return nil, err
		}
		if t := tables[tn]; t != nil {
			t.PK = append(t.PK, cn)
		}
	}
	return tables, rows.Err()
}

// introspectTarget reads the user tables of a SQLite or PostgreSQL database.
func introspectTarget(ctx context.Context, db *sql.DB, kind dbx.Kind) (map[string]*table, error) {
	names, err := dbx.Tables(ctx, db, kind)
	if err != nil {
		return nil, fmt.Errorf("rowcopy: list target tables: %w", err)
	}
	tables := map[string]*table{}
	for _, n := range names {
		tables[n] = &table{Name: n}
	}
	switch kind {
	case dbx.SQLite:
		for _, n := range names {
			rows, err := db.QueryContext(ctx, `SELECT name, type, "notnull", pk FROM pragma_table_xinfo(?) WHERE hidden = 0 ORDER BY cid`, n)
			if err != nil {
				return nil, fmt.Errorf("rowcopy: target columns of %s: %w", n, err)
			}
			for rows.Next() {
				var cn, decl string
				var notnull, pk int
				if err := rows.Scan(&cn, &decl, &notnull, &pk); err != nil {
					rows.Close()
					return nil, err
				}
				// An INTEGER PRIMARY KEY is NOT NULL in effect.
				tables[n].Columns = append(tables[n].Columns, column{Name: cn, Class: sqliteClass(decl), NotNull: notnull != 0 || pk != 0})
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return nil, err
			}
			rows.Close()
		}
	case dbx.Postgres:
		rows, err := db.QueryContext(ctx, `SELECT table_name, column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = current_schema() ORDER BY table_name, ordinal_position`)
		if err != nil {
			return nil, fmt.Errorf("rowcopy: target columns: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var tn, cn, dt, nullable string
			if err := rows.Scan(&tn, &cn, &dt, &nullable); err != nil {
				return nil, err
			}
			if t := tables[tn]; t != nil {
				t.Columns = append(t.Columns, column{Name: cn, Class: pgClass(dt), NotNull: nullable == "NO"})
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("rowcopy: unsupported target kind %q", kind)
	}
	return tables, nil
}

// pair is the intersection of one table in source and target.
type pair struct {
	Name string
	// Source and target columns, same length and order (target column order).
	SrcCols []column
	DstCols []column
	PK      []string // source primary key columns, source spelling
	SrcOnly []string
	DstOnly []string
}

func intersect(src, dst *table) pair {
	p := pair{Name: dst.Name, PK: src.PK}
	byLower := map[string]column{}
	for _, c := range src.Columns {
		byLower[strings.ToLower(c.Name)] = c
	}
	used := map[string]bool{}
	for _, d := range dst.Columns {
		s, ok := byLower[strings.ToLower(d.Name)]
		if !ok {
			p.DstOnly = append(p.DstOnly, d.Name)
			continue
		}
		used[strings.ToLower(d.Name)] = true
		p.SrcCols = append(p.SrcCols, s)
		p.DstCols = append(p.DstCols, d)
	}
	for _, c := range src.Columns {
		if !used[strings.ToLower(c.Name)] {
			p.SrcOnly = append(p.SrcOnly, c.Name)
		}
	}
	return p
}

// pgForeignOrder returns the PostgreSQL tables of names ordered so every
// table follows the tables its foreign keys reference. A cycle is an error.
func pgForeignOrder(ctx context.Context, db *sql.DB, names []string) ([]string, bool, error) {
	in := map[string]bool{}
	for _, n := range names {
		in[n] = true
	}
	rows, err := db.QueryContext(ctx, `SELECT child.relname, parent.relname FROM pg_constraint c JOIN pg_class child ON child.oid = c.conrelid JOIN pg_class parent ON parent.oid = c.confrelid JOIN pg_namespace ns ON ns.oid = c.connamespace WHERE c.contype = 'f' AND ns.nspname = current_schema()`)
	if err != nil {
		return nil, false, fmt.Errorf("rowcopy: foreign keys: %w", err)
	}
	defer rows.Close()
	parents := map[string][]string{}
	for rows.Next() {
		var child, parent string
		if err := rows.Scan(&child, &parent); err != nil {
			return nil, false, err
		}
		if in[child] && in[parent] && child != parent {
			parents[child] = append(parents[child], parent)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	var out []string
	state := map[string]int{}
	var visit func(n string) error
	visit = func(n string) error {
		switch state[n] {
		case 1:
			return fmt.Errorf("rowcopy: foreign key cycle through %s", n)
		case 2:
			return nil
		}
		state[n] = 1
		ps := parents[n]
		sort.Strings(ps)
		for _, p := range ps {
			if err := visit(p); err != nil {
				return err
			}
		}
		state[n] = 2
		out = append(out, n)
		return nil
	}
	for _, n := range sorted {
		if err := visit(n); err != nil {
			return nil, false, err
		}
	}
	return out, len(parents) > 0, nil
}
