//go:build !linux

package vmbackend

import (
	"fmt"
	"os"
)

// OpenTap needs Linux tun/tap.
func OpenTap(name string) (*os.File, error) {
	return nil, fmt.Errorf("%w: tap devices need Linux (tap %q)", ErrUnsupported, name)
}
