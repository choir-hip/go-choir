//go:build !linux

package autoputer

import (
	"time"

	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
)

func configuredCapsuleExecutor(*store.Store, string, time.Time) (*capsule.Executor, bool, error) {
	return nil, false, nil
}
