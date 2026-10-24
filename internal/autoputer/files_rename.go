package autoputer

import "os"

// renameLinkFallback moves src to dst without ever replacing dst: a hard link
// fails if dst exists, and only then is the source name removed. It is the
// portable form of renameat2(RENAME_NOREPLACE).
func renameLinkFallback(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}
