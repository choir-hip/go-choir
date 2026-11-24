package vmmanager

import (
	"errors"
	"testing"
)

// The deployed proof fails when the journal says the btrfs extent measure was
// unavailable. A fully sparse file (a data image between its truncate and its
// mkfs) reads zero extents and holds no block: that is exactly 0, not a
// fallback, and must not be logged as one. A zero reading for a file that holds
// blocks, and a failed search, still are fallbacks.
func TestAHoleOnlyFileIsAnExactZeroNotAFallback(t *testing.T) {
	for name, tc := range map[string]struct {
		n         int64
		err       error
		allocated int64
		trusted   bool
	}{
		"no extents, no blocks (a hole-only file)":    {0, nil, 0, true},
		"no extents but st_blocks holds blocks":       {0, nil, 4096, false},
		"extents read":                                {123456, nil, 4096, true},
		"extents read, st_blocks larger (compressed)": {1 << 20, nil, 8 << 20, true},
		"the search failed (no CAP_SYS_ADMIN)":        {0, errors.New("operation not permitted"), 0, false},
	} {
		if got := btrfsReadingTrusted(tc.n, tc.err, tc.allocated); got != tc.trusted {
			t.Errorf("%s: trusted = %v, want %v", name, got, tc.trusted)
		}
	}
}
