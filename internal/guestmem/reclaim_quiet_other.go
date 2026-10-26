//go:build !linux

package guestmem

// quietThread: only a Linux guest runs the act.
func quietThread() {}
