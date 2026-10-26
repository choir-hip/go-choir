// Package rssfall waits for the host to see a guest return page cache: the
// Firecracker process's RSS falls once free page reporting hands the freed
// blocks back (docs/design/density-d6-page-cache-return-2026-10-26.md
// section 4.3, review 320 F1).
//
// A fenced or idle guest's RSS is already flat when the wait begins, and
// reporting runs on a 2 second delay, so a wait for "two close readings"
// would end before the return started. This waits for a fall of half the
// bytes the guest says it reclaimed and then for the last two readings to
// agree, up to a cap. The guest's number only sizes the wait; the exit is
// decided by the host's readings. Time is counted in polls, so a test needs
// no clock.
package rssfall

import "time"

const (
	// Poll is the interval between readings.
	Poll = 250 * time.Millisecond
	// DefaultCap is the longest wait.
	DefaultCap = 5 * time.Second
	// MinReclaimed: under this the guest returned too little to wait for.
	MinReclaimed = 128 << 20
	// Stable: two readings this close are no longer moving.
	Stable = 16 << 20
)

// Exit words.
const (
	ExitFell   = "fell"
	ExitCap    = "cap"
	ExitNoWait = "no_wait"
	ExitGone   = "gone"
)

// Result is what the wait saw. RSS is the last reading (the starting one if
// nothing could be read); MS is the time waited.
type Result struct {
	RSS  uint64
	MS   int64
	Exit string
}

// Wait polls read every Poll (through sleep) until the RSS has fallen by at
// least half of reclaimed from rss0 and two successive readings agree, the
// VM is gone (read reports false), or limit has been waited.
func Wait(read func() (uint64, bool), sleep func(time.Duration), rss0, reclaimed uint64, limit time.Duration) Result {
	if reclaimed < MinReclaimed {
		cur, ok := read()
		if !ok {
			cur = rss0
		}
		return Result{RSS: cur, Exit: ExitNoWait}
	}
	if limit <= 0 {
		return Result{RSS: rss0, Exit: ExitCap}
	}
	prev := rss0
	var elapsed time.Duration
	for {
		// The last poll is clipped to what the limit leaves, so a short cap
		// (the pressure step's remaining time) is not overrun by a poll.
		step := Poll
		if left := limit - elapsed; left < step {
			step = left
		}
		sleep(step)
		elapsed += step
		cur, ok := read()
		if !ok {
			return Result{RSS: prev, MS: elapsed.Milliseconds(), Exit: ExitGone}
		}
		fell := rss0 >= cur && rss0-cur >= reclaimed/2
		moved := cur - prev
		if prev > cur {
			moved = prev - cur
		}
		if fell && moved < Stable {
			return Result{RSS: cur, MS: elapsed.Milliseconds(), Exit: ExitFell}
		}
		prev = cur
		if elapsed >= limit {
			return Result{RSS: cur, MS: elapsed.Milliseconds(), Exit: ExitCap}
		}
	}
}
