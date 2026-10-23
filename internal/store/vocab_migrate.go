package store

// Vocabulary migration for mission-2 (drill and cutover machinery).
//
// MigrateVocabularyToV2 rewrites every object-graph object and edge under
// the frozen §3 map; RevertVocabularyToV1 restores them via retained
// provenance (INV-PROV exact) or the canonical representative (INV-CANON).
// The drill test exercises migrate→verify→revert→verify→re-migrate→verify→
// revert; the cutover calls the same functions once, forward only. The
// legacy relational role columns are gone with their tables (Cut 1 L2, 6c).
//
// Deliberately out of drill round 1 (recorded cutover obligations):
// prompt/profile registry tags, computer-owned TOML overlays, grant
// attestation rows, digest-domain succession (§6), and bare-compound ID
// suffixes (e.g. run-cosuper style) beyond the prefix/infix rules below.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

// MigrationReport retains per-object source tokens for INV-PROV revert plus
// per-kind migration counts. It is JSON-serializable so the cutover can
// persist it durably across restarts.
type MigrationReport struct {
	Counts map[string]int64 `json:"counts"`
	// FencedAt marks that the serving fence verified this store after the
	// recorded migration completed. A report without it means a prior run
	// crashed between apply and verify; the next serving transition must
	// re-run the full path. When set, a boot that replayed zero events may
	// skip the rescan: replay is the only unguarded writer, and the write
	// guard has refused non-V2 rows since the report existed.
	FencedAt string `json:"fenced_at,omitempty"`
	// ReplayOpenedAt marks that replay wrote this fenced store after the
	// fence. It is persisted before the first replayed deposit commits and
	// cleared only when the fence is stamped again, so a replay that spans
	// processes (resume quantum, crash) is never trusted on the fast path
	// (problems/s0-vocab-rescan-fires-on-any-replay-2026-10-01.md).
	ReplayOpenedAt string `json:"replay_opened_at,omitempty"`
	// OGObjects records every migrated og_objects row for INV-PROV revert:
	// the original canonical ID plus every changed leaf's original value.
	// Revert restores recorded leaves exactly, recomputes content_hash and
	// canonical_id, and rewrites references through the recorded ID map.
	OGObjects []OGProvEntry `json:"og_objects,omitempty"`
	// OGDropped records canonical IDs of rows deleted by convergence dedup:
	// distinct V1 objects that migrated to identical content and identity.
	// Revert cannot restore them (no row remains); the entry preserves the
	// audit trail that they existed and were folded into the survivor.
	OGDropped []string `json:"og_dropped,omitempty"`
	// OGSuperseded records stale rows deleted because their migration
	// target was already held by a live row updated no earlier than them
	// (replay's pre-cutover V1 copy of an object the live computer kept
	// updating under its V2 identity). The original body and metadata are
	// kept so the superseded content is never silently lost.
	OGSuperseded []OGSupersededEntry `json:"og_superseded,omitempty"`
	// OGEdges records every rewritten og_edges row.
	OGEdges []OGEdgeProvEntry `json:"og_edges,omitempty"`
	// DepositUpcastVersion records the projection-deposit upcast version
	// (vocabmigrate.DepositUpcastVersion) that already transformed this
	// store's replayed deposits to the active V2 vocabulary before this
	// report was planned. It is 0 when the end-of-replay migration itself
	// performed the vocabulary transformation (a retained V1 store), so a
	// consumer of a published base can see which compatibility path produced
	// the projection.
	DepositUpcastVersion int `json:"deposit_upcast_version,omitempty"`
	// DepositUpcastAliases counts the old→new identity aliases the deposit
	// upcaster recorded in the workspace ledger.
	DepositUpcastAliases int `json:"deposit_upcast_aliases,omitempty"`
}

// OGSupersededEntry retains one superseded og_objects row's pre-migration
// content and the live row that replaced it.
type OGSupersededEntry struct {
	OldCanonicalID      string `json:"old_canonical_id"`
	SurvivorCanonicalID string `json:"survivor_canonical_id"`
	UpdatedAt           string `json:"updated_at"`
	Body                []byte `json:"body"`
	Metadata            []byte `json:"metadata"`
}

// OGProvEntry retains one migrated og_objects row's inverse record.
type OGProvEntry struct {
	OldCanonicalID string `json:"old_canonical_id"`
	NewCanonicalID string `json:"new_canonical_id"`
	// Fields maps a JSON path ("metadata.agent_id", "body.binding.role")
	// to the leaf's exact pre-migration value.
	Fields map[string]string `json:"fields"`
}

// OGEdgeProvEntry retains one rewritten og_edges row's inverse record.
type OGEdgeProvEntry struct {
	OldEdgeID string `json:"old_edge_id"`
	NewEdgeID string `json:"new_edge_id"`
	OldFrom   string `json:"old_from"`
	OldTo     string `json:"old_to"`
}

// migrateIDForward rewrites desk-bearing ID prefixes and hyphen-delimited
// desk infixes (run-super-assignment / work-super-assignment style per the
// charter tie-break). Command identities are never passed in: callers only
// supply agent/address/constructed-ID columns.
func migrateIDForward(id string) (string, bool) {
	for _, prefix := range []string{"super:", "co-super:", "cosuper:", "researcher:"} {
		if strings.HasPrefix(id, prefix) {
			v1tok := strings.TrimSuffix(prefix, ":")
			if v2, ok := vocabmigrate.ForwardV1ToV2(v1tok); ok {
				return v2 + ":" + strings.TrimPrefix(id, prefix), true
			}
			return id, false
		}
	}
	// Longest-token-first: "-super-" is a substring of "-Engineering-", so a
	// shorter infix must never match before a longer one containing it.
	for _, pair := range [][2]string{
		{"-co-super-", "-engineering-"}, {"-cosuper-", "-engineering-"},
		{"-researcher-", "-research-"}, {"-super-", "-management-"},
	} {
		if strings.Contains(id, pair[0]) {
			return strings.ReplaceAll(id, pair[0], pair[1]), true
		}
	}
	return id, false
}

// MigrateVocabularyToV2 forward-migrates every object-graph object and edge
// under the frozen map. Callers that need provenance durable before mutation
// (the cutover path) use the plan/persist/apply sequence inside
// MigrateAndFenceServingVocabulary.
func (s *Store) MigrateVocabularyToV2(ctx context.Context) (*MigrationReport, error) {
	rep := &MigrationReport{Counts: map[string]int64{}}
	// Replayed deposits may already have been upcast; their recorded aliases
	// resolve references still written against legacy IDs.
	seed := map[string]string{}
	depositLedger, err := s.depositUpcastSeed(seed)
	if err != nil {
		return nil, err
	}
	objs, edges, err := s.planOGMigration(ctx, rep, seed, nil)
	if err != nil {
		return nil, err
	}
	if err := s.applyOGMigration(ctx, objs, edges); err != nil {
		return nil, err
	}
	if depositLedger != nil {
		rep.DepositUpcastVersion = depositLedger.Version
		rep.DepositUpcastAliases = len(depositLedger.Objects) + len(depositLedger.Edges)
	}
	s.vocabCutover.Store(true)
	return rep, nil
}

// RevertVocabularyToV1 restores the migrated object-graph objects and edges:
// recorded leaves return to their exact original tokens (INV-PROV), the rest
// to the canonical V1 representatives (INV-CANON). With a complete
// provenance log the revert is byte-identical; without it, many-to-one
// spellings revert canonically.
func (s *Store) RevertVocabularyToV1(ctx context.Context, rep *MigrationReport) error {
	if err := s.revertOGMigration(ctx, rep); err != nil {
		return err
	}
	// Revert writes run outside the event paths; the next checkpoint must be
	// able to commit them (a no-op commit is tolerated by commitDoltCheckpoint).
	s.markDoltHistoryDirty()
	s.vocabCutover.Store(false)
	return nil
}

// servingRoleFields collects every distinct role-bearing value in the store
// for the serving fence. progress, when non-nil, is ticked once per scanned
// row so callers can expose liveness.
func (s *Store) servingRoleFields(ctx context.Context, progress func()) ([]vocabmigrate.Field, error) {
	return s.servingOGRoleFields(ctx, progress)
}

// VerifyServingVocabularyV2 refuses when any role-bearing value in the store
// falls outside the active V2 vocabulary (frozen protocol always passes).
func (s *Store) VerifyServingVocabularyV2(ctx context.Context, progress func()) error {
	fields, err := s.servingRoleFields(ctx, progress)
	if err != nil {
		return err
	}
	return vocabmigrate.VerifyServingVocabulary(vocabmigrate.VocabularyV2, fields...)
}

// MigrateAndFenceServingVocabulary is the cutover transition: plan the
// forward migration, persist the merged provenance report BEFORE any row
// mutates (crash-atomicity: an interrupted run leaves correct provenance
// for every planned change, and re-planning over the same input rows is
// deterministic so a restart converges), apply the writes, then hold the
// serving fence closed until every row class agrees with the active V2
// vocabulary. It is idempotent and safe to run at every serving transition
// (post-replay boot, rematerialize flip, base publish); a second run only
// picks up rows written since the last pass.
//
// replayed reports whether the tape replay applied events this boot. When
// zero events were applied and a previously fenced report exists, the full
// rescan is skipped: replay is the only writer that bypasses the write
// guard, so a fenced store with no replayed events cannot contain non-V2
// rows. When this process replayed onto a fenced store, only the replayed
// rows are checked (vocab_replay_fence.go); a replay an earlier process
// opened always takes the full pass. progress, when non-nil, is ticked once per scanned row so callers
// can expose liveness during long scans (the guest health gate reports it
// to the host stall detector).
func (s *Store) MigrateAndFenceServingVocabulary(ctx context.Context, replayed bool, progress func()) (*MigrationReport, error) {
	prior, err := s.loadVocabMigrationReport()
	if err != nil {
		return nil, err
	}
	if !replayed && prior != nil && prior.FencedAt != "" && prior.ReplayOpenedAt == "" {
		// Fast path: nothing replayed and the fence already verified this
		// store. The write guard has refused non-V2 rows since the report
		// existed, so the serving vocabulary is still V2. A replay marker
		// means an earlier process replayed rows after the fence and never
		// re-fenced them: the full pass decides.
		s.vocabCutover.Store(true)
		return prior, nil
	}
	if replayed && prior != nil && prior.FencedAt != "" {
		// Narrowed path: check only the rows this process replayed
		// (vocab_replay_fence.go). Anything it cannot prove takes the full
		// pass below.
		stays, err := s.vocabReplayStaysFenced(ctx, prior, progress)
		if err != nil {
			return nil, err
		}
		if stays {
			prior.FencedAt = time.Now().UTC().Format(time.RFC3339Nano)
			prior.ReplayOpenedAt = ""
			if err := s.persistVocabMigrationReport(prior); err != nil {
				return nil, err
			}
			s.vocabReplay.reset()
			s.vocabCutover.Store(true)
			return prior, nil
		}
	}
	rep := &MigrationReport{Counts: map[string]int64{}}
	// OG planning seeds its reference map from the persisted report so a
	// re-run after a mid-apply crash still resolves references to objects
	// already migrated under their new IDs. The deposit upcast ledger adds
	// every identity replayed deposits already renamed, so references
	// recorded against those legacy IDs still resolve. On a store whose
	// deposits were upcast this pass is verify-only (plus that straggler
	// rewrite); on a retained V1 store it remains the one-time migration.
	seed := ogSeedFromReport(prior)
	depositLedger, err := s.depositUpcastSeed(seed)
	if err != nil {
		return nil, err
	}
	objs, edges, err := s.planOGMigration(ctx, rep, seed, progress)
	if err != nil {
		return nil, err
	}
	if prior != nil {
		mergeVocabReports(prior, rep)
		rep = prior
	}
	if depositLedger != nil {
		rep.DepositUpcastVersion = depositLedger.Version
		rep.DepositUpcastAliases = len(depositLedger.Objects) + len(depositLedger.Edges)
	}
	// Provenance is durable before the first mutation: a crash after this
	// point can never lose or counterfeit the inverse record. A crash before
	// it leaves rows unmigrated and the prior report intact.
	if err := s.persistVocabMigrationReport(rep); err != nil {
		return nil, err
	}
	if err := s.applyOGMigration(ctx, objs, edges); err != nil {
		return nil, err
	}
	if err := s.VerifyServingVocabularyV2(ctx, progress); err != nil {
		return nil, fmt.Errorf("vocab serving fence: %w", err)
	}
	// The fence verified: stamp the report so a zero-replay boot can skip
	// the rescan, and persist the stamp before opening the gate.
	rep.FencedAt = time.Now().UTC().Format(time.RFC3339Nano)
	rep.ReplayOpenedAt = ""
	if err := s.persistVocabMigrationReport(rep); err != nil {
		return nil, err
	}
	s.vocabReplay.reset()
	s.vocabCutover.Store(true)
	return rep, nil
}

// mergeVocabReports folds a fresh migration report into the persisted one.
// Entries whose post-migration identity collides with a prior entry are
// dropped: the prior entry retains the original V1 token, which is the only
// correct INV-PROV source for that object.
func mergeVocabReports(dst, src *MigrationReport) {
	if dst.Counts == nil {
		dst.Counts = map[string]int64{}
	}
	for key, n := range src.Counts {
		dst.Counts[key] += n
	}
	// OG entries dedupe on the ORIGINAL canonical/edge ID: a re-run over a
	// partially migrated store re-plans the same rows under their current
	// (already migrated) IDs, so the prior entry — keyed by the original —
	// is the only correct INV-PROV source.
	seenObj := map[string]bool{}
	for _, e := range dst.OGObjects {
		seenObj[e.OldCanonicalID] = true
		seenObj[e.NewCanonicalID] = true
	}
	for _, e := range src.OGObjects {
		if !seenObj[e.OldCanonicalID] {
			dst.OGObjects = append(dst.OGObjects, e)
		}
	}
	seenEdge := map[string]bool{}
	for _, e := range dst.OGEdges {
		seenEdge[e.OldEdgeID] = true
		seenEdge[e.NewEdgeID] = true
	}
	for _, e := range src.OGEdges {
		if !seenEdge[e.OldEdgeID] {
			dst.OGEdges = append(dst.OGEdges, e)
		}
	}
	seenDropped := map[string]bool{}
	for _, id := range dst.OGDropped {
		seenDropped[id] = true
	}
	for _, id := range src.OGDropped {
		if !seenDropped[id] {
			dst.OGDropped = append(dst.OGDropped, id)
		}
	}
	seenSup := map[string]bool{}
	for _, e := range dst.OGSuperseded {
		seenSup[e.OldCanonicalID] = true
	}
	for _, e := range src.OGSuperseded {
		if !seenSup[e.OldCanonicalID] {
			dst.OGSuperseded = append(dst.OGSuperseded, e)
		}
	}
}

// vocabReportFileName is the sidecar carrying the migration report inside the
// Dolt workspace directory. It lives outside the database so the witness
// extractor never observes it (a report table would change the schema hash
// and break live↔replay witness equality), while still traveling with the
// store through rematerialize flips and projection-base blobs.
const vocabReportFileName = "vocab-migration-report.json"

func (s *Store) vocabReportPath() string {
	return filepath.Join(s.texturePath, vocabReportFileName)
}

func (s *Store) persistVocabMigrationReport(rep *MigrationReport) error {
	raw, err := json.Marshal(rep)
	if err != nil {
		return fmt.Errorf("vocab migrate report encode: %w", err)
	}
	path := s.vocabReportPath()
	tmp := path + ".tmp"
	// The report must be on disk, not only in the page cache, before the
	// mutation or deposit it precedes commits: provenance before apply, and
	// the replay marker before the first replayed deposit
	// (vocab_replay_fence.go).
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("vocab migrate report persist: %w", err)
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("vocab migrate report persist: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return fmt.Errorf("vocab migrate report persist: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("vocab migrate report persist: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("vocab migrate report persist: %w", err)
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("vocab migrate report persist: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("vocab migrate report persist: %w", err)
	}
	return nil
}

// armVocabFenceFromReport arms the vocabulary write fence at open when the
// workspace has cut over. A readable sidecar arms it; a missing one means the
// store never cut over and leaves it off. A sidecar that exists but cannot be
// read or decoded fails closed: the fence is armed and the error is logged,
// and the open is not refused, because a store that cannot open cannot reach
// the health listener and the operator has no surface to repair it from. The
// serving migration (MigrateAndFenceServingVocabulary) still returns the load
// error to its callers, which is the visible stop
// (docs/problems/selfdev-operations-replay-not-equivalent-2026-10-23.md,
// section "B. A corrupt vocabulary sidecar must not leave the fence off").
func (s *Store) armVocabFenceFromReport() {
	rep, err := s.loadVocabMigrationReport()
	if err != nil {
		log.Printf("runtime store: vocabulary migration sidecar %s is unreadable, opening with the write fence armed (fail closed): %v", s.vocabReportPath(), err)
		s.vocabCutover.Store(true)
		return
	}
	if rep != nil {
		s.vocabCutover.Store(true)
	}
}

func (s *Store) loadVocabMigrationReport() (*MigrationReport, error) {
	raw, err := os.ReadFile(s.vocabReportPath())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vocab migrate report load: %w", err)
	}
	rep := &MigrationReport{}
	if err := json.Unmarshal(raw, rep); err != nil {
		return nil, fmt.Errorf("vocab migrate report decode: %w", err)
	}
	return rep, nil
}
