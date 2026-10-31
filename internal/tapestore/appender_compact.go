package tapestore

import (
	"errors"
	"log"
	"sync"
	"time"
)

// Seal-time compaction (rule F.2 of
// docs/design/small-object-storage-2026-10-31.md): every segment this
// Appender seals (rotation, Seal, or the recovery seal of a segment it found
// open) is queued for CompactSealed on one background worker. The worker runs
// on its own OS thread at nice 19 and idle IO class, waits Delay after the
// seal, and copies at BytesPerSecond. It takes no appender lock, so appends
// never wait for it. A full queue drops the job (counted): the backfill tool
// (taperepack compact) finds any segment the worker missed.

// SealCompaction configures StartSealCompaction.
type SealCompaction struct {
	// Delay after a seal before the copy; 0 is 10 s.
	Delay time.Duration
	// BytesPerSecond paces each copy; 0 is 32 MiB/s.
	BytesPerSecond int64
	// Queue is how many sealed segments may wait; 0 is 64.
	Queue int
	// Compress asks btrfs to compress the copies (CompactOptions.Compress).
	Compress bool
	// Logf receives one line per failed or refused compaction; nil is log.Printf.
	Logf func(format string, args ...any)
}

type compactJob struct {
	id    string
	segno uint64
}

type sealCompactor struct {
	cfg  SealCompaction
	jobs chan compactJob
	stop chan struct{}
	done chan struct{}
}

// StartSealCompaction starts the seal-time compaction worker. A second call
// while it runs does nothing.
func (a *Appender) StartSealCompaction(cfg SealCompaction) {
	if cfg.Delay == 0 {
		cfg.Delay = 10 * time.Second
	}
	if cfg.BytesPerSecond == 0 {
		cfg.BytesPerSecond = 32 << 20
	}
	if cfg.Queue <= 0 {
		cfg.Queue = 64
	}
	if cfg.Logf == nil {
		cfg.Logf = log.Printf
	}
	a.compMu.Lock()
	defer a.compMu.Unlock()
	if a.comp != nil {
		return
	}
	c := &sealCompactor{cfg: cfg, jobs: make(chan compactJob, cfg.Queue), stop: make(chan struct{}), done: make(chan struct{})}
	a.comp = c
	go func() {
		defer close(c.done)
		lockLowPriorityThread()
		a.compactLoop(c)
	}()
}

// StopSealCompaction stops the worker and waits for it; a copy in progress is
// abandoned (its temp file removed, the segment unchanged). Queued jobs are
// dropped.
func (a *Appender) StopSealCompaction() {
	a.compMu.Lock()
	c := a.comp
	a.comp = nil
	a.compMu.Unlock()
	if c == nil {
		return
	}
	close(c.stop)
	<-c.done
}

// queueCompaction hands a just-sealed segment to the worker without blocking.
func (a *Appender) queueCompaction(id string, segno uint64) {
	a.compMu.Lock()
	c := a.comp
	a.compMu.Unlock()
	if c == nil {
		return
	}
	select {
	case c.jobs <- compactJob{id: id, segno: segno}:
	default:
		a.compDropped.Add(1)
	}
}

func (a *Appender) compactLoop(c *sealCompactor) {
	for {
		var job compactJob
		select {
		case <-c.stop:
			return
		case job = <-c.jobs:
		}
		if sleepCancel(c.cfg.Delay, c.stop) != nil {
			return
		}
		res, err := CompactSealed(a.root, job.id, job.segno, CompactOptions{BytesPerSecond: c.cfg.BytesPerSecond, Compress: c.cfg.Compress, Cancel: c.stop})
		switch {
		case errors.Is(err, ErrCompactCanceled):
			return
		case err != nil:
			a.compErrors.Add(1)
			c.cfg.Logf("tapestore: compact computer %s segment %d: %v", job.id, job.segno, err)
		case res.Action == CompactActionCompacted:
			a.compacted.Add(1)
			a.compactedBytes.Add(uint64(res.Size))
		default:
			a.compSkipped.Add(1)
		}
	}
}

// compactorState is embedded in Appender.
type compactorState struct {
	compMu sync.Mutex
	comp   *sealCompactor
}
