//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package updater

import "io/fs"

func allocatedBytes(info fs.FileInfo) int64 { return info.Size() }
