//go:build !linux

package autoputer

import (
	"github.com/yusefmosiah/go-choir/internal/capsule"
	"github.com/yusefmosiah/go-choir/internal/store"
)

func configuredCapsuleExecutor(*store.Store) (*capsule.Executor, bool, error) {
	return nil, false, nil
}
