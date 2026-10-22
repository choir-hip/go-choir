//go:build !linux

package server

import "errors"

type fileResidencyResult struct {
	dev, ino       uint64
	uid            uint32
	size, resident int64
}

var errPageCacheUnsupported = errors.New("pagecache: mincore residency is read on Linux only")

func fileDevice(string) (uint64, bool) { return 0, false }

func fileResidency(string) (fileResidencyResult, bool) { return fileResidencyResult{}, false }

func pageCacheSupported() error { return errPageCacheUnsupported }

var pageCacheMayMeasure = func(uint32) bool { return false }
