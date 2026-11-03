//go:build !linux

package ownerfiles

// renameNoReplace is the portable form (the guest is Linux; see
// rename_linux.go for the atomic one).
func renameNoReplace(src, dst string) error { return renameLinkFallback(src, dst) }
