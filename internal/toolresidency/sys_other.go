//go:build !linux

package toolresidency

// The guest is Linux; elsewhere the helper measures and drops nothing.

func FileResident(string, int64) int64 { return 0 }
func DropFile(string)                  {}
func SyncAll()                         {}
