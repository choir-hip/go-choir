package agentcore

import (
	"errors"
	"testing"
	"time"
)

// A run's end must reach the store
// (problems/failed-terminal-run-write-blocks-desk-for-an-hour-2026-10-16.md).
// Ways the retry can fail, written before the code:
//
//   - it gives up after the first failure, so a stored run stays
//     "running" and absorbs its desk's wakes until the progress sweep;
//   - it retries forever, or past its bound, on a store that never recovers;
//   - it keeps writing after another path ended the run (persist reports
//     not persisted), overwriting that fate;
//   - the terminal follow-ups (outcome binding, inbox recheck) run twice,
//     or never once the write lands.
func TestRetryRunEndLandsAfterTransientFailures(t *testing.T) {
	var waits []time.Duration
	wait := func(d time.Duration) { waits = append(waits, d) }
	delays := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second}

	t.Run("lands after the store recovers", func(t *testing.T) {
		waits = nil
		attempts, after := 0, 0
		persisted, err := retryRunEnd(func() (bool, error) {
			attempts++
			if attempts < 3 {
				return false, errors.New("corpusd returned 400: no space left on device")
			}
			return true, nil
		}, func() { after++ }, wait, delays)
		if !persisted || err != nil || attempts != 3 || after != 1 {
			t.Fatalf("persisted=%v err=%v attempts=%d after=%d, want landed on attempt 3 with one follow-up", persisted, err, attempts, after)
		}
		if len(waits) != 2 || waits[0] != time.Second || waits[1] != 2*time.Second {
			t.Fatalf("waits = %v, want [1s 2s]", waits)
		}
	})

	t.Run("stops when another path ended the run", func(t *testing.T) {
		waits = nil
		attempts, after := 0, 0
		persisted, err := retryRunEnd(func() (bool, error) {
			attempts++
			if attempts == 1 {
				return false, errors.New("transient")
			}
			return false, nil
		}, func() { after++ }, wait, delays)
		if persisted || err != nil || attempts != 2 || after != 0 {
			t.Fatalf("persisted=%v err=%v attempts=%d after=%d, want stop on attempt 2 with no follow-up", persisted, err, attempts, after)
		}
	})

	t.Run("gives up at its bound", func(t *testing.T) {
		waits = nil
		attempts, after := 0, 0
		persisted, err := retryRunEnd(func() (bool, error) {
			attempts++
			return false, errors.New("store down")
		}, func() { after++ }, wait, delays)
		if persisted || err == nil || attempts != len(delays)+1 || after != 0 || len(waits) != len(delays) {
			t.Fatalf("persisted=%v err=%v attempts=%d after=%d waits=%v, want %d attempts then the last error", persisted, err, attempts, after, waits, len(delays)+1)
		}
	})
}
