package computerversion

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"math"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	// SQLiteStateMaterializer names the realization produced by the SQLite
	// state extractor: the function that reads a guest store and produces
	// content observations.
	SQLiteStateMaterializer = "sqlite-state-extractor"
	// SQLiteStateSubstrate names the SQLite substrate for the observations.
	SQLiteStateSubstrate = "sqlite/state-db"

	// WitnessV3Domain tags every v3 digest so a table digest can never be
	// mistaken for a digest of another construction.
	witnessV3Domain = "choir/witness/v3"
)

// SQLiteStateExtractor reads content observations from a guest SQLite store
// (witness v3, docs/design/data-system-build-sheet-2026-10-30.md decision 6).
// It performs read-only queries inside one read transaction, so every digest
// is of one committed state, and it streams rows, so memory stays bounded.
//
// Observations produced, for database D:
//   - sqlite:D:schema: SHA-256 over the sqlite_master entries (type, name,
//     table name, sql text), sorted by name, of every observed table and its
//     indexes and triggers.
//   - sqlite:D:schema:T: the same digest for one table's entries.
//   - sqlite:D:table:T: SHA-256 over the rows of T in primary-key order in
//     the canonical row encoding below, after the configured exclusions. A
//     table without a primary key is digested over its sorted row encodings
//     (spilling to disk past witnessSpillBytes).
//   - sqlite:D:content_root: the digest of the schema hash and every table
//     digest (WitnessV3ContentRoot), the cross-substrate equivalence signal.
//
// Canonical row encoding: for each column in declaration order, minus the
// excluded ones, one tag byte then the value: 'n' NULL; 'i' int64 (8 bytes
// big endian); 'r' float64 bits (8 bytes); 't' text, 'b' blob (uvarint length
// then the bytes); 'd' time as RFC 3339 nanoseconds in UTC (text form). Live
// and replay roots are computed by this one function.
type SQLiteStateExtractor struct {
	// DB is the store to read. A read-pool handle is enough; the extractor
	// only begins a read-only transaction.
	DB *sql.DB

	// Database names the store in observation keys ("texture" for the guest
	// store). It never touches the file system.
	Database string

	// IgnoredContentColumns lists operational, non-semantic columns left out
	// of a table's row digest (a replay writes fresh wall-clock values).
	// Schema observations still include them.
	IgnoredContentColumns map[string]map[string]struct{}

	// IgnoredTables lists tables left out of the observation entirely, name,
	// schema and rows: derived indexes that are rebuilt from other tables
	// and so are not state.
	IgnoredTables map[string]struct{}

	// SnapshotHook, when set, runs once inside the read transaction before any
	// table is read. A hook that reads first (a head, under a lock that holds
	// writers) fixes the transaction's WAL snapshot at a state it names, then
	// may release the lock: the scan that follows still reads that state.
	// A hook error fails the extraction.
	SnapshotHook func(ctx context.Context, tx *sql.Tx) error
}

var _ Extractor = SQLiteStateExtractor{}

// StateDigests is the extractor's raw result: lowercase hex SHA-256 digests.
type StateDigests struct {
	Database     string
	SchemaHash   string
	TableSchemas map[string]string
	Tables       map[string]string
	ContentRoot  string
}

// WitnessV3ContentRoot is the root over the schema hash and the table
// digests. It is defined here so the extractor, the witness builder and the
// host judge agree on one construction.
func WitnessV3ContentRoot(schemaHash string, tables map[string]string) string {
	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	hasher := sha256.New()
	hasher.Write([]byte(witnessV3Domain + "\nschema " + schemaHash + "\n"))
	for _, name := range names {
		hasher.Write([]byte("table " + name + " " + tables[name] + "\n"))
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

// EmptyTableDigest is the observation value ("sha256:" and the digest) of a
// table with no rows, whether or not it has a primary key.
func EmptyTableDigest(table string) string {
	hasher := sha256.New()
	hasher.Write([]byte(witnessV3Domain + "\ntable " + table + "\n"))
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

// Digests computes the v3 digests of the store in one read transaction.
func (e SQLiteStateExtractor) Digests(ctx context.Context) (StateDigests, error) {
	if err := ctx.Err(); err != nil {
		return StateDigests{}, err
	}
	if e.DB == nil {
		return StateDigests{}, fmt.Errorf("sqlite state extraction: store is required")
	}
	database := strings.TrimSpace(e.Database)
	if database == "" {
		return StateDigests{}, fmt.Errorf("sqlite state extraction: database is required")
	}
	tx, err := e.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return StateDigests{}, fmt.Errorf("sqlite state extraction: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if e.SnapshotHook != nil {
		if err := e.SnapshotHook(ctx, tx); err != nil {
			return StateDigests{}, err
		}
	}
	return e.DigestsTx(ctx, tx)
}

// DigestsTx computes the v3 digests inside a read transaction the caller
// opened and owns (e.DB is not used, SnapshotHook is not run). The snapshot
// producer streams the database's pages and computes the witness in one
// transaction, so head, pages and witness are one committed state.
func (e SQLiteStateExtractor) DigestsTx(ctx context.Context, tx *sql.Tx) (StateDigests, error) {
	if err := ctx.Err(); err != nil {
		return StateDigests{}, err
	}
	if tx == nil {
		return StateDigests{}, fmt.Errorf("sqlite state extraction: transaction is required")
	}
	database := strings.TrimSpace(e.Database)
	if database == "" {
		return StateDigests{}, fmt.Errorf("sqlite state extraction: database is required")
	}

	entries, err := e.schemaEntries(ctx, tx)
	if err != nil {
		return StateDigests{}, err
	}
	out := StateDigests{
		Database:     database,
		TableSchemas: map[string]string{},
		Tables:       map[string]string{},
	}
	out.SchemaHash = schemaDigest(entries, "")
	tables := make([]string, 0)
	for _, entry := range entries {
		if entry.typ == "table" {
			tables = append(tables, entry.name)
		}
	}
	sort.Strings(tables)
	for _, table := range tables {
		if err := ctx.Err(); err != nil {
			return StateDigests{}, err
		}
		out.TableSchemas[table] = schemaDigest(entries, table)
		digest, err := e.tableDigest(ctx, tx, table)
		if err != nil {
			return StateDigests{}, fmt.Errorf("sqlite state extraction: content for %s: %w", table, err)
		}
		out.Tables[table] = digest
	}
	out.ContentRoot = WitnessV3ContentRoot(out.SchemaHash, out.Tables)
	return out, nil
}

// Extract produces the observation set for request.Version.
func (e SQLiteStateExtractor) Extract(ctx context.Context, request ExtractRequest) (ObservationSet, error) {
	if !request.Version.Valid() {
		return ObservationSet{}, fmt.Errorf("sqlite state extraction: invalid computer version")
	}
	digests, err := e.Digests(ctx)
	if err != nil {
		return ObservationSet{}, err
	}
	return ObservationSetFromDigests(digests, request)
}

// ObservationSetFromDigests builds the observation set for request from
// digests already computed (Digests), for a caller whose version depends on
// something the extraction read (the head its SnapshotHook named).
func ObservationSetFromDigests(digests StateDigests, request ExtractRequest) (ObservationSet, error) {
	if !request.Version.Valid() {
		return ObservationSet{}, fmt.Errorf("sqlite state extraction: invalid computer version")
	}
	prefix := "sqlite:" + digests.Database + ":"
	observations := make([]Observation, 0, 2+2*len(digests.Tables))
	observations = append(observations, Observation{Kind: ObservationDoltHead, Key: prefix + "schema", Value: "sha256:" + digests.SchemaHash})
	tables := make([]string, 0, len(digests.Tables))
	for table := range digests.Tables {
		tables = append(tables, table)
	}
	sort.Strings(tables)
	for _, table := range tables {
		observations = append(observations,
			Observation{Kind: ObservationDoltHead, Key: prefix + "schema:" + table, Value: "sha256:" + digests.TableSchemas[table]},
			Observation{Kind: ObservationDoltHead, Key: prefix + "table:" + table, Value: "sha256:" + digests.Tables[table]},
		)
	}
	observations = append(observations, Observation{Kind: ObservationDoltHead, Key: prefix + "content_root", Value: "sha256:" + digests.ContentRoot})
	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "sqlite-state"
	}
	return ObservationSet{
		Name:         name,
		Version:      request.Version,
		Required:     []ObservationKind{ObservationDoltHead},
		Observations: observations,
	}, nil
}

type schemaEntry struct {
	typ, name, table, sql string
}

// schemaEntries reads sqlite_master minus SQLite's own objects, auto indexes
// (no SQL text) and the ignored tables with their indexes and triggers.
func (e SQLiteStateExtractor) schemaEntries(ctx context.Context, tx *sql.Tx) ([]schemaEntry, error) {
	rows, err := tx.QueryContext(ctx, `SELECT type, name, tbl_name, sql FROM sqlite_master WHERE sql IS NOT NULL AND name NOT LIKE 'sqlite\_%' ESCAPE '\' ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("sqlite state extraction: read sqlite_master: %w", err)
	}
	defer rows.Close()
	var entries []schemaEntry
	for rows.Next() {
		var entry schemaEntry
		if err := rows.Scan(&entry.typ, &entry.name, &entry.table, &entry.sql); err != nil {
			return nil, fmt.Errorf("sqlite state extraction: scan sqlite_master: %w", err)
		}
		if _, ignored := e.IgnoredTables[entry.table]; ignored {
			continue
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite state extraction: iterate sqlite_master: %w", err)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	return entries, nil
}

// schemaDigest hashes the entries of one table (its own and its indexes and
// triggers), or all of them when table is empty.
func schemaDigest(entries []schemaEntry, table string) string {
	hasher := sha256.New()
	hasher.Write([]byte(witnessV3Domain + "\nschema\n"))
	for _, entry := range entries {
		if table != "" && entry.table != table {
			continue
		}
		writeLP(hasher, entry.typ)
		writeLP(hasher, entry.name)
		writeLP(hasher, entry.table)
		writeLP(hasher, entry.sql)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func writeLP(h hash.Hash, s string) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], uint64(len(s)))
	h.Write(buf[:n])
	h.Write([]byte(s))
}

type columnInfo struct {
	name string
	pk   int
}

// tableColumns returns the table's stored columns in declaration order.
// Generated columns (hidden 2 virtual, 3 stored) are derivations of other
// columns and indexes; the witness covers what is stored, so it stays
// independent of them.
func tableColumns(ctx context.Context, tx *sql.Tx, table string) ([]columnInfo, error) {
	rows, err := tx.QueryContext(ctx, `SELECT name, pk FROM pragma_table_xinfo(?) WHERE hidden IN (0, 1) ORDER BY cid`, table)
	if err != nil {
		return nil, fmt.Errorf("table_info %s: %w", table, err)
	}
	defer rows.Close()
	var cols []columnInfo
	for rows.Next() {
		var c columnInfo
		if err := rows.Scan(&c.name, &c.pk); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// tableDigest streams the table's rows and returns the lowercase hex digest.
func (e SQLiteStateExtractor) tableDigest(ctx context.Context, tx *sql.Tx, table string) (string, error) {
	cols, err := tableColumns(ctx, tx, table)
	if err != nil {
		return "", err
	}
	if len(cols) == 0 {
		return "", fmt.Errorf("table %s has no columns", table)
	}
	ignored := e.IgnoredContentColumns[table]
	var pk []columnInfo
	for _, c := range cols {
		if c.pk > 0 {
			pk = append(pk, c)
		}
	}
	sort.Slice(pk, func(i, j int) bool { return pk[i].pk < pk[j].pk })

	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = quoteIdent(c.name)
	}
	query := "SELECT " + strings.Join(names, ", ") + " FROM " + quoteIdent(table)
	if len(pk) > 0 {
		order := make([]string, len(pk))
		for i, c := range pk {
			order[i] = quoteIdent(c.name)
		}
		query += " ORDER BY " + strings.Join(order, ", ")
	}
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return "", fmt.Errorf("select from %s: %w", table, err)
	}
	defer rows.Close()

	hasher := sha256.New()
	hasher.Write([]byte(witnessV3Domain + "\ntable " + table + "\n"))
	var sorter *rowSorter
	if len(pk) == 0 {
		sorter = newRowSorter(os.Getenv(witnessSpillDirEnv), witnessSpillBytes)
		defer sorter.Close()
	}
	values := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	var row []byte
	count := 0
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", fmt.Errorf("scan row from %s: %w", table, err)
		}
		row = row[:0]
		for i, c := range cols {
			if _, skip := ignored[c.name]; skip {
				continue
			}
			var encErr error
			row, encErr = appendCanonicalValue(row, values[i])
			if encErr != nil {
				return "", fmt.Errorf("encode %s.%s: %w", table, c.name, encErr)
			}
		}
		count++
		if sorter != nil {
			if err := sorter.Add(hex.EncodeToString(row)); err != nil {
				return "", fmt.Errorf("digest row from %s: %w", table, err)
			}
			continue
		}
		var length [binary.MaxVarintLen64]byte
		n := binary.PutUvarint(length[:], uint64(len(row)))
		hasher.Write(length[:n])
		hasher.Write(row)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate rows from %s: %w", table, err)
	}
	if sorter != nil && count > 0 {
		sum, err := sorter.Sum()
		if err != nil {
			return "", err
		}
		hasher.Write([]byte(strings.TrimPrefix(sum, "sha256:")))
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func appendCanonicalValue(dst []byte, value any) ([]byte, error) {
	var num [8]byte
	switch v := value.(type) {
	case nil:
		return append(dst, 'n'), nil
	case int64:
		binary.BigEndian.PutUint64(num[:], uint64(v))
		return append(append(dst, 'i'), num[:]...), nil
	case float64:
		binary.BigEndian.PutUint64(num[:], math.Float64bits(v))
		return append(append(dst, 'r'), num[:]...), nil
	case bool:
		if v {
			return appendCanonicalValue(dst, int64(1))
		}
		return appendCanonicalValue(dst, int64(0))
	case string:
		return appendLenBytes(append(dst, 't'), []byte(v)), nil
	case []byte:
		return appendLenBytes(append(dst, 'b'), v), nil
	case time.Time:
		return appendLenBytes(append(dst, 'd'), []byte(v.UTC().Format(time.RFC3339Nano))), nil
	default:
		return nil, fmt.Errorf("unsupported column value type %T", value)
	}
}

func appendLenBytes(dst, b []byte) []byte {
	var length [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(length[:], uint64(len(b)))
	dst = append(dst, length[:n]...)
	return append(dst, b...)
}
