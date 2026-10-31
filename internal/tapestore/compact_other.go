//go:build !linux

package tapestore

import (
	"errors"
	"os"
	"runtime"
)

var errNoFiemap = errors.New("tapestore: extent map not available on this platform")

func fileExtents(*os.File) (int, bool, error) { return -1, false, errNoFiemap }

func setCompressHint(*os.File) {}

func dropCache(*os.File) {}

func lockLowPriorityThread() { runtime.LockOSThread() }
