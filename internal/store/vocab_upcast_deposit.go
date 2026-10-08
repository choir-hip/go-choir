package store

// Versioned projection-deposit upcasting for replay (mission-2 residual:
// cutover-aware replay; owner-authorized recovery policy 2026-10-07).
//
// The canonical tape is immutable and is never rewritten. Replay verifies the
// original event, receipt, payload digest and reducer commitment first (the
// appender's reconstruct loop), and only then hands the resolved
// computerevent.ProjectionBatch to this store. This file transforms that
// verified DEPOSIT VIEW — and nothing else — before projectBatchForReplay
// applies it:
//
//   - `object_recorded` deposits migrate desk tokens and desk-bearing IDs
//     through the same frozen rules the end-of-replay migration uses
//     (ogLeafMigrate + the per-kind identity formulas), recompute
//     content_hash from the migrated bytes, and re-derive canonical_id from
//     verified identity formulas — never by free-text guessing and never by
//     wall-clock comparison.
//   - embedded obj:/edge: references, edge endpoints and the version_id /
//     superseded_by columns rewrite through an alias ledger of identities
//     this replay already upcast. That is how a delete or a late update
//     recorded under a legacy ID lands on the one canonical row (event order
//     decides, not timestamps).
//   - V2-spelled deposits pass through byte-identically, so replay of the
//     post-cutover tape and live finalization produce the same projection for
//     the same event.
//
// The alias ledger is a workspace sidecar (`vocab-deposit-upcast.jsonl`,
// append-only, header = vocabmigrate.DepositUpcastVersion + active
// vocabulary) written with the store's own workspace so it travels with
// published bases exactly like vocab-migration-report.json. It deliberately
// lives outside the database: a table would change the schema hash and break
// live↔replay witness equality. Alias deltas are appended as they are
// derived — before the deposit transaction commits — so a crash-resumed
// replay can still resolve references recorded against legacy IDs, and
// CommitReplay fsyncs them with the same checkpoint boundary.
//
// Activation (once per process, cached):
//   - a ledger sidecar exists (this store's deposits are declared upcast);
//   - or the replay path runs against a cut-over store (vocabCutover);
//   - or the replay path runs from scratch (no projection head yet — fresh
//     reconstruction). A retained V1 store (projection head present, not cut
//     over, no ledger) deliberately keeps byte-identical deposits: its
//     one-time cutover stays MigrateAndFenceServingVocabulary's job.
//
// The live finalize path transforms only when the ledger already exists (a
// store whose replayed deposits were upcast); a live writer's own spelling
// stays authoritative until the boot-time cutover otherwise.
//
// The end-of-replay migration stays in place for every store: on an upcast
// store it is the verify-only fence plus the straggler pass (references no
// in-stream alias could resolve), seeded with this ledger; on a retained V1
// store it remains the one-time migration.

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/objectgraph"
	"github.com/yusefmosiah/go-choir/internal/vocabmigrate"
)

// depositUpcastLedgerFileName is the sidecar carrying the deposit upcast
// version and the old→new identity aliases. It lives in the Dolt workspace
// directory beside vocab-migration-report.json and is never a table.
const depositUpcastLedgerFileName = "vocab-deposit-upcast.jsonl"

// depositUpcastLedgerDelta is one append-only ledger line: the header (frozen
// version + active vocabulary) or one batch of alias deltas.
type depositUpcastLedgerDelta struct {
	Version int               `json:"version,omitempty"`
	Active  string            `json:"active,omitempty"`
	Objects map[string]string `json:"objects,omitempty"`
	Edges   map[string]string `json:"edges,omitempty"`
}

// depositUpcastLedger is the merged durable view of the sidecar.
type depositUpcastLedger struct {
	Version int
	Active  string
	Objects map[string]string
	Edges   map[string]string
}

// loadDepositUpcastLedger merges the append-only sidecar. A missing file is
// absence, never an error; a malformed or unsupported file refuses loudly
// (silently ignoring a compatibility record would let a resumed replay
// diverge).
func loadDepositUpcastLedger(path string) (*depositUpcastLedger, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("deposit upcast ledger: open %s: %w", path, err)
	}
	defer file.Close()
	ledger := &depositUpcastLedger{Objects: map[string]string{}, Edges: map[string]string{}}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var delta depositUpcastLedgerDelta
		if err := json.Unmarshal([]byte(line), &delta); err != nil {
			return nil, fmt.Errorf("deposit upcast ledger %s line %d: %w", path, lineNumber, err)
		}
		if delta.Version != 0 {
			if delta.Version != vocabmigrate.DepositUpcastVersion {
				return nil, fmt.Errorf("deposit upcast ledger %s: unsupported version %d", path, delta.Version)
			}
			ledger.Version = delta.Version
		}
		if delta.Active != "" {
			ledger.Active = delta.Active
		}
		for old, current := range delta.Objects {
			if existing, ok := ledger.Objects[old]; ok && existing != current {
				return nil, fmt.Errorf("deposit upcast ledger %s: conflicting object alias %s", path, old)
			}
			ledger.Objects[old] = current
		}
		for old, current := range delta.Edges {
			if existing, ok := ledger.Edges[old]; ok && existing != current {
				return nil, fmt.Errorf("deposit upcast ledger %s: conflicting edge alias %s", path, old)
			}
			ledger.Edges[old] = current
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("deposit upcast ledger %s: %w", path, err)
	}
	if ledger.Version == 0 {
		return nil, fmt.Errorf("deposit upcast ledger %s: missing version header", path)
	}
	if ledger.Active != vocabmigrate.VocabularyV2 {
		return nil, fmt.Errorf("deposit upcast ledger %s: unsupported active vocabulary %q", path, ledger.Active)
	}
	return ledger, nil
}

// depositUpcaster is the process-wide projection-deposit upcaster. All state
// is guarded by mu; replay finalization is serialized upstream, but live
// finalization and ledger sync must stay safe against each other.
type depositUpcaster struct {
	mu      sync.Mutex
	path    string
	loaded  bool
	present bool
	active  bool
	objects map[string]string
	edges   map[string]string
	file    *os.File
}

func (u *depositUpcaster) isActive() bool {
	if u == nil {
		return false
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.active
}

// resolve consults the ledger sidecar once and activates upcasting when the
// store's deposit vocabulary is declared (ledger present), cut over, or being
// reconstructed fresh on the replay path.
func (u *depositUpcaster) resolve(replay bool, fresh bool, cutover bool) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.loaded {
		// Mark loaded only after validation succeeds: an invalid ledger must
		// refuse every call, not just the first (frozen-candidate panel
		// 2026-10-08).
		ledger, err := loadDepositUpcastLedger(u.path)
		if err != nil {
			return err
		}
		u.loaded = true
		if ledger != nil {
			u.present = true
			if ledger.Objects != nil {
				u.objects = ledger.Objects
			}
			if ledger.Edges != nil {
				u.edges = ledger.Edges
			}
		}
	}
	if u.active {
		return nil
	}
	if !(u.present || (replay && (cutover || fresh))) {
		return nil
	}
	if err := u.ensureOpenLocked(); err != nil {
		return err
	}
	u.active = true
	return nil
}

// ensureOpenLocked creates the ledger header (or opens the existing ledger)
// so the declared compatibility version is durable before the first
// transformed deposit can commit.
func (u *depositUpcaster) ensureOpenLocked() error {
	if u.file != nil {
		return nil
	}
	if strings.TrimSpace(u.path) == "" {
		return fmt.Errorf("deposit upcast ledger: store workspace path is unavailable")
	}
	if !u.present {
		if _, err := os.Stat(u.path); err == nil {
			u.present = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("deposit upcast ledger: stat %s: %w", u.path, err)
		}
	}
	if u.present {
		file, err := os.OpenFile(u.path, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("deposit upcast ledger: open %s: %w", u.path, err)
		}
		u.file = file
		return nil
	}
	header, err := json.Marshal(depositUpcastLedgerDelta{
		Version: vocabmigrate.DepositUpcastVersion,
		Active:  vocabmigrate.VocabularyV2,
	})
	if err != nil {
		return fmt.Errorf("deposit upcast ledger: encode header: %w", err)
	}
	file, err := os.OpenFile(u.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("deposit upcast ledger: create %s: %w", u.path, err)
	}
	if _, err := file.Write(append(header, '\n')); err != nil {
		_ = file.Close()
		return fmt.Errorf("deposit upcast ledger: write header: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("deposit upcast ledger: sync header: %w", err)
	}
	u.present = true
	u.file = file
	return nil
}

// appendDeltaLocked writes one alias delta. The write happens before the
// deposit transaction commits, so an alias can never be missing while the row
// it renamed is durable.
func (u *depositUpcaster) appendDeltaLocked(delta depositUpcastLedgerDelta) error {
	if err := u.ensureOpenLocked(); err != nil {
		return err
	}
	line, err := json.Marshal(delta)
	if err != nil {
		return fmt.Errorf("deposit upcast ledger: encode delta: %w", err)
	}
	if _, err := u.file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("deposit upcast ledger: append %s: %w", u.path, err)
	}
	return nil
}

// syncLedger flushes alias deltas to stable storage. It is called on the same
// boundary as the replay durable checkpoint (CommitReplay).
func (u *depositUpcaster) syncLedger() error {
	if u == nil {
		return nil
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.file == nil {
		return nil
	}
	if err := u.file.Sync(); err != nil {
		return fmt.Errorf("deposit upcast ledger: sync %s: %w", u.path, err)
	}
	return nil
}

func (u *depositUpcaster) close() error {
	if u == nil {
		return nil
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.file == nil {
		return nil
	}
	// Best-effort durability on clean shutdown; the replay checkpoint
	// boundary already fsyncs, so a sync error here is not a live-path
	// failure.
	_ = u.file.Sync()
	err := u.file.Close()
	u.file = nil
	return err
}

func (u *depositUpcaster) lookupAlias(id string) (string, bool) {
	if u == nil || id == "" {
		return "", false
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if current, ok := u.objects[id]; ok {
		return current, true
	}
	if current, ok := u.edges[id]; ok {
		return current, true
	}
	return "", false
}

// recordAlias makes one identity rename durable before the deposit that
// produced it can commit. Re-registering the same alias is idempotent (a
// resumed replay re-derives the same names); a conflicting rename refuses.
func (u *depositUpcaster) recordAlias(old, current string, edge bool) error {
	if old == "" || current == "" || old == current {
		return nil
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if !u.active {
		return nil
	}
	aliases := u.objects
	delta := depositUpcastLedgerDelta{Objects: map[string]string{old: current}}
	if edge {
		aliases = u.edges
		delta = depositUpcastLedgerDelta{Edges: map[string]string{old: current}}
	}
	if existing, ok := aliases[old]; ok {
		if existing != current {
			return fmt.Errorf("deposit upcast ledger: conflicting alias for %s (%s vs %s)", old, existing, current)
		}
		return nil
	}
	aliases[old] = current
	return u.appendDeltaLocked(delta)
}

// rewriteAliasID rewrites one ID-valued column through the alias ledger.
func (u *depositUpcaster) rewriteAliasID(value *string) bool {
	if value == nil || *value == "" {
		return false
	}
	current, ok := u.lookupAlias(*value)
	if !ok || current == *value {
		return false
	}
	*value = current
	return true
}

// upcastBatch transforms the verified deposit view of one projection batch in
// place and in canonical event order. record=false keeps dry-run validation
// free of ledger side effects.
func (u *depositUpcaster) upcastBatch(batch *computerevent.ProjectionBatch, record bool) error {
	if u == nil || batch == nil {
		return nil
	}
	for i := range batch.Ops {
		if err := u.upcastOp(&batch.Ops[i], record); err != nil {
			return fmt.Errorf("deposit upcast: op %d (%s): %w", i, batch.Ops[i].Kind, err)
		}
	}
	return nil
}

func (u *depositUpcaster) upcastOp(op *computerevent.ProjectionOp, record bool) error {
	switch strings.TrimSpace(op.Kind) {
	case computerevent.ProjectionOpObject:
		return u.upcastObjectOp(op, record)
	case computerevent.ProjectionOpObjectEdge:
		return u.upcastEdgeOp(op, record)
	default:
		// The remaining replay-projected ops (desktop, run memory,
		// self-development, texture) carry no desk vocabulary in the
		// migration inventory (vocabValueTargets / ogIdentityFormulas): their
		// role-bearing SQL tables are live-only and stay owned by the
		// one-time MigrateAndFenceServingVocabulary cutover.
		return nil
	}
}

// migrateDepositLeaves applies the frozen token/ID rules to every string leaf
// of one decoded document, recording original values for identity-formula
// verification exactly like planOGMigration does.
func (u *depositUpcaster) migrateDepositLeaves(row *ogObjectRow, node any, prefix string) bool {
	changed := false
	ogWalkStrings(node, "", func(path, value string) string {
		migrated, leafChanged := ogLeafMigrate(value)
		if !leafChanged {
			return value
		}
		if _, seen := row.fields[prefix+path]; !seen {
			row.fields[prefix+path] = value
		}
		changed = true
		return migrated
	})
	return changed
}

// rewriteDepositRefs rewrites embedded canonical-ID references through the
// alias ledger, recording the pre-rewrite leaf for identity verification.
func (u *depositUpcaster) rewriteDepositRefs(row *ogObjectRow, node any, prefix string) bool {
	rewrote := false
	ogWalkStrings(node, "", func(path, value string) string {
		current, changed := u.rewriteLeafRefs(value)
		if !changed {
			return value
		}
		if _, seen := row.fields[prefix+path]; !seen {
			row.fields[prefix+path] = value
		}
		rewrote = true
		return current
	})
	return rewrote
}

// rewriteLeafRefs rewrites well-formed canonical object/edge ID tokens in one
// string leaf. Whole-leaf IDs (the common shape) resolve with one map lookup;
// embedded tokens are parsed by shape so the rewrite stays linear in the leaf
// length. References no ledger entry covers — including references forward in
// the chain — are left for the end-of-replay migration, whose alias seed is
// this same ledger.
func (u *depositUpcaster) rewriteLeafRefs(value string) (string, bool) {
	if value == "" || (!strings.Contains(value, "obj:") && !strings.Contains(value, "edge:")) {
		return value, false
	}
	if current, ok := u.lookupAlias(value); ok {
		return current, true
	}
	var builder strings.Builder
	changed := false
	index := 0
	for index < len(value) {
		next := strings.Index(value[index:], "obj:")
		if edgeNext := strings.Index(value[index:], "edge:"); next < 0 || (edgeNext >= 0 && edgeNext < next) {
			next = edgeNext
		}
		if next < 0 {
			break
		}
		next += index
		end := depositRefTokenEnd(value, next)
		if end > next {
			if current, ok := u.lookupAlias(value[next:end]); ok {
				builder.WriteString(value[index:next])
				builder.WriteString(current)
				index = end
				changed = true
				continue
			}
		}
		builder.WriteString(value[index : next+1])
		index = next + 1
	}
	if !changed {
		return value, false
	}
	builder.WriteString(value[index:])
	return builder.String(), true
}

// depositRefTokenEnd parses one obj:/edge: canonical ID starting at value[start]
// and returns its exclusive end, or start when the bytes are not a canonical
// ID. Object IDs are obj:<kind>:<base64url owner>:<suffix>; edge IDs are
// edge:<kind>:<hex>. Every segment is URL-safe, so ':' separates them.
func depositRefTokenEnd(value string, start int) int {
	index := start
	switch {
	case strings.HasPrefix(value[start:], "obj:"):
		index += len("obj:")
		return depositRefSegmentsEnd(value, index, 3)
	case strings.HasPrefix(value[start:], "edge:"):
		index += len("edge:")
		return depositRefSegmentsEnd(value, index, 2)
	default:
		return start
	}
}

func depositRefSegmentsEnd(value string, index, segments int) int {
	for segment := range segments {
		if segment > 0 {
			if index >= len(value) || value[index] != ':' {
				return -1
			}
			index++
		}
		end := index
		for end < len(value) && depositRefByte(value[end]) {
			end++
		}
		if end == index {
			return -1
		}
		index = end
	}
	return index
}

func depositRefByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_':
		return true
	default:
		return false
	}
}

// upcastObjectOp upcasts one object_recorded deposit. The transformation
// mirrors planOGMigration exactly (leaf rules, hash recomputation, verified
// identity formula, alias resolution), but applies per deposit in event order
// instead of once over the settled rows.
func (u *depositUpcaster) upcastObjectOp(op *computerevent.ProjectionOp, record bool) error {
	if len(op.Body) == 0 {
		return nil
	}
	var obj objectgraph.Object
	if err := json.Unmarshal(op.Body, &obj); err != nil {
		return fmt.Errorf("decode object deposit: %w", err)
	}
	recordedID := strings.TrimSpace(obj.CanonicalID)
	if recordedID == "" {
		// projectObject refuses the missing identity; leave its error intact.
		return nil
	}
	row := &ogObjectRow{
		canonicalID:  recordedID,
		kind:         string(obj.ObjectKind),
		ownerID:      obj.OwnerID,
		computerID:   obj.ComputerID,
		versionID:    obj.VersionID,
		body:         obj.Body,
		metadata:     obj.Metadata,
		createdAt:    obj.CreatedAt.UTC().Format(time.RFC3339Nano),
		updatedAt:    obj.UpdatedAt.UTC().Format(time.RFC3339Nano),
		tombstone:    obj.Tombstone,
		supersededBy: obj.SupersededBy,
		fields:       map[string]string{},
	}
	if err := json.Unmarshal(obj.Metadata, &row.metaJSON); err != nil {
		row.metaJSON = nil
	}
	if len(obj.Body) > 0 {
		var bodyJSON any
		if err := json.Unmarshal(obj.Body, &bodyJSON); err == nil {
			row.bodyJSON = bodyJSON
		}
	}
	metaChanged := false
	if row.metaJSON != nil {
		metaChanged = u.migrateDepositLeaves(row, row.metaJSON, "metadata.")
	}
	bodyChanged := false
	if row.bodyJSON != nil {
		bodyChanged = u.migrateDepositLeaves(row, row.bodyJSON, "body.")
	}
	refsChanged := false
	if row.metaJSON != nil && u.rewriteDepositRefs(row, row.metaJSON, "metadata.") {
		refsChanged = true
	}
	if row.bodyJSON != nil && u.rewriteDepositRefs(row, row.bodyJSON, "body.") {
		refsChanged = true
	}
	row.dirty = metaChanged || bodyChanged || refsChanged

	// A recorded ID the ledger already renamed is a legacy identity for an
	// object this replay upcast earlier: later updates and deletes recorded
	// under it resolve to the one canonical row.
	newID := ""
	if current, ok := u.lookupAlias(recordedID); ok {
		newID = current
	}
	if row.dirty {
		if row.bodyJSON != nil {
			encoded, err := json.Marshal(row.bodyJSON)
			if err != nil {
				return fmt.Errorf("deposit upcast %s body encode: %w", recordedID, err)
			}
			row.body = encoded
		}
		normalized, err := objectgraph.NormalizeMetadata(row.metaJSON)
		if err != nil {
			return fmt.Errorf("deposit upcast %s metadata encode: %w", recordedID, err)
		}
		row.metadata = normalized
		row.newHash = objectgraph.ContentHash(objectgraph.ObjectKind(row.kind), row.body, row.metadata)
		if err := row.ogRekey(); err != nil {
			return err
		}
		switch {
		case newID == "":
			newID = row.newID
		case row.newID == newID:
			// Independent re-derivation and the ledger agree.
		case row.newID == recordedID:
			// Content re-derives to the recorded (legacy) identity while the
			// ledger holds the identity a previous deposit actually applied;
			// the ledger wins.
		default:
			return fmt.Errorf("deposit upcast: %s re-derives to %s but the alias ledger holds %s", recordedID, row.newID, newID)
		}
	}
	if newID == "" {
		newID = recordedID
	}
	versionChanged := u.rewriteAliasID(&row.versionID)
	supersededChanged := u.rewriteAliasID(&row.supersededBy)
	idChanged := newID != recordedID
	if !row.dirty && !idChanged && !versionChanged && !supersededChanged {
		return nil
	}
	obj.CanonicalID = newID
	if row.dirty {
		obj.ContentHash = row.newHash
		obj.Body = row.body
		obj.Metadata = row.metadata
	}
	obj.VersionID = row.versionID
	obj.SupersededBy = row.supersededBy
	encoded, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("deposit upcast %s object encode: %w", recordedID, err)
	}
	op.Body = encoded
	if strings.TrimSpace(op.CanonicalID) != "" {
		op.CanonicalID = newID
	}
	if record && idChanged {
		return u.recordAlias(recordedID, newID, false)
	}
	return nil
}

// upcastEdgeOp rewrites one object_edge_recorded deposit through the alias
// ledger: endpoints resolve to current identities and edge_id re-derives via
// BuildEdgeID, mirroring planOGMigration's edge handling.
func (u *depositUpcaster) upcastEdgeOp(op *computerevent.ProjectionOp, record bool) error {
	if len(op.Body) == 0 {
		return nil
	}
	var edge objectgraph.Edge
	if err := json.Unmarshal(op.Body, &edge); err != nil {
		return fmt.Errorf("decode edge deposit: %w", err)
	}
	recordedID := strings.TrimSpace(edge.EdgeID)
	if recordedID == "" {
		return nil
	}
	from, to := edge.FromID, edge.ToID
	if current, ok := u.lookupAlias(from); ok {
		from = current
	}
	if current, ok := u.lookupAlias(to); ok {
		to = current
	}
	newID := ""
	if current, ok := u.lookupAlias(recordedID); ok {
		newID = current
	}
	if from == edge.FromID && to == edge.ToID && newID == "" {
		return nil
	}
	if newID == "" {
		built, err := objectgraph.BuildEdgeID(from, to, edge.Kind, edge.Metadata)
		if err != nil {
			return fmt.Errorf("deposit upcast edge %s rebuild: %w", recordedID, err)
		}
		newID = built
	}
	edge.EdgeID, edge.FromID, edge.ToID = newID, from, to
	encoded, err := json.Marshal(edge)
	if err != nil {
		return fmt.Errorf("deposit upcast edge %s encode: %w", recordedID, err)
	}
	op.Body = encoded
	if strings.TrimSpace(op.CanonicalID) != "" {
		op.CanonicalID = newID
	}
	if record && newID != recordedID {
		return u.recordAlias(recordedID, newID, true)
	}
	return nil
}

// depositUpcastLedgerPath names the sidecar inside the store workspace.
func (s *Store) depositUpcastLedgerPath() string {
	if s == nil || strings.TrimSpace(s.texturePath) == "" {
		return ""
	}
	return filepath.Join(s.texturePath, depositUpcastLedgerFileName)
}

// depositUpcastForBatch returns the process-wide upcaster, resolving it on
// first use. replay marks the projection-deposit replay path; fresh marks a
// store with no projection head for this computer (a from-scratch
// reconstruction).
func (s *Store) depositUpcastForBatch(replay, fresh bool) (*depositUpcaster, error) {
	if s == nil {
		return nil, nil
	}
	s.depositUpcastMu.Lock()
	upcaster := s.depositUpcast
	if upcaster == nil {
		upcaster = &depositUpcaster{
			path:    s.depositUpcastLedgerPath(),
			objects: map[string]string{},
			edges:   map[string]string{},
		}
		s.depositUpcast = upcaster
	}
	s.depositUpcastMu.Unlock()
	if err := upcaster.resolve(replay, fresh, s.vocabCutover.Load()); err != nil {
		return nil, err
	}
	return upcaster, nil
}

// upcastProjectionDeposit transforms a verified projection batch's deposit
// view to the active serving vocabulary when this store's deposits are upcast.
// The decision is resolved even for a nil/empty batch so a batchless genesis
// still pins the reconstruction mode while the missing head row is
// observable. record=false keeps validation (dry-run) reads free of ledger
// side effects.
func (s *Store) upcastProjectionDeposit(batch *computerevent.ProjectionBatch, replay bool, fresh bool, record bool) error {
	if s == nil {
		return nil
	}
	upcaster, err := s.depositUpcastForBatch(replay, fresh)
	if err != nil {
		return err
	}
	if batch == nil || len(batch.Ops) == 0 || !upcaster.isActive() {
		return nil
	}
	return upcaster.upcastBatch(batch, record)
}

// syncDepositUpcastLedger flushes buffered deposit-upcast alias deltas at the
// replay durable checkpoint boundary.
func (s *Store) syncDepositUpcastLedger() error {
	if s == nil {
		return nil
	}
	s.depositUpcastMu.Lock()
	upcaster := s.depositUpcast
	s.depositUpcastMu.Unlock()
	if upcaster == nil {
		return nil
	}
	return upcaster.syncLedger()
}

// closeDepositUpcastLedger releases the sidecar handle.
func (s *Store) closeDepositUpcastLedger() error {
	if s == nil {
		return nil
	}
	s.depositUpcastMu.Lock()
	upcaster := s.depositUpcast
	s.depositUpcastMu.Unlock()
	if upcaster == nil {
		return nil
	}
	return upcaster.close()
}

// depositUpcastLedgerState loads the durable deposit-upcast ledger sidecar
// (nil when the store was never upcast).
func (s *Store) depositUpcastLedgerState() (*depositUpcastLedger, error) {
	if s == nil {
		return nil, nil
	}
	return loadDepositUpcastLedger(s.depositUpcastLedgerPath())
}

// depositUpcastSeed merges the recorded old→new identity aliases into a
// migration seed so the end-of-replay pass rewrites references recorded
// against legacy IDs. It returns the ledger for report visibility (nil when
// the store was never upcast).
func (s *Store) depositUpcastSeed(seed map[string]string) (*depositUpcastLedger, error) {
	ledger, err := s.depositUpcastLedgerState()
	if err != nil {
		return nil, err
	}
	if ledger == nil {
		return nil, nil
	}
	if seed != nil {
		for old, current := range ledger.Objects {
			seed[old] = current
		}
		for old, current := range ledger.Edges {
			seed[old] = current
		}
	}
	return ledger, nil
}
