package selfdevprotocol

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
)

// Witness v2 is the hash-only checkpoint bind
// (docs/problems/paper-never-checkpointed-2026-10-27.md, S3). A v1 witness
// hashes every row of the store; the guest computes it with a full SELECT *
// under the append hold, which is seconds on d@e and minutes on the paper. A
// v2 witness carries Dolt's own per-table content hashes (DOLT_HASHOF_TABLE),
// read in milliseconds under the hold. Dolt's prolly trees are history-
// independent, so a tape-replayed store and the live store with the same rows,
// schema and indexes have the same table hash: measured on the checkpoint
// fixture, 24 of 26 tables match (TestCheckpointBindV2WitnessEqualsTapeReplay).
//
// The two that cannot match by construction carry wall-clock columns a replay
// writes afresh (computer_event_index prepared_at/finalized_at,
// computer_event_projection_heads updated_at; the v1 witness ignores those
// columns). A v2 witness names them as operational tables and leaves them out
// of its hashes: they are the tape's own bookkeeping, which the head H the
// checkpoint is published at already commits to. There is no row fallback: a
// v2 witness is judged by table hashes alone, and a restore rebuilds both
// tables from the tape whatever the live copies held.
//
// Fail closed on old readers: a v2 witness has no v1 schema or table maps, so
// a reader without v2 fails its Validate (the platform refuses the publish,
// checkpointd judges invalid_witness).

const (
	// WitnessVersionRows is the v1 row witness (the zero value).
	WitnessVersionRows = 1
	// WitnessVersionDoltHashes is the v2 hash-only witness.
	WitnessVersionDoltHashes = 2
)

// VerdictReasonWitnessVersion: the host's base has no witness of the
// checkpoint's version (a base recorded before v2 has no Dolt table hashes),
// so it cannot judge.
const VerdictReasonWitnessVersion = "witness_version"

func (w VMLocalContentWitness) version() int {
	if w.WitnessVersion == 0 {
		return WitnessVersionRows
	}
	return w.WitnessVersion
}

// IsDoltHashWitness reports a v2 witness.
func (w VMLocalContentWitness) IsDoltHashWitness() bool {
	return w.WitnessVersion == WitnessVersionDoltHashes
}

// WitnessV2FromDoltHashes seals a v2 witness from per-table Dolt hashes.
// operational names the tables left out by construction; a hash for one of
// them is refused, so neither side can compare a wall-clock table.
func WitnessV2FromDoltHashes(database, doltHead string, tables map[string]string, operational []string) (VMLocalContentWitness, error) {
	ops := append([]string(nil), operational...)
	sort.Strings(ops)
	w := VMLocalContentWitness{
		WitnessVersion:    WitnessVersionDoltHashes,
		Database:          strings.TrimSpace(database),
		DoltHead:          strings.TrimSpace(doltHead),
		DoltTables:        map[string]string{},
		OperationalTables: ops,
	}
	for table, h := range tables {
		w.DoltTables[table] = h
	}
	w.ContentRoot = doltTablesContentRoot(w.DoltTables)
	digest, err := witnessV2DerivabilityDigest(w)
	if err != nil {
		return VMLocalContentWitness{}, err
	}
	w.DerivabilityDigest = digest
	if err := w.Validate(); err != nil {
		return VMLocalContentWitness{}, err
	}
	return w, nil
}

func (w VMLocalContentWitness) validateV2() error {
	database := strings.TrimSpace(w.Database)
	if database == "" {
		return fmt.Errorf("self-development checkpoint: VM-local content witness is required")
	}
	if _, banned := restoreOutOfScopeDatabases[database]; banned {
		return fmt.Errorf("self-development checkpoint: %s store is out of restore scope", database)
	}
	if len(w.Schema) != 0 || len(w.Tables) != 0 {
		return fmt.Errorf("self-development checkpoint: a v2 witness carries no row hashes")
	}
	if strings.TrimSpace(w.DoltHead) == "" || len(w.DoltTables) == 0 {
		return fmt.Errorf("self-development checkpoint: v2 witness head and table hashes are required")
	}
	if err := validateDoltTables(w.DoltTables); err != nil {
		return err
	}
	for _, table := range w.OperationalTables {
		if _, hashed := w.DoltTables[table]; hashed {
			return fmt.Errorf("self-development checkpoint: v2 witness hashes operational table %s", table)
		}
	}
	if w.ContentRoot != doltTablesContentRoot(w.DoltTables) {
		return fmt.Errorf("self-development checkpoint: v2 witness content root does not match its table hashes")
	}
	digest, err := witnessV2DerivabilityDigest(w)
	if err != nil {
		return err
	}
	if w.DerivabilityDigest != digest {
		return fmt.Errorf("self-development checkpoint: v2 witness derivability digest does not match")
	}
	return nil
}

func validateDoltTables(tables map[string]string) error {
	for table, h := range tables {
		if strings.TrimSpace(table) == "" || !isDoltHash(h) {
			return fmt.Errorf("self-development checkpoint: Dolt table hash is incomplete")
		}
	}
	return nil
}

// isDoltHash reports Dolt's hash.String form: 32 characters of base32hex.
func isDoltHash(h string) bool {
	if len(h) != 32 {
		return false
	}
	for _, c := range h {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'v') {
			return false
		}
	}
	return true
}

func doltTablesContentRoot(tables map[string]string) string {
	names := make([]string, 0, len(tables))
	for table := range tables {
		names = append(names, table)
	}
	sort.Strings(names)
	hasher := sha256.New()
	for _, table := range names {
		hasher.Write([]byte(table + " " + tables[table] + "\n"))
	}
	return "sha256:" + hex.EncodeToString(hasher.Sum(nil))
}

func witnessV2DerivabilityDigest(w VMLocalContentWitness) (string, error) {
	proof, err := computerevent.CanonicalJSON(struct {
		Version     int               `json:"witness_version"`
		Database    string            `json:"database"`
		ContentRoot string            `json:"content_root"`
		Tables      map[string]string `json:"dolt_tables"`
		Operational []string          `json:"operational_tables"`
	}{w.WitnessVersion, w.Database, w.ContentRoot, w.DoltTables, w.OperationalTables})
	if err != nil {
		return "", err
	}
	return computerevent.DigestBytes(proof), nil
}

// doltTablesMatch compares the Dolt table hashes of two witnesses.
func doltTablesMatch(got, want VMLocalContentWitness) error {
	if got.Database != want.Database {
		return fmt.Errorf("rematerialize: reconstructed database does not match checkpoint")
	}
	if tables := differingDoltTables(got.DoltTables, want.DoltTables); len(tables) > 0 {
		return fmt.Errorf("rematerialize: reconstructed table %s does not match checkpoint", strings.Join(tables, ", "))
	}
	return nil
}

func differingDoltTables(a, b map[string]string) []string {
	seen := map[string]struct{}{}
	for table := range a {
		seen[table] = struct{}{}
	}
	for table := range b {
		seen[table] = struct{}{}
	}
	var out []string
	for table := range seen {
		av, aok := a[table]
		bv, bok := b[table]
		if !aok || !bok || av != bv {
			out = append(out, table)
		}
	}
	sort.Strings(out)
	if len(out) > maxVerdictTables {
		out = out[:maxVerdictTables]
	}
	return out
}

// judgeCheckpointWitnessV2 is JudgeCheckpointWitness for a v2 checkpoint: the
// rebuild's Dolt table hashes against the checkpoint's, with the same
// deposit-mode rule as v1.
func judgeCheckpointWitnessV2(depositMode string, checkpoint, rebuilt VMLocalContentWitness) CheckpointVerdict {
	if err := checkpoint.Validate(); err != nil {
		return CheckpointVerdict{Status: CheckpointVerdictUnverified, Reason: VerdictReasonInvalidWitness, Detail: "checkpoint witness: " + err.Error()}
	}
	if err := rebuilt.Validate(); err != nil {
		return CheckpointVerdict{Status: CheckpointVerdictUnverified, Reason: VerdictReasonInvalidWitness, Detail: "rebuilt witness: " + err.Error()}
	}
	if len(rebuilt.DoltTables) == 0 {
		return CheckpointVerdict{Status: CheckpointVerdictUnverified, Reason: VerdictReasonWitnessVersion, Detail: "the host's base records no Dolt table hashes"}
	}
	err := doltTablesMatch(rebuilt, checkpoint)
	if err == nil {
		return CheckpointVerdict{Status: CheckpointVerdictVerified}
	}
	verdict := CheckpointVerdict{DifferingTables: differingDoltTables(checkpoint.DoltTables, rebuilt.DoltTables), Detail: err.Error()}
	if depositMode == DepositModeUpcast {
		verdict.Status, verdict.Reason = CheckpointVerdictNotDerivable, VerdictReasonWitnessMismatch
	} else {
		verdict.Status, verdict.Reason = CheckpointVerdictUnverified, VerdictReasonBaseUpcast
	}
	return verdict
}
