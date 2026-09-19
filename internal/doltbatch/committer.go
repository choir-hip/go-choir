// Package doltbatch coalesces per-mutation DOLT_COMMIT calls into debounced
// snapshot commits. Dolt working-set durability comes from ordinary SQL
// COMMIT (verified on Dolt 2.1.9 by the 2026-09-19 kill-9 drill); DOLT_COMMIT
// only creates an addressable snapshot in the commit graph. Committing every
// mutation grew the platform store to 6.9M commits / 218G of reachable
// history that no GC can collect (docs/evidence/
// platform-dolt-oldgen-218g-dead-history-2026-08-26.md), so mutations mark the
// store dirty and one committer snapshots on a cadence — promptly on
// recovery-boundary writes (checkpoints, replay watermarks), debounced
// otherwise.
package doltbatch

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// DefaultDebounce is the fallback cadence for non-boundary mutations.
// Recovery-boundary writes (checkpoint/watermark) commit promptly via
// CommitNow; everything else lands within one debounce window.
const DefaultDebounce = 45 * time.Second

// maxReasons bounds the accumulated commit-message fragment list so a
// long-running dirty window cannot grow the message without bound.
const maxReasons = 32

// maxMessageLen caps the DOLT_COMMIT message length.
const maxMessageLen = 512

// DebounceFromEnv returns the debounce interval from env (a Go duration such
// as "45s"), falling back to DefaultDebounce on empty or invalid values.
func DebounceFromEnv(env string) time.Duration {
	if raw := strings.TrimSpace(os.Getenv(env)); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return DefaultDebounce
}

// Committer serializes DOLT_COMMIT for one *sql.DB. Dolt commits are
// branch-wide, so a single committer snapshots the whole working set
// regardless of which connection wrote.
type Committer struct {
	db       *sql.DB
	scope    string
	debounce time.Duration
	logf     func(format string, args ...any)

	mu        sync.Mutex
	dirty     bool
	markGen   uint64
	reasons   []string
	timer     *time.Timer
	commitMu  sync.Mutex
	closed    bool
	lastErr   error
	lastErrAt time.Time
}

// New returns a Committer for db. scope prefixes commit messages and log
// lines (e.g. "platform", "cycle"). debounce <= 0 uses DefaultDebounce.
func New(db *sql.DB, scope string, debounce time.Duration) *Committer {
	if debounce <= 0 {
		debounce = DefaultDebounce
	}
	return &Committer{
		db:       db,
		scope:    scope,
		debounce: debounce,
		logf:     log.Printf,
	}
}

// Mark records a mutation reason and schedules a debounced snapshot commit.
// It never blocks on the commit itself: SQL COMMIT already made the write
// durable; the snapshot is bookkeeping.
func (c *Committer) Mark(reason string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return
	}
	c.markLocked(reason)
	if c.timer == nil {
		c.timer = time.AfterFunc(c.debounce, c.fire)
	}
}

// CommitNow marks the store dirty and commits synchronously. Use it for
// recovery-boundary writes (checkpoints, replay watermarks) where an
// addressable snapshot should exist at the boundary. "Nothing to commit" is
// success — the working set already matches HEAD.
func (c *Committer) CommitNow(ctx context.Context, reason string) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("doltbatch %s: committer closed", c.scope)
	}
	c.markLocked(reason)
	c.mu.Unlock()
	return c.commit(ctx)
}

// Flush commits any pending dirty state synchronously. Nil-safe.
func (c *Committer) Flush(ctx context.Context) error {
	if c == nil {
		return nil
	}
	return c.commit(ctx)
}

// LastError returns the most recent commit failure, for health surfaces.
func (c *Committer) LastError() (error, time.Time) {
	if c == nil {
		return nil, time.Time{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastErr, c.lastErrAt
}

// Close stops the debounce timer and performs a final synchronous commit of
// any pending dirty state. Marks after Close are dropped.
func (c *Committer) Close(ctx context.Context) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.mu.Unlock()
	return c.commit(ctx)
}

func (c *Committer) markLocked(reason string) {
	c.dirty = true
	c.markGen++
	if len(c.reasons) < maxReasons {
		c.reasons = append(c.reasons, reason)
	}
}

func (c *Committer) fire() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	err := c.commit(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.timer = nil
	if err != nil {
		c.logf("doltbatch %s: snapshot commit failed: %v", c.scope, err)
		if !c.closed && c.dirty {
			// Retry on the same cadence rather than waiting for the next
			// mark — a transient failure must not strand the snapshot.
			c.timer = time.AfterFunc(c.debounce, c.fire)
		}
	}
}

// commit performs the DOLT_COMMIT if dirty. A generation counter guards the
// clear: a Mark arriving while the SQL call is in flight keeps the store
// dirty so its mutation is snapshotted by the next commit.
func (c *Committer) commit(ctx context.Context) error {
	c.commitMu.Lock()
	defer c.commitMu.Unlock()

	c.mu.Lock()
	if !c.dirty {
		c.mu.Unlock()
		return nil
	}
	gen := c.markGen
	reasons := c.reasons
	c.mu.Unlock()

	message := c.message(reasons)
	if _, err := c.db.ExecContext(ctx, "CALL DOLT_COMMIT('-Am', ?)", message); err != nil {
		// Dolt returns "nothing to commit" when the working set has no
		// changes relative to HEAD (e.g. an idempotent upsert). The data is
		// already in the desired state — treat as success and clear dirty.
		if strings.Contains(err.Error(), "nothing to commit") {
			c.clearIfUnmarked(gen, len(reasons))
			return nil
		}
		err = fmt.Errorf("doltbatch %s: dolt commit: %w", c.scope, err)
		c.mu.Lock()
		c.lastErr = err
		c.lastErrAt = time.Now()
		c.mu.Unlock()
		return err
	}

	c.clearIfUnmarked(gen, len(reasons))
	c.mu.Lock()
	c.lastErr = nil
	c.lastErrAt = time.Time{}
	c.mu.Unlock()
	return nil
}

// clearIfUnmarked clears dirty state only if no Mark arrived during the
// commit; otherwise it keeps the store dirty and retains only the reasons
// appended after the snapshot attempt began.
func (c *Committer) clearIfUnmarked(gen uint64, oldReasons int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.markGen == gen {
		c.dirty = false
		c.reasons = nil
		return
	}
	c.reasons = c.reasons[oldReasons:]
}

func (c *Committer) message(reasons []string) string {
	var b strings.Builder
	b.WriteString(c.scope)
	b.WriteString(" snapshot")
	if len(reasons) > 0 {
		b.WriteString(": ")
		b.WriteString(strings.Join(reasons, "; "))
	}
	msg := b.String()
	if len(msg) > maxMessageLen {
		msg = msg[:maxMessageLen-3] + "..."
	}
	return msg
}
