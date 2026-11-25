package auth

// After a restore from backup, an account that was deleted since the backup
// was taken is back: its row, its passkeys, its mail. The runner writes every
// finished deletion to a ledger file outside auth.db (internal/accountpurge,
// account-deletion-ledger.jsonl), and this reads the ledger against a restored
// auth.db: every user whose hash is in it is a deletion to apply again.
// Applying it again is the ordinary path: a deletion record old enough to
// purge, which the purge runner then carries out against every service
// (mail, data, computers, gateway, tape), so nothing restored survives.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/purgeguard"
)

// RestoredDeletion is a user in a restored auth.db whose deletion was finished
// after the backup.
type RestoredDeletion struct {
	UserID   string
	UserHash string
	JobID    string
	// Reapplied is true when a deletion record was written for it.
	Reapplied bool
}

// ReadDeletionLedger reads the ledger's user hashes, with the job of each.
func ReadDeletionLedger(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("deletion ledger: %w", err)
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	// A bad line is an error, except a last line with no newline: a crash in
	// the middle of an append leaves one, and the runner writes it again.
	badLine := 0
	for n := 1; sc.Scan(); n++ {
		if badLine != 0 {
			return nil, fmt.Errorf("deletion ledger line %d is not a ledger line", badLine)
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var l struct {
			UserHash string `json:"user_hash"`
			JobID    string `json:"job_id"`
		}
		if err := json.Unmarshal([]byte(line), &l); err != nil || l.UserHash == "" {
			badLine = n
			continue
		}
		out[l.UserHash] = l.JobID
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if badLine != 0 {
		if st, err := f.Stat(); err == nil && st.Size() > 0 {
			last := make([]byte, 1)
			if _, err := f.ReadAt(last, st.Size()-1); err == nil && last[0] == '\n' {
				return nil, fmt.Errorf("deletion ledger line %d is not a ledger line", badLine)
			}
		}
	}
	return out, nil
}

// RestoredDeletions finds the users of this (restored) store whose deletion
// the ledger says was finished, and, when reapply is set, writes each a
// deletion record already past its grace period so the purge runner finishes
// the job against every service on its next pass, and starts the steps of
// every live deletion record again.
func (s *Store) RestoredDeletions(ledger map[string]string, reapply bool, now time.Time) ([]RestoredDeletion, error) {
	rows, err := s.db.Query(`SELECT id FROM users`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	var out []RestoredDeletion
	for _, id := range ids {
		hash := DeletionUserHash(id)
		job, ok := ledger[hash]
		if !ok {
			continue
		}
		r := RestoredDeletion{UserID: id, UserHash: hash, JobID: job}
		if reapply {
			if _, _, err := s.RequestAccountDeletion(id, now.Add(-purgeguard.GracePeriod-time.Minute)); err != nil {
				return out, fmt.Errorf("reapply a deletion: %w", err)
			}
			r.Reapplied = true
		}
		out = append(out, r)
	}
	if reapply {
		// Every live record starts its steps again. The services were
		// restored from copies that can predate a step the restored auth.db
		// records as done (the backup stages PostgreSQL before auth.db), so a
		// step skipped as done would leave restored rows behind for good.
		// Every step is idempotent; the at-once steps (stop, suspend, the
		// gateway identity) are asserted again too.
		if _, err := s.db.Exec(`UPDATE account_deletions SET steps = '{}', attempt_started_at = NULL, updated_at = ? WHERE `+liveDeletion, now.UTC()); err != nil {
			return out, fmt.Errorf("reset the steps of live deletions: %w", err)
		}
	}
	return out, nil
}
