package store

// The store copy an engineering capsule gets at /computer/state.db
// (docs/design/capsule-overlay-computer-state-2026-11-03.md, slice S3). A
// private copy as data: VACUUM INTO in one read transaction, then, on the
// copy only, the secret-shape scan of every text column, blanking under
// secure_delete and a second VACUUM, and a close to one file. The live store
// is read once and never written.

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/storeschema"
)

// SnapshotReceipt says what one store copy holds.
type SnapshotReceipt struct {
	// ComputerID, Sequence and Head are the event head the copy holds;
	// empty on a store with no projection head.
	ComputerID string `json:"computer_id,omitempty"`
	Sequence   uint64 `json:"sequence,omitempty"`
	Head       string `json:"head,omitempty"`
	Bytes      int64  `json:"bytes"`
	CopyMS     int64  `json:"copy_ms"`
	ScanMS     int64  `json:"scan_ms"`
	// ScannedBytes is the text the secret-shape scan read; Blanked counts
	// the values it blanked, by table.column (never the value).
	ScannedBytes int64          `json:"scanned_bytes"`
	Blanked      map[string]int `json:"blanked,omitempty"`
	// LeftOut names the sidecar parts the copy does not carry.
	LeftOut []string `json:"left_out,omitempty"`
}

// snapshotSidecarFiles are the sidecar files a reopened store reads: the
// floor, the schema receipt and the lease-untrust marker, and the two
// vocabulary files that travel with published bases (the cutover report arms
// the write fence; the deposit ledger resolves legacy ids). Telemetry
// partitions and image blobs are not carried.
var snapshotSidecarFiles = []string{storeschema.FloorFile, storeschema.File, storeschema.LeaseUntrustedFile, vocabReportFileName, depositUpcastLedgerFileName}

// SnapshotEstimate is the bytes a copy will take: the store's live pages.
func (s *Store) SnapshotEstimate(ctx context.Context) (int64, error) {
	if s == nil || s.ReadDB() == nil {
		return 0, errors.New("store: no store")
	}
	var pageCount, freelist, pageSize int64
	for q, dst := range map[string]*int64{"PRAGMA page_count": &pageCount, "PRAGMA freelist_count": &freelist, "PRAGMA page_size": &pageSize} {
		if err := s.ReadDB().QueryRowContext(ctx, q).Scan(dst); err != nil {
			return 0, fmt.Errorf("%s: %w", q, err)
		}
	}
	return (pageCount - freelist) * pageSize, nil
}

// SnapshotTo writes a private copy of the store into dir: dir/state.db (one
// file, no WAL) and dir/state.texture/ with the sidecar files Open reads.
// dir must exist; any earlier copy in it is replaced. On error nothing it
// wrote is left.
func (s *Store) SnapshotTo(ctx context.Context, dir string) (SnapshotReceipt, error) {
	receipt := SnapshotReceipt{}
	if s == nil || s.statePath == "" || s.ReadDB() == nil {
		return receipt, errors.New("store: no store to copy")
	}
	dir = filepath.Clean(dir)
	// Never the live store's own directory: the cleanup below removes
	// dir/state.db and dir/state.texture before the copy.
	if live, err := filepath.Abs(filepath.Dir(s.statePath)); err == nil {
		if target, err := filepath.Abs(dir); err != nil || target == live {
			return receipt, errors.New("store: a snapshot into the live store's directory is refused")
		}
	}
	final := filepath.Join(dir, "state.db")
	tmp := filepath.Join(dir, ".state.db.snapshot-tmp")
	sidecar := filepath.Join(dir, "state.texture")
	cleanup := func() {
		removeSQLiteFiles(tmp)
		removeSQLiteFiles(final)
		_ = os.RemoveAll(sidecar)
	}
	cleanup()
	started := time.Now()
	err := vacuumIntoCopy(ctx, s.statePath, tmp)
	// The copy's read transaction pinned the live WAL for its duration.
	s.CheckpointAfterLongRead(ctx, "capsule store copy")
	if err != nil {
		cleanup()
		return receipt, fmt.Errorf("copy: %w", err)
	}
	receipt.CopyMS = time.Since(started).Milliseconds()
	scanStarted := time.Now()
	if err := sanitizeSnapshot(ctx, tmp, &receipt); err != nil {
		cleanup()
		return receipt, err
	}
	receipt.ScanMS = time.Since(scanStarted).Milliseconds()
	if err := fsyncFile(tmp); err != nil {
		cleanup()
		return receipt, err
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(tmp + suffix); err == nil {
			cleanup()
			return receipt, fmt.Errorf("copy left %s beside it", suffix)
		}
	}
	if err := os.Mkdir(sidecar, 0o755); err != nil {
		cleanup()
		return receipt, err
	}
	for _, name := range snapshotSidecarFiles {
		if err := copyRegularFile(filepath.Join(s.texturePath, name), filepath.Join(sidecar, name)); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			cleanup()
			return receipt, fmt.Errorf("sidecar %s: %w", name, err)
		}
	}
	if entries, err := os.ReadDir(s.texturePath); err == nil {
		kept := map[string]bool{}
		for _, name := range snapshotSidecarFiles {
			kept[name] = true
		}
		for _, entry := range entries {
			if !kept[entry.Name()] {
				receipt.LeftOut = append(receipt.LeftOut, "state.texture/"+entry.Name())
			}
		}
	}
	// The image blobs beside the store (imageblob.DirForStorePath): images
	// in documents show as missing in the capsule.
	if blobs := deriveSuffixedWorkspacePath(s.path, ".image-blobs"); s.path != "" {
		if _, err := os.Stat(blobs); err == nil {
			receipt.LeftOut = append(receipt.LeftOut, filepath.Base(blobs))
		}
	}
	sort.Strings(receipt.LeftOut)
	if err := os.Rename(tmp, final); err != nil {
		cleanup()
		return receipt, err
	}
	info, err := os.Stat(final)
	if err != nil {
		cleanup()
		return receipt, err
	}
	receipt.Bytes = info.Size()
	return receipt, nil
}

// sanitizeSnapshot runs on the copy only: it reads the head the copy holds,
// scans every text value with the tape's private-secret patterns, blanks
// each hit under secure_delete, vacuums again when anything was blanked, and
// closes the copy to one file.
func sanitizeSnapshot(ctx context.Context, path string, receipt *SnapshotReceipt) error {
	db, err := sql.Open("sqlite", dbx.SQLiteDSN(path, dbx.SQLiteOptions{}, false))
	if err != nil {
		return err
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if head, err := snapshotHead(ctx, conn); err != nil {
		return fmt.Errorf("read head: %w", err)
	} else {
		receipt.ComputerID, receipt.Sequence, receipt.Head = head.ComputerID, head.Sequence, head.Head
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA secure_delete=ON`); err != nil {
		return fmt.Errorf("secure_delete: %w", err)
	}
	tables, err := snapshotTables(ctx, conn)
	if err != nil {
		return err
	}
	// The placeholder names a hit without letting it be checked offline: a
	// digest under a salt drawn for this copy only (distinct values stay
	// distinct, so a UNIQUE column keeps holding).
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	hitsTotal := 0
	for _, table := range tables {
		cols, err := snapshotColumns(ctx, conn, table)
		if err != nil {
			return err
		}
		for _, col := range cols {
			hits, scanned, err := scanColumnForSecrets(ctx, conn, table, col)
			if err != nil {
				return err
			}
			receipt.ScannedBytes += scanned
			for _, hit := range hits {
				placeholder := "[redacted:" + shortDigest(append(append([]byte(nil), salt...), hit.raw...)) + "]"
				var match any = string(hit.raw)
				if hit.blob {
					match = hit.raw
				}
				res, err := conn.ExecContext(ctx, `UPDATE `+sqlIdent(table)+` SET `+sqlIdent(col)+` = ? WHERE `+sqlIdent(col)+` = ?`, placeholder, match)
				if err != nil {
					return fmt.Errorf("blank a secret-shaped value in %s.%s: %w", table, col, err)
				}
				n, err := res.RowsAffected()
				if err != nil || n == 0 {
					// A flagged value the update did not reach never ships.
					return fmt.Errorf("blank a secret-shaped value in %s.%s: no row was changed (%v)", table, col, err)
				}
				if receipt.Blanked == nil {
					receipt.Blanked = map[string]int{}
				}
				receipt.Blanked[table+"."+col] += int(n)
				hitsTotal++
			}
		}
	}
	if hitsTotal > 0 {
		// secure_delete zeroed the freed cells; the second VACUUM rewrites
		// the file so no old page remains.
		if _, err := conn.ExecContext(ctx, `VACUUM`); err != nil {
			return fmt.Errorf("vacuum after blanking: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, `PRAGMA journal_mode=DELETE`); err != nil {
		return err
	}
	return nil
}

func snapshotHead(ctx context.Context, conn *sql.Conn) (compactSidecar, error) {
	rows, err := conn.QueryContext(ctx, `SELECT computer_id, sequence, canonical_event_head FROM computer_event_projection_heads`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return compactSidecar{}, nil
		}
		return compactSidecar{}, err
	}
	defer rows.Close()
	var out compactSidecar
	n := 0
	for rows.Next() {
		if err := rows.Scan(&out.ComputerID, &out.Sequence, &out.Head); err != nil {
			return compactSidecar{}, err
		}
		n++
	}
	if err := rows.Err(); err != nil {
		return compactSidecar{}, err
	}
	if n > 1 {
		return compactSidecar{}, fmt.Errorf("%d projection heads, want at most one", n)
	}
	return out, nil
}

func snapshotTables(ctx context.Context, conn *sql.Conn) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// secretHit is one distinct value that holds a private-secret pattern, with
// its storage class, so the blanking matches it by the same type.
type secretHit struct {
	raw  []byte
	blob bool
}

// scanColumnForSecrets returns the distinct text or blob values of one column
// that hold a private-secret pattern, and the bytes it read.
func scanColumnForSecrets(ctx context.Context, conn *sql.Conn, table, col string) ([]secretHit, int64, error) {
	rows, err := conn.QueryContext(ctx, `SELECT DISTINCT typeof(`+sqlIdent(col)+`), CAST(`+sqlIdent(col)+` AS BLOB) FROM `+sqlIdent(table)+` WHERE typeof(`+sqlIdent(col)+`) IN ('text', 'blob')`)
	if err != nil {
		return nil, 0, fmt.Errorf("scan %s.%s: %w", table, col, err)
	}
	defer rows.Close()
	var hits []secretHit
	var scanned int64
	for rows.Next() {
		var kind string
		var raw []byte
		if err := rows.Scan(&kind, &raw); err != nil {
			return nil, 0, fmt.Errorf("scan %s.%s: %w", table, col, err)
		}
		scanned += int64(len(raw))
		if len(computerevent.LocatePrivateSecrets(raw)) > 0 {
			hits = append(hits, secretHit{raw: append([]byte(nil), raw...), blob: kind == "blob"})
		}
	}
	return hits, scanned, rows.Err()
}

func snapshotColumns(ctx context.Context, conn *sql.Conn, table string) ([]string, error) {
	rows, err := conn.QueryContext(ctx, `SELECT name FROM pragma_table_info(?, 'main')`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func sqlIdent(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

func shortDigest(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:8])
}

func copyRegularFile(src, dst string) error {
	// O_NOFOLLOW: a symlink in the live sidecar is refused, never followed.
	in, err := os.OpenFile(src, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close())
}
