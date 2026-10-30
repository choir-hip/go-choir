package dbx

import (
	"context"
	"math/rand/v2"
	"time"
)

// Retry runs fn up to attempts times while it fails with a conflict
// IsRetryable matches (a PostgreSQL serialization failure or deadlock,
// SQLITE_BUSY). Any other error, success, or a done context ends it. fn must
// be one whole transaction: under SERIALIZABLE the retry re-reads what the
// winner committed. The pause between tries is a few jittered milliseconds,
// so two losers do not collide again in step.
func Retry[T any](ctx context.Context, attempts int, fn func() (T, error)) (T, error) {
	for attempt := 1; ; attempt++ {
		v, err := fn()
		if err == nil || attempt >= attempts || !IsRetryable(err) || ctx.Err() != nil {
			return v, err
		}
		pause := time.Duration(attempt)*time.Millisecond + time.Duration(rand.Int64N(int64(4*time.Millisecond)))
		select {
		case <-ctx.Done():
			return v, err
		case <-time.After(pause):
		}
	}
}
