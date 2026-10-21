//go:build !linux

package capfloor

import "fmt"

// Apply is Linux only: enforce refuses elsewhere; off and observe do nothing.
func Apply(mode Mode) error {
	if mode == ModeEnforce {
		return fmt.Errorf("capfloor: enforce needs Linux")
	}
	return nil
}
