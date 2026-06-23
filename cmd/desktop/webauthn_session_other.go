//go:build !darwin

package main

import "errors"

func startWebAuthSession(authURL, callbackScheme string) (string, error) {
	return "", errors.New("ASWebAuthenticationSession not available on this platform")
}
