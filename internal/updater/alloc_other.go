//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package updater

import "io/fs"

func allocatedBytes(info fs.FileInfo) int64 { return info.Size() }

// syncFilesystems is a no-op where there is no sync(2).
func syncFilesystems() {}
