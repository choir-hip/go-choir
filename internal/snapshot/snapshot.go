// Package snapshot is snapshot version 1: the guest SQLite store streamed page
// by page from one read snapshot, sealed under the computer's key, stored
// chunk by chunk in the platform's file content store, and named on the tape
// by a synchronous snapshot_recorded event
// (docs/design/data-system-build-sheet-2026-10-30.md, decision 7). It replaces
// the projection bases.
//
// Capture opens one read-only transaction on the store's read pool and reads
// the event head as its first statement, so the head belongs to the
// transaction's WAL snapshot. It then reads every database page of that
// snapshot through SQLite's sqlite_dbpage virtual table (pages as the snapshot
// sees them, WAL included) and computes the witness v3 in the same
// transaction: head, pages and witness are one state. There is no temporary
// copy of the store and no size cap; memory is a handful of 4 MiB buffers.
// Writers and appends never wait for the stream, but the open transaction pins
// the WAL until it ends, so the stream has a time budget.
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
// store head sequence, the file-tree root and the witness v3 root, is the
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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yusefmosiah/go-choir/internal/agentprofile"
	"github.com/yusefmosiah/go-choir/internal/computerevent"
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

// ErrTimeBudget means the page stream and the witness did not finish within
// Config.MaxDuration. The read transaction is rolled back, nothing is
// registered and no event is appended.
var ErrTimeBudget = errors.New("snapshot: exceeded the time budget")

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
	// hash of the captured file.
	WitnessRoot string `json:"witness_root"`
	SchemaHash  string `json:"schema_hash"`
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
	// StreamTime is the time to read every page, WitnessTime the time to
	// compute the witness, ReadTxTime how long the read transaction (and so
	// the WAL pin) lasted.
	StreamTime  time.Duration
	WitnessTime time.Duration
	ReadTxTime  time.Duration
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
	// Witness computes the witness v3 inside the same transaction, with the
	// same extractor configuration the restore check uses on the reassembled
	// file.
	Witness func(ctx context.Context, tx *sql.Tx) (selfdevprotocol.VMLocalContentWitness, error)
	// FileTreeRoot optionally names the Files tree root to record.
	FileTreeRoot func(ctx context.Context) (string, error)
	// Partitions, when set, adds the store's telemetry partitions.
	Partitions *PartitionSource
	Cipher     *computerevent.PrivateArtifactCipher
	Chunks     ChunkStore
	Appender   Appender
	// MaxDuration, when positive, bounds the read transaction and the chunk
	// uploads (a long read pins the WAL). The stream aborts with
	// ErrTimeBudget, which leaves no root and no event.
	MaxDuration time.Duration
	// Now overrides the clock in tests.
	Now func() time.Time

	// pageHook, when set, runs in the reader before each page is taken (tests
	// commit writes in the middle of the stream).
	pageHook func(pgno int64)
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

// TakeSnapshot streams the store's pages from one read snapshot on the read
// pool, seals and stores the chunks the host does not already hold, and
// appends the snapshot_recorded event. Nothing is held: writers and appends
// continue while the pages are read. The head, the pages and the witness are
// all of the one transaction's snapshot, and the events committed after it are
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
		Version:       Version,
		ComputerID:    cfg.ComputerID,
		CreatedAt:     now().UTC().Format(time.RFC3339Nano),
		PlaintextSize: entry.Size,
		PageSize:      got.pageSize,
		PageCount:     got.pageCount,
		PagesPerChunk: got.pagesPerChunk,
		WitnessRoot:   got.witness.ContentRoot,
		SchemaHash:    got.witness.SchemaHash,
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
		ActorProfile:   agentprofile.Management,
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
	witness       selfdevprotocol.VMLocalContentWitness
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

// capture opens the one read transaction and does everything that must see one
// state: the head (first statement), the page stream, the witness.
func capture(ctx context.Context, cfg Config, dek []byte) (out captured, retErr error) {
	budgetCtx := ctx
	if cfg.MaxDuration > 0 {
		var stop context.CancelFunc
		budgetCtx, stop = context.WithTimeoutCause(ctx, cfg.MaxDuration, ErrTimeBudget)
		defer stop()
	}
	runCtx, cancel := context.WithCancelCause(budgetCtx)
	defer cancel(nil)
	// cause is why the run ended: the budget, a worker's failure, or the
	// caller's cancellation.
	cause := func(err error) error {
		if c := context.Cause(runCtx); c != nil {
			return c
		}
		return err
	}

	// The partitions stay in place from before the read transaction until the
	// last one is read.
	if cfg.Partitions != nil && cfg.Partitions.Hold != nil {
		release := cfg.Partitions.Hold()
		defer release()
	}
	started := time.Now()
	tx, err := cfg.DB.BeginTx(runCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return out, fmt.Errorf("snapshot: begin read transaction: %w", cause(err))
	}
	txOpen := true
	rollback := func() {
		if txOpen {
			txOpen = false
			_ = tx.Rollback()
			out.stats.ReadTxTime = time.Since(started)
		}
	}
	defer rollback()

	// The head is the transaction's first statement: it fixes the WAL
	// snapshot, so the head names the state the pages are read from.
	if out.head, err = cfg.Head(runCtx, tx); err != nil {
		return out, fmt.Errorf("snapshot: read head: %w", cause(err))
	}
	var parts []PartitionFile
	if cfg.Partitions != nil && cfg.Partitions.List != nil {
		if parts, err = cfg.Partitions.List(runCtx, tx); err != nil {
			return out, fmt.Errorf("snapshot: list telemetry partitions: %w", cause(err))
		}
		for _, part := range parts {
			if !validPartitionPath(part.Path) || part.DB == nil {
				return out, fmt.Errorf("snapshot: unusable telemetry partition %q", part.Path)
			}
		}
	}

	streamStart := time.Now()
	fs, err := streamFile(runCtx, cfg, dek, cancel, cause, tx, cfg.pageHook)
	if err != nil {
		return out, err
	}
	out.stats.StreamTime = time.Since(streamStart)

	// The witness is computed in the same transaction while the workers drain
	// the last chunks.
	witnessStart := time.Now()
	if out.witness, err = cfg.Witness(runCtx, tx); err != nil {
		cancel(err)
		fs.up.finish()
		return out, fmt.Errorf("snapshot: witness of the snapshot: %w", cause(err))
	}
	out.stats.WitnessTime = time.Since(witnessStart)
	rollback()

	if err := fs.finish(runCtx); err != nil {
		return out, err
	}
	out.pageSize, out.pageCount, out.pagesPerChunk = fs.pageSize, fs.pageCount, fs.pagesPerChunk
	out.entry = filecas.FileEntry{Path: StateFileName, Mode: 0o600, Size: int64(fs.pageSize) * fs.pageCount, Chunks: fs.up.digests}
	out.stats.Pages = fs.pageCount
	out.stats.PageSize = fs.pageSize
	fs.addStats(&out.stats)

	// The partitions, each from its own read transaction, begun after the
	// state file's: a partition commits before the state file does, so each
	// holds every row at or below the head (and maybe rows above it, which
	// Open drops at the restore and the replay writes again).
	for _, part := range parts {
		entry, err := capturePartition(runCtx, cfg, dek, cancel, cause, part)
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

// fileStream is one SQLite file's page stream: its geometry and the uploader
// sealing its chunks.
type fileStream struct {
	up            *chunkUploader
	pageSize      int
	pageCount     int64
	pagesPerChunk int
	chunkCount    int
	cut           int
}

// streamFile reads every page of tx's database snapshot into chunks handed to
// a new uploader. On an error it stops and drains the uploader itself; on
// success the caller finishes it (fileStream.finish).
func streamFile(runCtx context.Context, cfg Config, dek []byte, cancel context.CancelCauseFunc, cause func(error) error, tx *sql.Tx, hook func(int64)) (*fileStream, error) {
	fs := &fileStream{}
	if err := tx.QueryRowContext(runCtx, `PRAGMA page_size`).Scan(&fs.pageSize); err != nil {
		return nil, fmt.Errorf("snapshot: page size: %w", cause(err))
	}
	if err := tx.QueryRowContext(runCtx, `PRAGMA page_count`).Scan(&fs.pageCount); err != nil {
		return nil, fmt.Errorf("snapshot: page count: %w", cause(err))
	}
	pageSize := fs.pageSize
	if pageSize <= 0 || pageSize > ChunkSize || ChunkSize%pageSize != 0 || fs.pageCount <= 0 {
		return nil, fmt.Errorf("snapshot: unusable store geometry: %d pages of %d bytes", fs.pageCount, pageSize)
	}
	fs.pagesPerChunk = ChunkSize / pageSize
	fs.chunkCount = int((fs.pageCount + int64(fs.pagesPerChunk) - 1) / int64(fs.pagesPerChunk))

	up := newChunkUploader(cfg, dek, ChunkSize, fs.chunkCount, cancel)
	up.start(runCtx)
	fs.up = up
	fail := func(err error) (*fileStream, error) {
		cancel(err)
		up.finish()
		return nil, err
	}

	streamStart := time.Now()
	var pages int64
	progress := func(err error) error {
		return fmt.Errorf("%w (%d of %d pages read, %s into the stream)", cause(err), pages, fs.pageCount, time.Since(streamStart).Round(time.Second))
	}
	rows, err := tx.QueryContext(runCtx, `SELECT pgno, data FROM sqlite_dbpage`)
	if err != nil {
		return fail(fmt.Errorf("snapshot: read pages: %w", progress(err)))
	}
	buf := <-up.free
	chunkIndex := 0
	flush := func() error {
		select {
		case up.jobs <- chunkJob{index: chunkIndex, data: buf}:
		case <-runCtx.Done():
			return runCtx.Err()
		}
		chunkIndex++
		select {
		case buf = <-up.free:
		case <-runCtx.Done():
			return runCtx.Err()
		}
		return nil
	}
	for rows.Next() {
		var pgno int64
		var data sql.RawBytes
		if err := rows.Scan(&pgno, &data); err != nil {
			_ = rows.Close()
			return fail(fmt.Errorf("snapshot: scan page: %w", progress(err)))
		}
		if pgno != pages+1 || len(data) != pageSize {
			_ = rows.Close()
			return fail(fmt.Errorf("snapshot: page stream out of order or short: got page %d of %d bytes after %d pages (page size %d)", pgno, len(data), pages, pageSize))
		}
		if hook != nil {
			hook(pgno)
		}
		buf = append(buf, data...)
		pages++
		if len(buf) == fs.pagesPerChunk*pageSize {
			if err := flush(); err != nil {
				_ = rows.Close()
				return fail(fmt.Errorf("snapshot: %w", progress(err)))
			}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
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
	return fs, nil
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

// capturePartition streams one partition from its own read transaction.
func capturePartition(runCtx context.Context, cfg Config, dek []byte, cancel context.CancelCauseFunc, cause func(error) error, part PartitionFile) (capturedPartition, error) {
	ptx, err := part.DB.BeginTx(runCtx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return capturedPartition{}, fmt.Errorf("snapshot: begin read of %s: %w", part.Path, cause(err))
	}
	// The first statement fixes the partition's read snapshot.
	var one int
	if err := ptx.QueryRowContext(runCtx, `SELECT 1`).Scan(&one); err != nil {
		_ = ptx.Rollback()
		return capturedPartition{}, fmt.Errorf("snapshot: read %s: %w", part.Path, cause(err))
	}
	fs, err := streamFile(runCtx, cfg, dek, cancel, cause, ptx, nil)
	_ = ptx.Rollback()
	if err != nil {
		return capturedPartition{}, fmt.Errorf("%s: %w", part.Path, err)
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
	// Verify optionally checks the unsealed file (for example its witness
	// against Record.WitnessRoot) before it is installed.
	Verify func(ctx context.Context, record Record, path string) error
	// PartitionDir is the store's telemetry partition directory
	// (store.TelemetryPartitionDir). When set, whatever it holds is removed
	// and the snapshot's partitions are installed there before the state
	// file. A snapshot with partitions and no PartitionDir is refused: its
	// telemetry below the head would be lost without a word.
	PartitionDir string
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
		if !installed {
			_ = temp.Close()
			_ = os.Remove(tempPath)
		}
	}()
	if err := assemble(ctx, cfg, dek, entry.Chunks, entry.Size, chunkBytes, record.PageSize, temp); err != nil {
		return Record{}, err
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

// assemble fetches, opens and writes the chunks of one file to w, checking
// the cut, and syncs and closes w.
func assemble(ctx context.Context, cfg RestoreConfig, dek []byte, chunks []string, size, chunkBytes int64, pageSize int, w *os.File) error {
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
		if err := assemble(ctx, cfg, dek, part.Chunks, part.Size, int64(part.PageSize)*int64(part.PagesPerChunk), part.PageSize, temp); err != nil {
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
