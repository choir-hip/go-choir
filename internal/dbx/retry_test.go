package dbx_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yusefmosiah/go-choir/internal/dbx"
)

// Ways a serialization retry could go wrong, each checked below:
//  1. it gives up on a wrapped 40001 or 40P01 instead of retrying;
//  2. it retries an error that is not a conflict, repeating a refusal or a
//     side effect;
//  3. it retries without end;
//  4. it keeps retrying after the caller's context is done;
//  5. it drops the value of the attempt that succeeded.
func TestRetryRetriesOnlyConflicts(t *testing.T) {
	ctx := context.Background()
	conflict := fmt.Errorf("commit: %w", &pgconn.PgError{Code: "40001"})
	deadlock := fmt.Errorf("update: %w", &pgconn.PgError{Code: "40P01"})

	calls := 0
	got, err := dbx.Retry(ctx, 5, func() (int, error) {
		calls++
		switch calls {
		case 1:
			return 0, conflict
		case 2:
			return 0, deadlock
		}
		return 7, nil
	})
	if err != nil || got != 7 || calls != 3 {
		t.Fatalf("conflicts then success: got %d, %v after %d calls; want 7, nil after 3", got, err, calls)
	}

	refusal := errors.New("guest projection mismatch")
	calls = 0
	if _, err := dbx.Retry(ctx, 5, func() (int, error) { calls++; return 0, refusal }); !errors.Is(err, refusal) || calls != 1 {
		t.Fatalf("a refusal was retried: %d calls, err %v", calls, err)
	}

	calls = 0
	if _, err := dbx.Retry(ctx, 4, func() (int, error) { calls++; return 0, conflict }); !dbx.IsRetryable(err) || calls != 4 {
		t.Fatalf("a standing conflict: %d calls, err %v; want 4 calls and the conflict", calls, err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	calls = 0
	if _, err := dbx.Retry(cancelled, 5, func() (int, error) { calls++; return 0, conflict }); err == nil || calls != 1 {
		t.Fatalf("retried after the context ended: %d calls, err %v", calls, err)
	}
}
