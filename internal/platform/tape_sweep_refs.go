package platform

import (
	"context"
	"crypto/sha256"
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
// 8-byte digest prefixes that range contributed (the same prefixes the
// in-memory set takes: event digest, event pin, payload pins, and the
// payload digests the event file names), the checksum of that set, the
// event digest at `to` (the anchor) and whether the walk read event files.
//
// Why a resumed walk equals an uninterrupted one (the correctness argument).
//
//  1. A computer's chain is append-only and hash-chained: event n names the
//     canonical head after n-1, so the event digest at sequence n commits to
//     every event before it. The references of rows 1..n are a function of
//     those rows (the receipt's pins and the event file the digest names), so
//     they do not change while the chain at n is unchanged.
//  2. A chunk is used only when every one of these holds, otherwise ALL of the
//     computer's chunks are deleted and its walk restarts at sequence 0:
//     the chunks tile 1..k with no gap and no overlap, k is not above the
//     computer's current head, each chunk's checksum matches its set, each
//     chunk's anchor equals the event digest the receipt table holds at that
//     sequence now, and the chunk was walked with event reads whenever this
//     walk reads them (a chunk walked without reads lacks payload references;
//     one walked with reads serves a walk that does not need them, since
//     extra references only keep files).
//  3. So the union of the used chunks is the set an uninterrupted walk of
//     1..k would take, and the walk continues at k+1 as before. A chunk is
//     written only after its whole range was walked without error, in one
//     transaction, so an interrupt leaves either a complete chunk or none.
//  4. A chunk whose range held an unreadable event is written flagged as
//     walked without reads. A later walk that checks readability discards it
//     and re-walks, so the "payload references unknown" refusal is never
//     lost to a resume.
//  5. A prefix collision can only add a false reference. Nothing here removes
//     a reference a walk would have taken.
//
// The chain is never rewritten by the platform (rollback and restore append
// events; they do not edit rows), so the only ways stored progress goes
// stale are a restored control database, a purge, or tampering; the head and
// anchor checks catch the first, the purge deletes the chunks with the
// computer's other rows, and the checksum and anchor catch the rest.
//
// A restarted corpusd does NOT resume a sweep by itself: a restart never
// resumes work (CLAUDE.md, Restarts). The caller issues the dry-run again and
// it finds the chunks.

const (
	// defaultTapeRefChunkEvents is the events walked per persisted chunk:
	// about a minute of walking on the paper, a few MB of prefixes.
	defaultTapeRefChunkEvents = 50_000

	tapeRefChunksSchemaDDL = `
CREATE TABLE IF NOT EXISTS tape_sweep_ref_chunks (
  computer_id VARCHAR(128) NOT NULL,
  from_sequence BIGINT NOT NULL,
  to_sequence BIGINT NOT NULL,
  read_events BOOLEAN NOT NULL,
  anchor_event_digest CHAR(64) NOT NULL,
  refs BYTEA NOT NULL,
  refs_sha256 CHAR(64) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (computer_id, from_sequence)
);`
)

func (s *Store) ensureTapeRefChunksSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, tapeRefChunksSchemaDDL); err != nil {
		return fmt.Errorf("platform store: bootstrap tape sweep ref chunks: %w", err)
	}
	return nil
}

// tapeRefChunk is one persisted range of a computer's reference walk.
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

func decodeTapeRefs(b []byte) ([]uint64, bool) {
	if len(b)%8 != 0 {
		return nil, false
	}
	out := make([]uint64, len(b)/8)
	for i := range out {
		out[i] = binary.BigEndian.Uint64(b[8*i:])
	}
	return out, true
}

func tapeRefsSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// addPrefix adds an already-extracted prefix.
func (p *digestPrefixSet) addPrefix(v uint64) { p.v = append(p.v, v) }

func (s *Service) tapeRefChunkEvents() uint64 {
	if n := s.tapeSweep.refChunkEvents; n > 0 {
		return uint64(n)
	}
	return defaultTapeRefChunkEvents
}

func (s *Store) saveTapeRefChunk(ctx context.Context, computerID string, c tapeRefChunk) error {
	blob := encodeTapeRefs(c.Refs)
	_, err := s.db.ExecContext(ctx, `INSERT INTO tape_sweep_ref_chunks (computer_id,from_sequence,to_sequence,read_events,anchor_event_digest,refs,refs_sha256,created_at) VALUES (?,?,?,?,?,?,?,?)
		ON CONFLICT (computer_id, from_sequence) DO UPDATE SET to_sequence=excluded.to_sequence,read_events=excluded.read_events,anchor_event_digest=excluded.anchor_event_digest,refs=excluded.refs,refs_sha256=excluded.refs_sha256,created_at=excluded.created_at`,
		computerID, c.From, c.To, c.ReadEvents, c.Anchor, blob, tapeRefsSum(blob), time.Now().UTC())
	if err != nil {
		return fmt.Errorf("tape sweep: save reference chunk %s (%d,%d]: %w", computerID, c.From, c.To, err)
	}
	return nil
}

func (s *Store) deleteTapeRefChunks(ctx context.Context, computerID string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM tape_sweep_ref_chunks WHERE computer_id=?`, computerID); err != nil {
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

// loadTapeRefChunks returns the computer's stored chunks when they all verify
// (see the correctness argument above), else none, after deleting them.
// needReads is whether this walk reads event files. An error means the check
// could not run; the caller then walks from 0 without trusting anything.
func (s *Service) loadTapeRefChunks(ctx context.Context, computerID string, head uint64, needReads bool) ([]tapeRefChunk, error) {
	rows, err := s.store.db.QueryContext(ctx, `SELECT from_sequence,to_sequence,read_events,anchor_event_digest,refs,refs_sha256 FROM tape_sweep_ref_chunks WHERE computer_id=? ORDER BY from_sequence`, computerID)
	if err != nil {
		return nil, fmt.Errorf("tape sweep: load reference chunks of %s: %w", computerID, err)
	}
	var chunks []tapeRefChunk
	bad := ""
	next := uint64(0)
	for rows.Next() {
		var c tapeRefChunk
		var blob []byte
		var sum string
		if err := rows.Scan(&c.From, &c.To, &c.ReadEvents, &c.Anchor, &blob, &sum); err != nil {
			rows.Close()
			return nil, fmt.Errorf("tape sweep: scan reference chunk of %s: %w", computerID, err)
		}
		refs, ok := decodeTapeRefs(blob)
		switch {
		case bad != "":
		case c.From != next || c.To <= c.From:
			bad = fmt.Sprintf("chunk (%d,%d] does not continue at %d", c.From, c.To, next)
		case c.To > head:
			bad = fmt.Sprintf("chunk to %d is above the head %d", c.To, head)
		case needReads && !c.ReadEvents:
			bad = fmt.Sprintf("chunk (%d,%d] was walked without event reads", c.From, c.To)
		case !ok || tapeRefsSum(blob) != sum:
			bad = fmt.Sprintf("chunk (%d,%d] fails its checksum", c.From, c.To)
		}
		if bad == "" {
			c.Refs = refs
			chunks = append(chunks, c)
			next = c.To
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("tape sweep: iterate reference chunks of %s: %w", computerID, err)
	}
	rows.Close()
	if bad == "" {
		for _, c := range chunks {
			anchor, err := s.tapeAnchorAt(ctx, computerID, c.To)
			if err != nil {
				bad = fmt.Sprintf("anchor at %d unreadable: %v", c.To, err)
				break
			}
			if anchor != c.Anchor {
				bad = fmt.Sprintf("anchor at %d is %s, stored %s", c.To, anchor, c.Anchor)
				break
			}
		}
	}
	if bad != "" {
		log.Printf("tape sweep: computer=%s reference progress discarded: %s", computerID, bad)
		if err := s.store.deleteTapeRefChunks(ctx, computerID); err != nil {
			return nil, err
		}
		return nil, nil
	}
	return chunks, nil
}

// tapeRefPersistError is a failure to persist progress (or a test restart).
// It ends the run: unlike a chain read error it says nothing about the chain.
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
// tapeRefChunkEvents rows walked. A read or persist error stops the walk;
// rows added so far stay in set (the caller treats the walk as failed).
func (s *Service) walkTapeRefs(ctx context.Context, computerID string, head uint64, needReads, checkReadable bool, set *digestPrefixSet, unreadable func(tapeChainRow)) (tapeRefWalk, error) {
	var out tapeRefWalk
	chunks, err := s.loadTapeRefChunks(ctx, computerID, head, needReads)
	if err != nil {
		// Could not verify stored progress: walk everything, trust nothing.
		log.Printf("tape sweep: %v; walking computer=%s from 0", err, computerID)
		chunks = nil
	}
	pos := uint64(0)
	for _, c := range chunks {
		for _, v := range c.Refs {
			set.addPrefix(v)
		}
		pos = c.To
	}
	out.Resumed = pos

	size := s.tapeRefChunkEvents()
	for pos < head {
		end := min(head, pos+size)
		local := map[uint64]struct{}{}
		allReadable := needReads
		anchor := ""
		visited := uint64(0)
		err := s.walkTapeChain(ctx, computerID, pos, end, needReads, func(row tapeChainRow) error {
			visited++
			add := func(d string) {
				if v, ok := digestPrefix(d); ok {
					set.addPrefix(v)
					local[v] = struct{}{}
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
		if hook := s.tapeSweep.afterRefChunk; hook != nil {
			if err := hook(computerID, end); err != nil {
				return out, &tapeRefPersistError{err}
			}
		}
		pos = end
	}
	return out, nil
}
