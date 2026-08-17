package capsule

import (
	"context"
	"fmt"
	"time"
)

const capsuleBrokerStartTimeout = 10 * time.Second

func waitForCommandStart(ctx context.Context, timeout time.Duration, start func() error) error {
	if start == nil {
		return fmt.Errorf("capsule start broker launcher: start function is required")
	}
	if timeout <= 0 {
		timeout = capsuleBrokerStartTimeout
	}
	started := make(chan error, 1)
	go func() {
		started <- start()
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case err := <-started:
		return err
	case <-ctx.Done():
		return fmt.Errorf("capsule start broker launcher: %w", ctx.Err())
	case <-timer.C:
		return fmt.Errorf("capsule start broker launcher timed out")
	}
}
