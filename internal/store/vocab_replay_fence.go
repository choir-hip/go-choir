package store

// Narrowed serving fence for a boot that replayed onto a fenced store
// (problems/s0-vocab-rescan-fires-on-any-replay-2026-10-01.md, 2026-10-16).
//
// Replay is the one writer the OG write guard does not see, so a fenced store
// that replayed rows used to pay the whole-table migration and fence: 3 min
// 44 s on the paper for a one-event tail. Instead, the replay path records
// the object and edge IDs it writes, and the boot fence re-reads exactly
// those rows. If none of them would change under the migration plan and their
// role fields pass the fence, the plan's fixpoint is unchanged and the store
// is still fenced. Anything the narrowed check cannot prove falls back to the
// full pass.
//
// The first replayed deposit on a fenced store durably marks the report
// (ReplayOpenedAt) before it commits. Only the process that set the marker
// may take the narrowed path, so rows a crashed or resumed earlier process
// replayed are always re-checked by the full pass, on a replaying boot and on
// a zero-replay boot alike.
//
// Rows written live since the fence are trusted exactly as the zero-replay
// fast path trusts them: the write guard checked their role fields.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

// vocabReplayTrackLimit bounds the replayed IDs a process keeps in memory.
// A longer replay falls back to the full pass, whose cost it already pays in
// replay time.
const vocabReplayTrackLimit = 20000

// vocabReplayTrack is this process's record of replay since the last fence.
type vocabReplayTrack struct {
	mu sync.Mutex
	// finalized counts replayed events this store finalized.
	finalized int
	// opened: this process marked the fenced report. foreign: the report
	// already carried a marker from another process. unfenced: the store had
	// no fenced report when replay began.
	opened, foreign, unfenced bool
	// overflow: more IDs than vocabReplayTrackLimit; the sets were dropped.
	overflow bool
	objects  map[string]struct{}
	edges    map[string]struct{}
}

func (t *vocabReplayTrack) reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.finalized = 0
	t.opened, t.foreign, t.unfenced, t.overflow = false, false, false, false
	t.objects, t.edges = nil, nil
}

// noteVocabReplayDeposit records one replayed event's deposit after the
// upcast and before it commits. The first call in a process marks a fenced
// report durably, so a crash after the commit leaves the marker behind.
func (s *Store) noteVocabReplayDeposit(batch *computerevent.ProjectionBatch) error {
	t := &s.vocabReplay
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.finalized == 0 {
		rep, err := s.loadVocabMigrationReport()
		if err != nil {
			return err
		}
		switch {
		case rep == nil || rep.FencedAt == "":
			t.unfenced = true
		case rep.ReplayOpenedAt != "":
			t.foreign = true
		default:
			rep.ReplayOpenedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := s.persistVocabMigrationReport(rep); err != nil {
				return err
			}
			t.opened = true
		}
	}
	t.finalized++
	if batch == nil || t.overflow || !t.opened {
		return nil
	}
	if t.objects == nil {
		t.objects, t.edges = map[string]struct{}{}, map[string]struct{}{}
	}
	for _, op := range batch.Ops {
		var ids struct {
			CanonicalID string `json:"canonical_id"`
			EdgeID      string `json:"edge_id"`
		}
		switch strings.TrimSpace(op.Kind) {
		case computerevent.ProjectionOpObject:
			if err := json.Unmarshal(op.Body, &ids); err != nil || ids.CanonicalID == "" {
				t.overflow = true
			} else {
				t.objects[ids.CanonicalID] = struct{}{}
			}
		case computerevent.ProjectionOpObjectEdge:
			if err := json.Unmarshal(op.Body, &ids); err != nil || ids.EdgeID == "" {
				t.overflow = true
			} else {
				t.edges[ids.EdgeID] = struct{}{}
			}
		}
		if t.overflow || len(t.objects)+len(t.edges) > vocabReplayTrackLimit {
			t.overflow = true
			t.objects, t.edges = nil, nil
			return nil
		}
	}
	return nil
}

// vocabReplayStaysFenced reports whether the rows this process replayed
// leave a fenced store fenced: none would change under the migration plan
// (no leaf migrates, no reference, column or endpoint names a renamed ID, the
// upcast renamed nothing) and their role fields pass the serving fence.
// false means the full pass must decide; it is never a refusal.
func (s *Store) vocabReplayStaysFenced(ctx context.Context, prior *MigrationReport, progress func()) (bool, error) {
	t := &s.vocabReplay
	t.mu.Lock()
	ok := t.opened && !t.foreign && !t.unfenced && !t.overflow && t.finalized > 0
	objects := make([]string, 0, len(t.objects))
	for id := range t.objects {
		objects = append(objects, id)
	}
	edges := make([]string, 0, len(t.edges))
	for id := range t.edges {
		edges = append(edges, id)
	}
	t.mu.Unlock()
	if !ok || prior == nil || prior.FencedAt == "" || prior.ReplayOpenedAt == "" {
		return false, nil
	}
	if s.depositUpcastAliasesAdded() > 0 {
		// A replayed V1 deposit was renamed: rows outside the replay may
		// reference its old ID.
		return false, nil
	}
	seed := ogSeedFromReport(prior)
	ledger, err := s.depositUpcastSeed(seed)
	if err != nil {
		return false, err
	}
	aliases := 0
	if ledger != nil {
		aliases = len(ledger.Objects) + len(ledger.Edges)
	}
	if aliases != prior.DepositUpcastAliases {
		// The append-only ledger grew since the fence without this process
		// adding to it: a process that crashed before marking the report, or
		// a batch that rolled back after recording an alias. Rows outside
		// the replay may reference the old ID.
		return false, nil
	}
	renamed := func(id string) bool {
		_, hit := seed[id]
		return id != "" && hit
	}
	var fields []vocabmigrate.Field
	for _, id := range objects {
		if progress != nil {
			progress()
		}
		var kind, versionID, supersededBy string
		var body, metadata []byte
		err := s.db.QueryRowContext(ctx, `SELECT object_kind, version_id, superseded_by, body, metadata FROM og_objects WHERE canonical_id=?`, id).
			Scan(&kind, &versionID, &supersededBy, &body, &metadata)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("vocab replay fence: read %s: %w", id, err)
		}
		if renamed(id) || renamed(versionID) || renamed(supersededBy) {
			return false, nil
		}
		for _, raw := range [][]byte{metadata, body} {
			var doc any
			if json.Unmarshal(raw, &doc) != nil {
				continue // the plan leaves a non-JSON document untouched
			}
			if !vocabReplayLeavesAtFixpoint(doc, seed) {
				return false, nil
			}
		}
		ogRoleFieldsFromJSON(kind, metadata, "metadata.", &fields)
		ogRoleFieldsFromJSON(kind, body, "body.", &fields)
	}
	for _, id := range edges {
		if progress != nil {
			progress()
		}
		var from, to string
		err := s.db.QueryRowContext(ctx, `SELECT from_id, to_id FROM og_edges WHERE edge_id=?`, id).Scan(&from, &to)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("vocab replay fence: read edge %s: %w", id, err)
		}
		if renamed(id) || renamed(from) || renamed(to) {
			return false, nil
		}
	}
	if vocabmigrate.VerifyServingVocabulary(vocabmigrate.VocabularyV2, fields...) != nil {
		return false, nil
	}
	return true, nil
}

// vocabReplayLeavesAtFixpoint mirrors planOGMigration's per-leaf rules: no
// leaf migrates, and no reference-bearing leaf contains a renamed ID (the
// plan rewrites by substring, so this checks by substring too).
func vocabReplayLeavesAtFixpoint(doc any, seed map[string]string) bool {
	clean := true
	ogWalkStrings(doc, "", func(_ string, v string) string {
		if !clean {
			return v
		}
		if _, changed := ogLeafMigrate(v); changed {
			clean = false
			return v
		}
		if !strings.Contains(v, "obj:") && !strings.Contains(v, "edge:") {
			return v
		}
		for old := range seed {
			if strings.Contains(v, old) {
				clean = false
				break
			}
		}
		return v
	})
	return clean
}
