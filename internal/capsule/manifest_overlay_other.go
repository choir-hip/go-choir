//go:build !linux

package capsule

import "os"

// overlayMarks: there is no overlayfs off Linux, so an entry is never a
// whiteout and a directory is never opaque.
func overlayMarks(string, os.FileInfo) (whiteout, opaque bool) { return false, false }
