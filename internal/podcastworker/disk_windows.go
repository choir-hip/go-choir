//go:build windows

package podcastworker

import "math"

func freeBytes(string) (uint64, error) { return math.MaxUint64, nil }
