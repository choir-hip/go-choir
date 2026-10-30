//go:build !unix

package agentcore

import "errors"

// freeBytesOnVolume is unknown off unix; the producer refuses (fails closed).
func freeBytesOnVolume(string) (uint64, error) {
	return 0, errors.New("free space is not measurable on this platform")
}
