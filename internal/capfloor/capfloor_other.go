//go:build !linux

package capfloor

import "fmt"

// Apply is Linux only: enforce and an unknown mode refuse elsewhere; off
// and observe do nothing.
func Apply(mode Mode) error {
	switch mode {
	case ModeOff, ModeObserve:
		return nil
	case ModeEnforce:
		return fmt.Errorf("capfloor: enforce needs Linux")
	}
	return fmt.Errorf("capfloor: unknown mode %q", mode)
}
