package platform

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"sort"
	"time"
)

// Durable reference progress for the tape sweep's references phase.
//
// The phase walks every computer's whole append-receipt chain (11 million
// events on the paper, more than three hours) and corpusd restarts on every
// deploy, so an in-memory walk could never finish on a day with a deploy. The
// walk now writes its progress to the control database as it goes, in
// chunks, and a later dry-run reuses the chunks that still verify.
//
// What a chunk is. One computer, the sequence range (from, to], the set of
// 8-byte digest prefixes that range contributed (tapeRowPrefixes: event
// digest, event pin, payload pins, and the payload digests the event file
// names), the checksum of that set, the event digest at `to` (the anchor),
// whether the walk read event files, and tapeRefFormat.
//
// THE FORMAT CONSTANT. tapeRefFormat names the extraction that produced a
// chunk. Bump it whenever any of these change: digestPrefix (width or
// encoding), hex64RE or tapeHexRefs, which receipt or event fields
// walkTapeChain turns into PayloadPins or PayloadRefs, or tapeRowPrefixes. A
// chunk of another format is discarded and its computer re-walked from 0.
// Without the bump every stored chunk would silently lack the new
// references and apply would delete files they name.
// TestTapeRefExtractionGolden fails when the extraction changes under the
// current number.
//
// Why a resumed walk equals an uninterrupted one (the correctness argument).
//
//  1. The invariant everything rests on: a receipt row is never changed
//     except losslessly (the append-receipt compaction rewrites the stored
//     body into a form that expands to the same JSON). The rows 1..n are
//     therefore the same rows a fresh walk reads, and the references of those
//     rows are a function of them and of the event files their digests name,
//     which are content-addressed. TestAppendReceiptRowWritersAreKnown lists
//     the code that writes or deletes receipt rows; any new writer that is not
//     lossless must delete the computer's chunks.
//  2. The anchor is not the guard for that invariant. It does prove the chain
//     prefix: event n names the canonical head after n-1, so the event digest
//     at sequence n commits to the digests of every event before it. It does
//     NOT commit to the receipt body (the pin_receipt_digests live in the
//     receipt, outside the event), and the loader checks only the anchors at
//     chunk ends, not interior rows. A non-lossless in-place edit of a
//     receipt body that keeps event_digest is invisible to the loader, and no
//     cheap check exists short of re-reading the range. So the guard is the
//     invariant in 1, enforced by listing the writers.
//  3. A chunk is used only when every one of these holds, otherwise ALL of
//     the computer's chunks are deleted and its walk restarts at sequence 0:
//     every chunk is of this tapeRefFormat; the chunks tile 1..k with no gap
//     and no overlap; k is not above the computer's current head; each
//     chunk's checksum matches its set; each chunk's anchor equals the event
//     digest the receipt table holds at that sequence now.
//  4. The reads rule, which only shortens the prefix: a chunk walked without
//     event reads lacks payload references. When this walk reads events, the
//     verified prefix ends before the first such chunk, that chunk and the
//     rest are deleted, and the walk re-reads from there. A chunk walked with
//     reads serves a walk that does not read, since extra references only
//     keep files. A chunk whose range held an unreadable event is stored as
//     walked without reads, so the "payload references unknown" refusal is
//     never lost to a resume, and one bad event costs a re-walk of its own
//     chunk and the rows after it, not of the computer.
//  5. So the union of the used chunks is the set an uninterrupted walk of
//     1..k would take, and the walk continues at k+1. A chunk is written only
//     after its whole range was walked without error, in one statement, so an
//     interrupt leaves a complete chunk or none.
//  6. A prefix collision can only add a false reference. Nothing here removes
//     a reference a walk would have taken.
//
// What a resumed sweep does not do: it does not re-read the media of covered
// ranges. A corrupt segment or a vanished event file inside a covered range
// does not raise ErrCorrupt or the unreadable refusal; a from-scratch walk
// would. References are unaffected (a failed read yields none, so the stored
// set is a superset), so no file is wrongly deleted. The report's
// chain_events_resumed says how much was not re-read.
//
// The platform's own operations (docs/computer-ontology.md): a fork is a
// sibling with its own ComputerID and chain, so it starts at genesis and
// never inherits a parent's chunks; rollback and restore append events, they
// do not edit rows, so the watermark stays valid; a restored control
// database is a head below the watermark or an anchor that differs, and the
// chunks are dropped; a purge removes the chunks with the computer's rows. A
// row deleted from the middle of a covered range is caught only by the
// receipt-count check (tapeReceiptsMatchSnapshot), which refuses the orphan
// sweep; it needs database damage. Chunks are compacted past a count so a
// load stays a few rows per computer.
//
// A restarted corpusd does NOT resume a sweep by itself: a restart never
// resumes work (CLAUDE.md, Restarts). The caller issues the dry-run again and
// it finds the chunks.

const (
	// tapeRefFormat: see THE FORMAT CONSTANT above.
	tapeRefFormat = 1

	// defaultTapeRefChunkEvents is the events walked per persisted chunk:
	// about a minute of walking on the paper, a few MB of prefixes.
	defaultTapeRefChunkEvents = 50_000

	// defaultTapeRefCompactAfter is the chunk count per computer past which
	// the chunks are merged; defaultTapeRefCompactMaxEvents caps a merged
	// chunk's range (1M events hold a few tens of MB of prefixes).
	defaultTapeRefCompactAfter     = 32
	defaultTapeRefCompactMaxEvents = 1_000_000

	tapeRefChunksSchemaDDL = `
CREATE TABLE IF NOT EXISTS tape_sweep_ref_chunks (
  computer_id VARCHAR(128) NOT NULL,
  from_sequence BIGINT NOT NULL,
  to_sequence BIGINT NOT NULL,
  read_events BOOLEAN NOT NULL,
  anchor_event_digest CHAR(64) NOT NULL,
  refs BYTEA NOT NULL,
  refs_sha256 CHAR(64) NOT NULL,
  walker_version INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (computer_id, from_sequence)
);`
	// A table made before the column existed gets version 0, which no
	// walker accepts.
	tapeRefChunksVersionDDL = `ALTER TABLE tape_sweep_ref_chunks ADD COLUMN IF NOT EXISTS walker_version INT NOT NULL DEFAULT 0`
)

func (s *Store) ensureTapeRefChunksSchema(ctx context.Context) error {
	for _, ddl := range []string{tapeRefChunksSchemaDDL, tapeRefChunksVersionDDL} {
		if _, err := s.db.ExecContext(ctx, ddl); err != nil {
			return fmt.Errorf("platform store: bootstrap tape sweep ref chunks: %w", err)
		}
	}
	return nil
}

// tapeRefChunk is one persisted range of a computer's reference walk. Refs
// is filled only when a chunk is being written.
type tapeRefChunk struct {
	From, To   uint64
	ReadEvents bool
	Anchor     string
	Refs       []uint64 // sorted, unique
}

func encodeTapeRefs(refs []uint64) []byte {
	out := make([]byte, 8*len(refs))
	for i, v := range refs {
		binary.BigEndian.PutUint64(out[8*i:], v)
	}
	return out
}

func tapeRefsSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// tapeRowPrefixes is the one extraction of a chain row's references. See THE
// FORMAT CONSTANT: a change here is a tapeRefFormat bump.
func tapeRowPrefixes(row tapeChainRow) []uint64 {
	var out []uint64
	add := func(d string) {
		if v, ok := digestPrefix(d); ok {
			out = append(out, v)
		}
	}
	add(row.EventDigest)
	add(row.EventPin)
	for _, d := range row.PayloadPins {
		add(d)
	}
	for _, d := range row.PayloadRefs {
		add(d)
	}
	return out
}

// addPrefix adds an already-extracted prefix.
func (p *digestPrefixSet) addPrefix(v uint64) { p.v = append(p.v, v) }

// reserve makes room for n more prefixes in one allocation, so loading a
// large watermark does not grow the slice by doubling.
func (p *digestPrefixSet) reserve(n int) {
	if cap(p.v)-len(p.v) >= n {
		return
	}
	grown := make([]uint64, len(p.v), len(p.v)+n)
	copy(grown, p.v)
	p.v = grown
}

func (s *Service) tapeRefChunkEvents() uint64 {
	if n := s.tapeSweep.refChunkEvents; n > 0 {
		return uint64(n)
	}
	return defaultTapeRefChunkEvents
}

func (s *Store) saveTapeRefChunk(ctx context.Context, computerID string, c tapeRefChunk) error {
	blob := encodeTapeRefs(c.Refs)
	_, err := s.db.ExecContext(ctx, `INSERT INTO tape_sweep_ref_chunks (computer_id,from_sequence,to_sequence,read_events,anchor_event_digest,refs,refs_sha256,walker_version,created_at) VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT (computer_id, from_sequence) DO UPDATE SET to_sequence=excluded.to_sequence,read_events=excluded.read_events,anchor_event_digest=excluded.anchor_event_digest,refs=excluded.refs,refs_sha256=excluded.refs_sha256,walker_version=excluded.walker_version,created_at=excluded.created_at`,
		computerID, c.From, c.To, c.ReadEvents, c.Anchor, blob, tapeRefsSum(blob), tapeRefFormat, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("tape sweep: save reference chunk %s (%d,%d]: %w", computerID, c.From, c.To, err)
	}
	return nil
}

// deleteTapeRefChunks drops the computer's chunks from `from` on (0 drops
// all of them).
func (s *Store) deleteTapeRefChunks(ctx context.Context, computerID string, from uint64) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM tape_sweep_ref_chunks WHERE computer_id=? AND from_sequence>=?`, computerID, from); err != nil {
		return fmt.Errorf("tape sweep: drop reference chunks of %s: %w", computerID, err)
	}
	return nil
}

// tapeAnchorAt returns the event digest the receipt table holds at one
// sequence, through the same expanded-receipt read the walk uses.
func (s *Service) tapeAnchorAt(ctx context.Context, computerID string, seq uint64) (string, error) {
	var digest string
	found := false
	err := s.walkTapeChain(ctx, computerID, seq-1, seq, false, func(row tapeChainRow) error {
		digest, found = row.EventDigest, true
		return nil
	})
	if err != nil || !found {
		return "", err
	}
	return digest, nil
}

// tapeRefChunkBlob reads one chunk's set and its stored checksum.
func (s *Store) tapeRefChunkBlob(ctx context.Context, computerID string, from uint64) ([]byte, string, error) {
	var blob []byte
	var sum string
	err := s.db.QueryRowContext(ctx, `SELECT refs, refs_sha256 FROM tape_sweep_ref_chunks WHERE computer_id=? AND from_sequence=?`, computerID, from).Scan(&blob, &sum)
	return blob, sum, err
}

// loadTapeRefChunks returns the verified prefix of the computer's stored
// chunks (metadata only; addTapeRefChunks reads the sets), per the
// correctness argument above. A chunk that fails a hard check (format,
// tiling, head, checksum, anchor) deletes all of the computer's chunks and
// returns none. needReads shortens the prefix at the first chunk walked
// without reads and deletes it and the rest. An error means the check could
// not run; the caller then walks from 0 and trusts nothing.
func (s *Service) loadTapeRefChunks(ctx context.Context, computerID string, head uint64, needReads bool) ([]tapeRefChunk, error) {
	rows, err := s.store.db.QueryContext(ctx, `SELECT from_sequence,to_sequence,read_events,anchor_event_digest,walker_version FROM tape_sweep_ref_chunks WHERE computer_id=? ORDER BY from_sequence`, computerID)
	if err != nil {
		return nil, fmt.Errorf("tape sweep: load reference chunks of %s: %w", computerID, err)
	}
	var chunks []tapeRefChunk
	bad, cutFrom, cut := "", uint64(0), false
	next := uint64(0)
	for rows.Next() {
		var c tapeRefChunk
		var version int
		if err := rows.Scan(&c.From, &c.To, &c.ReadEvents, &c.Anchor, &version); err != nil {
			rows.Close()
			return nil, fmt.Errorf("tape sweep: scan reference chunk of %s: %w", computerID, err)
		}
		if bad != "" || cut {
			continue
		}
		switch {
		case needReads && !c.ReadEvents:
			cut, cutFrom = true, c.From
			continue
		case version != tapeRefFormat:
			bad = fmt.Sprintf("chunk (%d,%d] is format %d, this walker is %d", c.From, c.To, version, tapeRefFormat)
		case c.From != next || c.To <= c.From:
			bad = fmt.Sprintf("chunk (%d,%d] does not continue at %d", c.From, c.To, next)
		case c.To > head:
			bad = fmt.Sprintf("chunk to %d is above the head %d", c.To, head)
		}
		if bad == "" {
			chunks = append(chunks, c)
			next = c.To
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("tape sweep: iterate reference chunks of %s: %w", computerID, err)
	}
	rows.Close()
	for i := 0; bad == "" && i < len(chunks); i++ {
		c := chunks[i]
		anchor, err := s.tapeAnchorAt(ctx, computerID, c.To)
		switch {
		case err != nil:
			bad = fmt.Sprintf("anchor at %d unreadable: %v", c.To, err)
		case anchor != c.Anchor:
			bad = fmt.Sprintf("anchor at %d is %s, stored %s", c.To, anchor, c.Anchor)
		}
	}
	// Checksums, one set in memory at a time.
	for i := 0; bad == "" && i < len(chunks); i++ {
		c := chunks[i]
		blob, sum, err := s.store.tapeRefChunkBlob(ctx, computerID, c.From)
		switch {
		case err != nil:
			return nil, fmt.Errorf("tape sweep: read reference chunk of %s: %w", computerID, err)
		case len(blob)%8 != 0 || tapeRefsSum(blob) != sum:
			bad = fmt.Sprintf("chunk (%d,%d] fails its checksum", c.From, c.To)
		}
	}
	if bad != "" {
		if err := s.store.deleteTapeRefChunks(ctx, computerID, 0); err != nil {
			return nil, fmt.Errorf("reference progress unusable (%s) and not dropped: %w", bad, err)
		}
		log.Printf("tape sweep: computer=%s reference progress discarded: %s", computerID, bad)
		return nil, nil
	}
	if cut {
		if err := s.store.deleteTapeRefChunks(ctx, computerID, cutFrom); err != nil {
			return nil, fmt.Errorf("reference progress after %d not dropped: %w", cutFrom, err)
		}
		log.Printf("tape sweep: computer=%s reference progress kept to %d; the rest was walked without event reads", computerID, next)
	}
	return chunks, nil
}

// addTapeRefChunks streams the verified chunks' sets into set, one chunk in
// memory at a time, into a slice reserved once.
func (s *Service) addTapeRefChunks(ctx context.Context, computerID string, chunks []tapeRefChunk, set *digestPrefixSet) error {
	var total int64
	for _, c := range chunks {
		var n sql.NullInt64
		if err := s.store.db.QueryRowContext(ctx, `SELECT octet_length(refs) FROM tape_sweep_ref_chunks WHERE computer_id=? AND from_sequence=?`, computerID, c.From).Scan(&n); err != nil {
			return err
		}
		total += n.Int64 / 8
	}
	set.reserve(int(total))
	for _, c := range chunks {
		blob, sum, err := s.store.tapeRefChunkBlob(ctx, computerID, c.From)
		if err != nil {
			return err
		}
		if len(blob)%8 != 0 || tapeRefsSum(blob) != sum {
			return fmt.Errorf("reference chunk (%d,%d] of %s changed while it was loaded", c.From, c.To, computerID)
		}
		for i := 0; i < len(blob); i += 8 {
			set.addPrefix(binary.BigEndian.Uint64(blob[i:]))
		}
	}
	return nil
}

// tapeRefPersistError is a failure to persist or load progress (or a test
// restart). It ends the run: unlike a chain read error it says nothing about
// the chain.
type tapeRefPersistError struct{ err error }

func (e *tapeRefPersistError) Error() string { return e.err.Error() }
func (e *tapeRefPersistError) Unwrap() error { return e.err }

// tapeRefWalk is what walkTapeRefs did.
type tapeRefWalk struct {
	Resumed uint64 // rows taken from stored chunks
	Walked  uint64 // rows read from the chain
}

// walkTapeRefs adds every reference of the computer's chain 1..head to set.
// needReads makes the walk read event files; checkReadable additionally
// reports each event whose file could not be read (a subset of needReads).
// It resumes from verified stored chunks and persists a chunk per
// tapeRefChunkEvents rows walked. A chain read error is returned and the
// caller treats the computer's walk as failed (rows added so far stay in
// set); a persist error is returned as *tapeRefPersistError and ends the run.
func (s *Service) walkTapeRefs(ctx context.Context, computerID string, head uint64, needReads, checkReadable bool, set *digestPrefixSet, unreadable func(tapeChainRow)) (tapeRefWalk, error) {
	var out tapeRefWalk
	chunks, err := s.loadTapeRefChunks(ctx, computerID, head, needReads)
	if err != nil {
		// Could not verify stored progress: walk everything, trust nothing.
		log.Printf("tape sweep: %v; walking computer=%s from 0", err, computerID)
		chunks = nil
	}
	pos := uint64(0)
	if len(chunks) > 0 {
		if err := s.addTapeRefChunks(ctx, computerID, chunks, set); err != nil {
			return out, &tapeRefPersistError{err}
		}
		pos = chunks[len(chunks)-1].To
	}
	out.Resumed = pos

	size := s.tapeRefChunkEvents()
	written := len(chunks)
	for pos < head {
		end := min(head, pos+size)
		local := map[uint64]struct{}{}
		allReadable := needReads
		anchor := ""
		visited := uint64(0)
		err := s.walkTapeChain(ctx, computerID, pos, end, needReads, func(row tapeChainRow) error {
			visited++
			for _, v := range tapeRowPrefixes(row) {
				set.addPrefix(v)
				local[v] = struct{}{}
			}
			if needReads && !row.EventReadable {
				allReadable = false
				if checkReadable && unreadable != nil {
					unreadable(row)
				}
			}
			anchor = row.EventDigest
			return nil
		})
		out.Walked += visited
		if err != nil {
			return out, err
		}
		refs := make([]uint64, 0, len(local))
		for v := range local {
			refs = append(refs, v)
		}
		sort.Slice(refs, func(i, j int) bool { return refs[i] < refs[j] })
		chunk := tapeRefChunk{From: pos, To: end, ReadEvents: needReads && allReadable, Anchor: anchor, Refs: refs}
		if err := s.store.saveTapeRefChunk(ctx, computerID, chunk); err != nil {
			return out, &tapeRefPersistError{err}
		}
		written++
		if hook := s.tapeSweep.afterRefChunk; hook != nil {
			if err := hook(computerID, end); err != nil {
				return out, &tapeRefPersistError{err}
			}
		}
		pos = end
	}
	after := defaultTapeRefCompactAfter
	if n := s.tapeSweep.refCompactAfter; n > 0 {
		after = n
	}
	if written > after {
		if err := s.compactTapeRefChunks(ctx, computerID); err != nil {
			// Compaction is housekeeping; the chunks it did not replace stay valid.
			log.Printf("tape sweep: computer=%s compact reference chunks: %v", computerID, err)
		}
	}
	return out, nil
}

// compactTapeRefChunks merges runs of adjacent chunks into chunks of at most
// refCompactMaxEvents rows, one group at a time and one transaction per
// group, so memory holds one group. A merge never joins chunks walked with
// and without event reads (the reads rule needs the boundary). The merged
// chunk's anchor is the last chunk's.
func (s *Service) compactTapeRefChunks(ctx context.Context, computerID string) error {
	maxEvents := uint64(defaultTapeRefCompactMaxEvents)
	if n := s.tapeSweep.refCompactMaxEvents; n > 0 {
		maxEvents = uint64(n)
	}
	rows, err := s.store.db.QueryContext(ctx, `SELECT from_sequence,to_sequence,read_events,anchor_event_digest FROM tape_sweep_ref_chunks WHERE computer_id=? AND walker_version=? ORDER BY from_sequence`, computerID, tapeRefFormat)
	if err != nil {
		return err
	}
	var groups [][]tapeRefChunk
	var cur []tapeRefChunk
	var span uint64
	for rows.Next() {
		var c tapeRefChunk
		if err := rows.Scan(&c.From, &c.To, &c.ReadEvents, &c.Anchor); err != nil {
			rows.Close()
			return err
		}
		if len(cur) > 0 && (span+(c.To-c.From) > maxEvents || c.ReadEvents != cur[0].ReadEvents || c.From != cur[len(cur)-1].To) {
			groups = append(groups, cur)
			cur, span = nil, 0
		}
		cur = append(cur, c)
		span += c.To - c.From
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if len(cur) > 0 {
		groups = append(groups, cur)
	}
	for _, g := range groups {
		if len(g) < 2 {
			continue
		}
		if err := s.mergeTapeRefGroup(ctx, computerID, g); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) mergeTapeRefGroup(ctx context.Context, computerID string, g []tapeRefChunk) error {
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var refs []uint64
	for _, c := range g {
		var blob []byte
		var sum string
		if err := tx.QueryRowContext(ctx, `SELECT refs, refs_sha256 FROM tape_sweep_ref_chunks WHERE computer_id=? AND from_sequence=?`, computerID, c.From).Scan(&blob, &sum); err != nil {
			return err
		}
		if len(blob)%8 != 0 || tapeRefsSum(blob) != sum {
			return fmt.Errorf("chunk (%d,%d] fails its checksum; not merged", c.From, c.To)
		}
		for i := 0; i < len(blob); i += 8 {
			refs = append(refs, binary.BigEndian.Uint64(blob[i:]))
		}
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i] < refs[j] })
	uniq := refs[:0]
	for i, v := range refs {
		if i == 0 || v != refs[i-1] {
			uniq = append(uniq, v)
		}
	}
	first, last := g[0], g[len(g)-1]
	blob := encodeTapeRefs(uniq)
	if _, err := tx.ExecContext(ctx, `DELETE FROM tape_sweep_ref_chunks WHERE computer_id=? AND from_sequence>=? AND from_sequence<?`, computerID, first.From, last.To); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO tape_sweep_ref_chunks (computer_id,from_sequence,to_sequence,read_events,anchor_event_digest,refs,refs_sha256,walker_version,created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
		computerID, first.From, last.To, first.ReadEvents, last.Anchor, blob, tapeRefsSum(blob), tapeRefFormat, time.Now().UTC()); err != nil {
		return err
	}
	return tx.Commit()
}
