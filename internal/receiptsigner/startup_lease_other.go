//go:build !unix

package receiptsigner

import "fmt"

type StartupLease struct{}

func AcquireStartupLease(string) (*StartupLease, error) {
	return nil, fmt.Errorf("receipt signer: startup lease requires Unix")
}

func (l *StartupLease) Close() error {
	return nil
}

func StartupLeaseHeld(string) (bool, error) {
	return false, fmt.Errorf("receipt signer: startup lease requires Unix")
}
