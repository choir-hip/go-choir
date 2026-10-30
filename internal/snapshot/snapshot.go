// Package snapshot is snapshot version 1: the guest SQLite store written with
// VACUUM INTO, sealed under the computer's key, stored chunk by chunk in the
// platform's file content store, and named on the tape by a synchronous
// snapshot_recorded event (docs/design/data-system-build-sheet-2026-10-30.md,
// decision 7). It replaces the projection bases.
//
// Storage reuses the file CAS: the file is cut into 4 MiB chunks, each sealed
// with filecas.SealChunk (deterministic per key and plaintext, so successive
// snapshots share unchanged chunks) and PUT under the computer's chunk
// namespace. The manifest of chunk digests, with the store head sequence, the
// file-tree root and the witness v3 root, is the private payload of the
// snapshot_recorded event (computerevent.PrivateArtifactCipher), so the sealed
// artifact digest on the tape names the snapshot and nothing readable leaves
// the computer.
//
// Boot reads the computer's own disk. A missing disk is rebuilt by Restore:
// the newest snapshot_recorded on the tape is fetched and unsealed, installed
// as the state file, and the existing tape replay applies the events after its
// sequence.
package snapshot

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/dbx"
	"github.com/yusefmosiah/go-choir/internal/filecas"
	"github.com/yusefmosiah/go-choir/internal/selfdevprotocol"
)

const (
	// Version is the snapshot format version.
	Version = 1
	// RecordMediaType is the media type of the sealed snapshot record.
	RecordMediaType = "application/vnd.choir.snapshot-record+json"
	// StateFileName is the manifest path of the SQLite file.
	StateFileName = "state.db"
	// ChunkSize is the plaintext size of one snapshot chunk.
	ChunkSize = 4 << 20

	authorityRef = "authority:guest-snapshot"
	// scanPageSize is the replay page size Restore scans the tape with.
	scanPageSize = computerevent.EventReplayPageSize
)

// ErrTooLarge means the copy exceeds Config.MaxPlaintextBytes.
var ErrTooLarge = errors.New("snapshot: store copy exceeds the size cap")

// ErrNoSnapshot means the tape holds no usable snapshot_recorded event.
var ErrNoSnapshot = errors.New("snapshot: no snapshot on the tape")

// Record is the sealed payload a snapshot_recorded event cites.
type Record struct {
	Version    int    `json:"version"`
	ComputerID string `json:"computer_id"`
	CreatedAt  string `json:"created_at"`
	// StoreHeadSequence and StoreHead are the event head the file captures:
	// replay resumes with the event after StoreHeadSequence.
	StoreHeadSequence uint64 `json:"store_head_sequence"`
	StoreHead         string `json:"store_head"`
	// PlaintextSize is the size of the SQLite file in bytes.
	PlaintextSize int64 `json:"plaintext_size"`
	// FileTreeRoot is the file CAS root of the Files tree at the snapshot,
	// when the computer has one.
	FileTreeRoot string `json:"file_tree_root,omitempty"`
	// WitnessRoot and SchemaHash are the witness v3 content root and schema
	// hash of the captured file.
	WitnessRoot string `json:"witness_root"`
	SchemaHash  string `json:"schema_hash"`
	// Manifest addresses the sealed chunks of the file (one entry,
	// StateFileName).
	Manifest filecas.Manifest `json:"manifest"`
}

// Ref names a recorded snapshot.
type Ref struct {
	// ArtifactDigest is the sealed record's digest: the snapshot_recorded
	// event's payload commitment.
	ArtifactDigest string
	// EventID is the snapshot_recorded event.
	EventID string
	Record  Record
}

// ChunkStore moves sealed chunks to and from the platform's file CAS.
type ChunkStore interface {
	PutChunk(ctx context.Context, digest string, sealed []byte) error
	GetChunk(ctx context.Context, digest string) ([]byte, error)
}

// RootRegistrar is implemented by a ChunkStore whose host sweeps unreferenced
// chunks (HTTPChunks): TakeSnapshot declares the snapshot's chunk digests
// through it, before the snapshot_recorded event is appended, because the host
// cannot read the sealed manifest and would otherwise collect the chunks.
type RootRegistrar interface {
	RegisterSnapshotRoot(ctx context.Context, root string, headSequence uint64, chunks []string) error
}

// Appender is the slice of computerevent.ComputerEventAppender TakeSnapshot
// needs. It does not include HoldAppends: the capture takes no append hold.
type Appender interface {
	AppendNewPrivatePayload(ctx context.Context, event computerevent.Event, input computerevent.TransitionInput, plaintext []byte, mediaType string, cipher *computerevent.PrivateArtifactCipher) (computerevent.Receipt, string, error)
}

// Config is what TakeSnapshot needs.
type Config struct {
	ComputerID string
	// DB is the live store's READ pool (dbx SQLiteDB.Read, mode=ro), never the
	// single write connection: VACUUM INTO reads one committed snapshot
	// through it while writers keep committing, so a 30 GB store blocks no
	// write and no append for the minutes the copy takes.
	DB *sql.DB
	// TempDir holds the VACUUM INTO file until it is uploaded. It must have
	// room for the store: the data volume beside the store, not tmpfs.
	TempDir string
	// Head reads the event head from a handle on the COPY (nil for an empty
	// store). The copy carries the store's own head and projection tables,
	// committed in the same transactions as the state, so the head read there
	// is exactly the head the file captures; a head read from the live store
	// would race the writers.
	Head func(ctx context.Context, snapshotDB *sql.DB) (*computerevent.Head, error)
	// Witness computes the witness v3 of a read handle on the snapshot copy,
	// with the same extractor configuration as the live witness.
	Witness func(ctx context.Context, snapshotDB *sql.DB) (selfdevprotocol.VMLocalContentWitness, error)
	// FileTreeRoot optionally names the Files tree root to record.
	FileTreeRoot func(ctx context.Context) (string, error)
	Cipher       *computerevent.PrivateArtifactCipher
	Chunks       ChunkStore
	Appender     Appender
	// MaxPlaintextBytes, when positive, refuses a copy larger than it before
	// anything is uploaded (ErrTooLarge).
	MaxPlaintextBytes int64
	// Now overrides the clock in tests.
	Now func() time.Time
}

func (c Config) validate() error {
	switch {
	case strings.TrimSpace(c.ComputerID) == "":
		return errors.New("snapshot: computer id is required")
	case c.DB == nil:
		return errors.New("snapshot: store is required")
	case c.Head == nil || c.Witness == nil:
		return errors.New("snapshot: head and witness readers are required")
	case c.Cipher == nil || c.Chunks == nil || c.Appender == nil:
		return errors.New("snapshot: cipher, chunk store and appender are required")
	}
	return nil
}

// TakeSnapshot writes the store with VACUUM INTO on the read pool, seals and
// stores it, and appends the snapshot_recorded event. Nothing is held: writers
// and appends continue while the copy is made. VACUUM INTO reads one
// consistent committed state, the head and the witness are both read from the
// copy, and the events committed after the copy are the tape tail Restore
// replays.
func TakeSnapshot(ctx context.Context, cfg Config) (Ref, error) {
	if err := cfg.validate(); err != nil {
		return Ref{}, err
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	tempDir := cfg.TempDir
	if tempDir == "" {
		tempDir = os.TempDir()
	}
	if err := os.MkdirAll(tempDir, 0o700); err != nil {
		return Ref{}, fmt.Errorf("snapshot: temp dir: %w", err)
	}
	// VACUUM INTO refuses an existing file, so reserve a name and remove it.
	reserve, err := os.CreateTemp(tempDir, "snapshot-*.db")
	if err != nil {
		return Ref{}, fmt.Errorf("snapshot: temp file: %w", err)
	}
	copyPath := reserve.Name()
	_ = reserve.Close()
	_ = os.Remove(copyPath)
	defer func() {
		_ = os.Remove(copyPath)
		_ = os.Remove(copyPath + "-wal")
		_ = os.Remove(copyPath + "-shm")
	}()

	if _, err := cfg.DB.ExecContext(ctx, "VACUUM INTO ?", copyPath); err != nil {
		return Ref{}, fmt.Errorf("snapshot: vacuum into: %w", err)
	}
	if cfg.MaxPlaintextBytes > 0 {
		if info, err := os.Stat(copyPath); err == nil && info.Size() > cfg.MaxPlaintextBytes {
			return Ref{}, fmt.Errorf("%w: %d bytes, cap %d", ErrTooLarge, info.Size(), cfg.MaxPlaintextBytes)
		}
	}

	// The copy is a rollback-journal file; the dbx opener switches it to WAL,
	// which a read-only open cannot do. Closing it checkpoints and removes the
	// WAL, leaving a complete file.
	snapDB, err := dbx.OpenSQLite(copyPath, dbx.SQLiteOptions{ReadConns: 1})
	if err != nil {
		return Ref{}, fmt.Errorf("snapshot: open copy: %w", err)
	}
	head, herr := cfg.Head(ctx, snapDB.Read)
	var witness selfdevprotocol.VMLocalContentWitness
	var werr error
	if herr == nil {
		witness, werr = cfg.Witness(ctx, snapDB.Read)
	}
	_ = snapDB.Close()
	if herr != nil {
		return Ref{}, fmt.Errorf("snapshot: read head of the copy: %w", herr)
	}
	if werr != nil {
		return Ref{}, fmt.Errorf("snapshot: witness of the copy: %w", werr)
	}

	manifest, size, err := uploadFile(ctx, cfg, copyPath, now())
	if err != nil {
		return Ref{}, err
	}
	record := Record{
		Version:       Version,
		ComputerID:    cfg.ComputerID,
		CreatedAt:     now().UTC().Format(time.RFC3339Nano),
		PlaintextSize: size,
		WitnessRoot:   witness.ContentRoot,
		SchemaHash:    witness.SchemaHash,
		Manifest:      *manifest,
	}
	if head != nil {
		record.StoreHeadSequence = head.Sequence
		record.StoreHead = head.CanonicalEventHead
	}
	if cfg.FileTreeRoot != nil {
		root, err := cfg.FileTreeRoot(ctx)
		if err != nil {
			return Ref{}, fmt.Errorf("snapshot: file tree root: %w", err)
		}
		record.FileTreeRoot = root
	}
	if registrar, ok := cfg.Chunks.(RootRegistrar); ok {
		if err := registrar.RegisterSnapshotRoot(ctx, manifest.Root, record.StoreHeadSequence, manifest.Files[0].Chunks); err != nil {
			return Ref{}, fmt.Errorf("snapshot: register snapshot root: %w", err)
		}
	}
	plaintext, err := json.Marshal(record)
	if err != nil {
		return Ref{}, fmt.Errorf("snapshot: encode record: %w", err)
	}
	eventID, err := computerevent.NewEventID()
	if err != nil {
		return Ref{}, err
	}
	event := computerevent.Event{
		SchemaVersion:  computerevent.SchemaVersionV1,
		EventID:        eventID,
		ComputerID:     cfg.ComputerID,
		EventKind:      computerevent.EventSnapshotRecorded,
		OccurredAt:     now().UTC().Format(time.RFC3339Nano),
		IdempotencyKey: "snapshot:" + manifest.Root,
		ActorProfile:   agentprofile.Management,
		AuthorityRef:   authorityRef,
		PrivacyClass:   "private",
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	_, artifactDigest, err := cfg.Appender.AppendNewPrivatePayload(ctx, event, computerevent.TransitionInput{}, plaintext, RecordMediaType, cfg.Cipher)
	if err != nil {
		return Ref{}, fmt.Errorf("snapshot: append snapshot_recorded: %w", err)
	}
	return Ref{ArtifactDigest: artifactDigest, EventID: eventID, Record: record}, nil
}

// uploadFile seals and uploads path in ChunkSize chunks and returns the
// manifest of the one-file tree.
func uploadFile(ctx context.Context, cfg Config, path string, now time.Time) (*filecas.Manifest, int64, error) {
	dek, err := cfg.Cipher.ExportKeyForEscrow(ctx, cfg.ComputerID)
	if err != nil {
		return nil, 0, fmt.Errorf("snapshot: export key: %w", err)
	}
	defer clear(dek)
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("snapshot: open copy: %w", err)
	}
	defer file.Close()
	entry := filecas.FileEntry{Path: StateFileName, Mode: 0o600}
	buf := make([]byte, ChunkSize)
	for {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		n, err := io.ReadFull(file, buf)
		if n > 0 {
			sealed, digest, sealErr := filecas.SealChunk(dek, cfg.ComputerID, buf[:n])
			if sealErr != nil {
				return nil, 0, fmt.Errorf("snapshot: seal chunk: %w", sealErr)
			}
			if putErr := cfg.Chunks.PutChunk(ctx, digest, sealed); putErr != nil {
				return nil, 0, fmt.Errorf("snapshot: store chunk %s: %w", digest, putErr)
			}
			entry.Chunks = append(entry.Chunks, digest)
			entry.Size += int64(n)
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return nil, 0, fmt.Errorf("snapshot: read copy: %w", err)
		}
	}
	manifest, err := filecas.BuildManifest(cfg.ComputerID, []filecas.FileEntry{entry}, now)
	if err != nil {
		return nil, 0, fmt.Errorf("snapshot: build manifest: %w", err)
	}
	return manifest, entry.Size, nil
}

// Source is the tape access Restore needs; *computerevent.HTTPClient
// satisfies it.
type Source interface {
	EventsPage(ctx context.Context, computerID string, afterSequence uint64, pageSize int) ([]computerevent.DurableEvent, error)
	FetchPayload(ctx context.Context, computerID, artifactDigest string) ([]byte, error)
}

// RestoreConfig is what Restore needs.
type RestoreConfig struct {
	ComputerID string
	// DestPath is the state file to install (store.StateDBPath). Restore
	// refuses to replace an existing file.
	DestPath string
	Source   Source
	Chunks   ChunkStore
	Cipher   *computerevent.PrivateArtifactCipher
	// MaxSequence, when set, restricts the choice to snapshots whose store
	// head sequence is at or below it (a restore to an earlier point).
	MaxSequence uint64
	// Verify optionally checks the unsealed file (for example its witness
	// against Record.WitnessRoot) before it is installed.
	Verify func(ctx context.Context, record Record, path string) error
}

// Candidate is a snapshot_recorded event seen on the tape.
type Candidate struct {
	EventID  string
	Digest   string
	Sequence uint64
}

// Restore installs the newest usable snapshot at cfg.DestPath and returns its
// record. It returns ErrNoSnapshot when the tape has none. The caller opens the
// store and lets the tape replay apply the events after
// Record.StoreHeadSequence.
func Restore(ctx context.Context, cfg RestoreConfig) (Record, error) {
	candidates, err := Scan(ctx, cfg.Source, cfg.ComputerID, nil)
	if err != nil {
		return Record{}, err
	}
	return Install(ctx, cfg, candidates)
}

// Install installs the newest usable of candidates (oldest first, as Scan
// returns them).
func Install(ctx context.Context, cfg RestoreConfig, candidates []Candidate) (Record, error) {
	if strings.TrimSpace(cfg.ComputerID) == "" || strings.TrimSpace(cfg.DestPath) == "" || cfg.Source == nil || cfg.Chunks == nil || cfg.Cipher == nil {
		return Record{}, errors.New("snapshot: computer id, destination, source, chunk store and cipher are required")
	}
	if _, err := os.Lstat(cfg.DestPath); err == nil {
		return Record{}, fmt.Errorf("snapshot: refusing to replace existing state file %s", cfg.DestPath)
	}
	if len(candidates) == 0 {
		return Record{}, ErrNoSnapshot
	}
	var failures []error
	for i := len(candidates) - 1; i >= 0; i-- {
		record, err := restoreOne(ctx, cfg, candidates[i])
		if err == nil {
			return record, nil
		}
		if ctx.Err() != nil {
			return Record{}, ctx.Err()
		}
		failures = append(failures, fmt.Errorf("snapshot event %s (sequence %d): %w", candidates[i].EventID, candidates[i].Sequence, err))
	}
	return Record{}, fmt.Errorf("snapshot: no usable snapshot: %w", errors.Join(failures...))
}

// maxCandidates bounds the snapshots kept while scanning; older ones are only
// reached if the newer ones are unusable.
const maxCandidates = 4

// MaxCandidates is the number of newest snapshots a restore considers. The
// platform keeps at least this many snapshot roots live
// (internal/platform snapshot_roots.go), so every candidate's chunks survive.
const MaxCandidates = maxCandidates

// Scan reads the tape from its start and returns the newest snapshot_recorded
// events (oldest first). visit, when set, sees every event in order and stops
// the scan by returning true; the stopping event itself is still considered.
func Scan(ctx context.Context, src Source, computerID string, visit func(computerevent.DurableEvent) (stop bool)) ([]Candidate, error) {
	if src == nil || strings.TrimSpace(computerID) == "" {
		return nil, errors.New("snapshot: source and computer id are required")
	}
	var candidates []Candidate
	after := uint64(0)
	for {
		page, err := src.EventsPage(ctx, computerID, after, scanPageSize)
		if err != nil {
			return nil, fmt.Errorf("snapshot: scan tape after %d: %w", after, err)
		}
		if len(page) == 0 {
			return candidates, nil
		}
		for _, durable := range page {
			event := durable.Request.Event
			after = event.Sequence
			if event.EventKind == computerevent.EventSnapshotRecorded && computerevent.IsSHA256(event.PayloadCommitment) {
				candidates = append(candidates, Candidate{EventID: event.EventID, Digest: event.PayloadCommitment, Sequence: event.Sequence})
				if len(candidates) > maxCandidates {
					candidates = candidates[1:]
				}
			}
			if visit != nil && visit(durable) {
				return candidates, nil
			}
		}
	}
}

func restoreOne(ctx context.Context, cfg RestoreConfig, f Candidate) (Record, error) {
	envelope, err := cfg.Source.FetchPayload(ctx, cfg.ComputerID, f.Digest)
	if err != nil {
		return Record{}, fmt.Errorf("fetch record: %w", err)
	}
	plaintext, _, err := cfg.Cipher.Decrypt(ctx, envelope, cfg.ComputerID, f.EventID)
	if err != nil {
		return Record{}, fmt.Errorf("unseal record: %w", err)
	}
	var record Record
	if err := json.Unmarshal(plaintext, &record); err != nil {
		return Record{}, fmt.Errorf("decode record: %w", err)
	}
	if record.Version != Version || record.ComputerID != cfg.ComputerID {
		return Record{}, fmt.Errorf("record is version %d of computer %q", record.Version, record.ComputerID)
	}
	if cfg.MaxSequence != 0 && record.StoreHeadSequence > cfg.MaxSequence {
		return Record{}, fmt.Errorf("snapshot head %d is above the restore point %d", record.StoreHeadSequence, cfg.MaxSequence)
	}
	if err := record.Manifest.VerifyRoot(); err != nil {
		return Record{}, fmt.Errorf("manifest: %w", err)
	}
	if len(record.Manifest.Files) != 1 || record.Manifest.Files[0].Path != StateFileName || record.Manifest.ComputerID != cfg.ComputerID {
		return Record{}, errors.New("manifest does not describe one state file of this computer")
	}
	entry := record.Manifest.Files[0]
	if entry.Size != record.PlaintextSize {
		return Record{}, errors.New("manifest size does not match the record")
	}

	dek, err := cfg.Cipher.ExportKeyForEscrow(ctx, cfg.ComputerID)
	if err != nil {
		return Record{}, fmt.Errorf("export key: %w", err)
	}
	defer clear(dek)

	dir := filepath.Dir(cfg.DestPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Record{}, fmt.Errorf("create state directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".snapshot-restore-*")
	if err != nil {
		return Record{}, fmt.Errorf("create staging file: %w", err)
	}
	tempPath := temp.Name()
	installed := false
	defer func() {
		if !installed {
			_ = temp.Close()
			_ = os.Remove(tempPath)
		}
	}()
	var written int64
	for _, digest := range entry.Chunks {
		if err := ctx.Err(); err != nil {
			return Record{}, err
		}
		sealed, err := cfg.Chunks.GetChunk(ctx, digest)
		if err != nil {
			return Record{}, fmt.Errorf("fetch chunk %s: %w", digest, err)
		}
		plain, err := filecas.OpenChunk(dek, cfg.ComputerID, digest, sealed)
		if err != nil {
			return Record{}, fmt.Errorf("open chunk %s: %w", digest, err)
		}
		n, err := temp.Write(plain)
		written += int64(n)
		if err != nil {
			return Record{}, fmt.Errorf("write state file: %w", err)
		}
	}
	if written != entry.Size {
		return Record{}, fmt.Errorf("restored %d bytes, the manifest has %d", written, entry.Size)
	}
	if err := temp.Chmod(0o600); err != nil {
		return Record{}, err
	}
	if err := temp.Sync(); err != nil {
		return Record{}, err
	}
	if err := temp.Close(); err != nil {
		return Record{}, err
	}
	if cfg.Verify != nil {
		if err := cfg.Verify(ctx, record, tempPath); err != nil {
			return Record{}, fmt.Errorf("verify: %w", err)
		}
	}
	// A stale WAL beside the destination would be applied to the new file.
	_ = os.Remove(cfg.DestPath + "-wal")
	_ = os.Remove(cfg.DestPath + "-shm")
	if err := os.Rename(tempPath, cfg.DestPath); err != nil {
		return Record{}, fmt.Errorf("install state file: %w", err)
	}
	installed = true
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return record, nil
}
