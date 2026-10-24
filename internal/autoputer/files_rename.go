package autoputer

import "os"

// renameLinkFallback moves src to dst without ever replacing dst: a hard link
// fails if dst exists, and only then is the source name removed. It is the
// portable form of renameat2(RENAME_NOREPLACE). It is the cold path (the
// guest's ext4 data volume has RENAME_NOREPLACE). Its one window: a source
// unlinked and recreated by a non-API writer between the link and the unlink
// is removed by the unlink; no instant leaves both names missing, and a failed
// unlink leaves both names, which reconcile pairs by digest.
func renameLinkFallback(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}
