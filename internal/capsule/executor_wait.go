package capsule

import (
	"context"
	"fmt"
	"time"
)

const capsuleProcessWaitTimeout = 10 * time.Second

func waitCapsuleProcess(ctx context.Context, wait func() error, kill func() error, timeout time.Duration) error {
	if wait == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = capsuleProcessWaitTimeout
	}
	done := make(chan error, 1)
	go func() { done <- wait() }()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		if kill != nil {
			_ = kill()
		}
		select {
		case err := <-done:
			if err != nil {
				return err
			}
			return ctx.Err()
		case <-time.After(timeout):
			return fmt.Errorf("capsule process wait timed out after cancel: %w", ctx.Err())
		}
	case <-timer.C:
		if kill != nil {
			_ = kill()
		}
		select {
		case err := <-done:
			if err != nil {
				return err
			}
			return fmt.Errorf("capsule process wait timed out")
		case <-time.After(timeout):
			return fmt.Errorf("capsule process wait timed out")
		}
	}
}
