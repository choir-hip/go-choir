package tracelog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// recover runs at Open, with the directory lock held, so no other writer can
// act while it works. It seals every segment a dead writer left without an
// index, each at the end of its last whole record, and sets aside any file
// whose header is not ours. It truncates and rewrites nothing. A failure
// returns an error and Open fails: the caller falls back to the tape.
func (l *Log) recover() error {
	ents, err := os.ReadDir(l.dir)
	if err != nil {
		return fmt.Errorf("tracelog: read %s: %w", l.dir, err)
	}
	type present struct{ tlog, tidx bool }
	segs := map[uint64]*present{}
	var maxSeg uint64
	for _, e := range ents {
		name := e.Name()
		var n uint64
		var kind string
		if v, ok := parseSegName(name, ".tlog"); ok {
			n, kind = v, "tlog"
		} else if v, ok := parseSegName(name, ".tidx"); ok {
			n, kind = v, "tidx"
		} else if strings.HasSuffix(name, ".bad") {
			if v, ok := parseSegName(strings.TrimSuffix(name, ".bad"), ".tlog"); ok {
				maxSeg = max(maxSeg, v)
			}
			continue
		} else {
			continue
		}
		if e.Type()&fs.ModeType != 0 {
			return corruptf("%s is not a regular file", filepath.Join(l.dir, name))
		}
		maxSeg = max(maxSeg, n)
		p := segs[n]
		if p == nil {
			p = &present{}
			segs[n] = p
		}
		if kind == "tlog" {
			p.tlog = true
		} else {
			p.tidx = true
		}
	}
	var unsealed []uint64
	for n, p := range segs {
		if p.tlog && !p.tidx {
			unsealed = append(unsealed, n)
		}
	}
	sort.Slice(unsealed, func(i, j int) bool { return unsealed[i] < unsealed[j] })
	for _, n := range unsealed {
		if err := l.recoverSegment(n); err != nil {
			return err
		}
	}
	l.nextSeg = maxSeg + 1
	l.pending = ReasonOpen
	if len(unsealed) > 0 {
		l.pending = ReasonCrashRecovered
	}
	return nil
}

// recoverSegment seals one unsealed segment, or sets it aside.
func (l *Log) recoverSegment(segno uint64) error {
	path := filepath.Join(l.dir, segmentName(segno))
	f, err := openRegular(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("tracelog: open segment %d: %w", segno, err)
	}
	defer f.Close()
	res, err := scanSegment(f, segno)
	if err != nil {
		return fmt.Errorf("tracelog: read segment %d: %w", segno, err)
	}
	switch res.class {
	case headerNewer:
		return fmt.Errorf("%w: %w: segment %d", ErrCorrupt, ErrUnsupportedVersion, segno)
	case headerTorn, headerBad:
		// A torn header means the crash came before the segment was durable,
		// so nothing in it was ever acknowledged; a bad header is not ours.
		// Either way the file holds no line anyone can read. It is renamed,
		// not rewritten, and its number is never used again.
		if err := os.Rename(path, path+".bad"); err != nil {
			return fmt.Errorf("tracelog: set aside segment %d: %w", segno, err)
		}
		if err := l.syncDir(l.dir); err != nil {
			return fmt.Errorf("tracelog: fsync directory after setting aside segment %d: %w", segno, err)
		}
		l.stats.Quarantined++
		return nil
	}
	// The whole records are sealed as they are; whatever follows stays on disk
	// and is never read. Read-back from the page cache is no proof of
	// durability, so the segment is fsynced before the index names it.
	if err := l.syncFile(f); err != nil {
		return fmt.Errorf("tracelog: fsync segment %d to seal it: %w", segno, err)
	}
	sum := res.idx.summarize(res.validEnd, res.created, l.now())
	if err := writeIndex(l.dir, segno, res.validEnd, res.size, sum, res.idx, l.syncFile, l.syncDir); err != nil {
		return err
	}
	switch res.state {
	case endTorn:
		l.stats.TornTailsCut++
	case endDamaged:
		l.stats.DamagedTails++
	}
	l.stats.SegmentsSealed++
	return nil
}

// writeIndex writes seg-N.tidx for a segment whose records are already
// fsynced: temp file, fsync, rename, fsync of the directory. A crash before
// the rename leaves the segment unsealed, which the next Open seals again.
func writeIndex(dir string, segno uint64, validEnd, tlogSize int64, sum *Summary, x *segIdx,
	syncFile func(*os.File) error, syncDir func(string) error) error {
	b := encodeIndex(segno, validEnd, tlogSize, sum, x)
	final := filepath.Join(dir, indexName(segno))
	tmp := final + ".tmp"
	if fi, err := os.Lstat(tmp); err == nil {
		if !fi.Mode().IsRegular() {
			return fmt.Errorf("tracelog: %s is not a regular file", tmp)
		}
		if err := os.Remove(tmp); err != nil {
			return fmt.Errorf("tracelog: remove stale index temp: %w", err)
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	t, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL|openNoFollow, 0o600)
	if err != nil {
		return fmt.Errorf("tracelog: create index: %w", err)
	}
	if _, err := t.Write(b); err != nil {
		t.Close()
		return fmt.Errorf("tracelog: write index: %w", err)
	}
	if err := syncFile(t); err != nil {
		t.Close()
		return fmt.Errorf("tracelog: fsync index: %w", err)
	}
	if err := t.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		return fmt.Errorf("tracelog: install index: %w", err)
	}
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("tracelog: fsync directory after installing index: %w", err)
	}
	return nil
}
