//go:build !linux

package guestmem

import "errors"

// residentBytes needs mincore over a mapping, which this package reads only
// on Linux guests.
func residentBytes(string, int64) (int64, error) {
	return 0, errors.New("page cache residency is read only on linux")
}
