package computerversion

import (
	"context"
	"database/sql"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	embedded "github.com/dolthub/driver"
)

const (
	// DoltStateMaterializer names the realization produced by the Dolt
	// state extractor — the function that reads a live embedded Dolt
	// database and produces dolt_head observations.
	DoltStateMaterializer = "dolt-state-extractor"
	// DoltStateSubstrate names the embedded-Dolt substrate for dolt_head
	// extraction. The substrate identity comes from the Dolt workspace
	// path, not from a VM or filesystem directory.
	DoltStateSubstrate = "embedded-dolt/workspace"
)

// DoltStateExtractor reads dolt_head observations from a live embedded Dolt
// database. Unlike DoltHeadSnapshot (which is a fixture-only declared head),
// this extractor connects to a real Dolt workspace, queries table content,
// and produces content-addressed observations from the actual database state.
//
// The extractor performs read-only SQL queries. It does NOT commit, branch,
// merge, checkout, mutate tables, or invoke any Dolt version-control
// operations. It opens a connection, reads table schemas and row content,
// computes deterministic content hashes, and closes the connection.
//
// Observations produced:
//   - dolt:{db}:head — the HASHOF('HEAD') commit hash (substrate-specific;
//     included for audit identity but NOT required for cross-substrate
//     equivalence, because independent Dolt commits with identical data
//     produce different commit hashes due to timestamp inclusion).
//   - dolt:{db}:content_root — a deterministic hash of all table content
//     hashes combined. This IS the cross-substrate equivalence signal:
//     two databases with identical schema and data produce the same
//     content root hash regardless of commit history.
//   - dolt:{db}:schema:{table} — a deterministic hash of each table's
//     column definitions. Two databases with the same schema produce
//     the same schema hashes.
//   - dolt:{db}:table:{table} — a deterministic hash of each table's
//     row content. Two databases with the same row data produce the
//     same table content hash.
//
// This is the production-capable Dolt extractor that the SIAC framework
// requires for Gate 4 cross-substrate proof on the Dolt/app-state ledger.
type DoltStateExtractor struct {
	// WorkspacePath is the filesystem path to the Dolt workspace root
	// (the directory containing the .dolt folder). Must be non-empty.
	WorkspacePath string

	// Database is the Dolt database name within the workspace. Must be
	// non-empty. For the VM-embedded store this is typically "choir";
	// for the platform store it is typically "platform".
	Database string

	// CommitName is the Dolt commit author name used when opening the
	// connection. Defaults to "Choir" if empty. The extractor does not
	// commit, but the Dolt driver requires a commit name for the
	// connection.
	CommitName string

	// CommitEmail is the Dolt commit author email. Defaults to
	// "system@choir.local" if empty.
	CommitEmail string
}

var _ Extractor = DoltStateExtractor{}

// Extract connects to the embedded Dolt workspace, queries table schemas
// and row content, computes deterministic content hashes, and produces
// dolt_head observations for request.Version.
//
// All queries are read-only. The extractor does not mutate the database.
func (e DoltStateExtractor) Extract(ctx context.Context, request ExtractRequest) (ObservationSet, error) {
	if err := ctx.Err(); err != nil {
		return ObservationSet{}, err
	}
	if !request.Version.Valid() {
		return ObservationSet{}, fmt.Errorf("dolt state extraction: invalid computer version")
	}
	workspace := strings.TrimSpace(e.WorkspacePath)
	if workspace == "" {
		return ObservationSet{}, fmt.Errorf("dolt state extraction: workspace path is required")
	}
	database := strings.TrimSpace(e.Database)
	if database == "" {
		return ObservationSet{}, fmt.Errorf("dolt state extraction: database is required")
	}
	commitName := strings.TrimSpace(e.CommitName)
	if commitName == "" {
		commitName = "Choir"
	}
	commitEmail := strings.TrimSpace(e.CommitEmail)
	if commitEmail == "" {
		commitEmail = "system@choir.local"
	}

	db, connector, err := openDoltWorkspace(workspace, database, commitName, commitEmail)
	if err != nil {
		return ObservationSet{}, fmt.Errorf("dolt state extraction: %w", err)
	}
	defer func() {
		_ = db.Close()
		if connector != nil {
			_ = connector.Close()
		}
	}()

	// Query the HEAD commit hash for audit identity. This is NOT the
	// cross-substrate equivalence signal — independent Dolt commits with
	// identical data produce different commit hashes.
	var commitHash string
	if err := db.QueryRowContext(ctx, "SELECT HASHOF('HEAD')").Scan(&commitHash); err != nil {
		return ObservationSet{}, fmt.Errorf("dolt state extraction: query HEAD: %w", err)
	}
	commitHash = strings.TrimSpace(commitHash)

	// Query table names.
	tables, err := queryDoltTables(ctx, db)
	if err != nil {
		return ObservationSet{}, fmt.Errorf("dolt state extraction: %w", err)
	}

	observations := make([]Observation, 0, 1+len(tables)*2+1)

	// Head observation: commit hash for audit identity.
	headPayload := doltHeadPayload{
		Database:   database,
		CommitHash: commitHash,
		Derivation: "dolt-state-extractor live embedded Dolt HASHOF('HEAD')",
	}
	headValue, err := json.Marshal(headPayload)
	if err != nil {
		return ObservationSet{}, fmt.Errorf("dolt state extraction: encode head: %w", err)
	}
	observations = append(observations, Observation{
		Kind:  ObservationDoltHead,
		Key:   "dolt:" + database + ":head",
		Value: string(headValue),
	})

	// Schema and content observations for each table.
	tableContentHashes := make([]string, 0, len(tables))
	for _, table := range tables {
		if err := ctx.Err(); err != nil {
			return ObservationSet{}, err
		}

		// Schema hash.
		schemaHash, err := queryTableSchemaHash(ctx, db, table)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("dolt state extraction: schema for %s: %w", table, err)
		}
		observations = append(observations, Observation{
			Kind:  ObservationDoltHead,
			Key:   "dolt:" + database + ":schema:" + table,
			Value: schemaHash,
		})

		// Content hash.
		contentHash, err := queryTableContentHash(ctx, db, table)
		if err != nil {
			return ObservationSet{}, fmt.Errorf("dolt state extraction: content for %s: %w", table, err)
		}
		observations = append(observations, Observation{
			Kind:  ObservationDoltHead,
			Key:   "dolt:" + database + ":table:" + table,
			Value: contentHash,
		})
		tableContentHashes = append(tableContentHashes, contentHash)
	}

	// Content root hash: deterministic hash of all table content hashes.
	// This IS the cross-substrate equivalence signal.
	sort.Strings(tableContentHashes)
	rootHasher := sha256.New()
	for _, h := range tableContentHashes {
		rootHasher.Write([]byte(h))
		rootHasher.Write([]byte("\n"))
	}
	contentRoot := "sha256:" + hex.EncodeToString(rootHasher.Sum(nil))
	observations = append(observations, Observation{
		Kind:  ObservationDoltHead,
		Key:   "dolt:" + database + ":content_root",
		Value: contentRoot,
	})

	name := strings.TrimSpace(request.Name)
	if name == "" {
		name = "dolt-state"
	}
	return ObservationSet{
		Name:         name,
		Version:      request.Version,
		Required:     []ObservationKind{ObservationDoltHead},
		Observations: observations,
	}, nil
}

// DoltStateCapabilityManifest declares the observation scope for the Dolt
// state extractor. The extractor produces dolt_head observations by querying
// a live embedded Dolt database, so it supports that kind. It does not
// produce file_manifest, blob_set, or other observation kinds.
func DoltStateCapabilityManifest(materializer, substrate string) CapabilityManifest {
	materializer = strings.TrimSpace(materializer)
	if materializer == "" {
		materializer = DoltStateMaterializer
	}
	substrate = strings.TrimSpace(substrate)
	if substrate == "" {
		substrate = DoltStateSubstrate
	}
	return CapabilityManifest{
		Materializer: materializer,
		Substrate:    substrate,
		Supported:    []ObservationKind{ObservationDoltHead},
		Unsupported: []UnsupportedCapability{
			{Kind: ObservationFileManifest, Reason: "dolt extractor does not prove file manifest equivalence"},
			{Kind: ObservationBlobSet, Reason: "dolt extractor does not prove blob set equivalence"},
			{Kind: ObservationObjectGraphHead, Reason: "dolt extractor does not prove object graph head equivalence"},
			{Kind: ObservationProvenanceAnswer, Reason: "dolt extractor does not answer provenance queries"},
			{Kind: ObservationLiveProcessContinuity, Reason: "dolt extractor does not prove live-process continuity"},
			{Kind: ObservationVMStateManifest, Reason: "dolt extractor does not classify VM launch metadata"},
			{Kind: ObservationPromotionCertificate, Reason: "dolt extractor does not produce a promotion certificate"},
		},
	}
}

// openDoltWorkspace opens an embedded Dolt connection to the named database
// within the workspace. The connector is returned so the caller can close
// it after the database handle is closed.
func openDoltWorkspace(workspace, database, commitName, commitEmail string) (*sql.DB, interface{ Close() error }, error) {
	dsn := fmt.Sprintf(
		"file://%s?commitname=%s&commitemail=%s&database=%s&multistatements=true&clientfoundrows=true",
		workspace, commitName, commitEmail, database,
	)
	cfg, err := embedded.ParseDSN(dsn)
	if err != nil {
		return nil, nil, fmt.Errorf("parse dsn: %w", err)
	}
	connector, err := embedded.NewConnector(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("new connector: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, connector, nil
}

// queryDoltTables returns the sorted list of table names in the current
// database.
func queryDoltTables(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return nil, fmt.Errorf("show tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}
		tables = append(tables, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables: %w", err)
	}
	sort.Strings(tables)
	return tables, nil
}

// queryTableSchemaHash queries SHOW COLUMNS FROM <table> and produces a
// deterministic JSON encoding of the column definitions, sorted by field name.
func queryTableSchemaHash(ctx context.Context, db *sql.DB, table string) (string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SHOW COLUMNS FROM `%s`", table))
	if err != nil {
		return "", fmt.Errorf("show columns from %s: %w", table, err)
	}
	defer rows.Close()

	type columnDef struct {
		Field    string  `json:"field"`
		Type     string  `json:"type"`
		Null     string  `json:"null"`
		Key      string  `json:"key,omitempty"`
		Default  *string `json:"default,omitempty"`
		Extra    string  `json:"extra,omitempty"`
	}

	columns := make([]columnDef, 0)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		var col columnDef
		var defaultVal sql.NullString
		if err := rows.Scan(&col.Field, &col.Type, &col.Null, &col.Key, &defaultVal, &col.Extra); err != nil {
			return "", fmt.Errorf("scan column from %s: %w", table, err)
		}
		if defaultVal.Valid {
			s := defaultVal.String
			col.Default = &s
		}
		columns = append(columns, col)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate columns from %s: %w", table, err)
	}

	sort.Slice(columns, func(i, j int) bool { return columns[i].Field < columns[j].Field })

	encoded, err := json.Marshal(columns)
	if err != nil {
		return "", fmt.Errorf("encode schema for %s: %w", table, err)
	}
	return string(encoded), nil
}

// queryTableContentHash queries all rows from the table, encodes them as
// deterministic JSON, and returns a content-addressed hash. Rows are sorted
// by their JSON encoding to ensure determinism regardless of insertion order.
func queryTableContentHash(ctx context.Context, db *sql.DB, table string) (string, error) {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("SELECT * FROM `%s`", table))
	if err != nil {
		return "", fmt.Errorf("select from %s: %w", table, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("columns from %s: %w", table, err)
	}

	rowJSONs := make([]string, 0)
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		values := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return "", fmt.Errorf("scan row from %s: %w", table, err)
		}

		// Encode each row as a deterministic JSON object keyed by column name.
		obj := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			val := values[i]
			// Convert []byte to string for stable encoding.
			if b, ok := val.([]byte); ok {
				obj[col] = string(b)
			} else {
				obj[col] = val
			}
		}
		encoded, err := json.Marshal(obj)
		if err != nil {
			return "", fmt.Errorf("encode row from %s: %w", table, err)
		}
		rowJSONs = append(rowJSONs, string(encoded))
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate rows from %s: %w", table, err)
	}

	// Sort row JSONs for determinism regardless of insertion order.
	sort.Strings(rowJSONs)

	hasher := sha256.New()
	for _, rj := range rowJSONs {
		hasher.Write([]byte(rj))
		hasher.Write([]byte("\n"))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil)), nil
}
