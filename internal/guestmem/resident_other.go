//go:build !linux

package guestmem

import "errors"

// residentBytes needs mincore over a mapping, which this package reads only
// on Linux guests.
func residentBytes(string, int64, *scan) (int64, bool, error) {
	return 0, false, errors.New("page cache residency is read only on linux")
}
