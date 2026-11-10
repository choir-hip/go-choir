package server

import (
	"errors"
	"net/http"
	"regexp"
	"strings"
)

// PlatformOperatorHeader carries the platform operator account's id from the
// proxy to a platform computer's guest on an operator act
// (docs/design/platform-operator-account-2026-11-06.md section 5). Only the
// proxy sets it: the proxy strips it from every client request and writes it
// from its own gate, and the guest reads it through PlatformOperator, which
// believes it only from the host side of the guest boundary and never from an
// in-process caller (a desk's choir.ProductAPI call is in-process).
const PlatformOperatorHeader = "X-Choir-Platform-Operator"

// ErrPlatformOperatorHeader is a present but unusable operator marker.
var ErrPlatformOperatorHeader = errors.New("platform operator marker refused")

var platformOperatorIDRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// PlatformOperator reports the operator account id the proxy attached to this
// request. present is false when the request carries no marker at all. A
// marker that is present but not believable (a second value, a malformed id,
// a caller that is not the host peer, an in-process caller) returns present
// true with an error, so a caller refuses the request rather than treating it
// as an ordinary owner act.
func PlatformOperator(r *http.Request) (id string, present bool, err error) {
	if r == nil {
		return "", false, nil
	}
	values := r.Header.Values(PlatformOperatorHeader)
	if len(values) == 0 {
		return "", false, nil
	}
	if len(values) != 1 || !GuestHostCaller(r) || InProcessCaller(r) {
		return "", true, ErrPlatformOperatorHeader
	}
	id = strings.TrimSpace(values[0])
	if !platformOperatorIDRE.MatchString(id) {
		return "", true, ErrPlatformOperatorHeader
	}
	return id, true, nil
}
