//go:build !linux

package autoputer

import "os"

// dropFromCache is Linux only; elsewhere (tests on a developer host) reads
// may come from the cache.
func dropFromCache(*os.File, int64) {}

func fdatasync(f *os.File) error { return f.Sync() }
