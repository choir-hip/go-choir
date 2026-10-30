package selfdevprotocol

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/computerversion"
)

// Witness v3 is the content witness of the guest SQLite store
// (docs/design/data-system-build-sheet-2026-10-30.md, decision 6). It carries
// a SHA-256 per table, over the table's rows in primary-key order in a
// canonical row encoding, and one schema hash over sqlite_master. Both the
// live root and the replay root are computed by
// computerversion.SQLiteStateExtractor, so a tape-replayed store and the live
// store with the same rows and schema have the same witness.
//
// Versions 1 (row hashes) and 2 (Dolt table hashes) were witnesses of the
// retired Dolt store. Their bases are deleted at the cutover, so they are
// unjudgeable: every reader refuses them with a clear reason.

const (
	// WitnessVersionSQLite is the only witness version this release reads.
	WitnessVersionSQLite = 3
)

// VerdictReasonWitnessVersion: the checkpoint's witness is not version 3, so
// the host cannot judge it.
const VerdictReasonWitnessVersion = "witness_version"

func (w VMLocalContentWitness) version() int { return w.WitnessVersion }

// RefuseWitnessVersion explains why a witness of another version cannot be
// judged, or returns nil for version 3.
func RefuseWitnessVersion(version int) error {
	if version == WitnessVersionSQLite {
		return nil
	}
	return fmt.Errorf("self-development checkpoint: VM-local content witness version %d is not judgeable: this release reads witness version %d (SQLite store); versions 1 and 2 are witnesses of the retired Dolt store", version, WitnessVersionSQLite)
}

// NewWitnessV3 seals a v3 witness from the store's schema hash and per-table
// digests (computerversion.StateDigests).
func NewWitnessV3(database, schemaHash string, tables map[string]string) (VMLocalContentWitness, error) {
	w := VMLocalContentWitness{
		WitnessVersion: WitnessVersionSQLite,
		Database:       strings.TrimSpace(database),
		SchemaHash:     schemaHash,
		Tables:         map[string]string{},
	}
	for table, digest := range tables {
		w.Tables[table] = digest
	}
	w.ContentRoot = computerversion.WitnessV3ContentRoot(w.SchemaHash, w.Tables)
	digest, err := witnessDerivabilityDigest(w)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	w.DerivabilityDigest = digest
	if err := w.Validate(); err != nil {
		return VMLocalContentWitness{}, err
	}
	return w, nil
}

// WitnessFromDigests seals a v3 witness from an extraction.
func WitnessFromDigests(d computerversion.StateDigests) (VMLocalContentWitness, error) {
	return NewWitnessV3(d.Database, d.SchemaHash, d.Tables)
}

func (w VMLocalContentWitness) validateV3() error {
	database := strings.TrimSpace(w.Database)
	if database == "" {
		return fmt.Errorf("self-development checkpoint: VM-local content witness is required")
	}
	if _, banned := restoreOutOfScopeDatabases[database]; banned {
		return fmt.Errorf("self-development checkpoint: %s store is out of restore scope", database)
	}
	if !computerevent.IsSHA256(w.SchemaHash) || !computerevent.IsSHA256(w.ContentRoot) || !computerevent.IsSHA256(w.DerivabilityDigest) {
		return fmt.Errorf("self-development checkpoint: VM-local content witness digests are required")
	}
	if len(w.Tables) == 0 {
		return fmt.Errorf("self-development checkpoint: VM-local table digests are required")
	}
	for table, digest := range w.Tables {
		if strings.TrimSpace(table) == "" || !computerevent.IsSHA256(digest) {
			return fmt.Errorf("self-development checkpoint: VM-local table digest is incomplete")
		}
	}
	if w.ContentRoot != computerversion.WitnessV3ContentRoot(w.SchemaHash, w.Tables) {
		return fmt.Errorf("self-development checkpoint: witness content root does not match its schema and table digests")
	}
	digest, err := witnessDerivabilityDigest(w)
	if err != nil {
		return err
	}
	if w.DerivabilityDigest != digest {
		return fmt.Errorf("self-development checkpoint: witness derivability digest does not match")
	}
	return nil
}

func witnessDerivabilityDigest(w VMLocalContentWitness) (string, error) {
	proof, err := computerevent.CanonicalJSON(struct {
		Version     int               `json:"witness_version"`
		Database    string            `json:"database"`
		ContentRoot string            `json:"content_root"`
		SchemaHash  string            `json:"schema_hash"`
		Tables      map[string]string `json:"tables"`
	}{w.WitnessVersion, w.Database, w.ContentRoot, w.SchemaHash, w.Tables})
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(proof), nil
}

// schemaPseudoTable names a schema difference in a verdict's differing list.
const schemaPseudoTable = "<schema>"

// differingTables names the tables whose digest differs between two
// witnesses, or that only one has, plus the schema when its hash differs;
// sorted and bounded.
func differingTables(a, b VMLocalContentWitness) []string {
	seen := map[string]struct{}{}
	for table := range a.Tables {
		seen[table] = struct{}{}
	}
	for table := range b.Tables {
		seen[table] = struct{}{}
	}
	var out []string
	for table := range seen {
		av, aok := a.Tables[table]
		bv, bok := b.Tables[table]
		if !aok || !bok || av != bv {
			out = append(out, table)
		}
	}
	if a.SchemaHash != b.SchemaHash {
		out = append(out, schemaPseudoTable)
	}
	sort.Strings(out)
	if len(out) > maxVerdictTables {
		out = out[:maxVerdictTables]
	}
	return out
}
