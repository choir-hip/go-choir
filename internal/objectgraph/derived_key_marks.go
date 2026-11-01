package objectgraph

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

// The derived-key marks (docs/design/derived-key-marks-2026-11-01.md): one
// sentinel row per family in og_key_index that says "this family's key rows
// were complete and current when the projection head was H". It is the only
// persisted validity, and it is bound to the head, which every og_objects
// writer that does not maintain the table moves in its own transaction.
//
// The row: canonical_id derivedKeyMarkID (outside every kind's id range),
// key_name the family, key_value NULL (no equality read can match it),
// owner_id the mark identity (generation and the family's definition),
// content_hash the head fingerprint. An older binary's pass drops it as an
// orphan key row outside the family's range; this binary's pass skips it.

// derivedKeyMarkID is the canonical_id of a family's mark row.
const derivedKeyMarkID = ""

// derivedKeyMarkGeneration is the mark format and the maintenance contract
// it vouches for. Raise it when a change makes marks written by this binary
// untrustworthy to the next one (a writer that stops maintaining the table).
const derivedKeyMarkGeneration = 1

// DerivedKeyTx is the part of *sql.Tx (or *sql.DB) the marks need.
type DerivedKeyTx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// DerivedKeyMarkHead reads the fingerprint the marks are bound to, inside q's
// transaction. An empty fingerprint means the store has no single head: no
// mark is trusted or stamped.
type DerivedKeyMarkHead func(ctx context.Context, q DerivedKeyTx) (string, error)

// markIdentity names the generation and the family's definition, so a mark
// written for another expression or kind is never trusted.
func (fam *derivedFamily) markIdentity() string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%t", derivedKeyMarkGeneration, fam.Name, fam.Kind, fam.expr, fam.ordered)))
	return fmt.Sprintf("og_key_index mark g%d %s", derivedKeyMarkGeneration, hex.EncodeToString(sum[:8]))
}

// BindMarkHead gives the side table the store's head fingerprint. Without it
// no mark is trusted or stamped, and every open validates as before.
func (d *DerivedKeys) BindMarkHead(fn DerivedKeyMarkHead) {
	if d != nil {
		d.markHead.Store(&fn)
	}
}

func (d *DerivedKeys) headFingerprint(ctx context.Context, q DerivedKeyTx) (string, error) {
	fn := d.markHead.Load()
	if fn == nil || *fn == nil {
		return "", nil
	}
	return (*fn)(ctx, q)
}

const markUpsertSQL = `INSERT INTO ` + DerivedKeysTable + ` (canonical_id, key_name, key_value, owner_id, updated_at, content_hash, derive_ok)
	VALUES (?, ?, NULL, ?, ?, ?, 1)
	ON CONFLICT (canonical_id, key_name) DO UPDATE SET
		key_value = NULL, owner_id = excluded.owner_id, updated_at = excluded.updated_at,
		content_hash = excluded.content_hash, derive_ok = 1`

const markDeleteSQL = `DELETE FROM ` + DerivedKeysTable + ` WHERE canonical_id = '' AND key_name = ?`

// dropMark deletes one family's mark inside the caller's transaction. A
// failure is logged: the next head-moving transaction or pass deletes it.
func (d *DerivedKeys) dropMark(ctx context.Context, x DerivedKeyExecer, name string) {
	if _, err := x.ExecContext(ctx, markDeleteSQL, name); err != nil {
		log.Printf("objectgraph sql: derived keys %s: delete mark: %v", name, err)
	}
}

// StampMarks binds the marks to the head the caller's transaction has just
// written: the mark of every family readable now is written with that head,
// every other family's mark is deleted. Call it in every transaction that
// moves the projection head, after its og_objects writes and their
// maintenance. It never fails the transaction: a mark that could not be
// written still names the old head, which no longer matches.
func (d *DerivedKeys) StampMarks(ctx context.Context, tx DerivedKeyTx) {
	if d == nil || !d.tableReady.Load() {
		return
	}
	head, err := d.headFingerprint(ctx, tx)
	if err != nil {
		log.Printf("objectgraph sql: derived keys: read the head for the marks: %v", err)
		head = ""
	}
	var drop []any
	for i := range derivedFamilies {
		fam := &derivedFamilies[i]
		if head != "" && d.readable(fam.Name) {
			if _, err := tx.ExecContext(ctx, markUpsertSQL, derivedKeyMarkID, fam.Name, fam.markIdentity(), time.Now().UTC(), head); err == nil {
				continue
			} else {
				log.Printf("objectgraph sql: derived keys %s: stamp mark: %v", fam.Name, err)
			}
		}
		drop = append(drop, fam.Name)
	}
	if len(drop) == 0 {
		return
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(drop)), ",")
	if _, err := tx.ExecContext(ctx, `DELETE FROM `+DerivedKeysTable+` WHERE canonical_id = '' AND key_name IN (`+marks+`)`, drop...); err != nil {
		log.Printf("objectgraph sql: derived keys: delete marks: %v", err)
	}
}

// stampFamilyMark is a pass's publication made durable: in its own
// transaction, the family's mark is written with the current head if the
// family is still readable, and deleted if not.
// A store with no head binding stamps nothing and leaves the write connection
// alone (and a mark could not have been written on it by this binary).
func (s *SQLStore) stampFamilyMark(ctx context.Context, fam *derivedFamily) error {
	if fn := s.derived.markHead.Load(); fn == nil || *fn == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	head, err := s.derived.headFingerprint(ctx, tx)
	if err != nil {
		return err
	}
	if head != "" && s.derived.readable(fam.Name) {
		_, err = tx.ExecContext(ctx, markUpsertSQL, derivedKeyMarkID, fam.Name, fam.markIdentity(), time.Now().UTC(), head)
	} else {
		_, err = tx.ExecContext(ctx, markDeleteSQL, fam.Name)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

// deleteFamilyMark durably deletes a family's mark, before a pass over it. It
// looks on readDB first, so a pass over a family with no mark (the usual
// case: an unvalidated family's mark went with the withdrawal) does not wait
// for the write connection.
func (s *SQLStore) deleteFamilyMark(ctx context.Context, readDB *sql.DB, fam *derivedFamily) error {
	var one int
	err := readDB.QueryRowContext(ctx, `SELECT 1 FROM `+DerivedKeysTable+` WHERE canonical_id = '' AND key_name = ?`, fam.Name).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, markDeleteSQL, fam.Name)
	return err
}

// TrustDerivedKeyMarks runs once at open, before any writer: a family whose
// mark names this binary's generation and definition and the store's current
// head is validated without a pass. Every other mark is deleted. It reads
// four rows by primary key and the head; nothing scales with og_objects. It
// returns the trusted families. A failure trusts nothing and keeps the boot.
func (s *SQLStore) TrustDerivedKeyMarks(ctx context.Context) []string {
	if s == nil || s.derived == nil || !s.derived.tableReady.Load() {
		return nil
	}
	done := BootPhase("store: open", "objectgraph-schema/derived-key-marks")
	trusted, err := s.trustDerivedKeyMarks(ctx)
	done(err)
	if err != nil {
		log.Printf("objectgraph sql: derived keys: marks not read, every family validates online: %v", err)
		return nil
	}
	return trusted
}

func (s *SQLStore) trustDerivedKeyMarks(ctx context.Context) ([]string, error) {
	d := s.derived
	head, err := d.headFingerprint(ctx, s.db)
	if err != nil {
		return nil, err
	}
	var trusted, untrusted []string
	for i := range derivedFamilies {
		fam := &derivedFamilies[i]
		var identity, markHead string
		err := s.db.QueryRowContext(ctx, `SELECT owner_id, content_hash FROM `+DerivedKeysTable+` WHERE canonical_id = '' AND key_name = ?`, fam.Name).Scan(&identity, &markHead)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return trusted, err
		}
		if head == "" || identity != fam.markIdentity() || markHead != head {
			untrusted = append(untrusted, fam.Name)
			if _, err := s.db.ExecContext(ctx, markDeleteSQL, fam.Name); err != nil {
				return trusted, err
			}
			continue
		}
		d.pubMu.Lock()
		d.bad[fam.Name].Store(false)
		d.passWrites[fam.Name].Store(d.writes[fam.Name].Load())
		d.validated[fam.Name].Store(true)
		d.pubMu.Unlock()
		trusted = append(trusted, fam.Name)
	}
	log.Printf("objectgraph sql: derived keys: %d families trusted from their marks at the store's head (%s), no validation pass for them; %d marks distrusted and deleted (%s)",
		len(trusted), strings.Join(trusted, ","), len(untrusted), strings.Join(untrusted, ","))
	return trusted, nil
}
