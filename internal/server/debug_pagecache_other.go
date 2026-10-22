//go:build !linux

package server

import (
	"errors"
	"io/fs"
	"time"
)

type fileID struct {
	dev, ino uint64
	uid      uint32
	size     int64
}

var errPageCacheUnsupported = errors.New("pagecache: mincore residency is read on Linux only")

func fileDevice(string) (uint64, bool) { return 0, false }

func fileIdentity(fs.DirEntry) (fileID, bool) { return fileID{}, false }

func fileResidency(string, fileID, time.Time) (int64, int) { return 0, residencyFailed }

func pageCacheSupported() error { return errPageCacheUnsupported }

var pageCacheMayMeasure = func(uint32) bool { return false }
