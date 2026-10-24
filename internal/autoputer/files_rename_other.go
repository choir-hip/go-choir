//go:build !linux

package autoputer

// renameNoReplace is the portable form (the guest is Linux; see
// files_rename_linux.go for the atomic one).
func renameNoReplace(src, dst string) error { return renameLinkFallback(src, dst) }
