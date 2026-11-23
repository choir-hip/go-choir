// Package snapshot is snapshot version 1: the guest SQLite store streamed page
// by page from one read snapshot, sealed under the computer's key, stored
// chunk by chunk in the platform's file content store, and named on the tape
// by a synchronous snapshot_recorded event
// (docs/design/data-system-build-sheet-2026-10-30.md, decision 7). It replaces
// the projection bases.
//
// Capture opens one read-only transaction on the store's read pool and reads
// the event head as its first statement, so the head belongs to the
// transaction's WAL snapshot. It then streams every database page of that
// snapshot through SQLite's sqlite_dbpage virtual table (pages as the snapshot
// sees them, WAL included) straight into chunk buffers that sealing workers
// upload, taking the page digest (NewPageDigest) on the same bytes as they are
// read. Nothing the size of the store is written: the pin on the WAL lasts the
// stream, which the uploader's backpressure paces, so the transaction has its
// own deadline (Config.PinDeadline) and the free space is watched while it
// lasts. The record is version 2 (VersionPageDigest): it carries the page digest
// and no logical witness; restore checks the digest as it writes the pages and
// then the restored file's head row. The telemetry partitions are streamed
// afterwards, each from its own read transaction.
//
// Storage reuses the file CAS: the page stream is cut into chunks of a fixed
// number of pages (ChunkSize / page size, so 1024 pages of 4 KiB: chunk i is
// pages [i*N, (i+1)*N)), each sealed with filecas.SealChunk (deterministic per
// key and plaintext, so a range of pages that did not change yields the same
// sealed bytes and the same digest) and PUT under the computer's chunk
// namespace unless the host already holds that digest. The chunks concatenated
// are exactly the SQLite file, byte for byte as a size-ChunkSize cut of it, so
// a record needs no new format: page size, page count and pages per chunk are
// extra fields an old reader ignores. The manifest of chunk digests, with the
// store head sequence, the file-tree root and the page digest, is the
// private payload of the snapshot_recorded event
// (computerevent.PrivateArtifactCipher), so the sealed artifact digest on the
// tape names the snapshot and nothing readable leaves the computer.
//
// Boot reads the computer's own disk. A missing disk is rebuilt by Restore:
// the newest snapshot_recorded on the tape is fetched and unsealed, installed
// as the state file, and the existing tape replay applies the events after its
// sequence.
package snapshot

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/computerevent"
	"github.com/yusefmosiah/go-choir/internal/deskmanifest"
	"github.com/yusefmosiah/go-choir/internal/filecas"
)

const (
	// Version is the snapshot format version of a record that carries the
	// logical witness (WitnessRoot, SchemaHash) and no page digest.
	Version = 1
	// VersionPageDigest is the format version of a record that carries a
	// PageDigest and no witness. A binary that does not know it refuses the
	// record (restoreOne) and restores an older snapshot.
	VersionPageDigest = 2
	// RecordMediaType is the media type of the sealed snapshot record.
	RecordMediaType = "application/vnd.choir.snapshot-record+json"
	// StateFileName is the manifest path of the SQLite file.
	StateFileName = "state.db"
	// ChunkSize is the plaintext size of one snapshot chunk (a whole number
	// of pages of any SQLite page size).
	ChunkSize = 4 << 20

	authorityRef = "authority:guest-snapshot"
	// scanPageSize is the replay page size Restore scans the tape with.
	scanPageSize = computerevent.EventReplayPageSize
	// uploadWorkers seal, probe and upload chunks while the reader keeps
	// reading pages.
	uploadWorkers = 4
)

// ErrTimeBudget means the stream and the upload did not finish within
// Config.MaxDuration. The read transaction is rolled back, nothing is registered
// and no event is appended.
var ErrTimeBudget = errors.New("snapshot: exceeded the time budget")

// ErrPinDeadline means the read transaction was still open when
// Config.PinDeadline passed: a slow upload had held the stream (and with it
// the WAL) too long. The transaction is rolled back, the chunks already
// uploaded stay on the host (a retry finds them held), nothing is registered
// and no event is appended.
var ErrPinDeadline = errors.New("snapshot: the read transaction exceeded its pin deadline")

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
	// PageSize, PageCount and PagesPerChunk describe the page-range cut of a
	// streamed snapshot: PlaintextSize = PageSize * PageCount, and chunk i
	// holds pages [i*PagesPerChunk, (i+1)*PagesPerChunk). Snapshots recorded
	// from a VACUUM INTO copy (before the page stream) do not carry them; they
	// are cut at ChunkSize bytes, which is the same cut.
	PageSize      int   `json:"page_size,omitempty"`
	PageCount     int64 `json:"page_count,omitempty"`
	PagesPerChunk int   `json:"pages_per_chunk,omitempty"`
	// FileTreeRoot is the file CAS root of the Files tree at the snapshot,
	// when the computer has one.
	FileTreeRoot string `json:"file_tree_root,omitempty"`
	// WitnessRoot and SchemaHash are the witness v3 content root and schema
	// hash of the captured file. A Version record carries both; a
	// VersionPageDigest record carries neither.
	WitnessRoot string `json:"witness_root,omitempty"`
	SchemaHash  string `json:"schema_hash,omitempty"`
	// PageDigest is the lowercase hex of NewPageDigest over the file's pages in
	// page order. A VersionPageDigest record carries it (and its page geometry);
	// a Version record does not.
	PageDigest string `json:"page_digest,omitempty"`
	// Manifest addresses the sealed chunks of the file (one entry,
	// StateFileName).
	Manifest filecas.Manifest `json:"manifest"`
	// Partitions are the store's telemetry partition files at the snapshot
	// (internal/store/telemetry_partition.go), each streamed like the state
	// file from its own read transaction. They are outside the manifest, so a
	// reader that does not know them installs the state file alone (and its
	// replay lacks the partitions' rows below the head: telemetry, never
	// semantic state). A closed partition's chunks are the same bytes in
	// every snapshot, so a repeat snapshot uploads none of them.
	Partitions []PartitionEntry `json:"partitions,omitempty"`
}

// pageDigestDomain separates the page digest from every other SHA-256 here.
const pageDigestDomain = "choir.snapshot.page-digest.v1\x00"

// PageDigest is the running digest of a state file's pages: SHA-256 over a
// domain tag, the page size (4 bytes, big endian), the page count (8 bytes,
// big endian) and then the plaintext pages in page order. The geometry is
// inside the digest, so the same bytes under another page size do not match.
type PageDigest struct{ h hash.Hash }

// NewPageDigest starts the digest of a file of pageCount pages of pageSize bytes.
func NewPageDigest(pageSize int, pageCount int64) *PageDigest {
	h := sha256.New()
	var head [12]byte
	binary.BigEndian.PutUint32(head[:4], uint32(pageSize))
	binary.BigEndian.PutUint64(head[4:], uint64(pageCount))
	h.Write([]byte(pageDigestDomain))
	h.Write(head[:])
	return &PageDigest{h: h}
}

// Write adds plaintext pages; it never fails.
func (d *PageDigest) Write(p []byte) (int, error) { return d.h.Write(p) }

// Sum is the digest as lowercase hex.
func (d *PageDigest) Sum() string { return hex.EncodeToString(d.h.Sum(nil)) }

// PartitionEntry is one partition file of a snapshot.
type PartitionEntry struct {
	// Path is the file's path under the store workspace,
	// "telemetry/<name>.db".
	Path          string   `json:"path"`
	Size          int64    `json:"size"`
	PageSize      int      `json:"page_size"`
	PageCount     int64    `json:"page_count"`
	PagesPerChunk int      `json:"pages_per_chunk"`
	Chunks        []string `json:"chunks"`
}

// PartitionDirName is the directory every PartitionEntry.Path is in.
const PartitionDirName = "telemetry"

// PartitionFile is one partition the store hands the capture: its snapshot
// path and a read pool on it.
type PartitionFile struct {
	Path string
	DB   *sql.DB
}

// PartitionSource is the store's partition set as the capture sees it. Hold
// is called before the read transaction begins and its release after the last
// partition is read, so no file the head calls live is deleted under the
// stream; List runs inside the read transaction.
type PartitionSource struct {
	Hold func() (release func())
	List func(ctx context.Context, tx *sql.Tx) ([]PartitionFile, error)
}

// validPartitionPath reports whether path names a file directly inside
// PartitionDirName.
func validPartitionPath(path string) bool {
	name, ok := strings.CutPrefix(path, PartitionDirName+"/")
	return ok && name != "" && !strings.ContainsAny(name, `/\`) && name != "." && name != ".." && strings.HasSuffix(name, ".db")
}

// Stats is what one capture did, for the producer's log.
type Stats struct {
	Pages    int64
	PageSize int
	// Chunks is the number of chunks in the manifest. ChunksUploaded were
	// PUT, ChunksHeld were already on the host (skipped), ChunksRepeated
	// repeat an earlier chunk of this snapshot.
	Chunks         int
	ChunksUploaded int
	ChunksHeld     int
	ChunksRepeated int
	// ProbeErrors counts existence checks that failed; the chunk was
	// uploaded instead.
	ProbeErrors   int
	BytesUploaded int64
	// PartitionFiles is the telemetry partitions streamed; their pages and
	// chunks are in the counts above.
	PartitionFiles int
	// PinTime is the interval with an open reader on the state store, from the
	// transaction's start to its release (the whole stream of the state file).
	// UploadTime is what remained of the uploads after the pin ended. StateBytes
	// is the state file's size. PinWALStart and PinWALEnd are the WAL file's size
	// at both ends of the pin (Config.WALBytes), PinEvents the events appended
	// during it (filled by the producer).
	PinTime, UploadTime    time.Duration
	StateBytes             int64
	PinWALStart, PinWALEnd int64
	PinEvents              uint64
}

// Ref names a recorded snapshot.
type Ref struct {
	// ArtifactDigest is the sealed record's digest: the snapshot_recorded
	// event's payload commitment.
	ArtifactDigest string
	// EventID is the snapshot_recorded event.
	EventID string
	Record  Record
	Stats   Stats
}

// ChunkStore moves sealed chunks to and from the platform's file CAS.
type ChunkStore interface {
	PutChunk(ctx context.Context, digest string, sealed []byte) error
	GetChunk(ctx context.Context, digest string) ([]byte, error)
}

// ChunkProber is implemented by a ChunkStore that can say whether it already
// holds a chunk in the computer's own namespace (HTTPChunks: HEAD). A chunk it
// holds is not uploaded again, which makes a repeat snapshot incremental. A
// probe error means unknown: the chunk is uploaded.
type ChunkProber interface {
	HasChunk(ctx context.Context, digest string) (bool, error)
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
	// single write connection: the capture holds one read transaction on it
	// for the whole stream while writers keep committing, so a 30 GB store
	// blocks no write and no append.
	DB *sql.DB
	// Head reads the event head inside tx. It is the FIRST statement of the
	// read transaction, which fixes the transaction's WAL snapshot at the
	// state the head names (the projection head commits with the rows it
	// summarizes). It returns nil for an empty store.
	Head func(ctx context.Context, tx *sql.Tx) (*computerevent.Head, error)
	// FileTreeRoot optionally names the Files tree root to record.
	FileTreeRoot func(ctx context.Context) (string, error)
	// Partitions, when set, adds the store's telemetry partitions.
	Partitions *PartitionSource
	Cipher     *computerevent.PrivateArtifactCipher
	Chunks     ChunkStore
	Appender   Appender
	// MaxDuration, when positive, bounds the whole capture: the stream (which
	// pins the WAL), the uploads and the partitions. The capture aborts with
	// ErrTimeBudget, which leaves no root and no event.
	MaxDuration time.Duration
	// PinDeadline, when positive, bounds each read transaction (the state file's
	// and each partition's) from its start to its release. Past it the stream
	// stops, the transaction is rolled back, the WAL is released and the
	// capture fails with ErrPinDeadline; chunks already uploaded stay on the
	// host and a retry skips them.
	PinDeadline time.Duration
	// FreeSpace, when set, is checked every FreeSpaceEvery (default a minute)
	// while the capture runs and every sourceDropBytes read: the pinned WAL
	// grows on the guest's disk. A non-nil result ends the capture with that
	// error as its cause, so a snapshot cannot push the guest to ENOSPC.
	FreeSpace      func() error
	FreeSpaceEvery time.Duration
	// Now overrides the clock in tests.
	Now func() time.Time

	// pageHook, when set, runs in the reader before each page is taken (tests
	// commit writes in the middle of the stream).
	pageHook func(pgno int64)
	// streamEnded, when set, runs once the state file's last page is flushed, before
	// the transaction ends (tests let the pin deadline pass there).
	streamEnded func()

	// ReadReleased runs once, from the transaction's release, on every path
	// (success, error, cancellation, budget). It runs in its own goroutine with
	// a context the caller cannot cancel and, when MaxDuration is positive, a
	// fresh MaxDuration deadline; the stream does not wait for it and the
	// capture joins it before returning. A timeout is the hook's own concern: it
	// never fails the attempt.
	ReadReleased func(ctx context.Context)
	// WALBytes reports the state store's WAL file size, for the pin's growth.
	WALBytes func() int64
}

func (c Config) validate() error {
	switch {
	case strings.TrimSpace(c.ComputerID) == "":
		return errors.New("snapshot: computer id is required")
	case c.DB == nil:
		return errors.New("snapshot: store is required")
	case c.Head == nil:
		return errors.New("snapshot: a head reader is required")
	case c.Cipher == nil || c.Chunks == nil || c.Appender == nil:
		return errors.New("snapshot: cipher, chunk store and appender are required")
	}
	return nil
}

// TakeSnapshot streams the store's pages from one read snapshot on the read
// pool, seals and stores the chunks the host does not already hold, and
// appends the snapshot_recorded event. Nothing is held: writers and appends
// continue while the pages are read. The head and the pages are
// of the one transaction's snapshot, and the events committed after it are
// the tape tail Restore replays.
func TakeSnapshot(ctx context.Context, cfg Config) (Ref, error) {
	if err := cfg.validate(); err != nil {
		return Ref{}, err
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	dek, err := cfg.Cipher.ExportKeyForEscrow(ctx, cfg.ComputerID)
	if err != nil {
		return Ref{}, fmt.Errorf("snapshot: export key: %w", err)
	}
	defer clear(dek)

	got, err := capture(ctx, cfg, dek)
	if err != nil {
		return Ref{}, err
	}
	entry := got.entry
	manifest, err := filecas.BuildManifest(cfg.ComputerID, []filecas.FileEntry{entry}, now())
	if err != nil {
		return Ref{}, fmt.Errorf("snapshot: build manifest: %w", err)
	}
	record := Record{
		Version:       VersionPageDigest,
		ComputerID:    cfg.ComputerID,
		CreatedAt:     now().UTC().Format(time.RFC3339Nano),
		PlaintextSize: entry.Size,
		PageSize:      got.pageSize,
		PageCount:     got.pageCount,
		PagesPerChunk: got.pagesPerChunk,
		PageDigest:    got.pageDigest,
		Manifest:      *manifest,
		Partitions:    got.partitions,
	}
	if got.head != nil {
		record.StoreHeadSequence = got.head.Sequence
		record.StoreHead = got.head.CanonicalEventHead
	}
	if cfg.FileTreeRoot != nil {
		root, err := cfg.FileTreeRoot(ctx)
		if err != nil {
			return Ref{}, fmt.Errorf("snapshot: file tree root: %w", err)
		}
		record.FileTreeRoot = root
	}
	if registrar, ok := cfg.Chunks.(RootRegistrar); ok {
		chunks := append([]string(nil), manifest.Files[0].Chunks...)
		for _, part := range got.partitions {
			chunks = append(chunks, part.Chunks...)
		}
		if err := registrar.RegisterSnapshotRoot(ctx, manifest.Root, record.StoreHeadSequence, chunks); err != nil {
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
		ActorProfile:   deskmanifest.AdmitterName(),
		AuthorityRef:   authorityRef,
		PrivacyClass:   "private",
		ReducerVersion: computerevent.ReducerVersionV1,
	}
	_, artifactDigest, err := cfg.Appender.AppendNewPrivatePayload(ctx, event, computerevent.TransitionInput{}, plaintext, RecordMediaType, cfg.Cipher)
	if err != nil {
		return Ref{}, fmt.Errorf("snapshot: append snapshot_recorded: %w", err)
	}
	return Ref{ArtifactDigest: artifactDigest, EventID: eventID, Record: record, Stats: got.stats}, nil
}

// captured is what the read transaction yields.
type captured struct {
	head          *computerevent.Head
	pageDigest    string
	entry         filecas.FileEntry
	pageSize      int
	pageCount     int64
	pagesPerChunk int
	partitions    []PartitionEntry
	stats         Stats
}

// chunkJob is one page range handed from the reader to an upload worker.
type chunkJob struct {
	index int
	data  []byte
}

// chunkUploader seals, probes and uploads chunks on its workers.
type chunkUploader struct {
	cfg     Config
	dek     []byte
	prober  ChunkProber
	jobs    chan chunkJob
	free    chan []byte
	digests []string
	seen    sync.Map
	wg      sync.WaitGroup
	cancel  context.CancelCauseFunc

	uploaded, held, repeated, probeErrs atomic.Int64
	bytesUploaded                       atomic.Int64

	closeOnce sync.Once
}

func newChunkUploader(cfg Config, dek []byte, chunkBytes, chunkCount int, cancel context.CancelCauseFunc) *chunkUploader {
	u := &chunkUploader{
		cfg:     cfg,
		dek:     dek,
		jobs:    make(chan chunkJob, uploadWorkers),
		free:    make(chan []byte, 2*uploadWorkers+1),
		digests: make([]string, chunkCount),
		cancel:  cancel,
	}
	u.prober, _ = cfg.Chunks.(ChunkProber)
	for i := 0; i < cap(u.free); i++ {
		u.free <- make([]byte, 0, chunkBytes)
	}
	return u
}

func (u *chunkUploader) start(ctx context.Context) {
	for i := 0; i < uploadWorkers; i++ {
		u.wg.Add(1)
		go func() {
			defer u.wg.Done()
			for job := range u.jobs {
				if ctx.Err() == nil {
					if err := u.handle(ctx, job); err != nil {
						u.cancel(err)
					}
				}
				u.free <- job.data[:0]
			}
		}()
	}
}

func (u *chunkUploader) handle(ctx context.Context, job chunkJob) error {
	sealed, digest, err := filecas.SealChunk(u.dek, u.cfg.ComputerID, job.data)
	if err != nil {
		return fmt.Errorf("snapshot: seal chunk %d: %w", job.index, err)
	}
	u.digests[job.index] = digest
	if _, repeat := u.seen.LoadOrStore(digest, struct{}{}); repeat {
		u.repeated.Add(1)
		return nil
	}
	if u.prober != nil {
		has, perr := u.prober.HasChunk(ctx, digest)
		switch {
		case perr != nil:
			if ctx.Err() != nil {
				return nil
			}
			u.probeErrs.Add(1)
		case has:
			u.held.Add(1)
			return nil
		}
	}
	if err := u.cfg.Chunks.PutChunk(ctx, digest, sealed); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("snapshot: store chunk %s: %w", digest, err)
	}
	u.uploaded.Add(1)
	u.bytesUploaded.Add(int64(len(sealed)))
	return nil
}

// finish ends the job stream and waits for the workers.
func (u *chunkUploader) finish() {
	u.closeOnce.Do(func() { close(u.jobs) })
	u.wg.Wait()
}

// pinContext derives the context of one read transaction: it ends with the
// run, or when the pin deadline passes (ErrPinDeadline), whichever is first. A
// deadline of zero or less is no deadline of its own.
func pinContext(runCtx context.Context, deadline time.Duration) (context.Context, context.CancelFunc) {
	if deadline > 0 {
		return context.WithTimeoutCause(runCtx, deadline, ErrPinDeadline)
	}
	return context.WithCancel(runCtx)
}

// capture opens the one read transaction, reads the head (its first statement),
// lists the partitions and streams every page of the transaction's snapshot
// straight into sealed, uploaded chunks, taking the page digest on the same
// bytes as it reads them. There is no copy of the store and no logical witness:
// the transaction lasts as long as the reader is held back by the uploader, so
// it has its own deadline (Config.PinDeadline). When the last page is read the
// transaction ends; the uploads still in flight finish after it.
func capture(ctx context.Context, cfg Config, dek []byte) (out captured, retErr error) {
	budgetCtx := ctx
	if cfg.MaxDuration > 0 {
		var stop context.CancelFunc
		budgetCtx, stop = context.WithTimeoutCause(ctx, cfg.MaxDuration, ErrTimeBudget)
		defer stop()
	}
	runCtx, cancel := context.WithCancelCause(budgetCtx)
	defer cancel(nil)
	if cfg.FreeSpace != nil {
		every := cfg.FreeSpaceEvery
		if every <= 0 {
			every = time.Minute
		}
		guardDone := make(chan struct{})
		defer close(guardDone)
		go func() {
			tick := time.NewTicker(every)
			defer tick.Stop()
			for {
				select {
				case <-guardDone:
					return
				case <-runCtx.Done():
					return
				case <-tick.C:
					if err := cfg.FreeSpace(); err != nil {
						cancel(err)
						return
					}
				}
			}
		}()
	}

	// The partitions stay in place from before the read transaction until the
	// last one is read.
	if cfg.Partitions != nil && cfg.Partitions.Hold != nil {
		release := cfg.Partitions.Hold()
		defer release()
	}

	// The read-released hook is joined before capture returns. This defer is
	// registered before the rollback's, so it runs after it.
	var hookWG sync.WaitGroup
	defer hookWG.Wait()

	// The pin deadline counts from before the transaction begins. cause is why
	// the pin ended: the deadline, the budget, a worker's failure, or the
	// caller's cancellation.
	pinCtx, stopPin := pinContext(runCtx, cfg.PinDeadline)
	defer stopPin()
	cause := func(err error) error {
		if c := context.Cause(pinCtx); c != nil {
			return c
		}
		return err
	}

	tx, err := cfg.DB.BeginTx(pinCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, fmt.Errorf("snapshot: begin read transaction: %w", cause(err))
	}
	pinStart := time.Now()
	if cfg.WALBytes != nil {
		out.stats.PinWALStart = cfg.WALBytes()
	}
	txOpen := true
	// rollback is the one place the transaction ends, on every return. It takes
	// PinTime first, then starts the hook, so the hook is outside the timed span.
	rollback := func() {
		if !txOpen {
			return
		}
		txOpen = false
		_ = tx.Rollback()
		out.stats.PinTime = time.Since(pinStart)
		if cfg.WALBytes != nil {
			out.stats.PinWALEnd = cfg.WALBytes()
		}
		if cfg.ReadReleased != nil {
			hookWG.Add(1)
			go func() {
				defer hookWG.Done()
				hookCtx := context.WithoutCancel(ctx)
				if cfg.MaxDuration > 0 {
					var stop context.CancelFunc
					hookCtx, stop = context.WithTimeout(hookCtx, cfg.MaxDuration)
					defer stop()
				}
				cfg.ReadReleased(hookCtx)
			}()
		}
	}
	defer rollback()

	// The head is the transaction's first statement: it fixes the WAL
	// snapshot, so the head names the state the pages are read from.
	if out.head, err = cfg.Head(pinCtx, tx); err != nil {
		return out, fmt.Errorf("snapshot: read head: %w", cause(err))
	}
	var parts []PartitionFile
	if cfg.Partitions != nil && cfg.Partitions.List != nil {
		if parts, err = cfg.Partitions.List(pinCtx, tx); err != nil {
			return out, fmt.Errorf("snapshot: list telemetry partitions: %w", cause(err))
		}
		for _, part := range parts {
			if !validPartitionPath(part.Path) || part.DB == nil {
				return out, fmt.Errorf("snapshot: unusable telemetry partition %q", part.Path)
			}
		}
	}

	// The stream: every page of the snapshot to the uploader, then the pin ends.
	fs, err := streamFile(pinCtx, runCtx, cfg, dek, cancel, tx, true)
	// The pin's context is read before the transaction ends: a stream whose last
	// flush won the race against the deadline still held the WAL past it.
	pinCause := context.Cause(pinCtx)
	rollback()
	if err != nil {
		return out, err
	}
	if errors.Is(pinCause, ErrPinDeadline) {
		fs.abandon(cancel, pinCause)
		return out, fmt.Errorf("snapshot: the read transaction outlived its %s pin deadline: %w", cfg.PinDeadline, ErrPinDeadline)
	}
	drainStart := time.Now()
	if err := fs.finish(runCtx); err != nil {
		return out, err
	}
	out.stats.UploadTime = time.Since(drainStart)
	out.pageSize, out.pageCount, out.pagesPerChunk = fs.pageSize, fs.pageCount, fs.pagesPerChunk
	out.pageDigest = fs.pageDigest
	out.entry = filecas.FileEntry{Path: StateFileName, Mode: 0o600, Size: int64(fs.pageSize) * fs.pageCount, Chunks: fs.up.digests}
	out.stats.Pages = fs.pageCount
	out.stats.PageSize = fs.pageSize
	out.stats.StateBytes = out.entry.Size
	fs.addStats(&out.stats)

	// The partitions, each from its own read transaction, begun after the
	// state file's: a partition commits before the state file does, so each
	// holds every row at or below the head (and maybe rows above it, which
	// Open drops at the restore and the replay writes again).
	for _, part := range parts {
		entry, err := capturePartition(runCtx, cfg, dek, cancel, part)
		if err != nil {
			return out, err
		}
		out.partitions = append(out.partitions, entry.entry)
		entry.stream.addStats(&out.stats)
		out.stats.Pages += entry.stream.pageCount
		out.stats.PartitionFiles++
	}
	return out, nil
}

// sourceDropBytes is how much of the page stream is read between drops of the
// source file's page cache.
var sourceDropBytes int64 = 64 << 20

// dropSourceCache drops the clean page cache of the source database file's
// byte range [off, off+n); n 0 is the whole file, and then its WAL too. A var
// so a test can see the calls. Mid-stream it drops only the range just read
// (the stream reads in page order), so the daemon's hot pages elsewhere stay
// until the end; tradeoff: DONTNEED drops hot pages in the range too, which
// the daemon refaults from disk; the owner chose more disk reads over held
// cache ("More disk reads, less memory usage"). Best effort.
var dropSourceCache = func(path string, off, n int64) {
	adviseDropFileCache(path, off, n)
	if n == 0 {
		adviseDropFileCache(path+"-wal", 0, 0)
	}
}

// sourcePath is the main database file behind tx ("" when unknown, e.g. an
// in-memory database).
func sourcePath(ctx context.Context, tx *sql.Tx) string {
	rows, err := tx.QueryContext(ctx, `PRAGMA database_list`)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var name, file string
		if rows.Scan(&seq, &name, &file) == nil && name == "main" {
			return file
		}
	}
	return ""
}

// fileStream is one SQLite file's page stream: its geometry and the uploader
// sealing its chunks.
type fileStream struct {
	up            *chunkUploader
	pageSize      int
	pageCount     int64
	pagesPerChunk int
	chunkCount    int
	cut           int
	// pageDigest is the lowercase hex of NewPageDigest over the pages as they
	// were read, when the stream was asked for one.
	pageDigest string
}

// streamFile reads every page of tx's database snapshot into chunks handed to
// a new uploader. readCtx bounds the read (the transaction's pin: its deadline,
// or the run); runCtx is the uploaders'. With wantDigest the pages are also
// hashed as they are read, before they are copied into a chunk buffer. On an
// error it stops and drains the uploader itself; on success the caller finishes
// it (fileStream.finish). Whatever it started is released by defer, a panic in
// a callback included: the rows are closed, and on any exit but success the run
// is cancelled and the workers finished.
func streamFile(readCtx, runCtx context.Context, cfg Config, dek []byte, cancel context.CancelCauseFunc, tx *sql.Tx, wantDigest bool) (_ *fileStream, retErr error) {
	cause := func(err error) error {
		if c := context.Cause(readCtx); c != nil {
			return c
		}
		return err
	}
	fs := &fileStream{}
	if err := tx.QueryRowContext(readCtx, `PRAGMA page_size`).Scan(&fs.pageSize); err != nil {
		return nil, fmt.Errorf("snapshot: page size: %w", cause(err))
	}
	if err := tx.QueryRowContext(readCtx, `PRAGMA page_count`).Scan(&fs.pageCount); err != nil {
		return nil, fmt.Errorf("snapshot: page count: %w", cause(err))
	}
	pageSize := fs.pageSize
	if pageSize <= 0 || pageSize > ChunkSize || ChunkSize%pageSize != 0 || fs.pageCount <= 0 {
		return nil, fmt.Errorf("snapshot: unusable store geometry: %d pages of %d bytes", fs.pageCount, pageSize)
	}
	fs.pagesPerChunk = ChunkSize / pageSize
	fs.chunkCount = int((fs.pageCount + int64(fs.pagesPerChunk) - 1) / int64(fs.pagesPerChunk))
	var digest *PageDigest
	if wantDigest {
		digest = NewPageDigest(pageSize, fs.pageCount)
	}

	up := newChunkUploader(cfg, dek, ChunkSize, fs.chunkCount, cancel)
	up.start(runCtx)
	fs.up = up
	succeeded := false
	defer func() {
		if succeeded {
			return
		}
		// An error return, or a panic passing through (retErr is nil then): the
		// workers must not outlive the stream, and a worker blocked on a stalled
		// upload ends only with the run.
		if retErr == nil {
			retErr = errors.New("snapshot: the stream was abandoned by a panic")
		}
		cancel(retErr)
		up.finish()
	}()
	fail := func(err error) (*fileStream, error) { return nil, err }

	streamStart := time.Now()
	var pages int64
	progress := func(err error) error {
		return fmt.Errorf("%w (%d of %d pages read, %s into the stream)", cause(err), pages, fs.pageCount, time.Since(streamStart).Round(time.Second))
	}
	src := sourcePath(readCtx, tx)
	var dropFrom int64
	defer func() { // the end, and every failure path
		if src != "" {
			dropSourceCache(src, 0, 0)
		}
	}()
	rows, err := tx.QueryContext(readCtx, `SELECT pgno, data FROM sqlite_dbpage`)
	if err != nil {
		return fail(fmt.Errorf("snapshot: read pages: %w", progress(err)))
	}
	defer rows.Close()
	buf := <-up.free
	chunkIndex := 0
	flush := func() error {
		select {
		case up.jobs <- chunkJob{index: chunkIndex, data: buf}:
		case <-readCtx.Done():
			return readCtx.Err()
		}
		chunkIndex++
		select {
		case buf = <-up.free:
		case <-readCtx.Done():
			return readCtx.Err()
		}
		return nil
	}
	for rows.Next() {
		var pgno int64
		var data sql.RawBytes
		if err := rows.Scan(&pgno, &data); err != nil {
			return fail(fmt.Errorf("snapshot: scan page: %w", progress(err)))
		}
		if pages >= fs.pageCount {
			// The cut and the digest list are sized by the counted pages: a page
			// past them is refused before it is appended to a chunk.
			return fail(fmt.Errorf("snapshot: the page stream holds page %d, past the counted %d pages", pgno, fs.pageCount))
		}
		if pgno != pages+1 || len(data) != pageSize {
			return fail(fmt.Errorf("snapshot: page stream out of order or short: got page %d of %d bytes after %d pages (page size %d)", pgno, len(data), pages, pageSize))
		}
		if cfg.pageHook != nil && wantDigest {
			cfg.pageHook(pgno)
		}
		if digest != nil {
			_, _ = digest.Write(data)
		}
		buf = append(buf, data...)
		pages++
		if read := pages * int64(pageSize); read-dropFrom >= sourceDropBytes {
			if src != "" {
				dropSourceCache(src, dropFrom, read-dropFrom)
			}
			dropFrom = read
			// The pinned WAL grows on the guest's disk while the stream runs.
			if cfg.FreeSpace != nil {
				if err := cfg.FreeSpace(); err != nil {
					return fail(fmt.Errorf("snapshot: stream aborted after %d of %d pages: %w", pages, fs.pageCount, err))
				}
			}
		}
		if len(buf) == fs.pagesPerChunk*pageSize {
			if err := flush(); err != nil {
				return fail(fmt.Errorf("snapshot: %w", progress(err)))
			}
		}
	}
	if err := rows.Err(); err != nil {
		return fail(fmt.Errorf("snapshot: read pages: %w", progress(err)))
	}
	_ = rows.Close()
	if pages != fs.pageCount {
		return fail(fmt.Errorf("snapshot: the page stream held %d pages, the snapshot has %d", pages, fs.pageCount))
	}
	if len(buf) > 0 {
		if err := flush(); err != nil {
			return fail(fmt.Errorf("snapshot: %w", progress(err)))
		}
	}
	fs.cut = chunkIndex
	if digest != nil {
		fs.pageDigest = digest.Sum()
	}
	if cfg.streamEnded != nil && wantDigest {
		cfg.streamEnded()
	}
	succeeded = true
	return fs, nil
}

// abandon ends a stream that read every page but must not be used: the run is
// cancelled with cause and the workers finished, so nothing of it is recorded.
func (fs *fileStream) abandon(cancel context.CancelCauseFunc, cause error) {
	cancel(cause)
	fs.up.finish()
}

// finish waits for the uploader and checks the cut.
func (fs *fileStream) finish(runCtx context.Context) error {
	fs.up.finish()
	if err := runCtx.Err(); err != nil {
		if c := context.Cause(runCtx); c != nil {
			err = c
		}
		return fmt.Errorf("snapshot: %w (after %d of %d chunks)", err, fs.cut, fs.chunkCount)
	}
	if fs.cut != fs.chunkCount {
		return fmt.Errorf("snapshot: cut %d chunks, expected %d", fs.cut, fs.chunkCount)
	}
	return nil
}

func (fs *fileStream) addStats(st *Stats) {
	st.Chunks += fs.chunkCount
	st.ChunksUploaded += int(fs.up.uploaded.Load())
	st.ChunksHeld += int(fs.up.held.Load())
	st.ChunksRepeated += int(fs.up.repeated.Load())
	st.ProbeErrors += int(fs.up.probeErrs.Load())
	st.BytesUploaded += fs.up.bytesUploaded.Load()
}

type capturedPartition struct {
	entry  PartitionEntry
	stream *fileStream
}

// capturePartition streams one partition from its own read transaction, under
// its own pin deadline.
func capturePartition(runCtx context.Context, cfg Config, dek []byte, cancel context.CancelCauseFunc, part PartitionFile) (capturedPartition, error) {
	pinCtx, stopPin := pinContext(runCtx, cfg.PinDeadline)
	defer stopPin()
	cause := func(err error) error {
		if c := context.Cause(pinCtx); c != nil {
			return c
		}
		return err
	}
	ptx, err := part.DB.BeginTx(pinCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return capturedPartition{}, fmt.Errorf("snapshot: begin read of %s: %w", part.Path, cause(err))
	}
	// The transaction ends on every return, a panic in a callback included.
	ptxOpen := true
	endRead := func() {
		if ptxOpen {
			ptxOpen = false
			_ = ptx.Rollback()
		}
	}
	defer endRead()
	// The first statement fixes the partition's read snapshot.
	var one int
	if err := ptx.QueryRowContext(pinCtx, `SELECT 1`).Scan(&one); err != nil {
		return capturedPartition{}, fmt.Errorf("snapshot: read %s: %w", part.Path, cause(err))
	}
	fs, err := streamFile(pinCtx, runCtx, cfg, dek, cancel, ptx, false)
	pinCause := context.Cause(pinCtx)
	endRead()
	if err != nil {
		return capturedPartition{}, fmt.Errorf("%s: %w", part.Path, err)
	}
	if errors.Is(pinCause, ErrPinDeadline) {
		fs.abandon(cancel, pinCause)
		return capturedPartition{}, fmt.Errorf("%s: the read transaction outlived its %s pin deadline: %w", part.Path, cfg.PinDeadline, ErrPinDeadline)
	}
	if err := fs.finish(runCtx); err != nil {
		return capturedPartition{}, fmt.Errorf("%s: %w", part.Path, err)
	}
	return capturedPartition{
		entry: PartitionEntry{
			Path: part.Path, Size: int64(fs.pageSize) * fs.pageCount, PageSize: fs.pageSize, PageCount: fs.pageCount,
			PagesPerChunk: fs.pagesPerChunk, Chunks: fs.up.digests,
		},
		stream: fs,
	}, nil
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
	// Verify checks the unsealed file before it is installed: a version 1
	// record's witness against Record.WitnessRoot, a version 2 record's head row
	// against Record.StoreHeadSequence and StoreHead (the page digest is checked
	// by the install itself). Optional for a version 1 record; a version 2
	// record is refused when it is nil, because the head tie would be skipped.
	Verify func(ctx context.Context, record Record, path string) error
	// PartitionDir is the store's telemetry partition directory
	// (store.TelemetryPartitionDir). When set, whatever it holds is removed
	// and the snapshot's partitions are installed there before the state
	// file. A snapshot with partitions and no PartitionDir is refused: its
	// telemetry below the head would be lost without a word.
	PartitionDir string
	// CandidateBudget, when set, stops Install starting another candidate once
	// this long has passed since it began and a candidate has failed: a long
	// list of failures must not use the request the replay needs. The candidate
	// being installed is never cut off by it.
	CandidateBudget time.Duration
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
	began := time.Now()
	for i := len(candidates) - 1; i >= 0; i-- {
		if cfg.CandidateBudget > 0 && len(failures) > 0 && time.Since(began) > cfg.CandidateBudget {
			failures = append(failures, fmt.Errorf("the %s candidate budget is spent after %d failed candidates; %d older ones were not tried", cfg.CandidateBudget, len(failures), i+1))
			break
		}
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
// platform keeps at least this many snapshot roots live for a small store (two
// for a store over 8 GiB; internal/platform snapshot_roots.go), so the newest
// candidates' chunks survive.
const MaxCandidates = maxCandidates

// MaxRestoreAttempts caps how many snapshot events a staged restore tries
// (PlanCandidates keeps the newest). A candidate the host no longer holds
// costs one record fetch and a few probes, not a download.
//
// The cap is not what bounds the work: the time budget (RestoreConfig.
// CandidateBudget) is. The cap only keeps the list finite. A base up to the
// chain gap (360,000 events) below a checkpoint is reached within it iff the
// cadence inside the window is at least 360,000/1024 = 352 events; the
// producer's fastest cadence is the tick bound (10 minutes of appends, about
// 1,100 events at d@e's 1.8/s) or four minutes of replay at the measured rate,
// whichever is later.
const MaxRestoreAttempts = 1024

// Scan reads the tape from its start and returns the newest snapshot_recorded
// events (oldest first). visit, when set, sees every event in order and stops
// the scan by returning true; the stopping event itself is still considered.
func Scan(ctx context.Context, src Source, computerID string, visit func(computerevent.DurableEvent) (stop bool)) ([]Candidate, error) {
	return scan(ctx, src, computerID, maxCandidates, visit)
}

// ScanAll is Scan without the newest-four cut: every snapshot_recorded event
// the tape holds up to the stop (oldest first). The host prunes snapshot roots
// the tape keeps naming, so which of them is usable is for the install to find
// out (it probes the host), not for the scan to guess.
func ScanAll(ctx context.Context, src Source, computerID string, visit func(computerevent.DurableEvent) (stop bool)) ([]Candidate, error) {
	return scan(ctx, src, computerID, 0, visit)
}

// PlanCandidates is the restore's candidate list: the snapshot events at or
// below target and not further than bound events behind it (a base further
// back is a replay recovery refuses), oldest first, at most the newest
// MaxRestoreAttempts. The tape alone supplies them: a host cannot add one.
func PlanCandidates(all []Candidate, target, bound uint64) []Candidate {
	var out []Candidate
	for _, c := range all {
		if c.Sequence > target || (target > bound && c.Sequence < target-bound) {
			continue
		}
		out = append(out, c)
	}
	if len(out) > MaxRestoreAttempts {
		out = out[len(out)-MaxRestoreAttempts:]
	}
	return out
}

func scan(ctx context.Context, src Source, computerID string, limit int, visit func(computerevent.DurableEvent) (stop bool)) ([]Candidate, error) {
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
				if limit > 0 && len(candidates) > limit {
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
	if (record.Version != Version && record.Version != VersionPageDigest) || record.ComputerID != cfg.ComputerID {
		return Record{}, fmt.Errorf("record is version %d of computer %q", record.Version, record.ComputerID)
	}
	if err := checkRecordForm(record); err != nil {
		return Record{}, err
	}
	// The head tie of a page-digest record is Verify's (the install does not
	// know the store's schema), so an install without one would skip it.
	if record.Version == VersionPageDigest && cfg.Verify == nil {
		return Record{}, errors.New("a version 2 record needs a Verify that checks its head row")
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
	// A page-cut record states its geometry: the chunks are page ranges of
	// pagesPerChunk pages and the file is pageCount pages. A record from the
	// VACUUM INTO era states none; its chunks are ChunkSize byte cuts, the
	// same cut, and are checked by the manifest size alone.
	chunkBytes := int64(ChunkSize)
	if record.PageSize != 0 || record.PageCount != 0 || record.PagesPerChunk != 0 {
		if record.PageSize <= 0 || record.PageCount <= 0 || record.PagesPerChunk <= 0 ||
			int64(record.PageSize)*record.PageCount != record.PlaintextSize ||
			int64(record.PageSize)*int64(record.PagesPerChunk) > 2*ChunkSize ||
			int64(len(entry.Chunks)) != (record.PageCount+int64(record.PagesPerChunk)-1)/int64(record.PagesPerChunk) {
			return Record{}, errors.New("record page geometry does not describe its manifest")
		}
		chunkBytes = int64(record.PageSize) * int64(record.PagesPerChunk)
	}

	// A snapshot the host has pruned fails here, before a chunk is downloaded:
	// the restore tries the next older one, which may be a root the host keeps.
	if prober, ok := cfg.Chunks.(ChunkProber); ok {
		if err := probeChunks(ctx, prober, recordChunkDigests(record)); err != nil {
			return Record{}, err
		}
	} else {
		noProbeOnce.Do(func() {
			log.Printf("snapshot: restore: the chunk store cannot probe the host, so a snapshot the host has pruned is downloaded until its first missing chunk")
		})
	}

	dek, err := cfg.Cipher.ExportKeyForEscrow(ctx, cfg.ComputerID)
	if err != nil {
		return Record{}, fmt.Errorf("export key: %w", err)
	}
	defer clear(dek)

	if len(record.Partitions) > 0 && strings.TrimSpace(cfg.PartitionDir) == "" {
		return Record{}, errors.New("the snapshot holds telemetry partitions and the restore names no partition directory")
	}
	for _, part := range record.Partitions {
		if !validPartitionPath(part.Path) || part.PageSize <= 0 || part.PageCount <= 0 || part.PagesPerChunk <= 0 ||
			int64(part.PageSize)*part.PageCount != part.Size ||
			int64(part.PageSize)*int64(part.PagesPerChunk) > 2*ChunkSize ||
			int64(len(part.Chunks)) != (part.PageCount+int64(part.PagesPerChunk)-1)/int64(part.PagesPerChunk) {
			return Record{}, fmt.Errorf("partition %q geometry does not describe its chunks", part.Path)
		}
	}

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
		// Verify opens the staging file in WAL mode: its sidecars never outlive
		// the install (the main file is renamed alone).
		_ = os.Remove(tempPath + "-wal")
		_ = os.Remove(tempPath + "-shm")
		if !installed {
			_ = temp.Close()
			_ = os.Remove(tempPath)
		}
	}()
	// A page-digest record is checked as the pages are written, before
	// anything opens the file (Verify included).
	var pages *PageDigest
	var sum io.Writer // stays a nil interface for a version 1 record
	if record.Version == VersionPageDigest {
		pages = NewPageDigest(record.PageSize, record.PageCount)
		sum = pages
	}
	if err := assemble(ctx, cfg, dek, entry.Chunks, entry.Size, chunkBytes, record.PageSize, temp, sum); err != nil {
		return Record{}, err
	}
	if pages != nil && pages.Sum() != record.PageDigest {
		return Record{}, errors.New("the restored pages do not match the record's page digest")
	}
	if cfg.Verify != nil {
		if err := cfg.Verify(ctx, record, tempPath); err != nil {
			return Record{}, fmt.Errorf("verify: %w", err)
		}
	}
	// The partitions go in before the state file: the state file's presence
	// is the restore's completion, and a stale partition of another lineage
	// never survives a restore.
	if strings.TrimSpace(cfg.PartitionDir) != "" {
		if err := installPartitions(ctx, cfg, dek, record.Partitions); err != nil {
			return Record{}, fmt.Errorf("install telemetry partitions: %w", err)
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

// noProbeOnce makes the skipped probe a log line once, not a silence.
var noProbeOnce sync.Once

// checkRecordForm refuses a record whose form disagrees with its version, or
// that carries neither form: a Version record holds the logical witness and no
// page digest, a VersionPageDigest record holds the page digest, its geometry
// and no witness. It is the install's own check, so it does not depend on
// RestoreConfig.Verify, which may be absent.
func checkRecordForm(record Record) error {
	switch record.Version {
	case Version:
		if record.PageDigest != "" {
			return errors.New("a version 1 record carries a page digest")
		}
		if record.WitnessRoot == "" || record.SchemaHash == "" {
			return errors.New("a version 1 record carries no witness")
		}
	case VersionPageDigest:
		if record.WitnessRoot != "" || record.SchemaHash != "" {
			return errors.New("a version 2 record carries a witness")
		}
		if !computerevent.IsSHA256(record.PageDigest) {
			return errors.New("a version 2 record carries no valid page digest")
		}
		if record.PageSize <= 0 || record.PageCount <= 0 || record.PagesPerChunk <= 0 {
			return errors.New("a version 2 record states no page geometry")
		}
	}
	return nil
}

// recordChunkDigests is every chunk an install fetches: the state file's and
// the telemetry partitions'.
func recordChunkDigests(record Record) []string {
	var out []string
	for _, f := range record.Manifest.Files {
		out = append(out, f.Chunks...)
	}
	for _, part := range record.Partitions {
		out = append(out, part.Chunks...)
	}
	return out
}

// probeChunks fails when the host says it does not hold one of the chunks. A
// probe error is unknown, not a refusal: the download decides.
func probeChunks(ctx context.Context, prober ChunkProber, chunks []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		missing string
		next    atomic.Int64
	)
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				i := int(next.Add(1)) - 1
				if i >= len(chunks) {
					return
				}
				if has, err := prober.HasChunk(ctx, chunks[i]); err == nil && !has {
					mu.Lock()
					if missing == "" {
						missing = chunks[i]
					}
					mu.Unlock()
					cancel()
					return
				}
			}
		}()
	}
	wg.Wait()
	if missing != "" {
		return fmt.Errorf("the host no longer holds chunk %s: the root was pruned", missing)
	}
	return nil
}

// assemble fetches, opens and writes the chunks of one file to w, checking
// the cut, and syncs and closes w.
func assemble(ctx context.Context, cfg RestoreConfig, dek []byte, chunks []string, size, chunkBytes int64, pageSize int, w *os.File, sum io.Writer) error {
	var written int64
	for i, digest := range chunks {
		if err := ctx.Err(); err != nil {
			return err
		}
		sealed, err := cfg.Chunks.GetChunk(ctx, digest)
		if err != nil {
			return fmt.Errorf("fetch chunk %s: %w", digest, err)
		}
		plain, err := filecas.OpenChunk(dek, cfg.ComputerID, digest, sealed)
		if err != nil {
			return fmt.Errorf("open chunk %s: %w", digest, err)
		}
		// Every chunk but the last is a whole range; the last is the rest.
		last := i == len(chunks)-1
		if int64(len(plain)) > chunkBytes || (!last && int64(len(plain)) != chunkBytes) {
			return fmt.Errorf("chunk %d has %d bytes, the cut is %d", i, len(plain), chunkBytes)
		}
		if pageSize != 0 && len(plain)%pageSize != 0 {
			return fmt.Errorf("chunk %d has %d bytes, not a whole number of %d-byte pages", i, len(plain), pageSize)
		}
		if sum != nil {
			_, _ = sum.Write(plain)
		}
		n, err := w.Write(plain)
		written += int64(n)
		if err != nil {
			return fmt.Errorf("write %s: %w", w.Name(), err)
		}
	}
	if written != size {
		return fmt.Errorf("restored %d bytes, the manifest has %d", written, size)
	}
	if err := w.Chmod(0o600); err != nil {
		return err
	}
	if err := w.Sync(); err != nil {
		return err
	}
	return w.Close()
}

// installPartitions empties cfg.PartitionDir and writes the snapshot's
// partitions into it.
func installPartitions(ctx context.Context, cfg RestoreConfig, dek []byte, parts []PartitionEntry) error {
	dir := cfg.PartitionDir
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if len(parts) == 0 {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, part := range parts {
		name := strings.TrimPrefix(part.Path, PartitionDirName+"/")
		temp, err := os.CreateTemp(dir, ".partition-restore-*")
		if err != nil {
			return err
		}
		if err := assemble(ctx, cfg, dek, part.Chunks, part.Size, int64(part.PageSize)*int64(part.PagesPerChunk), part.PageSize, temp, nil); err != nil {
			_ = temp.Close()
			_ = os.Remove(temp.Name())
			return fmt.Errorf("%s: %w", part.Path, err)
		}
		if err := os.Rename(temp.Name(), filepath.Join(dir, name)); err != nil {
			_ = os.Remove(temp.Name())
			return err
		}
	}
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
