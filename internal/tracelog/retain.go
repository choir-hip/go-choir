package tracelog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// RetainResult says what one Retain call removed.
type RetainResult struct {
	Deleted        []uint64 // sealed segments removed, oldest first
	DeletedBytes   int64
	OrphansRemoved int // a .tidx whose .tlog was gone
	BadRemoved     int // old quarantined files
}

// Retain applies the retention bounds: whole sealed segments are deleted,
// oldest first, once they are older than MaxAge (measured from sealing) or
// while the directory holds more than MaxBytes. The open segment is never
// deleted. Nothing is compacted or rewritten. The .tlog is unlinked before
// its .tidx, so a crash between the two leaves an orphan index, not a
// segment that looks unsealed. A reader that has a segment open reads it to
// the end. A poisoned log refuses: it touches no file.
func (l *Log) Retain() (RetainResult, error) {
	var res RetainResult
	l.mu.Lock()
	if err := l.usableLocked(); err != nil {
		l.mu.Unlock()
		return res, err
	}
	var curNo uint64
	if l.cur != nil {
		curNo = l.cur.segno
	}
	l.mu.Unlock()

	l.retainMu.Lock()
	defer l.retainMu.Unlock()
	maxAge, maxBytes := l.retainMA, l.retainMB
	rd := l.Reader()
	ls, err := rd.list()
	if err != nil {
		return res, err
	}
	now := l.now()

	for _, o := range ls.orphans {
		if err := os.Remove(filepath.Join(l.dir, indexName(o.segno))); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return res, fmt.Errorf("tracelog: remove orphan index %d: %w", o.segno, err)
		}
		res.OrphansRemoved++
	}
	if maxAge > 0 {
		var keep []badFile
		for _, b := range ls.bad {
			if now.Sub(b.mod) > maxAge {
				if err := os.Remove(filepath.Join(l.dir, b.name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
					return res, fmt.Errorf("tracelog: remove %s: %w", b.name, err)
				}
				res.BadRemoved++
				continue
			}
			keep = append(keep, b)
		}
		ls.bad = keep
	}

	// The sealed segments, oldest first. The open one is not among them: it
	// has no index.
	var sealed []segFile
	var total int64
	for _, f := range ls.segs {
		total += f.tlogSize + f.idxSize
		if f.hasIdx && f.segno != curNo {
			sealed = append(sealed, f)
		}
	}
	for _, b := range ls.bad {
		total += b.size
	}
	remove := func(f segFile) error {
		if err := os.Remove(rd.tlogPath(f.segno)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("tracelog: remove segment %d: %w", f.segno, err)
		}
		if err := os.Remove(filepath.Join(l.dir, indexName(f.segno))); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("tracelog: remove index %d: %w", f.segno, err)
		}
		res.Deleted = append(res.Deleted, f.segno)
		res.DeletedBytes += f.tlogSize + f.idxSize
		total -= f.tlogSize + f.idxSize
		return nil
	}
	i := 0
	for ; i < len(sealed); i++ {
		f := sealed[i]
		old := false
		if maxAge > 0 {
			sealedAt := f.tlogMod
			if sum := rd.summary(f); sum != nil {
				sealedAt = time.Unix(0, sum.SealedAt)
			}
			old = now.Sub(sealedAt) > maxAge
		}
		if !old && !(maxBytes > 0 && total > maxBytes) {
			break
		}
		if err := remove(f); err != nil {
			return res, err
		}
	}
	if len(res.Deleted) > 0 || res.OrphansRemoved > 0 || res.BadRemoved > 0 {
		if err := l.syncDir(l.dir); err != nil {
			return res, fmt.Errorf("tracelog: fsync directory after retention: %w", err)
		}
	}
	return res, nil
}
