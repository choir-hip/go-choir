//go:build !linux

package sandbox

import "github.com/yusefmosiah/go-choir/internal/capsule"

func configuredCapsuleExecutor() (*capsule.Executor, bool, error) { return nil, false, nil }
