//go:build !unix

package storecomposition

import "os"

// sysIdentity: no device and inode off unix; the store identity checks that
// need them are skipped.
func sysIdentity(fi os.FileInfo) (dev, ino uint64) { return 0, 0 }
