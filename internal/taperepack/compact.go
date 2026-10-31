package taperepack

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/yusefmosiah/go-choir/internal/tapestore"
)

// Compact is the one-off backfill of rule F.1
// (docs/design/small-object-storage-2026-10-31.md): every sealed segment
// already on disk is rewritten once, byte for byte, as a fresh sequential
// file (tapestore.CompactSealed), which frees the copy-on-write bookends its
// fsynced appends left behind. New segments are compacted at seal by the
// appender; this tool covers the ones sealed before that and any the
// seal-time worker missed.
//
// It is a dry run unless Apply is set. Each copy is verified (size, SHA-256
// of the whole file, and with VerifyRecords every indexed record) before its
// rename; a segment that fails is left as it was and the run goes on. It runs
// on one low-priority OS thread (nice 19, idle IO class on Linux), paced by
// BytesPerSecond and Pause. Memory is one copy buffer (1 MiB) plus one index
// (at most 2.3 MB) at a time.
//
// st_blocks does not show the reclaim on btrfs (it counts referenced bytes,
// not the bookended extents a file pins), so the report carries the extent
// counts and, when a Measure function is given (the command passes compsize),
// the disk usage of each segment before and after, plus the filesystem's free
// bytes around the run (approximate: other writers, and btrfs frees space at
// its next commit).

// CompactOptions configure Compact.
type CompactOptions struct {
	// Root is the artifacts root (the directory holding tape-seg/).
	Root string
	// Computers limits the run; empty is every computer. Apply needs either
	// Computers or AllComputers.
	Computers    []string
	AllComputers bool
	// Apply rewrites; without it the run only measures and decides.
	Apply bool
	// BytesPerSecond paces each copy; 0 is 16 MiB/s.
	BytesPerSecond int64
	// Pause is the rest between segments; 0 is one second.
	Pause time.Duration
	// VerifyRecords re-reads every indexed record of each copy.
	VerifyRecords bool
	// Compress asks btrfs to compress the copies.
	Compress bool
	// MinAvgExtentBytes: see tapestore.CompactOptions.
	MinAvgExtentBytes int64
	// MaxSegments stops after this many compactions; 0 is no limit.
	MaxSegments int
	// Measure returns a file's disk usage in bytes (compsize); nil skips it.
	Measure func(path string) (int64, error)
	// Progress receives one line per segment.
	Progress func(string)
}

// CompactReport is the run's result.
type CompactReport struct {
	Apply        bool `json:"apply"`
	Computers    int  `json:"computers"`
	Segments     int  `json:"sealed_segments"`
	Compacted    int  `json:"compacted"`
	WouldCompact int  `json:"would_compact"`
	Skipped      int  `json:"skipped"`
	Errors       int  `json:"errors"`
	// Bytes is the logical size of the compacted (or would-be) segments.
	Bytes          int64 `json:"bytes"`
	StBlocksBefore int64 `json:"st_blocks_bytes_before"`
	StBlocksAfter  int64 `json:"st_blocks_bytes_after"`
	ExtentsBefore  int64 `json:"extents_before"`
	ExtentsAfter   int64 `json:"extents_after"`
	// DiskBefore/After sum Measure over the compacted segments; -1 without it.
	DiskBefore int64 `json:"disk_bytes_before"`
	DiskAfter  int64 `json:"disk_bytes_after"`
	// Reclaimed is DiskBefore-DiskAfter (with Measure).
	Reclaimed    int64                     `json:"disk_bytes_reclaimed"`
	FSFreeBefore int64                     `json:"fs_free_bytes_before"`
	FSFreeAfter  int64                     `json:"fs_free_bytes_after"`
	Skips        map[string]int            `json:"skips,omitempty"`
	Failures     []string                  `json:"failures,omitempty"`
	Results      []tapestore.CompactResult `json:"results,omitempty"`
}

// Compact runs the backfill.
func Compact(opts CompactOptions) (*CompactReport, error) {
	if opts.Root == "" {
		return nil, errors.New("taperepack compact: root is required")
	}
	if opts.Apply && len(opts.Computers) == 0 && !opts.AllComputers {
		return nil, errors.New("taperepack compact: --apply needs --computer or --all-computers")
	}
	if len(opts.Computers) > 0 && opts.AllComputers {
		return nil, errors.New("taperepack compact: --computer and --all-computers exclude each other")
	}
	if opts.BytesPerSecond == 0 {
		opts.BytesPerSecond = 16 << 20
	}
	if opts.Pause == 0 {
		opts.Pause = time.Second
	}
	root := filepath.Clean(opts.Root)
	ids := opts.Computers
	if len(ids) == 0 {
		entries, err := os.ReadDir(filepath.Join(root, tapestore.SegDirName))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() && tapestore.ValidComputerID(e.Name()) {
				ids = append(ids, e.Name())
			}
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		if !tapestore.ValidComputerID(id) {
			return nil, fmt.Errorf("taperepack compact: invalid computer id %q", id)
		}
	}
	rep := &CompactReport{Apply: opts.Apply, Computers: len(ids), DiskBefore: -1, DiskAfter: -1, Skips: map[string]int{}}
	rep.FSFreeBefore = fsFree(root)
	reader := tapestore.NewReader(root)
	var runErr error
	tapestore.RunLowPriority(func() {
		runErr = compactAll(opts, root, ids, reader, rep)
	})
	rep.FSFreeAfter = fsFree(root)
	if rep.DiskBefore >= 0 && rep.DiskAfter >= 0 {
		rep.Reclaimed = rep.DiskBefore - rep.DiskAfter
	}
	return rep, runErr
}

func compactAll(opts CompactOptions, root string, ids []string, reader *tapestore.Reader, rep *CompactReport) error {
	progress := opts.Progress
	if progress == nil {
		progress = func(string) {}
	}
	if opts.Measure != nil {
		rep.DiskBefore, rep.DiskAfter = 0, 0
	}
	for _, id := range ids {
		segnos, err := reader.SealedSegments(id)
		if err != nil {
			rep.Errors++
			rep.Failures = append(rep.Failures, fmt.Sprintf("%s: list: %v", id, err))
			continue
		}
		for _, segno := range segnos {
			if opts.MaxSegments > 0 && rep.Compacted >= opts.MaxSegments {
				return nil
			}
			rep.Segments++
			path := filepath.Join(root, tapestore.SegDirName, id, tapestore.SegmentFileName(segno))
			var diskBefore int64 = -1
			if opts.Measure != nil {
				if d, err := opts.Measure(path); err == nil {
					diskBefore = d
				} else {
					progress(fmt.Sprintf("%s seg %d: measure: %v", id, segno, err))
				}
			}
			res, err := tapestore.CompactSealed(root, id, segno, tapestore.CompactOptions{
				BytesPerSecond:    opts.BytesPerSecond,
				VerifyRecords:     opts.VerifyRecords,
				Compress:          opts.Compress,
				MinAvgExtentBytes: opts.MinAvgExtentBytes,
				DryRun:            !opts.Apply,
			})
			rep.Results = append(rep.Results, res)
			if err != nil {
				rep.Errors++
				rep.Failures = append(rep.Failures, fmt.Sprintf("%s seg %d: %v", id, segno, err))
				progress(fmt.Sprintf("%s seg %d: refused: %v", id, segno, err))
				continue
			}
			switch res.Action {
			case tapestore.CompactActionSkipped:
				rep.Skipped++
				rep.Skips[res.Skip]++
				progress(fmt.Sprintf("%s seg %d: skipped (%s), %d extents", id, segno, res.Skip, res.ExtentsBefore))
				continue
			case tapestore.CompactActionWouldCompact:
				rep.WouldCompact++
			case tapestore.CompactActionCompacted:
				rep.Compacted++
			}
			rep.Bytes += res.Size
			rep.StBlocksBefore += res.StBlocksBefore
			rep.StBlocksAfter += res.StBlocksAfter
			if res.ExtentsBefore > 0 {
				rep.ExtentsBefore += int64(res.ExtentsBefore)
			}
			if res.ExtentsAfter > 0 {
				rep.ExtentsAfter += int64(res.ExtentsAfter)
			}
			var diskAfter int64 = -1
			if opts.Measure != nil && diskBefore >= 0 {
				rep.DiskBefore += diskBefore
				if res.Action == tapestore.CompactActionCompacted {
					if d, err := opts.Measure(path); err == nil {
						diskAfter = d
					}
				} else {
					diskAfter = diskBefore
				}
				if diskAfter >= 0 {
					rep.DiskAfter += diskAfter
				} else {
					rep.DiskAfter += diskBefore
				}
			}
			progress(fmt.Sprintf("%s seg %d: %s, %d bytes, extents %d -> %d, disk %d -> %d", id, segno, res.Action, res.Size, res.ExtentsBefore, res.ExtentsAfter, diskBefore, diskAfter))
			if res.Action == tapestore.CompactActionCompacted {
				time.Sleep(opts.Pause)
			}
		}
	}
	return nil
}
